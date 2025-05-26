package cnicommands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/config"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/logging"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/sriov"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

type envArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
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

func CmdAdd(args *skel.CmdArgs) error {
	if err := config.SetLogging(args.StdinData, args.ContainerID, args.Netns, args.IfName); err != nil {
		return err
	}
	logging.Debug("function called",
		"func", "cmdAdd",
		"args.Path", args.Path, "args.StdinData", string(args.StdinData), "args.Args", args.Args)

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

	// Always use lower case for mac address
	netConf.MAC = strings.ToLower(netConf.MAC)

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	sm := sriov.NewSriovManager()
	err = sm.FillOriginalVfInfo(netConf)
	if err != nil {
		return fmt.Errorf("failed to get original vf information: %v", err)
	}
	defer func() {
		if err != nil {
			err := netns.Do(func(_ ns.NetNS) error {
				_, err := netlink.LinkByName(args.IfName)
				return err
			})
			if err == nil {
				_ = sm.ReleaseVF(netConf, args.IfName, netns)
			}
			// Reset the VF if failure occurs before the netconf is cached
			_ = sm.ResetVFConfig(netConf)
		}
	}()
	if err := sm.ApplyVFConfig(netConf); err != nil {
		return fmt.Errorf("SRIOV-CNI failed to configure VF %q", err)
	}

	result := &current.Result{}
	result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: netns.Path(),
	}}

	if !netConf.DPDKMode {
		err = sm.SetupVF(netConf, args.IfName, netns)

		if err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v", args.IfName, netConf.Master, err)
		}
	}

	result.Interfaces[0].Mac = config.GetMacAddressForResult(netConf)
	// check if we are able to find MTU for the virtual function
	if netConf.MTU != nil {
		result.Interfaces[0].Mtu = *netConf.MTU
	}

	doAnnounce := false

	// run the IPAM plugin
	if netConf.IPAM.Type != "" {
		var r types.Result
		r, err = ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", netConf.IPAM.Type, netConf.Master, err)
		}

		defer func() {
			if err != nil {
				_ = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
			}
		}()

		// Convert the IPAM result into the current Result type
		var newResult *current.Result
		newResult, err = current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		if len(newResult.IPs) == 0 {
			err = errors.New("IPAM plugin returned missing IP config")
			return err
		}

		newResult.Interfaces = result.Interfaces

		for _, ipc := range newResult.IPs {
			// All addresses apply to the container interface (move from host)
			ipc.Interface = current.Int(0)
		}

		if !netConf.DPDKMode {
			err = netns.Do(func(_ ns.NetNS) error {
				return ipam.ConfigureIface(args.IfName, newResult)
			})
			if err != nil {
				return err
			}
			doAnnounce = true
		}
		result = newResult
	}

	// Cache NetConf for CmdDel
	logging.Debug("Cache NetConf for CmdDel",
		"func", "cmdAdd",
		"config.DefaultCNIDir", config.DefaultCNIDir,
		"netConf", netConf)
	if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
		return fmt.Errorf("error saving NetConf %q", err)
	}

	// Mark the pci address as in use.
	logging.Debug("Mark the PCI address as in use",
		"func", "cmdAdd",
		"config.DefaultCNIDir", config.DefaultCNIDir,
		"netConf.DeviceID", netConf.DeviceID)
	allocator := utils.NewPCIAllocator(config.DefaultCNIDir)
	if err = allocator.SaveAllocatedPCI(netConf.DeviceID, args.Netns); err != nil {
		return fmt.Errorf("error saving the pci allocation for vf pci address %s: %v", netConf.DeviceID, err)
	}

	if doAnnounce {
		_ = netns.Do(func(_ ns.NetNS) error {
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
			 */

			/* The interface might not yet have carrier. Wait for it for a short time. */
			hasCarrier := utils.WaitForCarrier(args.IfName, 200*time.Millisecond)

			/* The error is ignored here because enabling this feature is only a performance enhancement. */
			err := utils.AnnounceIPs(args.IfName, result.IPs)

			logging.Debug("announcing IPs", "hasCarrier", hasCarrier, "IPs", result.IPs, "announceError", err)
			return nil
		})
	}

	return types.PrintResult(result, netConf.CNIVersion)
}

func CmdDel(args *skel.CmdArgs) error {
	if err := config.SetLogging(args.StdinData, args.ContainerID, args.Netns, args.IfName); err != nil {
		return err
	}
	logging.Debug("function called",
		"func", "cmdDel",
		"args.Path", args.Path, "args.StdinData", string(args.StdinData), "args.Args", args.Args)

	netConf, cRefPath, err := config.LoadConfFromCache(args)
	if err != nil {
		// If cmdDel() fails, cached netconf is cleaned up by
		// the followed defer call. However, subsequence calls
		// of cmdDel() from kubelet fail in a dead loop due to
		// cached netconf doesn't exist.
		// Return nil when LoadConfFromCache fails since the rest
		// of cmdDel() code relies on netconf as input argument
		// and there is no meaning to continue.
		logging.Error("Cannot load config file from cache",
			"func", "cmdDel",
			"err", err)
		return nil
	}

	allocator := utils.NewPCIAllocator(config.DefaultCNIDir)

	err = allocator.Lock(netConf.DeviceID)
	if err != nil {
		return fmt.Errorf("cmdDel() error obtaining lock for device [%s]: %w", netConf.DeviceID, err)
	}

	logging.Debug("Acquired device lock",
		"func", "cmdDel",
		"DeviceID", netConf.DeviceID)

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

	// Verify VF ID existence.
	if _, err := utils.GetVfid(netConf.DeviceID, netConf.Master); err != nil {
		return fmt.Errorf("cmdDel() error obtaining VF ID: %q", err)
	}

	sm := sriov.NewSriovManager()

	logging.Debug("Reset VF configuration",
		"func", "cmdDel",
		"netConf.DeviceID", netConf.DeviceID)
	/* ResetVFConfig resets a VF administratively. We must run ResetVFConfig
	   before ReleaseVF because some drivers will error out if we try to
	   reset netdev VF with trust off. So, reset VF MAC address via PF first.
	*/
	if err := sm.ResetVFConfig(netConf); err != nil {
		return fmt.Errorf("cmdDel() error reseting VF: %q", err)
	}

	hasDpdkDriver, err := utils.HasDpdkDriver(netConf.DeviceID)
	if err != nil {
		return fmt.Errorf("cmdDel() error checking DPDK driver: %q", err)
	}

	if !hasDpdkDriver {
		netns, err := ns.GetNS(args.Netns)
		if err != nil {
			// according to:
			// https://github.com/kubernetes/kubernetes/issues/43014#issuecomment-287164444
			// if provided path does not exist (e.x. when node was restarted)
			// plugin should silently return with success after releasing
			// IPAM resources
			_, ok := err.(ns.NSPathNotExistErr)
			if ok {
				logging.Debug("Exiting as the network namespace does not exists anymore",
					"func", "cmdDel",
					"netConf.DeviceID", netConf.DeviceID,
					"args.Netns", args.Netns)
				return nil
			}

			return fmt.Errorf("failed to open netns %s: %q", netns, err)
		}
		defer netns.Close()

		logging.Debug("Release the VF",
			"func", "cmdDel",
			"netConf.DeviceID", netConf.DeviceID,
			"args.Netns", args.Netns,
			"args.IfName", args.IfName)
		if err := sm.ReleaseVF(netConf, args.IfName, netns); err != nil {
			return err
		}
	}

	// Mark the pci address as released
	logging.Debug("Mark the PCI address as released",
		"func", "cmdDel",
		"config.DefaultCNIDir", config.DefaultCNIDir,
		"netConf.DeviceID", netConf.DeviceID)
	if err = allocator.DeleteAllocatedPCI(netConf.DeviceID); err != nil {
		return fmt.Errorf("error cleaning the pci allocation for vf pci address %s: %v", netConf.DeviceID, err)
	}

	return nil
}

func CmdCheck(_ *skel.CmdArgs) error {
	return nil
}
