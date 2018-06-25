package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

const defaultCNIDir = "/var/lib/cni/sriov"
const maxSharedVf = 2

type dpdkConf struct {
	PCIaddr    string `json:"pci_addr"`
	Ifname     string `json:"ifname"`
	KDriver    string `json:"kernel_driver"`
	DPDKDriver string `json:"dpdk_driver"`
	DPDKtool   string `json:"dpdk_tool"`
	VFID       int    `json: "vfid"`
}

type NetConf struct {
	types.NetConf
	DPDKMode bool
	Sharedvf bool
	DPDKConf dpdkConf `json:"dpdk,omitempty"`
	CNIDir   string   `json:"cniDir"`
	IF0      string   `json:"if0"`
	IF0NAME  string   `json:"if0name"`
	L2Mode   bool     `json:"l2enable"`
	Vlan     int      `json:"vlan"`
	DeviceId string   `json:"deviceid"` // Device ID holds an VF's PCI address
	VfId     int      `json: "vfid"`
}

// Link names given as os.FileInfo need to be sorted by their Index

type LinksByIndex []os.FileInfo

// LinksByIndex implements sort.Inteface
func (l LinksByIndex) Len() int { return len(l) }

func (l LinksByIndex) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func (l LinksByIndex) Less(i, j int) bool {
	link_a, _ := netlink.LinkByName(l[i].Name())
	link_b, _ := netlink.LinkByName(l[j].Name())

	return link_a.Attrs().Index < link_b.Attrs().Index
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func checkIf0name(ifname string) bool {
	op := []string{"eth0", "eth1", "lo", ""}
	for _, if0name := range op {
		if strings.Compare(if0name, ifname) == 0 {
			return false
		}
	}

	return true
}

func loadConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	if n.IF0NAME != "" {
		err := checkIf0name(n.IF0NAME)
		if err != true {
			return nil, fmt.Errorf(`"if0name" field should not be  equal to (eth0 | eth1 | lo | ""). It specifies the virtualized interface name in the pod`)
		}
	}

	if n.IF0 == "" && n.DeviceId == "" {
		return nil, fmt.Errorf(`either "if0" OR "deviceid" field is required. It specifies the host interface name to virtualize`)
	}

	if n.CNIDir == "" {
		n.CNIDir = defaultCNIDir
	}

	if (dpdkConf{}) != n.DPDKConf {
		n.DPDKMode = true
	}

	return n, nil
}

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create the sriov data directory(%q): %v", dataDir, err)
	}

	path := filepath.Join(dataDir, containerID)

	err := ioutil.WriteFile(path, netconf, 0600)
	if err != nil {
		return fmt.Errorf("failed to write container data in the path(%q): %v", path, err)
	}

	return err
}

func consumeScratchNetConf(containerID, dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, containerID)
	defer os.Remove(path)

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read container data in the path(%q): %v", path, err)
	}

	return data, err
}

func savedpdkConf(cid, dataDir string, conf *NetConf) error {
	dpdkconfBytes, err := json.Marshal(conf.DPDKConf)
	if err != nil {
		return fmt.Errorf("error serializing delegate netconf: %v", err)
	}

	s := []string{cid, conf.DPDKConf.Ifname}
	cRef := strings.Join(s, "-")

	// save the rendered netconf for cmdDel
	if err = saveScratchNetConf(cRef, dataDir, dpdkconfBytes); err != nil {
		return err
	}

	return nil
}

func (dc *dpdkConf) getdpdkConf(cid, podIfName, dataDir string, conf *NetConf) error {
	s := []string{cid, podIfName}
	cRef := strings.Join(s, "-")

	dpdkconfBytes, err := consumeScratchNetConf(cRef, dataDir)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(dpdkconfBytes, dc); err != nil {
		return fmt.Errorf("failed to parse netconf: %v", err)
	}

	return nil
}

func enabledpdkmode(conf *dpdkConf, ifname string, dpdkmode bool) error {
	stdout := &bytes.Buffer{}
	var driver string
	var device string

	if dpdkmode != false {
		driver = conf.DPDKDriver
		device = ifname
	} else {
		driver = conf.KDriver
		device = conf.PCIaddr
	}

	cmd := exec.Command(conf.DPDKtool, "-b", driver, device)
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("DPDK binding failed with err msg %q:", stdout.String())
	}

	stdout.Reset()
	return nil
}

func getpciaddress(ifName string, vf int) (string, error) {
	var pciaddr string
	vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d", ifName, vf)
	dirInfo, err := os.Lstat(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't get the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pciaddr, fmt.Errorf("No symbolic link for the virtfn%d dir of the device %q", vf, ifName)
	}

	pciinfo, err := os.Readlink(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't read the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	pciaddr = pciinfo[len("../"):]
	return pciaddr, nil
}

func getSharedPF(ifName string) (string, error) {
	pfName := ""
	pfDir := fmt.Sprintf("/sys/class/net/%s", ifName)
	dirInfo, err := os.Lstat(pfDir)
	if err != nil {
		return pfName, fmt.Errorf("can't get the symbolic link of the device %q: %v", ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pfName, fmt.Errorf("No symbolic link for dir of the device %q", ifName)
	}

	fullpath, err := filepath.EvalSymlinks(pfDir)
	parentDir := fullpath[:len(fullpath)-len(ifName)]
	dirList, err := ioutil.ReadDir(parentDir)

	for _, file := range dirList {
		if file.Name() != ifName {
			pfName = file.Name()
			return pfName, nil
		}
	}

	return pfName, fmt.Errorf("Shared PF not found")
}

func getsriovNumfs(ifName string) (int, error) {
	var vfTotal int

	sriovFile := fmt.Sprintf("/sys/class/net/%s/device/sriov_numvfs", ifName)
	if _, err := os.Lstat(sriovFile); err != nil {
		return vfTotal, fmt.Errorf("failed to open the sriov_numfs of device %q: %v", ifName, err)
	}

	data, err := ioutil.ReadFile(sriovFile)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to read the sriov_numfs of device %q: %v", ifName, err)
	}

	if len(data) == 0 {
		return vfTotal, fmt.Errorf("no data in the file %q", sriovFile)
	}

	sriovNumfs := strings.TrimSpace(string(data))
	vfTotal, err = strconv.Atoi(sriovNumfs)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to convert sriov_numfs(byte value) to int of device %q: %v", ifName, err)
	}

	return vfTotal, nil
}

func setSharedVfVlan(ifName string, vfIdx int, vlan int) error {
	var err error
	var sharedifName string

	vfDir := fmt.Sprintf("/sys/class/net/%s/device/net", ifName)
	if _, err := os.Lstat(vfDir); err != nil {
		return fmt.Errorf("failed to open the net dir of the device %q: %v", ifName, err)
	}

	infos, err := ioutil.ReadDir(vfDir)
	if err != nil {
		return fmt.Errorf("failed to read the net dir of the device %q: %v", ifName, err)
	}

	if len(infos) != maxSharedVf {
		return fmt.Errorf("Given PF - %q is not having shared VF", ifName)
	}

	for _, dir := range infos {
		if strings.Compare(ifName, dir.Name()) != 0 {
			sharedifName = dir.Name()
		}
	}

	if sharedifName == "" {
		return fmt.Errorf("Shared ifname can't be empty")
	}

	iflink, err := netlink.LinkByName(sharedifName)
	if err != nil {
		return fmt.Errorf("failed to lookup the shared ifname %q: %v", sharedifName, err)
	}

	if err := netlink.LinkSetVfVlan(iflink, vfIdx, vlan); err != nil {
		return fmt.Errorf("failed to set vf %d vlan: %v for shared ifname %q", vfIdx, err, sharedifName)
	}

	return nil
}

func moveIfToNetns(ifname string, netns ns.NetNS) error {
	vfDev, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("failed to lookup vf device %v: %q", ifname, err)
	}

	if err = netlink.LinkSetUp(vfDev); err != nil {
		return fmt.Errorf("failed to setup netlink device %v %q", ifname, err)
	}

	// move VF device to ns
	if err = netlink.LinkSetNsFd(vfDev, int(netns.Fd())); err != nil {
		return fmt.Errorf("failed to move device %+v to netns: %q", ifname, err)
	}

	return nil
}

func getDeviceNameFromPci(pciaddr string) (string, error) {
	var devName string
	vfDir := fmt.Sprintf("/sys/bus/pci/devices/%s/net/", pciaddr)
	_, err := os.Lstat(vfDir)
	if err != nil {
		return devName, fmt.Errorf("cannot get a network device with pci address %v %q", pciaddr, err)
	}
	dirContents, _ := ioutil.ReadDir(vfDir)

	if err != nil || len(dirContents) < 1 {
		return devName, fmt.Errorf("failed to get network device name in %v %v", vfDir, err)
	}

	if len(dirContents) < 1 {
		return devName, fmt.Errorf("no network device found in %v", vfDir)
	}

	devName = dirContents[0].Name() // assuming one net device in this directory
	return strings.TrimSpace(devName), nil
}

func setupWithVfInfo(conf *NetConf, netns ns.NetNS, cid, podifName string) error {
	var err error

	// Get PF link with given name
	m, err := netlink.LinkByName(conf.IF0)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.IF0, err)
	}

	// Get VF link name
	vfLinkName, err := getDeviceNameFromPci(conf.DeviceId)
	if err != nil {
		return err
	}

	// Set Vlan
	if conf.Vlan != 0 {
		if err = netlink.LinkSetVfVlan(m, conf.VfId, conf.Vlan); err != nil {
			return fmt.Errorf("failed to set vf %d vlan: %v", conf.VfId, err)
		}
	}

	// if dpdk mode then skip rest
	if conf.DPDKMode != false {
		conf.DPDKConf.PCIaddr = conf.DeviceId
		conf.DPDKConf.Ifname = podifName
		conf.DPDKConf.VFID = conf.VfId
		if err = savedpdkConf(cid, conf.CNIDir, conf); err != nil {
			return err
		}
		return enabledpdkmode(&conf.DPDKConf, vfLinkName, true)
	}

	// move VF to pod netns
	if err = moveIfToNetns(vfLinkName, netns); err != nil {
		return err
	}

	// Rename VF in Pod
	return netns.Do(func(_ ns.NetNS) error {
		err := renameLink(vfLinkName, podifName)
		if err != nil {
			return fmt.Errorf("failed to rename  interface %v to %v in Pod netns %q", vfLinkName, podifName, err)
		}
		return nil
	})
}

func setupVF(conf *NetConf, ifName string, podifName string, cid string, netns ns.NetNS) error {

	var vfIdx int
	var infos []os.FileInfo
	var pciAddr string

	// try to get VF using PF information
	m, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.IF0, err)
	}

	// get the ifname sriov vf num
	vfTotal, err := getsriovNumfs(ifName)
	if err != nil {
		return err
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device %q: %v", ifName)
	}

	// Select a free VF
	for vf := 0; vf <= (vfTotal - 1); vf++ {
		vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/net", ifName, vf)
		if _, err := os.Lstat(vfDir); err != nil {
			if vf == (vfTotal - 1) {
				return fmt.Errorf("failed to open the virtfn%d dir of the device %q: %v", vf, ifName, err)
			}
			continue
		}

		infos, err = ioutil.ReadDir(vfDir)
		if err != nil {
			return fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vf, ifName, err)
		}

		if (len(infos) == 0) && (vf == (vfTotal - 1)) {
			return fmt.Errorf("no Virtual function exist in directory %s, last vf is virtfn%d", vfDir, vf)
		}

		if (len(infos) == 0) && (vf != (vfTotal - 1)) {
			continue
		}

		if len(infos) == maxSharedVf {
			conf.Sharedvf = true
		}

		if len(infos) <= maxSharedVf {
			vfIdx = vf
			pciAddr, err = getpciaddress(ifName, vfIdx)
			if err != nil {
				return fmt.Errorf("err in getting pci address - %q", err)
			}
			break
		} else {
			return fmt.Errorf("mutiple network devices in directory %s", vfDir)
		}
	}

	// VF NIC name
	if len(infos) != 1 && len(infos) != maxSharedVf {
		return fmt.Errorf("no virutal network resources avaiable for the %q", conf.IF0)
	}

	if conf.Sharedvf != false && conf.L2Mode != true {
		return fmt.Errorf("l2enable mode must be true to use shared net interface %q", conf.IF0)
	}

	if conf.Vlan != 0 {
		if err = netlink.LinkSetVfVlan(m, vfIdx, conf.Vlan); err != nil {
			return fmt.Errorf("failed to set vf %d vlan: %v", vfIdx, err)
		}

		if conf.Sharedvf {
			if err = setSharedVfVlan(ifName, vfIdx, conf.Vlan); err != nil {
				return fmt.Errorf("failed to set shared vf %d vlan: %v", vfIdx, err)
			}
		}
	}

	if conf.DPDKMode != false {
		conf.DPDKConf.PCIaddr = pciAddr
		conf.DPDKConf.Ifname = podifName
		conf.DPDKConf.VFID = vfIdx
		if err = savedpdkConf(cid, conf.CNIDir, conf); err != nil {
			return err
		}
		return enabledpdkmode(&conf.DPDKConf, infos[0].Name(), true)
	}

	// Sort links name if there are 2 or more PF links found for a VF;
	if len(infos) > 1 {
		// sort Links FileInfo by their Link indices
		sort.Sort(LinksByIndex(infos))
	}

	for i := 1; i <= len(infos); i++ {
		// vfDev, err := netlink.LinkByName(infos[i-1].Name())
		linkName := infos[i-1].Name()

		if err = moveIfToNetns(linkName, netns); err != nil {
			return err
		}
	}

	return netns.Do(func(_ ns.NetNS) error {

		ifName := podifName
		for i := 1; i <= len(infos); i++ {
			if len(infos) == maxSharedVf && i == len(infos) {
				ifName = podifName + fmt.Sprintf("d%d", i-1)
			}

			err := renameLink(infos[i-1].Name(), ifName)
			if err != nil {
				return fmt.Errorf("failed to rename %d vf of the device %q to %q: %v", vfIdx, infos[i-1].Name(), ifName, err)
			}

			// for L2 mode enable the pod net interface
			if conf.L2Mode != false {
				err = setUpLink(ifName)
				if err != nil {
					return fmt.Errorf("failed to set up the pod interface name %q: %v", ifName, err)
				}
			}
		}
		return nil
	})
}

func releaseVF(conf *NetConf, podifName string, cid string, netns ns.NetNS) error {
	// check for the DPDK mode and release the allocated DPDK resources
	if conf.DPDKMode != false {
		df := &dpdkConf{}
		// get the DPDK net conf in cniDir
		if err := df.getdpdkConf(cid, podifName, conf.CNIDir, conf); err != nil {
			return err
		}

		// bind the sriov vf to the kernel driver
		if err := enabledpdkmode(df, df.Ifname, false); err != nil {
			return fmt.Errorf("DPDK: failed to bind %s to kernel space: %s", df.Ifname, err)
		}

		// reset vlan for DPDK code here
		pfLink, err := netlink.LinkByName(conf.IF0)
		if err != nil {
			return fmt.Errorf("DPDK: master device %s not found: %v", conf.IF0, err)
		}

		if err = netlink.LinkSetVfVlan(pfLink, df.VFID, 0); err != nil {
			return fmt.Errorf("DPDK: failed to reset vlan tag for vf %d: %v", df.VFID, err)
		}

		return nil
	}

	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if err = netns.Set(); err != nil {
		return fmt.Errorf("failed to enter netns %q: %v", netns, err)
	}

	if conf.L2Mode != false {
		//check for the shared vf net interface
		ifName := podifName + "d1"
		_, err := netlink.LinkByName(ifName)
		if err == nil {
			conf.Sharedvf = true
		}

	}

	if err != nil {
		fmt.Errorf("Enable to get shared PF device: %v", err)
	}

	for i := 1; i <= maxSharedVf; i++ {
		ifName := podifName
		pfName := conf.IF0
		if i == maxSharedVf {
			ifName = podifName + fmt.Sprintf("d%d", i-1)
			pfName, err = getSharedPF(conf.IF0)
			if err != nil {
				return fmt.Errorf("Failed to look up shared PF device: %v:", err)
			}
		}

		// get VF device
		vfDev, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to lookup vf device %q: %v", ifName, err)
		}

		// device name in init netns
		index := vfDev.Attrs().Index
		devName := fmt.Sprintf("dev%d", index)

		// shutdown VF device
		if err = netlink.LinkSetDown(vfDev); err != nil {
			return fmt.Errorf("failed to down vf device %q: %v", ifName, err)
		}

		// rename VF device
		err = renameLink(ifName, devName)
		if err != nil {
			return fmt.Errorf("failed to rename vf device %q to %q: %v", ifName, devName, err)
		}

		// move VF device to init netns
		if err = netlink.LinkSetNsFd(vfDev, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move vf device %q to init netns: %v", ifName, err)
		}

		// reset vlan
		if conf.Vlan != 0 {
			err = initns.Do(func(_ ns.NetNS) error {
				return resetVfVlan(pfName, devName)
			})
			if err != nil {
				return fmt.Errorf("failed to reset vlan: %v", err)
			}
		}

		//break the loop, if the namespace has no shared vf net interface
		if conf.Sharedvf != true {
			break
		}
	}

	return nil
}

func resetVfVlan(pfName, vfName string) error {

	// get the ifname sriov vf num
	vfTotal, err := getsriovNumfs(pfName)
	if err != nil {
		return err
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device %q: %v", pfName)
	}

	// Get VF id
	var vf int
	idFound := false
	for vf = 0; vf < vfTotal; vf++ {
		vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/net/%s", pfName, vf, vfName)
		if _, err := os.Stat(vfDir); !os.IsNotExist(err) {
			idFound = true
			break
		}
	}

	if !idFound {
		return fmt.Errorf("failed to get VF id for %s", vfName)
	}

	pfLink, err := netlink.LinkByName(pfName)
	if err != nil {
		return fmt.Errorf("Master device %s not found\n", pfName)
	}

	if err = netlink.LinkSetVfVlan(pfLink, vf, 0); err != nil {
		return fmt.Errorf("failed to reset vlan tag for vf %d: %v", vf, err)
	}
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	n, err := loadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	if n.IF0NAME != "" {
		args.IfName = n.IF0NAME
	}

	if n.DeviceId != "" && n.VfId >= 0 {
		if err = setupWithVfInfo(n, netns, args.ContainerID, args.IfName); err != nil {
			return err
		}
	} else {
		if err = setupVF(n, n.IF0, args.IfName, args.ContainerID, netns); err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, n.IF0, err)
		}
	}

	// skip the IPAM allocation for the DPDK and L2 mode
	var result *types.Result
	if n.DPDKMode != false || n.L2Mode != false {
		return result.Print()
	}

	// run the IPAM plugin and get back the config to apply
	result, err = ipam.ExecAdd(n.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", n.IPAM.Type, n.IF0, err)
	}

	if result.IP4 == nil {
		return errors.New("IPAM plugin returned missing IPv4 config")
	}

	err = netns.Do(func(_ ns.NetNS) error {
		return ipam.ConfigureIface(args.IfName, result)
	})
	if err != nil {
		return err
	}

	result.DNS = n.DNS
	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	n, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	// skip the IPAM release for the DPDK and L2 mode
	if n.IPAM.Type != "" {
		err = ipam.ExecDel(n.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
	}

	if args.Netns == "" {
		return nil
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		// according to:
		// https://github.com/kubernetes/kubernetes/issues/43014#issuecomment-287164444
		// if provided path does not exist (e.x. when node was restarted)
		// plugin should silently return with success after releasing
		// IPAM resources
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			return nil
		}

		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	if n.IF0NAME != "" {
		args.IfName = n.IF0NAME
	}

	if err = releaseVF(n, args.IfName, args.ContainerID, netns); err != nil {
		return err
	}

	return nil
}

func renameLink(curName, newName string) error {
	link, err := netlink.LinkByName(curName)
	if err != nil {
		return fmt.Errorf("failed to lookup device %q: %v", curName, err)
	}

	return netlink.LinkSetName(link, newName)
}

func setUpLink(ifName string) error {
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to set up device %q: %v", ifName, err)
	}

	return netlink.LinkSetUp(link)
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}
