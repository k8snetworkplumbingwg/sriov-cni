package factory

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {

	Context("Checking GetProviderConfig function", func() {
		It("Assuming existing vf", func() {
			_, err := GetProviderConfig("0000:af:06.1")
			// Expect(result).To(Equal(&providers.IntelTrunkProviderConfig{ProviderName: "Intel"}))
			Expect(err).NotTo(HaveOccurred(), "Existing vf should not return an error")
		})
		It("Assuming existing vf", func() {
			result, err := GetProviderConfig("0000:cf:06.0")
			Expect(result).To(BeNil())
			Expect(err).To(HaveOccurred(), "Existing vf should not return an error, unless the vendor is not supported")
		})
		It("Assuming not existing vf", func() {
			_, err := GetProviderConfig("0000:af:07.0")
			Expect(err).To(HaveOccurred(), "Not existing vf should return an error")
		})
	})
})
