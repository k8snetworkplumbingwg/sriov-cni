package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/vishvananda/netlink"

	. "github.com/hustcat/sriov-cni/config"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func setupVF(conf *SriovConf, ifName string, netns ns.NetNS) error {
	vfIdx := 0
	masterName := conf.Net.Master
	args := conf.Args

	if args.VF != 0 {
		vfIdx = int(args.VF)
	}
	// TODO: if conf.VF == nil, alloc vf randomly

	m, err := netlink.LinkByName(masterName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", masterName, err)
	}

	vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/net", masterName, vfIdx)
	if _, err := os.Lstat(vfDir); err != nil {
		return err
	}

	infos, err := ioutil.ReadDir(vfDir)
	if err != nil {
		return err
	}

	if len(infos) != 1 {
		return fmt.Errorf("no network devices in directory %s", vfDir)
	}

	// VF NIC name
	vfDevName := infos[0].Name()
	vfDev, err := netlink.LinkByName(vfDevName)
	if err != nil {
		return fmt.Errorf("failed to lookup vf device %q: %v", vfDevName, err)
	}

	// set hardware address
	if args.MAC != "" {
		macAddr, err := net.ParseMAC(string(args.MAC))
		if err != nil {
			return err
		}
		if err = netlink.LinkSetVfHardwareAddr(m, vfIdx, macAddr); err != nil {
			return fmt.Errorf("failed to set vf %d macaddress: %v", vfIdx, err)
		}
	}

	if args.VLAN != 0 {
		if err = netlink.LinkSetVfVlan(m, vfIdx, int(args.VLAN)); err != nil {
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
		err := renameLink(vfDevName, ifName)
		if err != nil {
			return fmt.Errorf("failed to rename vf %d device %q to %q: %v", vfIdx, vfDevName, ifName, err)
		}
		return nil
	})
}

func releaseVF(conf *SriovConf, ifName string, netns ns.NetNS) error {
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	// for IPAM in cmdDel
	return netns.Do(func(_ ns.NetNS) error {

		// get VF device
		vfDev, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to lookup device %s: %v", ifName, err)
		}

		// device name in init netns
		index := vfDev.Attrs().Index
		devName := fmt.Sprintf("dev%d", index)

		// shutdown VF device
		if err = netlink.LinkSetDown(vfDev); err != nil {
			return fmt.Errorf("failed to down device: %v", err)
		}

		// rename VF device
		err = renameLink(ifName, devName)
		if err != nil {
			return fmt.Errorf("failed to rename device %s to %s: %v", ifName, devName, err)
		}

		// move VF device to init netns
		if err = netlink.LinkSetNsFd(vfDev, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move device %s to init netns: %v", ifName, err)
		}

		return nil
	})
}

func cmdAdd(args *skel.CmdArgs) error {
	n, err := LoadConf(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	if err = setupVF(n, args.IfName, netns); err != nil {
		return err
	}

	// run the IPAM plugin and get back the config to apply
	result, err := ipam.ExecAdd(n.Net.IPAM.Type, args.StdinData)
	if err != nil {
		return err
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

	result.DNS = n.Net.DNS
	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	n, err := LoadConf(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	if err = releaseVF(n, args.IfName, netns); err != nil {
		return err
	}

	err = ipam.ExecDel(n.Net.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}

	return nil
}

func renameLink(curName, newName string) error {
	link, err := netlink.LinkByName(curName)
	if err != nil {
		return err
	}

	return netlink.LinkSetName(link, newName)
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}
