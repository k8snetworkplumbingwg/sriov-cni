package sriov

import (
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/sriov/mocks"
	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
)

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

var _ = Describe("Sriov", func() {
	var (
		t GinkgoTInterface
	)
	BeforeEach(func() {
		t = GinkgoT()
	})

	Context("Checking SetupVF function", func() {
		var (
			podifName string
			netconf   *sriovtypes.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				},
			}
			t = GinkgoT()
		})

		It("Assuming existing interface", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")

			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			mocked.On("LinkSetVfVlan", mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetVfVlanQos", mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).Return(nil)
			mockedPciUtils.On("EnableArpAndNdiscNotify", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			macAddr, err := sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal("6e:16:06:0e:b7:e9"))
		})
		It("Setting VF's MAC address", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())

			netconf.MAC = "e4:11:22:33:44:55"
			expMac, err := net.ParseMAC(netconf.MAC)
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetHardwareAddr", fakeLink, expMac).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			mockedPciUtils.On("EnableArpAndNdiscNotify", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			macAddr, err := sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal(netconf.MAC))
			mocked.AssertExpectations(t)
		})
	})

	Context("Checking ReleaseVF function", func() {
		var (
			podifName string
			netconf   *sriovtypes.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Assuming existing interface", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			sm := sriovManager{nLink: mocked}
			err = sm.ReleaseVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ReleaseVF function - restore config", func() {
		var (
			podifName string
			netconf   *sriovtypes.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				MAC:         "aa:f3:8d:65:1b:d4",
				ContIFNames: "net1",
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "c6:c8:7f:1f:21:90",
				},
			}
		})
		It("Restores Effective MAC address when provided in netconf", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			origEffMac, err := net.ParseMAC(netconf.OrigVfState.EffectiveMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetHardwareAddr", fakeLink, origEffMac).Return(nil)
			sm := sriovManager{nLink: mocked}
			err = sm.ReleaseVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking FillOriginalVfInfo function", func() {
		var (
			netconf *sriovtypes.NetConf
		)

		BeforeEach(func() {
			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Saves the current VF state", func() {
			mocked := &mocks_utils.NetlinkManager{}
			//fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
				Vfs: []netlink.VfInfo{
					{
						ID:  0,
						Mac: net.HardwareAddr(fakeMac),
					},
				},
			}}
			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			sm := sriovManager{nLink: mocked}
			err = sm.FillOriginalVfInfo(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ResetVFConfig function - restore config no user params", func() {
		var (
			netconf *sriovtypes.NetConf
		)

		BeforeEach(func() {
			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Does not change VF config if it wasnt requested to be changed in netconf", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			sm := sriovManager{nLink: mocked}
			err := sm.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ResetVFConfig function - restore config with user params", func() {
		var (
			netconf *sriovtypes.NetConf
		)

		BeforeEach(func() {
			vlan := 6
			vlanQos := 3
			maxTxRate := 4000
			minTxRate := 1000

			netconf = &sriovtypes.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        3,
				ContIFNames: "net1",
				MAC:         "d2:fc:22:a7:0d:e8",
				Vlan:        &vlan,
				VlanQoS:     &vlanQos,
				SpoofChk:    "on",
				MaxTxRate:   &maxTxRate,
				MinTxRate:   &minTxRate,
				Trust:       "on",
				LinkState:   "enable",
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					SpoofChk:     false,
					AdminMAC:     "aa:f3:8d:65:1b:d4",
					EffectiveMAC: "aa:f3:8d:65:1b:d4",
					Vlan:         1,
					VlanQoS:      1,
					MinTxRate:    0,
					MaxTxRate:    0,
					LinkState:    2, // disable
				},
			}
		})
		It("Restores original VF configurations", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQos", fakeLink, netconf.VFID, netconf.OrigVfState.Vlan, netconf.OrigVfState.VlanQoS).Return(nil)
			mocked.On("LinkSetVfSpoofchk", fakeLink, netconf.VFID, netconf.OrigVfState.SpoofChk).Return(nil)
			origMac, err := net.ParseMAC(netconf.OrigVfState.AdminMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetVfHardwareAddr", fakeLink, netconf.VFID, origMac).Return(nil)
			mocked.On("LinkSetVfTrust", fakeLink, netconf.VFID, false).Return(nil)
			mocked.On("LinkSetVfRate", fakeLink, netconf.VFID, netconf.OrigVfState.MinTxRate, netconf.OrigVfState.MaxTxRate).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, netconf.VFID, netconf.OrigVfState.LinkState).Return(nil)

			sm := sriovManager{nLink: mocked}
			err = sm.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
})
