package sriov

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/logging"
	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

type pciUtils interface {
	GetSriovNumVfs(ifName string) (int, error)
	GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error)
	GetPciAddress(ifName string, vf int) (string, error)
	EnableArpAndNdiscNotify(ifName string) error
	EnableOptimisticDad(ifName string) error
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

func (p *pciUtilsImpl) EnableOptimisticDad(ifName string) error {
	return utils.EnableOptimisticDad(ifName)
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
		nLink: utils.GetNetlinkManager(),
		utils: &pciUtilsImpl{},
	}
}

// SetupVF sets up a VF in Pod netns
func (s *sriovManager) SetupVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error {
	linkName := conf.OrigVfState.HostIFName
	// Save the original NS in case we need to restore it
	// after an error occurs
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get current NS: %v", err)
	}
	tempNS, err := ns.TempNetNS()
	if err != nil {
		return fmt.Errorf("failed to create tempNS: %v", err)
	}

	defer func() {
		if cerr := tempNS.Close(); cerr != nil {
			logging.Warning("failed to close temporary netns; VF may remain accessible in temp namespace",
				"func", "SetupVF",
				"linkName", linkName,
				"error", cerr)
		}
	}()

	linkObj, err := s.nLink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("error: %v. Failed to get VF netdevice with name %s", err, linkName)
	}

	// Save the original effective MAC address before overriding it
	conf.OrigVfState.EffectiveMAC = linkObj.Attrs().HardwareAddr.String()

	// 1.Move interface to tempNS
	logging.Debug("1. Move the interface to tempNS",
		"func", "SetupVF",
		"linkObj", linkObj)
	if err = s.nLink.LinkSetNsFd(linkObj, int(tempNS.Fd())); err != nil {
		return fmt.Errorf("failed to move %q to tempNS: %v", linkName, err)
	}
	err = tempNS.Do(func(linkNS ns.NetNS) error {
		// lookup the device in tempNS (index might have changed)
		tempNSLinkObj, err := s.nLink.LinkByName(linkName)
		if err != nil {
			return fmt.Errorf("failed to find %q in tempNS: %v", linkName, err)
		}
		// Rename the interface to pod interface name
		if err = s.nLink.LinkSetName(tempNSLinkObj, podifName); err != nil {
			return fmt.Errorf("failed to rename host device %q to %q: %v", linkName, podifName, err)
		}

		// 3. Remove alt name from the nic
		logging.Debug("3. Remove interface original name from alt names",
			"func", "SetupVF",
			"tempNSObj", tempNSLinkObj,
			"OriginalLinkName", linkName)
		for _, altName := range tempNSLinkObj.Attrs().AltNames {
			if altName == linkName {
				if err = s.nLink.LinkDelAltName(tempNSLinkObj, linkName); err != nil {
					return fmt.Errorf("error removing VF altname %s: %v", linkName, err)
				}
			}
		}

		// 4. Change netns
		logging.Debug("4. Change netns",
			"func", "SetupVF",
			"tempNSObj", tempNSLinkObj,
			"netns.Fd()", int(netns.Fd()))
		if err = s.nLink.LinkSetNsFd(tempNSLinkObj, int(netns.Fd())); err != nil {
			return fmt.Errorf("failed to move IF %s to netns: %w", podifName, err)
		}
		return nil
	})
	if err != nil {
		logging.Error("Move the interface back to initNS because of ", "error", err)
		renameAndMoveLinkErr := s.renameAndMoveLink(tempNS, initns, []string{podifName, linkName}, linkName)
		if renameAndMoveLinkErr != nil {
			return fmt.Errorf("setupVF failed: %v; rollback failed: %v", err, renameAndMoveLinkErr)
		}
		return fmt.Errorf("setupVF failed: %v", err)
	}

	err = netns.Do(func(_ ns.NetNS) error {
		netNSLinkObj, err := s.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("error: %v. Failed to get VF netdevice with name %s", err, podifName)
		}

		// 5. Enable IPv4 ARP notify and IPv6 Network Discovery notify
		// Error is ignored here because enabling this feature is only a performance enhancement.
		logging.Debug("5. Enable IPv4 ARP notify and IPv6 Network Discovery notify",
			"func", "SetupVF",
			"podifName", podifName)
		_ = s.utils.EnableArpAndNdiscNotify(podifName)

		// 6. Set MAC address
		if conf.MAC != "" {
			logging.Debug("6. Set MAC address",
				"func", "SetupVF",
				"s.nLink", s.nLink,
				"podifName", podifName,
				"conf.MAC", conf.MAC)
			err = utils.SetVFEffectiveMAC(s.nLink, podifName, conf.MAC)
			if err != nil {
				return fmt.Errorf("failed to set netlink MAC address to %s: %v", conf.MAC, err)
			}
		}

		logging.Debug("7. Enable Optimistic DAD for IPv6 addresses", "func", "SetupVF",
			"linkObj", netNSLinkObj)
		_ = s.utils.EnableOptimisticDad(podifName)

		// 8. Bring IF up in Pod netns
		logging.Debug("8. Bring IF up in Pod netns",
			"func", "SetupVF",
			"linkObj", netNSLinkObj)
		if err = s.nLink.LinkSetUp(netNSLinkObj); err != nil {
			return fmt.Errorf("error bringing interface up in container ns: %q", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error setting up interface in container namespace: %q", err)
	}

	// Copy the MTU value to a new variable
	// and use it as a pointer
	vfMTU := linkObj.Attrs().MTU
	conf.MTU = &vfMTU
	return nil
}

// renameAndMoveLink finds a link by iterating over possible names, renames it if needed, and moves it to target namespace
func (s *sriovManager) renameAndMoveLink(source, target ns.NetNS, possibleIfNames []string, targetIfname string) error {
	return source.Do(func(_ ns.NetNS) error {
		var linkObj netlink.Link
		var currentName string
		var err error

		// Iterate over possible interface names to find the link
		for _, name := range possibleIfNames {
			linkObj, err = s.nLink.LinkByName(name)
			if err == nil {
				currentName = name
				logging.Debug("Found interface",
					"func", "renameAndMoveLink",
					"currentName", currentName)
				break
			}
		}

		if linkObj == nil {
			return fmt.Errorf("failed to find interface with any of the possible names: %v", possibleIfNames)
		}

		// Rename the interface if its current name is different from target name
		if currentName != targetIfname {
			logging.Debug("Renaming interface",
				"func", "renameAndMoveLink",
				"from", currentName,
				"to", targetIfname)
			if err = s.nLink.LinkSetName(linkObj, targetIfname); err != nil {
				logging.Warning("LinkSetName failed when trying to rename", "error", err)
				return fmt.Errorf("failed to rename interface from %q to %q: %v", currentName, targetIfname, err)
			}
		}

		// Move interface to target namespace
		logging.Debug("Moving interface to target namespace",
			"func", "renameAndMoveLink",
			"ifname", targetIfname)
		if err = s.nLink.LinkSetNsFd(linkObj, int(target.Fd())); err != nil {
			logging.Warning("LinkSetNsFd failed when trying to move to target namespace", "error", err)
			return fmt.Errorf("failed to move interface %q to target namespace: %v", targetIfname, err)
		}

		return nil
	})
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (s *sriovManager) ReleaseVF(conf *sriovtypes.NetConf, podifName string, netns ns.NetNS) error {
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	return netns.Do(func(_ ns.NetNS) error {
		// get VF device
		logging.Debug("Get VF device",
			"func", "ReleaseVF",
			"podifName", podifName)
		linkObj, err := s.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", podifName, err)
		}

		// shutdown VF device
		logging.Debug("Shutdown VF device",
			"func", "ReleaseVF",
			"linkObj", linkObj)
		if err = s.nLink.LinkSetDown(linkObj); err != nil {
			return fmt.Errorf("failed to set link %s down: %q", podifName, err)
		}

		// rename VF device
		logging.Debug("Rename VF device",
			"func", "ReleaseVF",
			"linkObj", linkObj,
			"conf.OrigVfState.HostIFName", conf.OrigVfState.HostIFName)
		err = s.nLink.LinkSetName(linkObj, conf.OrigVfState.HostIFName)
		if err != nil {
			return fmt.Errorf("failed to rename link %s to host name %s: %q", podifName, conf.OrigVfState.HostIFName, err)
		}

		if conf.MAC != "" {
			// reset effective MAC address
			logging.Debug("Reset effective MAC address",
				"func", "ReleaseVF",
				"s.nLink", s.nLink,
				"conf.OrigVfState.HostIFName", conf.OrigVfState.HostIFName,
				"conf.OrigVfState.EffectiveMAC", conf.OrigVfState.EffectiveMAC)
			err = utils.SetVFEffectiveMAC(s.nLink, conf.OrigVfState.HostIFName, conf.OrigVfState.EffectiveMAC)
			if err != nil {
				return fmt.Errorf("failed to restore original effective netlink MAC address %s: %v", conf.OrigVfState.EffectiveMAC, err)
			}
		}

		// reset MTU for VF device until if the MTU was captured in the cache
		if conf.OrigVfState.MTU != 0 {
			logging.Debug("Reset VF device MTU",
				"func", "ReleaseVF",
				"linkObj", linkObj,
				"conf.OrigVfState.HostIFName", conf.OrigVfState.HostIFName,
				"conf.OrigVfState.MTU", conf.OrigVfState.MTU)
			err = s.nLink.LinkSetMTU(linkObj, conf.OrigVfState.MTU)
			if err != nil {
				return fmt.Errorf("failed to reset MTU for link link %s: %q", conf.OrigVfState.HostIFName, err)
			}
		}

		// move VF device to init netns
		logging.Debug("Move VF device to init netns",
			"func", "ReleaseVF",
			"linkObj", linkObj,
			"initns.Fd()", int(initns.Fd()))
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
	if conf.Vlan != nil {
		if err = s.nLink.LinkSetVfVlanQosProto(pfLink, conf.VFID, *conf.Vlan, *conf.VlanQoS, sriovtypes.VlanProtoInt[*conf.VlanProto]); err != nil {
			return fmt.Errorf("failed to set vf %d vlan configuration - id %d, qos %d and proto %s: %v", conf.VFID, *conf.Vlan, *conf.VlanQoS, *conf.VlanProto, err)
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
		//nolint:goconst
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
		//nolint:goconst
		case "auto":
			state = netlink.VF_LINK_STATE_AUTO
		//nolint:goconst
		case "enable":
			state = netlink.VF_LINK_STATE_ENABLE
		//nolint:goconst
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

	// Copy the MTU value to a new variable
	// and use it as a pointer
	pfMtu := pfLink.Attrs().MTU
	conf.MTU = &pfMtu

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

	// add also MTU to the vf info in the vf is we have an interface name
	if conf.OrigVfState.HostIFName != "" {
		vfLink, err := s.nLink.LinkByName(conf.OrigVfState.HostIFName)
		if err != nil {
			return fmt.Errorf("failed to lookup vf %q: %v", conf.OrigVfState.HostIFName, err)
		}
		conf.OrigVfState.MTU = vfLink.Attrs().MTU
	}

	return err
}

// ResetVFConfig reset a VF to its original state
func (s *sriovManager) ResetVFConfig(conf *sriovtypes.NetConf) error {
	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Set 802.1q as default in case cache config does not have a value for vlan proto.
	if conf.OrigVfState.VlanProto == 0 {
		conf.OrigVfState.VlanProto = sriovtypes.VlanProtoInt[sriovtypes.Proto8021q]
	}

	if conf.Vlan != nil {
		if err = s.nLink.LinkSetVfVlanQosProto(pfLink, conf.VFID, conf.OrigVfState.Vlan, conf.OrigVfState.VlanQoS, conf.OrigVfState.VlanProto); err != nil {
			return fmt.Errorf("failed to set vf %d vlan configuration - id %d, qos %d and proto %d: %v", conf.VFID, conf.OrigVfState.Vlan, conf.OrigVfState.VlanQoS, conf.OrigVfState.VlanProto, err)
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
