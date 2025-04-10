package sriov

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/sriov/mocks"
	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
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
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
					MTU:        1500,
				}},
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
			mockedPciUtils.On("EnableArpAndNdiscNotify", mock.AnythingOfType("string")).Return(nil)
			mockedPciUtils.On("EnableOptimisticDad", mock.AnythingOfType("string")).Return(nil)
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
			mockedPciUtils.On("EnableOptimisticDad", mock.AnythingOfType("string")).Return(nil)
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
			mockedPciUtils.On("EnableOptimisticDad", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			err = sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
		It("Return MTU from PF", func() {
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
				MTU:          1500,
			}}

			net1Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "net1",
				HardwareAddr: expMac,
				MTU:          1500,
			}}

			net2Link := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         "temp_1000",
				HardwareAddr: expMac,
				MTU:          1500,
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
			mockedPciUtils.On("EnableOptimisticDad", mock.AnythingOfType("string")).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			err = sm.SetupVF(netconf, podifName, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(*netconf.MTU).To(Equal(1500))
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ApplyVFConfig function", func() {
		var (
			netconf  *sriovtypes.NetConf
			mocked   *mocks_utils.NetlinkManager
			fakeLink *utils.FakeLink
		)

		BeforeEach(func() {
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master: "enp175s0f1",
				VFID:   0,
			}}
			mocked = &mocks_utils.NetlinkManager{}
			fakeLink = &utils.FakeLink{}
		})

		It("should not call functions to configure the VF when config has no optional parameters", func() {
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			sm := sriovManager{nLink: mocked}
			err := sm.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call functions to configure the VF when config has optional parameters", func() {
			vlan := 100
			netconf.Vlan = &vlan
			qos := 0
			netconf.VlanQoS = &qos
			vlanProto := "802.1q"
			netconf.VlanProto = &vlanProto

			hwaddr, err := net.ParseMAC("aa:f3:8d:65:1b:d4")
			Expect(err).NotTo(HaveOccurred())

			maxTxRate := 4000
			minTxRate := 1000
			netconf.MaxTxRate = &maxTxRate
			netconf.MinTxRate = &minTxRate

			netconf.SpoofChk = "on"
			netconf.Trust = "on"
			netconf.LinkState = "enable"

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQosProto", fakeLink, netconf.VFID, *netconf.Vlan, *netconf.VlanQoS, sriovtypes.VlanProtoInt[sriovtypes.Proto8021q]).Return(nil)
			mocked.On("LinkSetVfHardwareAddr", fakeLink, netconf.VFID, hwaddr).Return(nil)
			mocked.On("LinkSetVfRate", fakeLink, netconf.VFID, *netconf.MinTxRate, *netconf.MaxTxRate).Return(nil)
			mocked.On("LinkSetVfSpoofchk", fakeLink, netconf.VFID, true).Return(nil)
			mocked.On("LinkSetVfTrust", fakeLink, netconf.VFID, true).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, netconf.VFID, netlink.VF_LINK_STATE_ENABLE).Return(nil)

			sm := sriovManager{nLink: mocked}
			err = sm.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Checking ReleaseVF function", func() {
		var (
			podifName string
			netconf   *sriovtypes.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "6e:16:06:0e:b7:e9",
					MTU:          1500,
				}},
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
			mocked.On("LinkSetMTU", fakeLink, 1500).Return(nil)
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
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "c6:c8:7f:1f:21:90",
				}},
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
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				}},
			}
		})
		It("Saves the current VF state", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         netconf.Name,
				HardwareAddr: fakeMac,
				Vfs: []netlink.VfInfo{
					{
						ID:  0,
						Mac: fakeMac,
					},
				},
			}}

			fakeVFLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index: 1001,
				Name:  netconf.OrigVfState.HostIFName,
				MTU:   1500,
			}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			mocked.On("LinkByName", netconf.OrigVfState.HostIFName).Return(fakeVFLink, nil)
			sm := sriovManager{nLink: mocked}
			err = sm.FillOriginalVfInfo(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(netconf.OrigVfState.MTU).To(Equal(1500))
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ResetVFConfig function - restore config no user params", func() {
		var (
			netconf *sriovtypes.NetConf
		)

		BeforeEach(func() {
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:   "enp175s0f1",
				DeviceID: "0000:af:06.0",
				VFID:     0,
				OrigVfState: sriovtypes.VfState{
					HostIFName: "enp175s6",
				}},
			}
		})
		It("Does not change VF config if it wasnt requested to be changed in netconf", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

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

			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
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
					MTU:          1500,
				}},
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
	Context("Checking ApplyVFConfig function", func() {
		var (
			netconf *sriovtypes.NetConf
		)

		It("Return MTU from PF", func() {
			mocked := &mocks_utils.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}
			vlan := 0
			vlanProto := sriovtypes.Proto8021q
			netconf = &sriovtypes.NetConf{SriovNetConf: sriovtypes.SriovNetConf{
				Master:    "ens1s0",
				Vlan:      &vlan,
				VlanQoS:   &vlan,
				VlanProto: &vlanProto}}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index: 1000,
				Name:  "ens1s0",
				MTU:   9000,
			}}

			mocked.On("LinkByName", "ens1s0").Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQosProto", fakeLink, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			sm := sriovManager{nLink: mocked, utils: mockedPciUtils}
			err := sm.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(*netconf.MTU).To(Equal(9000))
			mocked.AssertExpectations(t)
		})
	})
})
