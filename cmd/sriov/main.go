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
	"github.com/intel/sriov-cni/pkg/sriov"
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
	netConf, err := config.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("SRIOV-CNI failed to load netconf: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	sm := sriov.NewSriovManager()
	if err := sm.ApplyVFConfig(netConf); err != nil {
		return fmt.Errorf("SRIOV-CNI failed to configure VF %q", err)
	}

	// skip the IPAM allocation for the DPDK
	var result *types.Result
	if netConf.DPDKMode {
		// Cache NetConf for CmdDel
		if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
			return fmt.Errorf("error saving NetConf %q", err)
		}

		return result.Print()
	}

	if netConf.DeviceID != "" && netConf.VFID >= 0 && netConf.Master != "" {
		err = sm.SetupVF(netConf, args.IfName, args.ContainerID, netns)
		defer func() {
			if err != nil {
				err := netns.Do(func(_ ns.NetNS) error {
					_, err := netlink.LinkByName(args.IfName)
					return err
				})
				if err == nil {
					sm.ReleaseVF(netConf, args.IfName, args.ContainerID, netns)
				}
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, netConf.Master, err)
		}
	} else {
		return fmt.Errorf("VF information are not available to invoke setupVF()")
	}

	// run the IPAM plugin and get back the config to apply
	result, err = ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", netConf.IPAM.Type, netConf.Master, err)
	}

	if result.IP4 == nil {
		return errors.New("IPAM plugin returned missing IPv4 config")
	}

	defer func() {
		if err != nil {
			ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
		}
	}()

	err = netns.Do(func(_ ns.NetNS) error {
		return ipam.ConfigureIface(args.IfName, result)
	})
	if err != nil {
		return err
	}

	// Cache NetConf for CmdDel
	if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
		return fmt.Errorf("error saving NetConf %q", err)
	}

	result.DNS = netConf.DNS
	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	// https://github.com/kubernetes/kubernetes/pull/35240
	if args.Netns == "" {
		return nil
	}

	netConf, cRefPath, err := config.LoadConfFromCache(args)
	if err != nil {
		return err
	}

	defer func() {
		if err == nil && cRefPath != "" {
			utils.CleanCachedNetConf(cRefPath)
		}
	}()

	sm := sriov.NewSriovManager()

	if !netConf.DPDKMode {
		if netConf.IPAM.Type != "" {
			err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
			if err != nil {
				return err
			}
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

			return fmt.Errorf("failed to open netns %s: %q", netns, err)
		}
		defer netns.Close()

		if err = sm.ReleaseVF(netConf, args.IfName, args.ContainerID, netns); err != nil {
			return err
		}
	}

	if err := sm.ResetVFConfig(netConf); err != nil {
		return fmt.Errorf("cmdDel() error reseting VF: %q", err)
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.Legacy)
}
