package main

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/intel/sriov-cni/pkg/config"
	"github.com/intel/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func cmdAdd(args *skel.CmdArgs) error {
	n, err := config.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("SRIOV-CNI failed to load netconf: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	// Try assigning a VF from PF
	if n.DeviceInfo == nil && n.Master != "" {
		// Populate device info from PF
		if err := config.AssignFreeVF(n); err != nil {
			return fmt.Errorf("unable to get VF information %+v", err)
		}
	}

	if n.Sharedvf && !n.L2Mode {
		return fmt.Errorf("l2enable mode must be true to use shared net interface %q", n.Master)
	}

	// fill in DpdkConf from DeviceInfo
	if n.DPDKMode {
		n.DPDKConf.PCIaddr = n.DeviceInfo.PCIaddr
		n.DPDKConf.Ifname = args.IfName
		n.DPDKConf.VFID = n.DeviceInfo.Vfid
	}

	if n.DeviceInfo != nil && n.DeviceInfo.PCIaddr != "" && n.DeviceInfo.Vfid >= 0 && n.DeviceInfo.Pfname != "" {
		err = setupVF(n, args.IfName, args.ContainerID, netns)
		defer func() {
			if err != nil {
				if !n.DPDKMode {
					err = netns.Do(func(_ ns.NetNS) error {
						_, err := netlink.LinkByName(args.IfName)
						return err
					})
				}
				if n.DPDKMode || err == nil {
					releaseVF(n, args.IfName, args.ContainerID, netns)
				}
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, n.Master, err)
		}
	} else {
		return fmt.Errorf("VF information are not available to invoke setupVF()")
	}

	netlinkExpected, err := utils.ShouldHaveNetlink(n.Master, n.DeviceInfo.Vfid)
	if err != nil {
		return fmt.Errorf("failed to determine if interface should have netlink device: %v", err)
	}

	// skip the IPAM allocation for the DPDK and L2 mode
	var result *types.Result
	if n.DPDKMode || n.L2Mode || !netlinkExpected {
		return result.Print()
	}

	// run the IPAM plugin and get back the config to apply
	result, err = ipam.ExecAdd(n.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", n.IPAM.Type, n.Master, err)
	}

	if result.IP4 == nil {
		return errors.New("IPAM plugin returned missing IPv4 config")
	}

	defer func() {
		if err != nil {
			ipam.ExecDel(n.IPAM.Type, args.StdinData)
		}
	}()

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
	n, err := config.LoadConf(args.StdinData)
	if err != nil {
		return err
	}

	// skip the IPAM release for the DPDK and L2 mode
	if !n.DPDKMode && !n.L2Mode && n.IPAM.Type != "" {
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

	if err = releaseVF(n, args.IfName, args.ContainerID, netns); err != nil {
		return err
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.Legacy)
}
