package config

import (
	"encoding/json"
	"fmt"
	"os"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

// mocked netlink interface
// required for unit tests
var nLink NetlinkManager

// NetlinkManager is an interface to get link by name
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
}

// MyNetlink NetlinkManager
type MyNetlink struct {
	lm NetlinkManager
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

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

	if n.CNIDir == "" {
		n.CNIDir = defaultCNIDir
	}

	if n.DPDKConf != nil {
		// TO-DO: Validate Ddpdk conf here
		n.DPDKMode = true
	}

	return n, nil
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

	_, err := nLink.LinkByName(pfName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// get the ifname sriov vf num
	vfTotal, err := utils.GetSriovNumVfs(pfName)
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
			if _, ok := err.(*os.PathError); ok {
				continue
			} else {
				return fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vf, pfName, err)
			}

		} else if len(infos) > 0 {

			if len(infos) == MaxSharedVf {
				conf.Sharedvf = true
			}

			if len(infos) <= MaxSharedVf {
				vfIdx = vf
				pciAddr, err = utils.GetPciAddress(pfName, vfIdx)
				if err != nil {
					return fmt.Errorf("err in getting pci address for VF %d of PF %s: %q", vf, pfName, err)
				}
				break
			} else {
				return fmt.Errorf("multiple network devices found with VF id: %d under PF %s: %+v", vf, pfName, infos)
			}
		}
	}

	if len(infos) == 0 {
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

func init() {
	nLink = &MyNetlink{}
}
