package utils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"

	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

var (
	arpPacketName    = "ARP"
	icmpV6PacketName = "ICMPv6"
)

// htons converts an uint16 from host to network byte order.
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

// formatPacketFieldWriteError builds an error string for the cases when writing to a field of a packet fails.
func formatPacketFieldWriteError(field string, packetType string, writeErr error) error {
	return fmt.Errorf("failed to write the %s field in the %s packet: %v", field, packetType, writeErr)
}

// SendGratuitousArp sends a gratuitous ARP packet with the provided source IP over the provided interface.
func SendGratuitousArp(srcIP net.IP, linkObj netlink.Link) error {
	/* As per RFC 5944 section 4.6, a gratuitous ARP packet can be sent by a node in order to spontaneously cause other nodes to update
	 * an entry in their ARP cache. In the case of SRIOV-CNI, an address can be reused for different pods. Each pod could likely have a
	 * different link-layer address in this scenario, which makes the ARP cache entries residing in the other nodes to be an invalid.
	 * The gratuitous ARP packet should update the link-layer address accordingly for the invalid ARP cache.
	 */

	// Construct the ARP packet following RFC 5944 section 4.6.
	arpPacket := new(bytes.Buffer)
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(1)); writeErr != nil { // Hardware Type: 1 is Ethernet
		return formatPacketFieldWriteError("Hardware Type", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(syscall.ETH_P_IP)); writeErr != nil { // Protocol Type: 0x0800 is IPv4
		return formatPacketFieldWriteError("Protocol Type", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint8(6)); writeErr != nil { // Hardware address Length: 6 bytes for MAC address
		return formatPacketFieldWriteError("Hardware address Length", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint8(4)); writeErr != nil { // Protocol address length: 4 bytes for IPv4 address
		return formatPacketFieldWriteError("Protocol address length", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(1)); writeErr != nil { // Operation: 1 is request, 2 is response
		return formatPacketFieldWriteError("Operation", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(linkObj.Attrs().HardwareAddr); writeErr != nil { // Sender hardware address
		return formatPacketFieldWriteError("Sender hardware address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(srcIP.To4()); writeErr != nil { // Sender protocol address
		return formatPacketFieldWriteError("Sender protocol address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); writeErr != nil { // Target hardware address is the Broadcast MAC.
		return formatPacketFieldWriteError("Target hardware address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(srcIP.To4()); writeErr != nil { // Target protocol address
		return formatPacketFieldWriteError("Target protocol address", arpPacketName, writeErr)
	}

	sockAddr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),                                // Ethertype of ARP (0x0806)
		Ifindex:  linkObj.Attrs().Index,                                   // Interface Index
		Hatype:   1,                                                       // Hardware Type: 1 is Ethernet
		Pkttype:  0,                                                       // Packet Type.
		Halen:    6,                                                       // Hardware address Length: 6 bytes for MAC address
		Addr:     [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // Address is the broadcast MAC address.
	}

	// Create a socket such that the Ethernet header would constructed by the OS. The arpPacket only contains the ARP payload.
	soc, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("failed to create AF_PACKET datagram socket: %v", err)
	}
	defer syscall.Close(soc)

	if err := syscall.Sendto(soc, arpPacket.Bytes(), 0, &sockAddr); err != nil {
		return fmt.Errorf("failed to send Gratuitous ARP for IPv4 %s on Interface %s: %v", srcIP.String(), linkObj.Attrs().Name, err)
	}

	return nil
}

// SendUnsolicitedNeighborAdvertisement sends an unsolicited neighbor advertisement packet with the provided source IP over the provided interface.
func SendUnsolicitedNeighborAdvertisement(srcIP net.IP, linkObj netlink.Link) error {
	/* As per RFC 4861, a link-layer address change can multicast a few unsolicited neighbor advertisements to all nodes to quickly
	 * update the cached link-layer addresses that have become invalid. In the case of SRIOV-CNI, an address can be reused for
	 * different pods. Each pod could likely have a different link-layer address in this scenario, which makes the Neighbor Cache
	 * entries residing in the neighbors to be an invalid. The unsolicited neighbor advertisement should update the link-layer address
	 * accordingly for the IPv6 entry.
	 * However if any of these conditions are true:
	 *  - The IPv6 address was not reused for the new pod.
	 *  - No prior established communication with the neighbor.
	 * Then the neighbor receiving this unsolicited neighbor advertisement would be silently discard. This behavior is described
	 * in RFC 4861 section 7.2.5. This is acceptable behavior since the purpose of sending an unsolicited neighbor advertisement
	 * is not to create a new entry but rather update already existing invalid entries.
	 */

	// Construct the ICMPv6 Neighbor Advertisement packet following RFC 4861.
	payload := new(bytes.Buffer)
	// ICMPv6 Flags: As per RFC 4861, the solicited flag must not be set and the override flag should be set (to
	// override existing cache entry) for unsolicited advertisements.
	if writeErr := binary.Write(payload, binary.BigEndian, uint32(0x20000000)); writeErr != nil {
		return formatPacketFieldWriteError("Flags", icmpV6PacketName, writeErr)
	}
	if _, writeErr := payload.Write(srcIP.To16()); writeErr != nil { // ICMPv6 Target IPv6 Address.
		return formatPacketFieldWriteError("Target IPv6 Address", icmpV6PacketName, writeErr)
	}
	if writeErr := binary.Write(payload, binary.BigEndian, uint8(2)); writeErr != nil { // ICMPv6 Option Type: 2 is target link-layer address.
		return formatPacketFieldWriteError("Option Type", icmpV6PacketName, writeErr)
	}
	if writeErr := binary.Write(payload, binary.BigEndian, uint8(1)); writeErr != nil { // ICMPv6 Option Length. Units of 8 bytes.
		return formatPacketFieldWriteError("Option Length", icmpV6PacketName, writeErr)
	}
	if _, writeErr := payload.Write(linkObj.Attrs().HardwareAddr); writeErr != nil { // ICMPv6 Option Link-layer Address.
		return formatPacketFieldWriteError("Option Link-layer Address", icmpV6PacketName, writeErr)
	}

	icmpv6Msg := icmp.Message{
		Type:     ipv6.ICMPTypeNeighborAdvertisement, // ICMPv6 type is neighbor advertisement.
		Code:     0,                                  // ICMPv6 Code: As per RFC 4861 section 7.1.2, the code is always 0.
		Checksum: 0,                                  // Checksum is calculated later.
		Body: &icmp.RawBody{
			Data: payload.Bytes(),
		},
	}

	// Get the byte array of the ICMPv6 Message.
	icmpv6Bytes, err := icmpv6Msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to Marshal ICMPv6 Message: %v", err)
	}

	// Create a socket such that the Ethernet header and IPv6 header would constructed by the OS.
	soc, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return fmt.Errorf("failed to create AF_INET6 raw socket: %v", err)
	}
	defer syscall.Close(soc)

	// As per RFC 4861 section 7.1.2, the IPv6 hop limit is always 255.
	if err := syscall.SetsockoptInt(soc, syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_HOPS, 255); err != nil {
		return fmt.Errorf("failed to set IPv6 multicast hops to 255: %v", err)
	}

	// Set the destination IPv6 address to the IPv6 link-local all nodes multicast address (ff02::1).
	var r [16]byte
	copy(r[:], net.IPv6linklocalallnodes.To16())
	sockAddr := syscall.SockaddrInet6{Addr: r}
	if err := syscall.Sendto(soc, icmpv6Bytes, 0, &sockAddr); err != nil {
		return fmt.Errorf("failed to send Unsolicited Neighbor Advertisement for IPv6 %s on Interface %s: %v", srcIP.String(), linkObj.Attrs().Name, err)
	}

	return nil
}

// AnnounceIPs sends either a GARP or Unsolicited NA depending on the IP address type (IPv4 vs. IPv6 respectively) configured on the interface.
func AnnounceIPs(ifName string, ipConfigs []*current.IPConfig) error {
	myNetLink := MyNetlink{}

	// Retrieve the interface name in the container.
	linkObj, err := myNetLink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to get netlink device with name %q: %v", ifName, err)
	}
	if !IsValidMACAddress(linkObj.Attrs().HardwareAddr) {
		return fmt.Errorf("invalid Ethernet MAC address: %q", linkObj.Attrs().HardwareAddr)
	}

	// For all the IP addresses assigned by IPAM, we will send either a GARP (IPv4) or Unsolicited NA (IPv6).
	for _, ipc := range ipConfigs {
		var err error
		if IsIPv6(ipc.Address.IP) {
			/* As per RFC 4861, sending unsolicited neighbor advertisements should be considered as a performance
			* optimization. It does not reliably update caches in all nodes. The Neighbor Unreachability Detection
			* algorithm is more reliable although it may take slightly longer to update.
			 */
			err = SendUnsolicitedNeighborAdvertisement(ipc.Address.IP, linkObj)
		} else if IsIPv4(ipc.Address.IP) {
			err = SendGratuitousArp(ipc.Address.IP, linkObj)
		} else {
			return fmt.Errorf("the IP %s on interface %q is neither IPv4 or IPv6", ipc.Address.IP.String(), ifName)
		}

		if err != nil {
			return fmt.Errorf("failed to send GARP/NA message for ip %s on interface %q: %v", ipc.Address.IP.String(), ifName, err)
		}
	}
	return nil
}
