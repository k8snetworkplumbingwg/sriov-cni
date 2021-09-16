package net

import (
	"fmt"
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/k8snetworkplumbingwg/sriov-cni/test/util/ethtool"
)

const (
	// LinkStateAuto - Link state auto
	LinkStateAuto = 0
	// LinkStateEnable - Link state enable
	LinkStateEnable = 1
	// LinkStateDisable - Link state disable
	LinkStateDisable = 2
)

// SetVfLinkState set VF link state
// state 0 - auto (linkStateAuto), 1 - enable (linkStateEnable), 2 - disable (linkStateDisable) - defined as constants
func SetVfLinkState(pfName, containerNs string, vf int, state uint32) error {
	// Get container network namespace from path
	ns, err := netns.GetFromPath(containerNs)
	if nil != err {
		return err
	}
	defer ns.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hostNetNs, err := netns.Get() // host network namespace
	if err != nil {
		return err
	}

	err = netns.Set(ns) // switch netns to container namespace
	if err != nil {
		return err
	}

	link, err := netlink.LinkByName(pfName)
	if nil != err {
		return err
	}

	err = netlink.LinkSetVfState(link, vf, state)
	if nil != err {
		return err
	}

	// Return to the host network namespace
	err = netns.Set(hostNetNs)
	if err != nil {
		return err
	}

	return nil
}

// SetAllVfOnLinkState sets all VFs of given PF to selected state
func SetAllVfOnLinkState(pfName, containerNs string, state uint32) error {
	links, err := GetVfsLinksInfoList(pfName, containerNs)

	if err != nil {
		return err
	}

	for vfIdx := 0; vfIdx < len(links); vfIdx++ {
		err = SetVfLinkState(pfName, containerNs, vfIdx, state)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetVfsMAC set MACs on VFs to defined state starting from 4a:ea:39:09:4e:XX
// NOTE: Loop will generate only last two positions
func SetVfsMAC(pfName, containerNs string) error {
	// Get container network namespace from path
	ns, err := netns.GetFromPath(containerNs)
	if nil != err {
		return err
	}
	defer ns.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hostNetNs, err := netns.Get() // host network namespace
	if err != nil {
		return err
	}
	err = netns.Set(ns) // switch netns to container namespace
	if err != nil {
		return err
	}

	link, err := netlink.LinkByName(pfName)
	if nil != err {
		return err
	}

	var baseMac string = "4a:ea:39:09:4e:"
	for index := 0; index < len(link.Attrs().Vfs); index++ {
		var mac string
		if index < 17 {
			mac = baseMac + fmt.Sprintf("0%x", index)
		} else {
			mac = baseMac + fmt.Sprintf("%x", index)
		}

		hwMac, err := net.ParseMAC(mac)
		if err != nil {
			return err
		}

		err = netlink.LinkSetVfHardwareAddr(link, index, hwMac)
		if nil != err {
			return err
		}
	}

	// Return to the host network namespace
	err = netns.Set(hostNetNs)
	if nil != err {
		return err
	}

	return nil
}

// GetVfsLinksInfoList - returns list with VfInfo structure
func GetVfsLinksInfoList(pfName, containerNs string) ([]netlink.VfInfo, error) {
	// Get container network namespace from path
	ns, err := netns.GetFromPath(containerNs)
	if nil != err {
		return nil, err
	}
	defer ns.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hostNetNs, _ := netns.Get() // host network namespace
	err = netns.Set(ns)         // switch netns to container namespace
	if nil != err {
		return nil, err
	}

	link, err := netlink.LinkByName(pfName)
	if nil != err {
		return nil, err
	}

	// Return to the host network namespace
	err = netns.Set(hostNetNs)
	if nil != err {
		return nil, err
	}

	return link.Attrs().Vfs, nil
}

// GetHostLinkList - returns all links from host network namespace
func GetHostLinkList() ([]netlink.Link, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	return links, nil
}

// MoveLinksToDocker - move links from host to specified network namespace, for the diff of two links slices
// In result, this function allows to move back links to KinD namespace. Sometimes, after test links are moved from Kind to host namespace.
func MoveLinksToDocker(containerNs string, linksBefore, linksAfter []netlink.Link) error {
	tempMap := make(map[string]struct{}, len(linksAfter))
	for _, link := range linksBefore {
		tempMap[link.Attrs().Name] = struct{}{}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	targetNetNs, err := netns.GetFromPath(containerNs)
	if err != nil {
		return err
	}

	defer targetNetNs.Close()

	for _, link := range linksAfter {
		if _, found := tempMap[link.Attrs().Name]; !found {
			// "ip link set {{ item }} netns {{ kindNet }}"
			err = netlink.LinkSetNsFd(link, int(targetNetNs))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SetInterfaceNamespace - set the interface namespace
func SetInterfaceNamespace(containerNs, pfName string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Get container network namespace from path
	ns, err := netns.GetFromPath(containerNs)
	if nil != err {
		return err
	}
	defer ns.Close()

	hostNetNs, _ := netns.Get() // host network namespace
	err = netns.Set(ns)         // switch netns to container namespace
	if nil != err {
		return err
	}

	mainLink, err := netlink.LinkByName(pfName)
	if err != nil {
		return err
	}

	allHostLinks, err := netlink.LinkList()
	if err != nil {
		return err
	}

	for _, link := range allHostLinks {
		// for each link check if it is a VF based on MAC, and if true move to Docker network namespace
		for _, vf := range mainLink.Attrs().Vfs {
			if link.Attrs().HardwareAddr.String() == vf.Mac.String() {
				// execute "ip link set {{ item }} netns {{ kindNet }}" and go to next link
				err = netlink.LinkSetNsFd(link, int(hostNetNs))
				if err != nil {
					return err
				}
				break
			}
		}
	}

	// at the end move main link
	err = netlink.LinkSetNsFd(mainLink, int(hostNetNs))
	if err != nil {
		return err
	}

	// Return to the host network namespace
	err = netns.Set(hostNetNs)
	if nil != err {
		return err
	}

	return nil
}

// SetTestInterfaceNetworkNamespace move PF with all VFs to the specified network namespace
func SetTestInterfaceNetworkNamespace(containerNs, pfName string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	targetNetNs, err := netns.GetFromPath(containerNs)
	if err != nil {
		return err
	}

	defer targetNetNs.Close()

	mainLink, err := netlink.LinkByName(pfName)
	if err != nil {
		return err
	}

	allHostLinks, err := netlink.LinkList()
	if err != nil {
		return err
	}

	for _, link := range allHostLinks {
		// for each link check if it is a VF based on MAC, and if true move to Docker network namespace
		for _, vf := range mainLink.Attrs().Vfs {
			if link.Attrs().HardwareAddr.String() == vf.Mac.String() {
				// execute "ip link set {{ item }} netns {{ kindNet }}" and go to next link
				err = netlink.LinkSetNsFd(link, int(targetNetNs))
				if nil != err {
					return err
				}
				break
			}
		}
	}

	// at the end move main link
	err = netlink.LinkSetNsFd(mainLink, int(targetNetNs))
	if nil != err {
		return err
	}

	return nil
}

// VerifyDriverForVfs - verifies if all Vfs have expected driver set. If not returns error.
// :param pfName - Physical Interface name - if empty then ignore interface verification
func VerifyDriverForVfs(pfName string, expectedDriverNames []string) error {
	if pfName == "" {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mainLink, err := netlink.LinkByName(pfName)
	if err != nil {
		return err
	}

	allHostLinks, err := netlink.LinkList()
	if err != nil {
		return err
	}

	for _, link := range allHostLinks {
		// for each link check if it is a VF based on MAC, if true collect name
		for _, vf := range mainLink.Attrs().Vfs {
			if link.Attrs().HardwareAddr.String() == vf.Mac.String() {
				driverName, err := ethtool.GetDriverInformation(link.Attrs().Name)
				if err != nil {
					return err
				}

				var isFound bool
				for _, supportedDriver := range expectedDriverNames {
					if supportedDriver == driverName {
						isFound = true
						break
					}
				}

				if !isFound {
					return fmt.Errorf("driver not found within supported. Name '%s' for Vf '%s'", driverName, link.Attrs().Name)
				}

				break
			}
		}
	}

	return nil
}
