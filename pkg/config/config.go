package config

import (
	"encoding/json"
	"fmt"
	"strings"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
)

var (
	defaultCNIDir = "/var/lib/cni/sriov"
	// MaxSharedVf defines maximum number of PFs a VF is being shared
	MaxSharedVf = 2
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
		vfInfo, err := getVfInfo(n.DeviceID)
		if err != nil {
			return nil, err
		}
		n.DeviceInfo = vfInfo
		n.Master = vfInfo.Pfname
	} else if n.Master == "" {
		return nil, fmt.Errorf("error: SRIOV-CNI loadConf: VF pci addr OR Master name is required")
	}

	if n.IF0NAME != "" {
		valid := checkIf0name(n.IF0NAME)
		if !valid {
			return nil, fmt.Errorf(`"if0name" field should not be  equal to (eth0 | eth1 | lo | ""). It specifies the virtualized interface name in the pod`)
		}
	}

	if n.CNIDir == "" {
		n.CNIDir = defaultCNIDir
	}

	if n.DPDKConf != nil {
		// TO-DO: Validate Ddpdk conf here
		n.DPDKMode = true
	}

	return n, nil
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

func getVfInfo(vfPci string) (*sriovtypes.VfInformation, error) {
	pf, err := utils.GetPfName(vfPci)
	if err != nil {
		return nil, err
	}
	vfID, err := utils.GetVfid(vfPci, pf)
	if err != nil {
		return nil, err
	}

	return &sriovtypes.VfInformation{
		PCIaddr: vfPci,
		Pfname:  pf,
		Vfid:    vfID,
	}, nil
}
