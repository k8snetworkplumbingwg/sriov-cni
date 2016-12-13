package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"crypto/rand"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

type NetConf struct {
	types.NetConf
	IF0        string `json:"if0"`
	IF0NAME    string `json:"if0name"`
	MAC        bool   `json:"createmac"`
	Vlan       int    `json:"vlan"`
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func createRandomMacAddr() (net.HardwareAddr, error) {
	//Reserve 6 bytes for the Mac Address
	addr := make([]byte, 6)
	if _, err := rand.Read(addr); err != nil {
		return nil, fmt.Errorf("err in Generating Mac addr: %v", err)
	}
	//refer, http://standards.ieee.org/regauth/oui/oui.txt
	//x2 perfix is used to set local administation and 0xfe for the unicast
	//address
	addr[0] = (addr[0] | 2) & 0xfe
	return net.HardwareAddr(addr), nil
}

func checkIf0name(ifname string) bool {
	op  := []string{"eth0", "eth1", "lo", ""}
	for _, if0name := range op {
		if (strings.Compare(if0name,ifname) == 0) {
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

	return n, nil
}

func setupVF(conf *NetConf, ifName string,  podifName string, netns ns.NetNS) error {

	var vfIdx int
	var infos []os.FileInfo

	m, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.IF0, err)
	}

	sriovFile := fmt.Sprintf("/sys/class/net/%s/device/sriov_numvfs", ifName)
	if _, err := os.Lstat(sriovFile); err != nil {
		return fmt.Errorf("failed to open the sriov_numfs of device %q: %v", ifName, err)
	}

	data, err := ioutil.ReadFile(sriovFile)
	if err != nil {
		return fmt.Errorf("failed to read the sriov_numfs of device %q: %v", ifName, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("no data in the file %q", sriovFile)
	}

	sriovNumfs := strings.TrimSpace(string(data))
	vfTotal, err := strconv.Atoi(sriovNumfs)
	if err != nil {
		return fmt.Errorf("failed to convert sriov_numfs(byte value) to int of device %q: %v", ifName, err)
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device %q: %v", ifName)
	}

	for vf := 0; vf <= (vfTotal-1); vf++ {
		vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/net", ifName, vf)
		if _, err := os.Lstat(vfDir); err != nil {
			return fmt.Errorf("failed to open the virtfn%d dir of the device %q: %v", vf, ifName, err)
		}

		infos, err = ioutil.ReadDir(vfDir)
		if err != nil {
			return fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vf, ifName, err)
		}

		if (len(infos) == 0) && (vf == (vfTotal-1)) {
			return fmt.Errorf("no Virtual function exist in directory %s, last vf is virtfn%d", vfDir, vf)
		}

		if (len(infos) == 0) && (vf != (vfTotal-1)) {
			continue
		}

		if len(infos) == 1 {
			vfIdx = vf
			break
		} else {
			return fmt.Errorf("mutiple network devices in directory %s", vfDir)
		}
	}

	// VF NIC name
	if len(infos) != 1 {
		return fmt.Errorf("no virutal network resources avaiable for the %q", conf.IF0)
	}

	vfDevName := infos[0].Name()
	vfDev, err := netlink.LinkByName(vfDevName)
	if err != nil {
		return fmt.Errorf("failed to lookup vf device %q: %v", vfDevName, err)
	}

	// set hardware address
	if conf.MAC != false {
		macAddr,err := createRandomMacAddr()
		if err != nil {
			return fmt.Errorf("err in getting Random Mac addr: %v", err)
		}

		if err = netlink.LinkSetVfHardwareAddr(m, vfIdx, macAddr); err != nil {
			return fmt.Errorf("failed to set vf %d macaddress: %v", vfIdx, err)
		}
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
		return nil
	})
}

func releaseVF(conf *NetConf, podifName string, netns ns.NetNS) error {
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

	if n.IF0NAME != ""{
		args.IfName = n.IF0NAME
	}

	if err = setupVF(n, n.IF0, args.IfName, netns); err != nil {
		return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, n.IF0, err)
	}

	// run the IPAM plugin and get back the config to apply
	result, err := ipam.ExecAdd(n.IPAM.Type, args.StdinData)
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

	if err = releaseVF(n, args.IfName, netns); err != nil {
		return err
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

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}
