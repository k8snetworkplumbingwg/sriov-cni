package utils

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = BeforeSuite(func() {
	// create test sys tree
	err := CreateTmpSysFs()
	Expect(err).Should(Succeed())
})

var _ = AfterSuite(func() {
	err := RemoveTmpSysFs()
	Expect(err).Should(Succeed())
})
