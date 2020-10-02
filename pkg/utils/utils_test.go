package utils

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

func FakeNoTrunkSupport(command string, args ...string) *exec.Cmd {
	command = "echo"
	args[0] = "1.2.3"
	cmd := exec.Command(command, args[0])

	return cmd
}

func FakeTrunkSupport(command string, args ...string) *exec.Cmd {
	command = "echo"
	args[0] = "2.7.11"
	cmd := exec.Command(command, args[0])

	return cmd
}

var _ = Describe("Utils", func() {

	Context("Checking GetSriovNumVfs function", func() {
		It("Assuming existing interface", func() {
			result, err := GetSriovNumVfs("enp175s0f1")
			Expect(result).To(Equal(2), "Existing sriov interface should return correct VFs count")
			Expect(err).NotTo(HaveOccurred(), "Existing sriov interface should not return an error")
		})
		It("Assuming not existing interface", func() {
			_, err := GetSriovNumVfs("enp175s0f2")
			Expect(err).To(HaveOccurred(), "Not existing sriov interface should return an error")
		})
	})
	Context("Checking GetVfid function", func() {
		It("Assuming existing interface", func() {
			result, err := GetVfid("0000:af:06.0", "enp175s0f1")
			Expect(result).To(Equal(0), "Existing VF should return correct VF index")
			Expect(err).NotTo(HaveOccurred(), "Existing VF should not return an error")
		})
		It("Assuming not existing interface", func() {
			_, err := GetVfid("0000:af:06.0", "enp175s0f2")
			Expect(err).To(HaveOccurred(), "Not existing interface should return an error")
		})
	})
	Context("Checking GetPfName function", func() {
		It("Assuming existing vf", func() {
			result, err := GetPfName("0000:af:06.0")
			Expect(err).NotTo(HaveOccurred(), "Existing VF should not return an error")
			Expect(result).To(Equal("enp175s0f1"), "Existing VF should return correct PF name")
		})
		It("Assuming not existing vf", func() {
			result, err := GetPfName("0000:af:07.0")
			Expect(result).To(Equal(""))
			Expect(err).To(HaveOccurred(), "Not existing VF should return an error")
		})
	})
	Context("Checking GetPciAddress function", func() {
		It("Assuming existing interface and vf", func() {
			Expect(GetPciAddress("enp175s0f1", 0)).To(Equal("0000:af:06.0"), "Existing PF and VF id should return correct VF pci address")
		})
		It("Assuming not existing interface", func() {
			_, err := GetPciAddress("enp175s0f2", 0)
			Expect(err).To(HaveOccurred(), "Not existing PF should return an error")
		})
		It("Assuming not existing vf", func() {
			result, err := GetPciAddress("enp175s0f1", 33)
			Expect(result).To(Equal(""), "Not existing VF id should not return pci address")
			Expect(err).To(HaveOccurred(), "Not existing VF id should return an error")
		})
	})
	Context("Checking GetSharedPF function", func() {
		/* TO-DO */
		// It("Assuming existing interface", func() {
		// 	result, err := GetSharedPF("enp175s0f1")
		// 	Expect(result).To(Equal("sharedpf"), "Looking for shared PF for supported NIC should return correct PF name")
		// 	Expect(err).NotTo(HaveOccurred(), "Looking for shared PF for supported NIC should not return an error")
		// })
		// It("Assuming not existing interface", func() {
		// 	_, err := GetSharedPF("enp175s0f2")
		// 	Expect(err).To(HaveOccurred(), "Looking for shared PF for not supported NIC should return an error")
		// })
	})
	Context("Checking GetVFLinkNames function", func() {
		It("Assuming existing vf", func() {
			result, err := GetVFLinkNamesFromVFID("enp175s0f1", 0)
			Expect(result).To(ContainElement("enp175s6"), "Existing PF should have at least one VF")
			Expect(err).NotTo(HaveOccurred(), "Existing PF should not return an error")
		})
		It("Assuming not existing vf", func() {
			_, err := GetVFLinkNamesFromVFID("enp175s0f1", 3)
			Expect(err).To(HaveOccurred(), "Not existing VF should return an error")
		})
	})
	Context("Checking CheckTrunkSupport function", func() {
		It("Assuming version higher or equal to 2.7.11", func() {
			execCommand = FakeTrunkSupport
			result := CheckTrunkSupport()
			Expect(result).To(Equal(true))
		})
		It("Assuming version lower than 2.7.11", func() {
			execCommand = FakeNoTrunkSupport
			result := CheckTrunkSupport()
			Expect(result).To(Equal(false))
		})
	})
	Context("Checking GetVlanTrunkRange function", func() {
		It("Assuming valid vlan range", func() {
			result, err := GetVlanTrunkRange("1,4-6,10")
			expectedRangeData := sriovtypes.VlanTrunkRangeData{
				VlanTrunkRanges: []sriovtypes.VlanTrunkRange{{Start: 1, End: 1}, {Start: 4, End: 6}, {Start: 10, End: 10}},
			}
			Expect(result).To(Equal(expectedRangeData))
			Expect(err).NotTo(HaveOccurred(), "Valid vlan range should not return an error")
		})
		It("Assuming not valid vlan range", func() {
			_, err := GetVlanTrunkRange("3,2-6,10")
			Expect(err).To(HaveOccurred(), "Invalid vlan range should return an error")
		})
	})
	Context("Checking ValidateVlanTrunkRange function", func() {
		It("Assuming valid vlan range", func() {
			testRangeData := sriovtypes.VlanTrunkRangeData{
				VlanTrunkRanges: []sriovtypes.VlanTrunkRange{{Start: 1, End: 1}, {Start: 4, End: 6}, {Start: 10, End: 10}},
			}
			err := ValidateVlanTrunkRange(testRangeData.VlanTrunkRanges)
			Expect(err).NotTo(HaveOccurred(), "Valid vlan range should not return an error")
		})
		It("Assuming invalid vlan range", func() {
			_, err := GetVlanTrunkRange("3,2-6,10")
			Expect(err).To(HaveOccurred(), "Invalid vlan range should return an error")
		})
		It("Assuming invalid vlan range", func() {
			_, err := GetVlanTrunkRange("5-4,10")
			Expect(err).To(HaveOccurred(), "Invalid vlan range should return an error")
		})
		It("Assuming invalid vlan range", func() {
			_, err := GetVlanTrunkRange("0,3-10")
			Expect(err).To(HaveOccurred(), "Invalid vlan range should return an error")
		})
		It("Assuming invalid vlan range", func() {
			_, err := GetVlanTrunkRange("1,4091-4095")
			Expect(err).To(HaveOccurred(), "Invalid vlan range should return an error")
		})
	})
	Context("Checking ReadVendorFile function", func() {
		It("Assuming existing vf", func() {
			// path := filepath.Join(NetDirectory, "/enp175s6/device/vendor")
			result, err := GetVendorID("0000:af:06.1")
			Expect(result).To(Equal("0x8086"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming existing vf", func() {
			result, err := GetVendorID("0000:cf:06.0")
			Expect(result).To(Equal("0x15b3"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming not existing vf", func() {
			_, err := GetVendorID("0000:af:07.0")
			Expect(err).To(HaveOccurred(), "Non-existing VF, no vendor installed")
		})
	})
})
