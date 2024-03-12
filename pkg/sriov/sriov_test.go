package sriov

import (
	"net"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"

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
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
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

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
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
			err = sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(netconf.OrigVfState.EffectiveMAC).To(Equal("6e:16:06:0e:b7:e9"))
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

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			net1Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "net1",
				HardwareAddr: expMac,
			}}

			net2Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "temp_1000",
				HardwareAddr: expMac,
			}}

			mocked.On("LinkByName", "enp175s6").Return(fakeLink, nil)
			mocked.On("LinkByName", "temp_1000").Return(net2Link, nil)
			mocked.On("LinkByName", "net1").Return(net1Link, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetName", net2Link, mock.Anything).Return(nil)
			mocked.On("LinkSetHardwareAddr", net1Link, expMac).Return(nil)
			mocked.On("LinkSetNsFd", net2Link, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", net2Link).Return(nil)
			mockedPciUtils.On("EnableArpAndNdiscNotify", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			err = sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
		It("Remove altName", func() {
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

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			net1Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "net1",
				HardwareAddr: expMac,
			}}

			net2Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "temp_1000",
				HardwareAddr: expMac,
				AltNames:     []string{"enp175s6"},
			}}

			mocked.On("LinkByName", "enp175s6").Return(fakeLink, nil)
			mocked.On("LinkByName", "temp_1000").Return(net2Link, nil)
			mocked.On("LinkByName", "net1").Return(net1Link, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetName", net2Link, mock.Anything).Return(nil)
			mocked.On("LinkDelAltName", net2Link, "enp175s6").Return(nil)
			mocked.On("LinkSetHardwareAddr", net1Link, expMac).Return(nil)
			mocked.On("LinkSetNsFd", net2Link, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", net2Link).Return(nil)
			mockedPciUtils.On("EnableArpAndNdiscNotify", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			err = sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
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
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "6e:16:06:0e:b7:e9",
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
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink", HardwareAddr: fakeMac}}

			mocked.On("LinkByName", podifName).Return(fakeLink, nil)
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
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "c6:c8:7f:1f:21:90",
				},
			}
		})
		It("Should not restores Effective MAC address when it is not provided in netconf", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}
			mocked := &mocks_utils.NetlinkManager{}

			mocked.On("LinkByName", podifName).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			sm := sriovManager{nLink: mocked}
			err = sm.ReleaseVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})

		It("Restores Effective MAC address when provided in netconf", func() {
			netconf.MAC = "aa:f3:8d:65:1b:d4"
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}
			mocked := &mocks_utils.NetlinkManager{}

			fakeMac, err := net.ParseMAC("c6:c8:7f:1f:21:90")
			Expect(err).NotTo(HaveOccurred())
			tempLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "enp175s6",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", podifName).Return(fakeLink, nil)
			mocked.On("LinkByName", netconf.OrigVfState.HostIFName).Return(tempLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetHardwareAddr", tempLink, fakeMac).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
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
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
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

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
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
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Does not change VF config if it wasnt requested to be changed in netconf", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQosProto", fakeLink, netconf.VFID, netconf.OrigVfState.Vlan, netconf.OrigVfState.VlanQoS, sriovtypes.VlanProtoInt[sriovtypes.Proto8021q]).Return(nil)
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
				Master:    "enp175s0f1",
				DeviceID:  "0000:af:06.0",
				VFID:      0,
				MAC:       "d2:fc:22:a7:0d:e8",
				Vlan:      &vlan,
				VlanQoS:   &vlanQos,
				SpoofChk:  "on",
				MaxTxRate: &maxTxRate,
				MinTxRate: &minTxRate,
				Trust:     "on",
				LinkState: "enable",
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
			origMac, err := net.ParseMAC(netconf.OrigVfState.AdminMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink", Vfs: []netlink.VfInfo{
				{Mac: origMac},
			}}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQosProto", fakeLink, netconf.VFID, netconf.OrigVfState.Vlan, netconf.OrigVfState.VlanQoS, sriovtypes.VlanProtoInt[sriovtypes.Proto8021q]).Return(nil)
			mocked.On("LinkSetVfSpoofchk", fakeLink, netconf.VFID, netconf.OrigVfState.SpoofChk).Return(nil)
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
