package utils

import (
	"net"

	"github.com/vishvananda/netlink"
)

// Mocked netlink interface, this is required for unit tests

// NetlinkManager is an interface to mock nelink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetVfVlanQosProto(netlink.Link, int, int, int, int) error
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
	LinkSetMTU(netlink.Link, int) error
	LinkDelAltName(netlink.Link, string) error
}

var netLinkLib NetlinkManager

func init() {
	handler, err := netlink.NewHandle()
	if err != nil {
		panic(err)
	}
	netLinkLib = &MyNetlink{handler: handler}
}

// MyNetlink NetlinkManager
type MyNetlink struct {
	NetlinkManager
	handler *netlink.Handle
}

func GetNetlinkManager() NetlinkManager {
	return netLinkLib
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return n.handler.LinkByName(name)
}

// LinkSetVfVlanQosProto sets VLAN ID, QoS and Proto field for given VF using NetlinkManager
func (n *MyNetlink) LinkSetVfVlanQosProto(link netlink.Link, vf, vlan, qos, proto int) error {
	return n.handler.LinkSetVfVlanQosProto(link, vf, vlan, qos, proto)
}

// LinkSetVfHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetVfHardwareAddr(link netlink.Link, vf int, hwaddr net.HardwareAddr) error {
	return n.handler.LinkSetVfHardwareAddr(link, vf, hwaddr)
}

// LinkSetHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return n.handler.LinkSetHardwareAddr(link, hwaddr)
}

// LinkSetUp using NetlinkManager
func (n *MyNetlink) LinkSetUp(link netlink.Link) error {
	return n.handler.LinkSetUp(link)
}

// LinkSetDown using NetlinkManager
func (n *MyNetlink) LinkSetDown(link netlink.Link) error {
	return n.handler.LinkSetDown(link)
}

// LinkSetNsFd using NetlinkManager
func (n *MyNetlink) LinkSetNsFd(link netlink.Link, fd int) error {
	return n.handler.LinkSetNsFd(link, fd)
}

// LinkSetName using NetlinkManager
func (n *MyNetlink) LinkSetName(link netlink.Link, name string) error {
	return n.handler.LinkSetName(link, name)
}

// LinkSetVfRate using NetlinkManager
func (n *MyNetlink) LinkSetVfRate(link netlink.Link, vf, minRate, maxRate int) error {
	return n.handler.LinkSetVfRate(link, vf, minRate, maxRate)
}

// LinkSetVfSpoofchk using NetlinkManager
func (n *MyNetlink) LinkSetVfSpoofchk(link netlink.Link, vf int, check bool) error {
	return n.handler.LinkSetVfSpoofchk(link, vf, check)
}

// LinkSetVfTrust using NetlinkManager
func (n *MyNetlink) LinkSetVfTrust(link netlink.Link, vf int, state bool) error {
	return n.handler.LinkSetVfTrust(link, vf, state)
}

// LinkSetVfState using NetlinkManager
func (n *MyNetlink) LinkSetVfState(link netlink.Link, vf int, state uint32) error {
	return n.handler.LinkSetVfState(link, vf, state)
}

// LinkSetMTU using NetlinkManager
func (n *MyNetlink) LinkSetMTU(link netlink.Link, mtu int) error {
	return n.handler.LinkSetMTU(link, mtu)
}

// LinkDelAltName using NetlinkManager
func (n *MyNetlink) LinkDelAltName(link netlink.Link, altName string) error {
	return n.handler.LinkDelAltName(link, altName)
}
