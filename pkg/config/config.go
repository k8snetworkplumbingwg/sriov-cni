package config

import (
	"encoding/json"
	"fmt"
	"strings"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

const (
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

// AssignFreeVF takes in a NetConf object and updates it with an self allocated VF information
func AssignFreeVF(conf *sriovtypes.NetConf) error {
	var vfIdx int
	var infos []string
	var pciAddr string
	pfName := conf.Master

	_, err := netlink.LinkByName(pfName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// get the ifname sriov vf num
	vfTotal, err := utils.GetsriovNumfs(pfName)
	if err != nil {
		return err
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device %s", pfName)
	}

	// Select a free VF
	for vf := 0; vf < vfTotal; vf++ {
		infos, err = utils.GetVFLinkNames(pfName, vf)
		if err != nil {
			if _, ok := err.(*utils.NetDeviceNotFoundErr); ok {
				if vf < vfTotal {
					continue
				}
			}
			return fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vf, pfName, err)
		}

		if (len(infos) == 0) && (vf == (vfTotal - 1)) {
			return fmt.Errorf("no free Virtual function exist for PF %s, last vf is virtfn%d", pfName, vf)
		}

		if (len(infos) == 0) && (vf != (vfTotal - 1)) {
			continue
		}

		if len(infos) == MaxSharedVf {
			conf.Sharedvf = true
		}

		if len(infos) <= MaxSharedVf {
			vfIdx = vf
			pciAddr, err = utils.GetPciAddress(pfName, vfIdx)
			if err != nil {
				return fmt.Errorf("err in getting pci address - %q", err)
			}
			break
		} else {
			return fmt.Errorf("multiple network devices found with VF id: %d under PF %s: %+v", vf, pfName, infos)
		}
	}

	if len(infos) != 1 && len(infos) != MaxSharedVf {
		return fmt.Errorf("no virtual network resources available for the %q", conf.Master)
	}

	// instantiate DeviceInfo
	if pciAddr != "" {
		vfInfo := &sriovtypes.VfInformation{
			PCIaddr: pciAddr,
			Pfname:  pfName,
			Vfid:    vfIdx,
		}
		conf.DeviceInfo = vfInfo
	}
	return nil
}
