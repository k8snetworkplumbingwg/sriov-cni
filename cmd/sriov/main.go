package main

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/intel/sriov-cni/pkg/config"
	"github.com/intel/sriov-cni/pkg/providers"
	"github.com/intel/sriov-cni/pkg/sriov"
	"github.com/intel/sriov-cni/pkg/utils"
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

func getVlanTrunkRange(vlanTrunkString string) (providers.VlanTrunkRangeArray, error) {
	re := regexp.MustCompile("^[0-9]+([,\\-][0-9]+)*$")
	if re.MatchString(vlanTrunkString) {

		var vlanRange = []providers.VlanTrunkRange{}
		trunkingRanges := strings.Split(vlanTrunkString, ",")
		fmt.Println("Vlan trunking ranges: ", trunkingRanges)

		for _, r := range trunkingRanges {

			values := strings.Split(r, "-")
			v1, errconv1 := strconv.Atoi(values[0])
			v2, errconv2 := strconv.Atoi(values[len(values)-1])

			if errconv1 != nil || errconv2 != nil {
				return providers.VlanTrunkRangeArray{}, fmt.Errorf("Trunk range error: invalid values")
			}

			v := providers.VlanTrunkRange{
				Start: uint(v1),
				End:   uint(v2),
			}

			vlanRange = append(vlanRange, v)
		}
		if err := validateVlanTrunkRange(vlanRange); err != nil {
			return providers.VlanTrunkRangeArray{}, err
		}

		vlanRanges := providers.VlanTrunkRangeArray{
			VlanTrunkRanges: vlanRange,
		}
		return vlanRanges, nil
	}

	return providers.VlanTrunkRangeArray{}, fmt.Errorf("No VLAN trunk ranges specified")
}

func validateVlanTrunkRange(vlanRanges []providers.VlanTrunkRange) error {

	for i, r1 := range vlanRanges {
		if r1.Start > r1.End {
			return fmt.Errorf("Invalid VlanTrunk range values")
		}
		for j, r2 := range vlanRanges {
			if r1.End > r2.Start && i < j {
				return fmt.Errorf("Invalid VlanTrunk range values")
			}
		}

	}
	return nil
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

	// skip the IPAM allocation for the DPDK
	if netConf.DPDKMode {
		// Cache NetConf for CmdDel
		if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
			return fmt.Errorf("error saving NetConf %q", err)
		}
		return result.Print()
	}

	macAddr, err = sm.SetupVF(netConf, args.IfName, args.ContainerID, netns)
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

	// run the IPAM plugin
	if netConf.IPAM.Type != "" {
		r, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v", netConf.IPAM.Type, netConf.Master, err)
		}

		defer func() {
			if err != nil {
				ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
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
		newResult.Interfaces[0].Mac = macAddr

		for _, ipc := range newResult.IPs {
			// All addresses apply to the container interface (move from host)
			ipc.Interface = current.Int(0)
		}

		err = netns.Do(func(_ ns.NetNS) error {
			return ipam.ConfigureIface(args.IfName, newResult)
		})
		if err != nil {
			return err
		}
		result = newResult
	}

	if netConf.VlanTrunk != "" {
		if vlanTrunkRange, err := getVlanTrunkRange(netConf.VlanTrunk); err == nil {
			vlanTrunkProviderConfig := providers.GetProviderConfig(netConf.DeviceID)
			vlanTrunkProviderConfig.InitConfig(&vlanTrunkRange)
		} else {
			fmt.Errorf("Unable to get vlanTrunkRange")
		}
	}

	// Cache NetConf for CmdDel
	if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
		return fmt.Errorf("error saving NetConf %q", err)
	}

	return types.PrintResult(result, current.ImplementedSpecVersion)
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

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "")
}
