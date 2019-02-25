package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
)

var (
	// DefaultCNIDir used for caching NetConf
	DefaultCNIDir = "/var/lib/cni/sriov"
	// sriovManager  vfProvider
)

// LoadConf parses and validates stdin netconf and returns NetConf object
func LoadConf(bytes []byte) (*sriovtypes.NetConf, error) {
	n := &sriovtypes.NetConf{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	// DeviceID takes precedence; if we are given a VF pciaddr then work from there
	if n.DeviceID != "" {
		// Get rest of the VF information
		pfName, vfID, err := getVfInfo(n.DeviceID)
		if err != nil {
			return nil, err
		}
		n.VFID = vfID
		n.Master = pfName
	} else {
		return nil, fmt.Errorf("error: SRIOV-CNI loadConf: VF pci addr is required")
	}

	if n.DPDKMode {
		// Detect attached driver
		if dpdkDriver, err := utils.HasDpdkDriver(n.DeviceID); err != nil {
			return nil, fmt.Errorf("error getting driver information for device %s %q", n.DeviceID, err)
		} else if n.DPDKMode != dpdkDriver {
			return nil, fmt.Errorf("dpdkMode requires dpdk supported driver")
		}
	} else {
		// Assuming VF is netdev interface; Get interface name(s)
		hostIFNames, err := utils.GetVFLinkNames(n.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("error reading netdev interface name for VF:%s %q", n.DeviceID, err)
		}

		if len(hostIFNames) > 0 {
			n.HostIFNames = hostIFNames
		}
		if len(hostIFNames) < 1 {
			return nil, fmt.Errorf("netdev interface name not found for VF: %s", n.DeviceID)
		}
	}

	n.ContIFNames = make([]string, 0)
	return n, nil
}

func getVfInfo(vfPci string) (string, int, error) {

	var vfID int

	pf, err := utils.GetPfName(vfPci)
	if err != nil {
		return "", vfID, err
	}

	vfID, err = utils.GetVfid(vfPci, pf)
	if err != nil {
		return "", vfID, err
	}

	return pf, vfID, nil
}

// LoadConfFromCache retrieves cached NetConf returns it along with a handle for removal
func LoadConfFromCache(args *skel.CmdArgs) (*sriovtypes.NetConf, string, error) {
	netConf := &sriovtypes.NetConf{}

	s := []string{args.ContainerID, args.IfName}
	cRef := strings.Join(s, "-")
	cRefPath := filepath.Join(DefaultCNIDir, cRef)

	netConfBytes, err := utils.ReadScratchNetConf(cRefPath)
	if err != nil {
		return nil, "", fmt.Errorf("error reading cached NetConf in %s with name %s", DefaultCNIDir, cRef)
	}

	if err = json.Unmarshal(netConfBytes, netConf); err != nil {
		return nil, "", fmt.Errorf("failed to parse NetConf: %q", err)
	}

	return netConf, cRefPath, nil
}
