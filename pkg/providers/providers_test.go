package providers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/intel/sriov-cni/pkg/types"
	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

var _ = Describe("Providers", func() {
	Context("Checking Init/Apply/RemoveConfig function", func() {
		It("Assuming valid config", func() {
			p := NewIntelTrunkProviderConfig()
			vlanRanges := &sriovtypes.VlanTrunkRangeData{
				VlanTrunkRanges: []sriovtypes.VlanTrunkRange{{Start: 1, End: 3}, {Start: 5, End: 10}, {Start: 13, End: 13}},
			}
			netConf := &types.NetConf{
				Master: "enp175s6",
				VFID:   0,
			}
			p.InitConfig(vlanRanges)

			err := p.ApplyConfig(netConf)
			Expect(err).NotTo(HaveOccurred())

			err = p.RemoveConfig(netConf)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Checking GetVlanData function", func() {
		It("Assuming correct VlanTrunkRangeData", func() {
			vlanRanges := &sriovtypes.VlanTrunkRangeData{
				VlanTrunkRanges: []sriovtypes.VlanTrunkRange{{Start: 1, End: 3}, {Start: 5, End: 10}, {Start: 13, End: 13}},
			}
			p := &IntelTrunkProviderConfig{
				ProviderName: "Intel",
			}
			p.GetVlanData(vlanRanges)
			Expect(p.VlanData).To(Equal("1-3,5-10,13"))
		})
	})
	Context("Checking AddVlanFiltering function", func() {
		It("Assuming existing vf", func() {
			vlanData := "2,6,100-200"
			pfname := "enp175s6"
			vfid := 0
			err := AddVlanFiltering(vlanData, pfname, vfid)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming non-existing vf", func() {
			vlanData := "2,6,100-200"
			pfname := "invalid-path/enp175s16"
			vfid := 0
			err := AddVlanFiltering(vlanData, pfname, vfid)
			Expect(err).To(HaveOccurred())
		})
	})
	Context("Checking RemoveVlanFiltering function", func() {
		It("Assuming existing vf", func() {
			vlanData := "2,6,100-200"
			pfname := "enp175s0f1"
			vfid := 1
			err := RemoveVlanFiltering(vlanData, pfname, vfid)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Assuming non-existing vf", func() {
			vlanData := "2,6,100-200"
			pfname := "invalid-path/enp175s16"
			vfid := 0
			err := RemoveVlanFiltering(vlanData, pfname, vfid)
			Expect(err).To(HaveOccurred())
		})
	})
})
