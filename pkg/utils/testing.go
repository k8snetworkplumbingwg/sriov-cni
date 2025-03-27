/*
	This file contains test helper functions to mock linux sysfs directory.
	If a package need to access system sysfs it should call CreateTmpSysFs() before test
	then call RemoveTmpSysFs() once test is done for clean up.
*/

package utils

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/vishvananda/netlink"
)

type tmpSysFs struct {
	dirRoot      string
	dirList      []string
	fileList     map[string][]byte
	netSymlinks  map[string]string
	devSymlinks  map[string]string
	vfSymlinks   map[string]string
	originalRoot *os.File
}

var ts = tmpSysFs{
	dirList: []string{
		"sys/class/net",
		"sys/bus/pci/devices",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/net/enp175s0f1",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/net/enp175s6",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/net/enp175s7",
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1",
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1d1",
	},
	fileList: map[string][]byte{
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/sriov_numvfs": []byte("2"),
		"sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/sriov_numvfs": []byte("0"),
	},
	netSymlinks: map[string]string{
		"sys/class/net/enp175s0f1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/net/enp175s0f1",
		"sys/class/net/enp175s6":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/net/enp175s6",
		"sys/class/net/enp175s7":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/net/enp175s7",
		"sys/class/net/ens1":       "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1",
		"sys/class/net/ens1d1":     "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0/net/ens1d1",
	},
	devSymlinks: map[string]string{
		"sys/class/net/enp175s0f1/device": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
		"sys/class/net/enp175s6/device":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/class/net/enp175s7/device":   "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/class/net/ens1/device":       "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",
		"sys/class/net/ens1d1/device":     "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",

		"sys/bus/pci/devices/0000:af:00.1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
		"sys/bus/pci/devices/0000:af:06.0": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/bus/pci/devices/0000:af:06.1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/bus/pci/devices/0000:05:00.0": "sys/devices/pci0000:00/0000:00:02.0/0000:05:00.0",
	},
	vfSymlinks: map[string]string{
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/virtfn0": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.0/physfn":  "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",

		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/virtfn1": "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1",
		"sys/devices/pci0000:ae/0000:ae:00.0/0000:af:06.1/physfn":  "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1",
	},
}

// CreateTmpSysFs create mock sysfs for testing
func CreateTmpSysFs() error {
	originalRoot, _ := os.Open("/")
	ts.originalRoot = originalRoot

	tmpdir, err := os.MkdirTemp("/tmp", "sriovplugin-testfiles-")
	if err != nil {
		return err
	}

	ts.dirRoot = tmpdir
	//syscall.Chroot(ts.dirRoot)

	for _, dir := range ts.dirList {
		if err := os.MkdirAll(filepath.Join(ts.dirRoot, dir), 0755); err != nil {
			return err
		}
	}

	for filename, body := range ts.fileList {
		if err := os.WriteFile(filepath.Join(ts.dirRoot, filename), body, 0600); err != nil {
			return err
		}
	}

	for link, target := range ts.netSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	for link, target := range ts.devSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	for link, target := range ts.vfSymlinks {
		if err := createSymlinks(filepath.Join(ts.dirRoot, link), filepath.Join(ts.dirRoot, target)); err != nil {
			return err
		}
	}

	SysBusPci = filepath.Join(ts.dirRoot, SysBusPci)
	NetDirectory = filepath.Join(ts.dirRoot, NetDirectory)
	return nil
}

func createSymlinks(link, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	return os.Symlink(target, link)
}

// RemoveTmpSysFs removes mocked sysfs
func RemoveTmpSysFs() error {
	err := ts.originalRoot.Chdir()
	if err != nil {
		return err
	}
	if err = syscall.Chroot("."); err != nil {
		return err
	}
	if err = ts.originalRoot.Close(); err != nil {
		return err
	}

	return os.RemoveAll(ts.dirRoot)
}

// FakeLink is a dummy netlink struct used during testing
type FakeLink struct {
	netlink.LinkAttrs
}

// type FakeLink struct {
// 	linkAtrrs *netlink.LinkAttrs
// }

func (l *FakeLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeLink) Type() string {
	return "FakeLink"
}


func MockNetlinkLib(methodCallRecordingDir string) (func(), error) {
	var err error
	oldnetlinkLib := netLinkLib
	// see `ts` variable in this file
	// "sys/devices/pci0000:ae/0000:ae:00.0/0000:af:00.1/sriov_numvfs": []byte("2"),
	netLinkLib, err = newPFMockNetlinkLib(methodCallRecordingDir, "enp175s0f1", 2)

	return func() {
		netLinkLib = oldnetlinkLib
	}, err
}

// pfMockNetlinkLib creates dummy interfaces for Physical and Virtual functions, recording method calls on a log file in the form
// <method_name> <arg1> <arg2> ...
type pfMockNetlinkLib struct {
	pf                           netlink.Link
	methodCallsRecordingFilePath string
}

func newPFMockNetlinkLib(recordDir, pfName string, numvfs int) (*pfMockNetlinkLib, error) {
	ret := &pfMockNetlinkLib{
		pf: &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name: pfName,
				Vfs: []netlink.VfInfo{},
			},
		},
	}

	for i := 0; i<numvfs; i++ {
		ret.pf.Attrs().Vfs = append(ret.pf.Attrs().Vfs, netlink.VfInfo{
			ID: i,
			Mac: mustParseMAC(fmt.Sprintf("ab:cd:ef:ab:cd:%02x", i)),
		})
	}

	ret.methodCallsRecordingFilePath = filepath.Join(recordDir, pfName+".calls")

	ret.recordMethodCall("---")

	return ret, nil
}

func (p *pfMockNetlinkLib) LinkByName(name string) (netlink.Link, error) {
	p.recordMethodCall("LinkByName %s", name)
	if name == p.pf.Attrs().Name {
		return p.pf, nil
	}
	return netlink.LinkByName(name)
}

func (p *pfMockNetlinkLib) LinkSetVfVlanQosProto(link netlink.Link, vfIndex int, vlan int, vlanQos int, vlanProto int) error {
	p.recordMethodCall("LinkSetVfVlanQosProto %s %d %d %d %d", link.Attrs().Name, vfIndex, vlan, vlanQos, vlanProto)
	return nil
}

func (p *pfMockNetlinkLib) LinkSetVfHardwareAddr(pfLink netlink.Link, vfIndex int, hwaddr net.HardwareAddr) error {
	p.recordMethodCall("LinkSetVfHardwareAddr %s %d %s", pfLink.Attrs().Name, vfIndex, hwaddr.String())
	pfLink.Attrs().Vfs[vfIndex].Mac = hwaddr
	return nil
}

func (p *pfMockNetlinkLib) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	p.recordMethodCall("LinkSetHardwareAddr %s %s", link.Attrs().Name, hwaddr.String())
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

func (p *pfMockNetlinkLib) LinkSetUp(link netlink.Link) error {
	p.recordMethodCall("LinkSetUp %s", link.Attrs().Name)
	return netlink.LinkSetUp(link)
}

func (p *pfMockNetlinkLib) LinkSetDown(link netlink.Link) error {
	p.recordMethodCall("LinkSetDown %s", link.Attrs().Name)
	return netlink.LinkSetDown(link)
}

func (p *pfMockNetlinkLib) LinkSetNsFd(link netlink.Link, nsFd int) error {
	p.recordMethodCall("LinkSetNsFd %s %d", link.Attrs().Name, nsFd)
	return netlink.LinkSetNsFd(link, nsFd)
}

func (p *pfMockNetlinkLib) LinkSetName(link netlink.Link, name string) error {
	p.recordMethodCall("LinkSetName %s %s", link.Attrs().Name, name)
	link.Attrs().Name = name
	return netlink.LinkSetName(link, name)
}

func (p *pfMockNetlinkLib) LinkSetVfRate(pfLink netlink.Link, vfIndex int, minRate int, maxRate int) error {
	p.recordMethodCall("LinkSetVfRate %s %d %d %d", pfLink.Attrs().Name, vfIndex, minRate, maxRate)
	pfLink.Attrs().Vfs[vfIndex].MaxTxRate = uint32(maxRate)
	pfLink.Attrs().Vfs[vfIndex].MinTxRate = uint32(minRate)
	return nil
}

func (p *pfMockNetlinkLib) LinkSetVfSpoofchk(pfLink netlink.Link, vfIndex int, spoofChk bool) error {
	p.recordMethodCall("LinkSetVfRate %s %d %t", pfLink.Attrs().Name, vfIndex, spoofChk)
	pfLink.Attrs().Vfs[vfIndex].Spoofchk = spoofChk
	return nil
}

func (p *pfMockNetlinkLib) LinkSetVfTrust(pfLink netlink.Link, vfIndex int, trust bool) error {
	p.recordMethodCall("LinkSetVfTrust %s %d %d", pfLink.Attrs().Name, vfIndex, trust)
	if trust {
		pfLink.Attrs().Vfs[vfIndex].Trust = 1
	} else {
		pfLink.Attrs().Vfs[vfIndex].Trust = 0
	}

	return nil
}

func (p *pfMockNetlinkLib) LinkSetVfState(pfLink netlink.Link, vfIndex int, state uint32) error {
	p.recordMethodCall("LinkSetVfState %s %d %d", pfLink.Attrs().Name, vfIndex, state)
	pfLink.Attrs().Vfs[vfIndex].LinkState = state
	return nil
}

func (p *pfMockNetlinkLib) LinkSetMTU(link netlink.Link, mtu int) error {
	p.recordMethodCall("LinkSetMTU %s %d", link.Attrs().Name, mtu)
	return netlink.LinkSetMTU(link, mtu)
}


func (p *pfMockNetlinkLib) LinkDelAltName(link netlink.Link, name string) error {
	p.recordMethodCall("LinkDelAltName %s %s", link.Attrs().Name, name)
	return netlink.LinkDelAltName(link, name)
}

func (p *pfMockNetlinkLib) recordMethodCall(format string, a ...any) {
	f, err := os.OpenFile(p.methodCallsRecordingFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf(format+"\n", a...)); err != nil {
		log.Println(err)
	}
}

func mustParseMAC(x string) net.HardwareAddr {
	ret, err := net.ParseMAC(x)
	if err != nil {
		panic(err)
	}
	return ret
}
