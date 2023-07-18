package config

import (
	"fmt"
	"os"

	"github.com/containernetworking/plugins/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

var _ = Describe("Config", func() {
	Context("Checking LoadConf function", func() {
		It("Assuming correct config file - existing DeviceID", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
        "deviceID": "0000:af:06.1",
        "vf": 0,
        "ipam": {
            "type": "host-local",
            "subnet": "10.55.206.0/26",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ],
            "gateway": "10.55.206.1"
        }
                        }`)
			_, err := LoadConf(conf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming incorrect config file - not existing DeviceID", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
        "deviceID": "0000:af:06.3",
        "vf": 0,
        "ipam": {
            "type": "host-local",
            "subnet": "10.55.206.0/26",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ],
            "gateway": "10.55.206.1"
        }
                        }`)
			_, err := LoadConf(conf)
			Expect(err).To(HaveOccurred())
		})
		It("Assuming incorrect config file - broken json", func() {
			conf := []byte(`{
        "name": "mynet"
		"type": "sriov",
		"deviceID": "0000:af:06.1",
        "vf": 0,
        "ipam": {
            "type": "host-local",
            "subnet": "10.55.206.0/26",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ],
            "gateway": "10.55.206.1"
        }
                        }`)
			_, err := LoadConf(conf)
			Expect(err).To(HaveOccurred())
		})

		validVlanID := 100
		zeroVlanID := 0
		invalidVlanID := 5000
		validQoS := 1
		invalidQoS := 10
		valid8021qProto := "802.1Q"
		valid8021adProto := "802.1ad"
		invalidProto := "802"
		DescribeTable("Vlan ID, QoS and Proto",
			func(vlanID *int, vlanQoS *int, vlanProto *string, failure bool) {
				s := `{
        "name": "mynet",
        "type": "sriov",
        "deviceID": "0000:af:06.1",
        "vf": 0`
				if vlanID != nil {
					s = fmt.Sprintf(`%s,
        "vlan": %d`, s, *vlanID)
				}
				if vlanQoS != nil {
					s = fmt.Sprintf(`%s,
        "vlanQoS": %d`, s, *vlanQoS)
				}
				if vlanProto != nil {
					s = fmt.Sprintf(`%s,
        "vlanProto": "%s"`, s, *vlanProto)
				}
				s = fmt.Sprintf(`%s
                        }`, s)
				conf := []byte(s)
				_, err := LoadConf(conf)
				if failure {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry("valid vlan ID", &validVlanID, nil, nil, false),
			Entry("invalid vlan ID", &invalidVlanID, nil, nil, true),
			Entry("vlan ID equal to zero and QoS set", &zeroVlanID, &validQoS, nil, true),
			Entry("vlan ID equal to zero and Proto set", &zeroVlanID, nil, &valid8021qProto, true),
			Entry("invalid QoS", &validVlanID, &invalidQoS, nil, true),
			Entry("invalid Proto", &validVlanID, nil, &invalidProto, true),
			Entry("valid 802.1q Proto", &validVlanID, nil, &valid8021qProto, false),
			Entry("valid 802.1ad Proto", &validVlanID, nil, &valid8021adProto, false),
			Entry("no vlan ID and QoS set", nil, &validQoS, nil, true),
			Entry("no vlan ID and Proto set", nil, nil, &valid8021adProto, true),
		)

		It("Assuming device is allocated", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
        "deviceID": "0000:af:06.1",
        "vf": 0,
        "ipam": {
            "type": "host-local",
            "subnet": "10.55.206.0/26",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ],
            "gateway": "10.55.206.1"
        }
                        }`)

			tmpdir, err := os.MkdirTemp("/tmp", "sriovplugin-testfiles-")
			Expect(err).ToNot(HaveOccurred())
			originCNIDir := DefaultCNIDir
			DefaultCNIDir = tmpdir
			defer func() {
				DefaultCNIDir = originCNIDir
			}()

			targetNetNS, err := testutils.NewNS()
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
					err = testutils.UnmountNS(targetNetNS)
				}
			}()

			allocator := utils.NewPCIAllocator(tmpdir)
			err = allocator.SaveAllocatedPCI("0000:af:06.1", targetNetNS.Path())
			Expect(err).ToNot(HaveOccurred())

			_, err = LoadConf(conf)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pci address 0000:af:06.1 is already allocated"))
		})

	})
	Context("Checking getVfInfo function", func() {
		It("Assuming existing PF", func() {
			_, _, err := getVfInfo("0000:af:06.0")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming not existing PF", func() {
			_, _, err := getVfInfo("0000:af:07.0")
			Expect(err).To(HaveOccurred())
		})
	})
	Context("Checking GetMacAddressForResult function", func() {
		It("Should return the mac address requested by the user", func() {
			netconf := &types.NetConf{
				MAC: "MAC",
				OrigVfState: types.VfState{
					EffectiveMAC: "EffectiveMAC",
					AdminMAC:     "AdminMAC",
				},
			}

			Expect(GetMacAddressForResult(netconf)).To(Equal("MAC"))
		})
		It("Should return the EffectiveMAC mac address if the user didn't request and the the driver is not DPDK", func() {
			netconf := &types.NetConf{
				DPDKMode: false,
				OrigVfState: types.VfState{
					EffectiveMAC: "EffectiveMAC",
					AdminMAC:     "AdminMAC",
				},
			}

			Expect(GetMacAddressForResult(netconf)).To(Equal("EffectiveMAC"))
		})
		It("Should return the AdminMAC mac address if the user didn't request and the the driver is DPDK", func() {
			netconf := &types.NetConf{
				DPDKMode: true,
				OrigVfState: types.VfState{
					EffectiveMAC: "EffectiveMAC",
					AdminMAC:     "AdminMAC",
				},
			}

			Expect(GetMacAddressForResult(netconf)).To(Equal("AdminMAC"))
		})
		It("Should return empty string if the user didn't request the the driver is DPDK and adminMac is 0", func() {
			netconf := &types.NetConf{
				DPDKMode: true,
				OrigVfState: types.VfState{
					AdminMAC: "00:00:00:00:00:00",
				},
			}

			Expect(GetMacAddressForResult(netconf)).To(Equal(""))
		})
	})
})
