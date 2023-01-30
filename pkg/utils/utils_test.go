package utils

import (
	"errors"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vishvananda/netlink"

	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
)

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
	Context("Checking Retry function", func() {
		It("Assuming calling function fails", func() {
			err := Retry(5, 10*time.Millisecond, func() error { return errors.New("") })
			Expect(err).To((HaveOccurred()), "Retry should return an error")
		})
		It("Assuming calling function does not fail", func() {
			err := Retry(5, 10*time.Millisecond, func() error { return nil })
			Expect(err).NotTo((HaveOccurred()), "Retry should not return an error")
		})
	})
	Context("Checking SetVFEffectiveMAC function", func() {
		It("assuming calling function fails", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).ToNot(HaveOccurred())
			fakeNewMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "enp175s0f1",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", "enp175s0f1").Return(fakeLink, nil)
			mocked.On("LinkSetHardwareAddr", fakeLink, fakeNewMac).Return(nil)

			err = SetVFEffectiveMAC(mocked, "enp175s0f1", "60:00:00:00:00:01")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("effective mac address is different from requested one"))
		})

		It("assuming calling function does not fails", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())
			fakeNewMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "enp175s0f1",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", "enp175s0f1").Return(fakeLink, nil)
			mocked.On("LinkSetHardwareAddr", fakeLink, fakeNewMac).Return(nil)

			err = SetVFEffectiveMAC(mocked, "enp175s0f1", "60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("Checking SetVFHardwareMAC function", func() {
		It("assuming calling function fails", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).ToNot(HaveOccurred())
			fakeNewMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index: 1000,
				Name:  "enp175s0f1",
				Vfs: []netlink.VfInfo{
					{Mac: fakeMac},
				},
			}}

			mocked.On("LinkByName", "enp175s0f1").Return(fakeLink, nil)
			mocked.On("LinkSetVfHardwareAddr", fakeLink, 0, fakeNewMac).Return(nil)

			err = SetVFHardwareMAC(mocked, "enp175s0f1", 0, "60:00:00:00:00:01")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("hardware mac address is different from requested one"))
		})

		It("assuming calling function does not fails", func() {
			mocked := &mocks_utils.NetlinkManager{}
			fakeMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())
			fakeNewMac, err := net.ParseMAC("60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index: 1000,
				Name:  "enp175s0f1",
				Vfs: []netlink.VfInfo{
					{Mac: fakeMac},
				},
			}}

			mocked.On("LinkByName", "enp175s0f1").Return(fakeLink, nil)
			mocked.On("LinkSetVfHardwareAddr", fakeLink, 0, fakeNewMac).Return(nil)

			err = SetVFHardwareMAC(mocked, "enp175s0f1", 0, "60:00:00:00:00:01")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
