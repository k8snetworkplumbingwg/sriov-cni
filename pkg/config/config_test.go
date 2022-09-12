package config

import (
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
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
})
