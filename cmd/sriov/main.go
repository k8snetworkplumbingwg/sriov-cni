package main

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/config"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/sriov"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

type envArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func getEnvArgs(envArgsString string) (*envArgs, error) {
	if envArgsString != "" {
		e := envArgs{}
		err := types.LoadArgs(envArgsString, &e)
		if err != nil {
			return nil, err
		}
		return &e, nil
	}
	return nil, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	var macAddr string
	netConf, err := config.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("SRIOV-CNI failed to load netconf: %v", err)
	}

	envArgs, err := getEnvArgs(args.Args)
	if err != nil {
		return fmt.Errorf("SRIOV-CNI failed to parse args: %v", err)
	}

	if envArgs != nil {
		MAC := string(envArgs.MAC)
		if MAC != "" {
			netConf.MAC = MAC
		}
	}

	// RuntimeConfig takes preference than envArgs.
	// This maintains compatibility of using envArgs
	// for MAC config.
	if netConf.RuntimeConfig.Mac != "" {
		netConf.MAC = netConf.RuntimeConfig.Mac
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

	result := &current.Result{}
	result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: netns.Path(),
	}}

	if !netConf.DPDKMode {
		macAddr, err = sm.SetupVF(netConf, args.IfName, args.ContainerID, netns)
		defer func() {
			if err != nil {
				err := netns.Do(func(_ ns.NetNS) error {
					_, err := netlink.LinkByName(args.IfName)
					return err
				})
				if err == nil {
					_ = sm.ReleaseVF(netConf, args.IfName, args.ContainerID, netns)
				}
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, netConf.Master, err)
		}
		result.Interfaces[0].Mac = macAddr
	}

	// run the IPAM plugin
	if netConf.IPAM.Type != "" {
		r, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", netConf.IPAM.Type, netConf.Master, err)
		}

		defer func() {
			if err != nil {
				_ = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
			}
		}()

		// Convert the IPAM result into the current Result type
		newResult, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		if len(newResult.IPs) == 0 {
			return errors.New("IPAM plugin returned missing IP config")
		}

		newResult.Interfaces = result.Interfaces

		for _, ipc := range newResult.IPs {
			// All addresses apply to the container interface (move from host)
			ipc.Interface = current.Int(0)
		}

		if !netConf.DPDKMode {
			err = netns.Do(func(_ ns.NetNS) error {
				err := ipam.ConfigureIface(args.IfName, newResult)
				if err != nil {
					return err
				}

				/* After IPAM configuration is done, the following needs to handle the case of an IP address being reused by a different pods.
				 * This is achieved by sending Gratuitous ARPs and/or Unsolicited Neighbor Advertisements unconditionally.
				 * Although we set arp_notify and ndisc_notify unconditionally on the interface (please see EnableArpAndNdiscNotify()), the kernel
				 * only sends GARPs/Unsolicited NA when the interface goes from down to up, or when the link-layer address changes on the interfaces.
				 * These scenarios are perfectly valid and recommended to be enabled for optimal network performance.
				 * However for our specific case, which the kernel is unaware of, is the reuse of IP addresses across pods where each pod has a different
				 * link-layer address for it's SRIOV interface. The ARP/Neighbor cache residing in neighbors would be invalid if an IP address is reused.
				 * In order to update the cache, the GARP/Unsolicited NA packets should be sent for performance reasons. Otherwise, the neighbors
				 * may be sending packets with the incorrect link-layer address. Eventually, most network stacks would send ARPs and/or Neighbor
				 * Solicitation packets when the connection is unreachable. This would correct the invalid cache; however this may take a significant
				 * amount of time to complete.
				 *
				 * The error is ignored here because enabling this feature is only a performance enhancement.
				 */
				_ = utils.AnnounceIPs(args.IfName, newResult.IPs)
				return nil
			})
			if err != nil {
				return err
			}
		}
		result = newResult
	}

	// Cache NetConf for CmdDel
	if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
		return fmt.Errorf("error saving NetConf %q", err)
	}

	allocator := utils.NewPCIAllocator(config.DefaultCNIDir)
	// Mark the pci address as in used
	if err = allocator.SaveAllocatedPCI(netConf.DeviceID, args.Netns); err != nil {
		return fmt.Errorf("error saving the pci allocation for vf pci address %s: %v", netConf.DeviceID, err)
	}

	return types.PrintResult(result, netConf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	netConf, cRefPath, err := config.LoadConfFromCache(args)
	if err != nil {
		// If cmdDel() fails, cached netconf is cleaned up by
		// the followed defer call. However, subsequence calls
		// of cmdDel() from kubelet fail in a dead loop due to
		// cached netconf doesn't exist.
		// Return nil when LoadConfFromCache fails since the rest
		// of cmdDel() code relies on netconf as input argument
		// and there is no meaning to continue.
		return nil
	}

	defer func() {
		if err == nil && cRefPath != "" {
			_ = utils.CleanCachedNetConf(cRefPath)
		}
	}()

	if netConf.IPAM.Type != "" {
		err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
	}

	// https://github.com/kubernetes/kubernetes/pull/35240
	if args.Netns == "" {
		return nil
	}

	sm := sriov.NewSriovManager()

	if !netConf.DPDKMode {
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

	// Mark the pci address as released
	allocator := utils.NewPCIAllocator(config.DefaultCNIDir)
	if err = allocator.DeleteAllocatedPCI(netConf.DeviceID); err != nil {
		return fmt.Errorf("error cleaning the pci allocation for vf pci address %s: %v", netConf.DeviceID, err)
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "")
}
