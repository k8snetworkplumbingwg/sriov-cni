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
	"strconv"
	"strings"
	"syscall"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

const defaultCNIDir = "/var/lib/cni/sriov"

type dpdkConf struct {
	PCIaddr    string `json:"pci_addr"`
	Ifname     string `json:"ifname"`
	KDriver    string `json:"kernel_driver"`
	DPDKDriver string `json:"dpdk_driver"`
	DPDKtool   string `json:"dpdk_tool"`
}

type NetConf struct {
	types.NetConf
	DPDKMode bool
	DPDKConf dpdkConf `json:"dpdk,omitempty"`
	CNIDir   string   `json:"cniDir"`
	IF0      string   `json:"if0"`
	IF0NAME  string   `json:"if0name"`
	L2Mode   bool     `json:"l2enable"`
	Vlan     int      `json:"vlan"`
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

	if n.IF0 == "" {
		return nil, fmt.Errorf(`"if0" field is required. It specifies the host interface name to virtualize`)
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

func (dc *dpdkConf) getdpdkConf(cid, dataDir string, conf *NetConf) error {
	s := []string{cid, conf.IF0NAME}
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

func getphyPortID(ifName string) (string, error) {
	var physportId string

	physportidFile := fmt.Sprintf("/sys/class/net/%s/phys_port_id", ifName)
	if _, err := os.Lstat(physportidFile); err != nil {
		return physportId, fmt.Errorf("failed to open the phys_port_id file of device %q: %v", ifName, err)
	}

	data, err := ioutil.ReadFile(physportidFile)
	if err != nil {
		// skip nics that don't support phy port id
		if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOTSUP {
			return physportId, nil
		}

		return physportId, fmt.Errorf("failed to read the phys_port_id file of device %q: %v", ifName, err)
	}

	if len(data) == 0 {
		return physportId, fmt.Errorf("no data in the file: %q", physportidFile)
	}

	physportId = strings.TrimSpace(string(data))

	return physportId, nil

}

func compareportID(ifName string, portId string) (bool, error) {
	physportId, err := getphyPortID(ifName)
	if err != nil {
		return false, err
	}
	if strings.Compare(physportId, portId) != 0 {
		return false, nil
	}

	return true, nil

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

func setupVF(conf *NetConf, ifName string, podifName string, cid string, netns ns.NetNS) error {

	var vfIdx int
	var infos []os.FileInfo
	var pciAddr string
	var vfDevName string

	m, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.IF0, err)
	}

	// get the ifname phy port id, if exist
	physportId, err := getphyPortID(ifName)
	if err != nil {
		return fmt.Errorf("failed to get phy port id of %q: %v", ifName, err)
	}

	// get the ifname sriov vf num
	vfTotal, err := getsriovNumfs(ifName)
	if err != nil {
		return err
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device %q: %v", ifName)
	}

	for vf := 0; vf <= (vfTotal - 1); vf++ {
		// get network interface of virtual function
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

		// if sriov vf has more than one interface
		if len(infos) <= 2 {
			var portId bool
			var err error

			vfIdx = vf
			for i := 1; i <= len(infos) && portId != true; i++ {
				vfDevName = infos[i-1].Name()

				if len(physportId) != 0 {
					// compare the phy port id of sriov vf net interface and pf
					portId, err = compareportID(infos[i-1].Name(), physportId)
					if err != nil {
						return err
					}

				}
			}

			//return error, if no VF is available for the NIC support port ID
			if !portId && len(physportId) != 0 && (vf == (vfTotal - 1)) {
				return fmt.Errorf("no Virtual function exist in directory %s, last vf is virtfn%d", vfDir, vf)
			}

			if !portId && len(physportId) != 0 {
				continue
			}

			// get the sriov vf net interface pci
			pciAddr, err = getpciaddress(ifName, vfIdx)
			if err != nil {
				return fmt.Errorf("err in getting pci address - %q", err)
			}
			break
		} else {
			return fmt.Errorf("mutiple network devices in directory %s", vfDir)
		}
	}

	if conf.DPDKMode != false {
		conf.DPDKConf.PCIaddr = pciAddr
		conf.DPDKConf.Ifname = podifName
		// save the DPDK net conf in cniDir
		if err = savedpdkConf(cid, conf.CNIDir, conf); err != nil {
			return err
		}
		// bind the sriov vf to the DPDK driver
		return enabledpdkmode(&conf.DPDKConf, vfDevName, true)
	}

	vfDev, err := netlink.LinkByName(vfDevName)
	if err != nil {
		return fmt.Errorf("failed to lookup vf device %q: %v", vfDevName, err)
	}

	if conf.Vlan != 0 {
		if err = netlink.LinkSetVfVlan(m, vfIdx, conf.Vlan); err != nil {
			return fmt.Errorf("failed to set vf %d vlan: %v", vfIdx, err)
		}
	}

	if err = netlink.LinkSetUp(vfDev); err != nil {
		return fmt.Errorf("failed to setup vf %d device: %v", vfIdx, err)
	}

	// move VF device to ns
	if err = netlink.LinkSetNsFd(vfDev, int(netns.Fd())); err != nil {
		return fmt.Errorf("failed to move vf %d to netns: %v", vfIdx, err)
	}

	return netns.Do(func(_ ns.NetNS) error {
		err := renameLink(vfDevName, podifName)
		if err != nil {
			return fmt.Errorf("failed to rename %d vf of the device %q to %q: %v", vfIdx, vfDevName, podifName, err)
		}

		// for L2 mode enable the pod net interface
		if conf.L2Mode != false {
			err = setUpLink(podifName)
			if err != nil {
				return fmt.Errorf("failed to set up the pod interface name %q: %v", podifName, err)
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
		if err := df.getdpdkConf(cid, conf.CNIDir, conf); err != nil {
			return err
		}
		// bind the sriov vf to the kernel driver
		return enabledpdkmode(df, df.Ifname, false)
	}

	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if err = netns.Set(); err != nil {
		return fmt.Errorf("failed to enter netns %q: %v", netns, err)
	}

	// get VF device
	vfDev, err := netlink.LinkByName(podifName)
	if err != nil {
		return fmt.Errorf("failed to lookup vf device %q: %v", podifName, err)
	}

	// device name in init netns
	index := vfDev.Attrs().Index
	devName := fmt.Sprintf("dev%d", index)

	// shutdown VF device
	if err = netlink.LinkSetDown(vfDev); err != nil {
		return fmt.Errorf("failed to down vf device %q: %v", podifName, err)
	}

	// rename VF device
	err = renameLink(podifName, devName)
	if err != nil {
		return fmt.Errorf("failed to rename vf device %q to %q: %v", podifName, devName, err)
	}

	// move VF device to init netns
	if err = netlink.LinkSetNsFd(vfDev, int(initns.Fd())); err != nil {
		return fmt.Errorf("failed to move vf device to init netns: %v", podifName, err)
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

	if err = setupVF(n, n.IF0, args.IfName, args.ContainerID, netns); err != nil {
		return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, n.IF0, err)
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

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	if n.IF0NAME != "" {
		args.IfName = n.IF0NAME
	}

	if err = releaseVF(n, args.IfName, args.ContainerID, netns); err != nil {
		return err
	}

	// skip the IPAM release for the DPDK and L2 mode
	if n.DPDKMode != false || n.L2Mode != false {
		return nil
	}

	err = ipam.ExecDel(n.IPAM.Type, args.StdinData)
	if err != nil {
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
