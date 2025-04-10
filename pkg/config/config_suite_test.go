package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = BeforeSuite(func() {
	// create test sys tree
	err := utils.CreateTmpSysFs()
	Expect(err).Should(Succeed())
})

var _ = AfterSuite(func() {
	err := utils.RemoveTmpSysFs()
	Expect(err).Should(Succeed())
})
