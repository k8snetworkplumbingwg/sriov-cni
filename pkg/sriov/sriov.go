package sriov

import (
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"

	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

type pciUtils interface {
	GetSriovNumVfs(ifName string) (int, error)
	GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error)
	GetPciAddress(ifName string, vf int) (string, error)
	EnableArpAndNdiscNotify(ifName string) error
}

type pciUtilsImpl struct{}

func (p *pciUtilsImpl) GetSriovNumVfs(ifName string) (int, error) {
	return utils.GetSriovNumVfs(ifName)
}

func (p *pciUtilsImpl) GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	return utils.GetVFLinkNamesFromVFID(pfName, vfID)
}

func (p *pciUtilsImpl) GetPciAddress(ifName string, vf int) (string, error) {
	return utils.GetPciAddress(ifName, vf)
}

func (p *pciUtilsImpl) EnableArpAndNdiscNotify(ifName string) error {
	return utils.EnableArpAndNdiscNotify(ifName)
}

// Manager provides interface invoke sriov nic related operations
type Manager interface {
	SetupVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error
	ReleaseVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error
	ResetVFConfig(conf *sriovtypes.NetConf) error
	ApplyVFConfig(conf *sriovtypes.NetConf) error
	FillOriginalVfInfo(conf *sriovtypes.NetConf) error
}

type sriovManager struct {
	nLink utils.NetlinkManager
	utils pciUtils
}

// NewSriovManager returns an instance of SriovManager
func NewSriovManager() Manager {
	return &sriovManager{
		nLink: &utils.MyNetlink{},
		utils: &pciUtilsImpl{},
	}
}

// SetupVF sets up a VF in Pod netns
func (s *sriovManager) SetupVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error {
	linkName := conf.OrigVfState.HostIFName

	linkObj, err := s.nLink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("error getting VF netdevice with name %s", linkName)
	}

	// tempName used as intermediary name to avoid name conflicts
	tempName := fmt.Sprintf("%s%d", "temp_", linkObj.Attrs().Index)

	// 1. Set link down
	if err := s.nLink.LinkSetDown(linkObj); err != nil {
		return fmt.Errorf("failed to down vf device %q: %v", linkName, err)
	}

	// 2. Set temp name
	if err := s.nLink.LinkSetName(linkObj, tempName); err != nil {
		return fmt.Errorf("error setting temp IF name %s for %s", tempName, linkName)
	}

	// Save the original effective MAC address before overriding it
	conf.OrigVfState.EffectiveMAC = linkObj.Attrs().HardwareAddr.String()
	// 3. Set MAC address
	if conf.MAC != "" {
		err = utils.SetVFEffectiveMAC(s.nLink, tempName, conf.MAC)
		if err != nil {
			return fmt.Errorf("failed to set netlink MAC address to %s: %v", conf.MAC, err)
		}
	}

	// 4. Change netns
	if err := s.nLink.LinkSetNsFd(linkObj, int(netns.Fd())); err != nil {
		return fmt.Errorf("failed to move IF %s to netns: %q", tempName, err)
	}

	if err := netns.Do(func(_ ns.NetNS) error {
		// 5. Set Pod IF name
		if err := s.nLink.LinkSetName(linkObj, podifName); err != nil {
			return fmt.Errorf("error setting container interface name %s for %s", linkName, tempName)
		}

		// 6. Enable IPv4 ARP notify and IPv6 Network Discovery notify
		// Error is ignored here because enabling this feature is only a performance enhancement.
		_ = s.utils.EnableArpAndNdiscNotify(podifName)

		// 7. Set allmulticast flag
		if conf.AllMulti != "" {
			conf.OrigVfState.AllMulti = linkObj.Attrs().Allmulti != 0
			if conf.AllMulti == "on" {
				if err := s.nLink.LinkSetAllmulticastOn(linkObj); err != nil {
					return fmt.Errorf("error setting allmulticast %s: %v", conf.AllMulti, err)
				}
			} else {
				if err := s.nLink.LinkSetAllmulticastOff(linkObj); err != nil {
					return fmt.Errorf("error setting allmulticast %s: %v", conf.AllMulti, err)
				}
			}
		}

		// 8. Bring IF up in Pod netns
		if err := s.nLink.LinkSetUp(linkObj); err != nil {
			return fmt.Errorf("error bringing interface up in container ns: %q", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error setting up interface in container namespace: %q", err)
	}
	conf.ContIFNames = podifName

	return nil
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (s *sriovManager) ReleaseVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error {
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if len(conf.ContIFNames) < 1 && len(conf.ContIFNames) != len(conf.OrigVfState.HostIFName) {
		return fmt.Errorf("number of interface names mismatch ContIFNames: %d HostIFNames: %d", len(conf.ContIFNames), len(conf.OrigVfState.HostIFName))
	}

	return netns.Do(func(_ ns.NetNS) error {
		// get VF device
		linkObj, err := s.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", podifName, err)
		}

		// shutdown VF device
		if err = s.nLink.LinkSetDown(linkObj); err != nil {
			return fmt.Errorf("failed to set link %s down: %q", podifName, err)
		}

		// rename VF device
		err = s.nLink.LinkSetName(linkObj, conf.OrigVfState.HostIFName)
		if err != nil {
			return fmt.Errorf("failed to rename link %s to host name %s: %q", podifName, conf.OrigVfState.HostIFName, err)
		}

		if conf.MAC != "" {
			// reset effective MAC address
			err = utils.SetVFEffectiveMAC(s.nLink, conf.OrigVfState.HostIFName, conf.OrigVfState.EffectiveMAC)
			if err != nil {
				return fmt.Errorf("failed to restore original effective netlink MAC address %s: %v", conf.OrigVfState.EffectiveMAC, err)
			}
		}

		// reset allmulticast flag
		if conf.AllMulti != "" {
			if conf.OrigVfState.AllMulti {
				if err := s.nLink.LinkSetAllmulticastOn(linkObj); err != nil {
					return fmt.Errorf("error setting allmulticast %s: %v", conf.AllMulti, err)
				}
			} else {
				if err := s.nLink.LinkSetAllmulticastOff(linkObj); err != nil {
					return fmt.Errorf("error setting allmulticast %s: %v", conf.AllMulti, err)
				}
			}
		}

		// move VF device to init netns
		if err = s.nLink.LinkSetNsFd(linkObj, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move interface %s to init netns: %v", conf.OrigVfState.HostIFName, err)
		}

		return nil
	})
}

func getVfInfo(link netlink.Link, id int) *netlink.VfInfo {
	attrs := link.Attrs()
	for _, vf := range attrs.Vfs {
		if vf.ID == id {
			return &vf
		}
	}
	return nil
}

// ApplyVFConfig configure a VF with parameters given in NetConf
func (s *sriovManager) ApplyVFConfig(conf *sriovtypes.NetConf) error {
	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}
	// 1. Set vlan
	if conf.Vlan == nil {
		vlan := new(int)
		*vlan = 0
		conf.Vlan = vlan
	}
	// set vlan qos if present in the config
	if conf.VlanQoS != nil {
		if err = s.nLink.LinkSetVfVlanQos(pfLink, conf.VFID, *conf.Vlan, *conf.VlanQoS); err != nil {
			return fmt.Errorf("failed to set vf %d vlan configuration: %v", conf.VFID, err)
		}
	} else {
		// set vlan id field only
		if err = s.nLink.LinkSetVfVlan(pfLink, conf.VFID, *conf.Vlan); err != nil {
			return fmt.Errorf("failed to set vf %d vlan: %v", conf.VFID, err)
		}
	}

	// 2. Set mac address
	if conf.MAC != "" {
		// when we restore the original hardware mac address we may get a device or resource busy. so we introduce retry
		if err := utils.SetVFHardwareMAC(s.nLink, conf.Master, conf.VFID, conf.MAC); err != nil {
			return fmt.Errorf("failed to set MAC address to %s: %v", conf.MAC, err)
		}
	}

	// 3. Set min/max tx link rate. 0 means no rate limiting. Support depends on NICs and driver.
	var minTxRate, maxTxRate int
	rateConfigured := false
	if conf.MinTxRate != nil {
		minTxRate = *conf.MinTxRate
		rateConfigured = true
	}

	if conf.MaxTxRate != nil {
		maxTxRate = *conf.MaxTxRate
		rateConfigured = true
	}

	if rateConfigured {
		if err = s.nLink.LinkSetVfRate(pfLink, conf.VFID, minTxRate, maxTxRate); err != nil {
			return fmt.Errorf("failed to set vf %d min_tx_rate to %d Mbps: max_tx_rate to %d Mbps: %v",
				conf.VFID, minTxRate, maxTxRate, err)
		}
	}

	// 4. Set spoofchk flag
	if conf.SpoofChk != "" {
		spoofChk := false
		if conf.SpoofChk == "on" {
			spoofChk = true
		}
		if err = s.nLink.LinkSetVfSpoofchk(pfLink, conf.VFID, spoofChk); err != nil {
			return fmt.Errorf("failed to set vf %d spoofchk flag to %s: %v", conf.VFID, conf.SpoofChk, err)
		}
	}

	// 5. Set trust flag
	if conf.Trust != "" {
		trust := false
		if conf.Trust == "on" {
			trust = true
		}
		if err = s.nLink.LinkSetVfTrust(pfLink, conf.VFID, trust); err != nil {
			return fmt.Errorf("failed to set vf %d trust flag to %s: %v", conf.VFID, conf.Trust, err)
		}
	}

	// 6. Set link state
	if conf.LinkState != "" {
		var state uint32
		switch conf.LinkState {
		case "auto":
			state = netlink.VF_LINK_STATE_AUTO
		case "enable":
			state = netlink.VF_LINK_STATE_ENABLE
		case "disable":
			state = netlink.VF_LINK_STATE_DISABLE
		default:
			// the value should have been validated earlier, return error if we somehow got here
			return fmt.Errorf("unknown link state %s when setting it for vf %d: %v", conf.LinkState, conf.VFID, err)
		}
		if err = s.nLink.LinkSetVfState(pfLink, conf.VFID, state); err != nil {
			return fmt.Errorf("failed to set vf %d link state to %d: %v", conf.VFID, state, err)
		}
	}

	return nil
}

// FillOriginalVfInfo fills the original vf info
func (s *sriovManager) FillOriginalVfInfo(conf *sriovtypes.NetConf) error {
	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}
	// Save current the VF state before modifying it
	vfState := getVfInfo(pfLink, conf.VFID)
	if vfState == nil {
		return fmt.Errorf("failed to find vf %d", conf.VFID)
	}
	conf.OrigVfState.FillFromVfInfo(vfState)

	return err
}

// ResetVFConfig reset a VF to its original state
func (s *sriovManager) ResetVFConfig(conf *sriovtypes.NetConf) error {
	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Restore VLAN
	if conf.Vlan != nil {
		if conf.VlanQoS != nil {
			if err = s.nLink.LinkSetVfVlanQos(pfLink, conf.VFID, conf.OrigVfState.Vlan, conf.OrigVfState.VlanQoS); err != nil {
				return fmt.Errorf("failed to restore vf %d vlan: %v", conf.VFID, err)
			}
		} else if err = s.nLink.LinkSetVfVlan(pfLink, conf.VFID, conf.OrigVfState.Vlan); err != nil {
			return fmt.Errorf("failed to restore vf %d vlan: %v", conf.VFID, err)
		}
	}

	// Restore spoofchk
	if conf.SpoofChk != "" {
		if err = s.nLink.LinkSetVfSpoofchk(pfLink, conf.VFID, conf.OrigVfState.SpoofChk); err != nil {
			return fmt.Errorf("failed to restore spoofchk for vf %d: %v", conf.VFID, err)
		}
	}

	// Restore the original administrative MAC address
	if conf.MAC != "" {
		// when we restore the original hardware mac address we may get a device or resource busy. so we introduce retry
		if err := utils.SetVFHardwareMAC(s.nLink, conf.Master, conf.VFID, conf.OrigVfState.AdminMAC); err != nil {
			return fmt.Errorf("failed to restore original administrative MAC address %s: %v", conf.OrigVfState.AdminMAC, err)
		}
	}

	// Restore VF trust
	if conf.Trust != "" {
		if err = s.nLink.LinkSetVfTrust(pfLink, conf.VFID, conf.OrigVfState.Trust); err != nil {
			return fmt.Errorf("failed to set trust for vf %d: %v", conf.VFID, err)
		}
	}

	// Restore rate limiting
	if conf.MinTxRate != nil || conf.MaxTxRate != nil {
		if err = s.nLink.LinkSetVfRate(pfLink, conf.VFID, conf.OrigVfState.MinTxRate, conf.OrigVfState.MaxTxRate); err != nil {
			return fmt.Errorf("failed to disable rate limiting for vf %d %v", conf.VFID, err)
		}
	}

	// Restore link state to `auto`
	if conf.LinkState != "" {
		// Reset only when link_state was explicitly specified, to  accommodate for drivers / NICs
		// that don't support the netlink command (e.g. igb driver)
		if err = s.nLink.LinkSetVfState(pfLink, conf.VFID, conf.OrigVfState.LinkState); err != nil {
			return fmt.Errorf("failed to set link state to auto for vf %d: %v", conf.VFID, err)
		}
	}

	return nil
}
