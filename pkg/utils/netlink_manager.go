package utils

import (
	"net"

	"github.com/vishvananda/netlink"
)

// Mocked netlink interface, this is required for unit tests

// NetlinkManager is an interface to mock nelink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetVfVlan(netlink.Link, int, int) error
	LinkSetVfVlanQos(netlink.Link, int, int, int) error
	LinkSetVfHardwareAddr(netlink.Link, int, net.HardwareAddr) error
	LinkSetHardwareAddr(netlink.Link, net.HardwareAddr) error
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
	LinkSetVfRate(netlink.Link, int, int, int) error
	LinkSetVfSpoofchk(netlink.Link, int, bool) error
	LinkSetVfTrust(netlink.Link, int, bool) error
	LinkSetVfState(netlink.Link, int, uint32) error
}

// MyNetlink NetlinkManager
type MyNetlink struct {
	NetlinkManager
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetVfVlan using NetlinkManager
func (n *MyNetlink) LinkSetVfVlan(link netlink.Link, vf, vlan int) error {
	return netlink.LinkSetVfVlan(link, vf, vlan)
}

// LinkSetVfVlanQos sets VLAN ID and QoS field for given VF using NetlinkManager
func (n *MyNetlink) LinkSetVfVlanQos(link netlink.Link, vf, vlan, qos int) error {
	return netlink.LinkSetVfVlanQos(link, vf, vlan, qos)
}

// LinkSetVfHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetVfHardwareAddr(link netlink.Link, vf int, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetVfHardwareAddr(link, vf, hwaddr)
}

// LinkSetHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
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

// LinkSetVfRate using NetlinkManager
func (n *MyNetlink) LinkSetVfRate(link netlink.Link, vf int, minRate int, maxRate int) error {
	return netlink.LinkSetVfRate(link, vf, minRate, maxRate)
}

// LinkSetVfSpoofchk using NetlinkManager
func (n *MyNetlink) LinkSetVfSpoofchk(link netlink.Link, vf int, check bool) error {
	return netlink.LinkSetVfSpoofchk(link, vf, check)
}

// LinkSetVfTrust using NetlinkManager
func (n *MyNetlink) LinkSetVfTrust(link netlink.Link, vf int, state bool) error {
	return netlink.LinkSetVfTrust(link, vf, state)
}

// LinkSetVfState using NetlinkManager
func (n *MyNetlink) LinkSetVfState(link netlink.Link, vf int, state uint32) error {
	return netlink.LinkSetVfState(link, vf, state)
}
