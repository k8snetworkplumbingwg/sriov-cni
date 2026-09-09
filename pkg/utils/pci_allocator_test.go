package utils

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
)

var _ = Describe("PCIAllocator", func() {
	var targetNetNS ns.NetNS
	var err error

	AfterEach(func() {
		if targetNetNS != nil {
			targetNetNS.Close()
			err = testutils.UnmountNS(targetNetNS)
		}
	})

	Context("PCI address validation", func() {
		DescribeTable("rejects invalid PCI addresses before accessing the filesystem",
			func(operation func(*PCIAllocator, string) error) {
				dataDir := GinkgoT().TempDir()
				allocator := NewPCIAllocator(dataDir)

				err := operation(allocator, "../escape")
				Expect(err).To(MatchError("invalid PCI address format: ../escape"))
				Expect(filepath.Join(dataDir, "pci")).ToNot(BeADirectory())
			},
			Entry("Lock", func(allocator *PCIAllocator, pciAddress string) error {
				return allocator.Lock(pciAddress)
			}),
			Entry("SaveAllocatedPCI", func(allocator *PCIAllocator, pciAddress string) error {
				return allocator.SaveAllocatedPCI(pciAddress, "/test/netns")
			}),
			Entry("DeleteAllocatedPCI", func(allocator *PCIAllocator, pciAddress string) error {
				return allocator.DeleteAllocatedPCI(pciAddress)
			}),
			Entry("IsAllocated", func(allocator *PCIAllocator, pciAddress string) error {
				_, err := allocator.IsAllocated(pciAddress)
				return err
			}),
		)
	})

	Context("IsAllocated", func() {
		It("Assuming is not allocated", func() {
			allocator := NewPCIAllocator(ts.dirRoot)
			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeFalse())
		})

		It("Assuming is allocated and namespace exist", func() {
			targetNetNS, err = testutils.NewNS()
			Expect(err).NotTo(HaveOccurred())
			allocator := NewPCIAllocator(ts.dirRoot)

			err = allocator.SaveAllocatedPCI("0000:af:00.1", targetNetNS.Path())
			Expect(err).ToNot(HaveOccurred())

			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeTrue())
		})

		It("Assuming is allocated and namespace doesn't exist", func() {
			targetNetNS, err = testutils.NewNS()
			Expect(err).NotTo(HaveOccurred())

			allocator := NewPCIAllocator(ts.dirRoot)
			err = allocator.SaveAllocatedPCI("0000:af:00.1", targetNetNS.Path())
			Expect(err).ToNot(HaveOccurred())
			err = targetNetNS.Close()
			Expect(err).ToNot(HaveOccurred())
			err = testutils.UnmountNS(targetNetNS)
			Expect(err).ToNot(HaveOccurred())

			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeFalse())
		})
	})
})
