package sriov

import (
	"fmt"
	"os"
	"sort"

	"github.com/containernetworking/plugins/pkg/ns"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

// mocked netlink interface
// required for unit tests

// NetlinkManager is an interface to mock nelink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetVfVlan(netlink.Link, int, int) error
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
}

// MyNetlink NetlinkManager
type MyNetlink struct {
	lm NetlinkManager
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetVfVlan using NetlinkManager
func (n *MyNetlink) LinkSetVfVlan(link netlink.Link, vf, vlan int) error {
	return netlink.LinkSetVfVlan(link, vf, vlan)
}

// LinkSetUp using NetlinkManager
func (n *MyNetlink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkSetDown using NetlinkManager
func (n *MyNetlink) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

// LinkSetNsFd using NetlinkManager
func (n *MyNetlink) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// LinkSetName using NetlinkManager
func (n *MyNetlink) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

/* Link names are given as os.FileInfo and need to be sorted by their Index */

// LinkByIndexSorter holds network interfaces names and NetLinkManager
type LinkByIndexSorter struct {
	linkNames []string
	nLink     NetlinkManager
}

// LinksByIndex implements sort.Inteface
func (l *LinkByIndexSorter) Len() int { return len(l.linkNames) }

// Swap implements Swap() method of sort interface
func (l *LinkByIndexSorter) Swap(i, j int) {
	l.linkNames[i], l.linkNames[j] = l.linkNames[j], l.linkNames[i]
}

// Less implements Less() method of sort interface
func (l *LinkByIndexSorter) Less(i, j int) bool {
	linkA, _ := l.nLink.LinkByName(l.linkNames[i])
	linkB, _ := l.nLink.LinkByName(l.linkNames[j])

	return linkA.Attrs().Index < linkB.Attrs().Index
}

type pciUtils interface {
	getSriovNumVfs(ifName string) (int, error)
	getVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error)
	getPciAddress(ifName string, vf int) (string, error)
}

type pciUtilsImpl struct{}

func (p *pciUtilsImpl) getSriovNumVfs(ifName string) (int, error) {
	return utils.GetSriovNumVfs(ifName)
}

func (p *pciUtilsImpl) getVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	return utils.GetVFLinkNamesFromVFID(pfName, vfID)
}

func (p *pciUtilsImpl) getPciAddress(ifName string, vf int) (string, error) {
	return utils.GetPciAddress(ifName, vf)
}

// Manager provides interface invoke sriov nic related operations
type Manager interface {
	SetupVF(conf *sriovtypes.NetConf, podifName string, cid string, netns ns.NetNS) (string, error)
	ReleaseVF(conf *sriovtypes.NetConf, podifName string, cid string, netns ns.NetNS) error
	ResetVFConfig(conf *sriovtypes.NetConf) error
	ApplyVFConfig(conf *sriovtypes.NetConf) error
}

type sriovManager struct {
	nLink NetlinkManager
	utils pciUtils
}

// NewSriovManager returns an instance of SriovManager
func NewSriovManager() Manager {
	return &sriovManager{
		nLink: &MyNetlink{},
		utils: &pciUtilsImpl{},
	}
}

// SetupVF sets up a VF in Pod netns returns first interface's MAC addres as string
func (s *sriovManager) SetupVF(conf *sriovtypes.NetConf, podifName string, cid string, netns ns.NetNS) (string, error) {
	var macAddress string
	vfLinks := conf.HostIFNames

	// Sort links name if there are 2 or more PF links found for a VF;
	if len(vfLinks) > 1 {
		// sort Links FileInfo by their Link indices
		sort.Sort(&LinkByIndexSorter{linkNames: vfLinks, nLink: s.nLink})
	}

	for i := 0; i < len(vfLinks); i++ {

		ifName := podifName
		if i > 0 {
			ifName = podifName + fmt.Sprintf("d%d", i)
		}

		linkName := vfLinks[i]
		linkObj, err := s.nLink.LinkByName(linkName)
		if err != nil {
			fmt.Errorf("error getting VF netdevice with name %s", linkName)
		}

		// tempName used as intermediary name to avoid name conflicts
		tempName := fmt.Sprintf("%s%d", linkName, linkObj.Attrs().Index)
		// tempName := "tempname"

		// 1. Set link down
		if err := s.nLink.LinkSetDown(linkObj); err != nil {
			return "", fmt.Errorf("failed to down vf device %q: %v", linkName, err)
		}

		// 2. Set temp name
		if err := s.nLink.LinkSetName(linkObj, tempName); err != nil {
			return "", fmt.Errorf("error setting temp IF name %s for %s", tempName, linkName)
		}

		// 3. Change netns
		if err := s.nLink.LinkSetNsFd(linkObj, int(netns.Fd())); err != nil {
			return "", fmt.Errorf("failed to move IF %s to netns: %q", tempName, err)
		}

		// 4. Set Pod IF name
		if err := netns.Do(func(_ ns.NetNS) error {
			if err := s.nLink.LinkSetName(linkObj, ifName); err != nil {
				return fmt.Errorf("error setting container interface name %s for %s", linkName, tempName)
			}

			// 5. Bring IF up in Pod netns
			if err := s.nLink.LinkSetUp(linkObj); err != nil {
				return fmt.Errorf("error bringing interface up in container ns: %q", err)
			}
			// Only adding one mac address
			if macAddress == "" {
				macAddress = linkObj.Attrs().HardwareAddr.String()
			}
			return nil
		}); err != nil {
			return "", fmt.Errorf("error setting up interface in container namespace: %q", err)
		}
		conf.ContIFNames = append(conf.ContIFNames, ifName)
	}

	return macAddress, nil
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (s *sriovManager) ReleaseVF(conf *sriovtypes.NetConf, podifName string, cid string, netns ns.NetNS) error {

	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if len(conf.ContIFNames) < 1 && len(conf.ContIFNames) != len(conf.HostIFNames) {
		return fmt.Errorf("number of interface names mismatch ContIFNames: %d HostIFNames: %d", len(conf.ContIFNames), len(conf.HostIFNames))
	}

	return netns.Do(func(_ ns.NetNS) error {
		for i, ifName := range conf.ContIFNames {
			hostIFName := conf.HostIFNames[i]

			// get VF device
			linkObj, err := s.nLink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("failed to get netlink device with name %s: %q", ifName, err)
			}

			// shutdown VF device
			if err = s.nLink.LinkSetDown(linkObj); err != nil {
				return fmt.Errorf("failed to set link %s down: %q", ifName, err)
			}

			// rename VF device
			err = s.nLink.LinkSetName(linkObj, hostIFName)
			if err != nil {
				return fmt.Errorf("failed to rename link %s to host name %s: %q", ifName, hostIFName, err)
			}

			// move VF device to init netns
			if err = s.nLink.LinkSetNsFd(linkObj, int(initns.Fd())); err != nil {
				return fmt.Errorf("failed to move interface %s to init netns: %v", hostIFName, err)
			}
		}
		return nil
	})
}

func (s *sriovManager) resetVfVlan(pfName, vfName string) error {

	// get the ifname sriov vf num
	vfTotal, err := utils.GetSriovNumVfs(pfName)
	if err != nil {
		return err
	}

	if vfTotal <= 0 {
		return fmt.Errorf("no virtual function in the device: %v", pfName)
	}

	// Get VF id
	var vf int
	idFound := false
	for vf = 0; vf < vfTotal; vf++ {
		vfDir := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/net/%s", pfName, vf, vfName)
		if _, err := os.Stat(vfDir); !os.IsNotExist(err) {
			idFound = true
			break
		}
	}

	if !idFound {
		return fmt.Errorf("failed to get VF id for %s", vfName)
	}

	pfLink, err := s.nLink.LinkByName(pfName)
	if err != nil {
		return fmt.Errorf("master device %s not found", pfName)
	}

	if err = s.nLink.LinkSetVfVlan(pfLink, vf, 0); err != nil {
		return fmt.Errorf("failed to reset vlan tag for vf %d: %v", vf, err)
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
	if conf.Vlan != 0 {
		if err = s.nLink.LinkSetVfVlan(pfLink, conf.VFID, conf.Vlan); err != nil {
			return fmt.Errorf("failed to set vf %d vlan: %v", conf.VFID, err)
		}
	}

	// 2. Set mac address

	// 3. Set link rate

	return nil
}

// ResetVFConfig reset a VF with default values
func (s *sriovManager) ResetVFConfig(conf *sriovtypes.NetConf) error {

	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Set vlan to 0
	if err = s.nLink.LinkSetVfVlan(pfLink, conf.VFID, 0); err != nil {
		return fmt.Errorf("failed to set vf %d vlan: %v", conf.VFID, err)
	}
	return nil
}
