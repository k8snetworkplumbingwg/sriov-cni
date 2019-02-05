package config

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Checking LoadConf function", func() {
		It("Assuming correct config file", func() {
			conf := []byte(`{
	"name": "mynet",
	"type": "sriov",
	"master": "enp175s0f1",
	"mac":"66:77:88:99:aa:bb",
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
		It("Assuming correct config file - existing DeviceID", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
        "master": "enp175s0f1",
        "deviceID": "0000:af:06.1",
        "mac":"66:77:88:99:aa:bb",
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
        "master": "enp175s0f1",
        "deviceID": "0000:af:06.3",
        "mac":"66:77:88:99:aa:bb",
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
        "master": "enp175s0f1",
        "mac":"66:77:88:99:aa:bb",
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
		It("Assuming incorrect config file - missing master", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
        "mac":"66:77:88:99:aa:bb",
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
			Expect(err).Should(MatchError("error: SRIOV-CNI loadConf: VF pci addr OR Master name is required"))
		})
		It("Assuming incorrect config file - forbidden if0name", func() {
			conf := []byte(`{
        "name": "mynet",
        "type": "sriov",
	"if0name": "eth0",
        "master": "enp175s0f1",
        "mac":"66:77:88:99:aa:bb",
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
			Expect(err).Should(MatchError("\"if0name\" field should not be  equal to (eth0 | eth1 | lo | \"\"). It specifies the virtualized interface name in the pod"))
		})
	})
	Context("Checking checkIf0name function", func() {
		It("Assuming correct interfaces", func() {
			reservedIface := []string{"eth0", "eth1", "lo", ""}
			for _, iface := range reservedIface {
				Expect(checkIf0name(iface)).Should(BeFalse())
			}
		})
		It("Assuming forbidden interfaces", func() {
			forbiddenIface := []string{"eno0", "eth11", "!@#"}
			for _, iface := range forbiddenIface {
				Expect(checkIf0name(iface)).Should(BeTrue())
			}
		})
	})
	Context("Checking getVfInfo function", func() {
		It("Assuming existing PF", func() {
			_, err := getVfInfo("0000:af:06.0")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming not existing PF", func() {
			_, err := getVfInfo("0000:af:07.0")
			Expect(err).To(HaveOccurred())
		})
	})
})
