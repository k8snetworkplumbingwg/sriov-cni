package dpdk

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//https://npf.io/2015/06/testing-exec-command
func FakeExecCommand(success bool) func(string, ...string) *exec.Cmd {
	return func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("DPDK_DEVBIND_SUCCESS=%s", strconv.FormatBool(success))}
		return cmd
	}
}

//https://npf.io/2015/06/testing-exec-command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if os.Getenv("DPDK_DEVBIND_SUCCESS") != "true" {
		fmt.Fprintf(os.Stdout, "DPDK binding failed")
		os.Exit(1)
	}
	os.Exit(0)
}

var _ = Describe("Dpdk", func() {
	dc := Conf{
		PCIaddr:    "0000:af:09.0",
		Ifname:     "net1",
		KDriver:    "i40evf",
		DPDKDriver: "vfio-pci",
		DPDKtool:   "/opt/dpdk/usertools/dpdk-devbind.py",
		VFID:       24}
	Context("Checking SaveDdpkConf function", func() {
		It("Assuming correct config file", func() {
			err := SaveDpdkConf("cidCorrect", dataDir, &dc)
			Expect(err).NotTo(HaveOccurred(), "Using correct configuration should not cause an error")
		})
		////TODO: this test will fail until ValidateDpdkConf will be added
		//It("Assuming incorrect config file - missing pci address", func() {
		//	dc.PCIaddr = ""
		//	err := SaveDpdkConf("cidBroken", dataDir, &dc)
		//	Expect(err).To(HaveOccurred(), "Using incorrect config file should cause an error")
		//})
	})
	Context("Checking GetDdpkConf function", func() {
		It("Assuming correct config file", func() {
			dc.PCIaddr = "0000:af:09.0"
			_, err := GetConf("cidCorrect", "net1", dataDir)
			Expect(err).NotTo(HaveOccurred(), "Using correct configuration should not cause an error")
		})
		////TODO: this test will fail until ValidateDpdkConf will be added
		//It("Assuming incorrect config file - missing pci address", func() {
		//	dc.PCIaddr = ""
		//	_, err := GetConf("cidBroken", "net1", dataDir)
		//	//TODO: this test will fail until ValidateDpdkConf will be added
		//	Expect(err).To(HaveOccurred(), "Using incorrect config file should cause an error")
		//})
		It("Assuming not existing config file", func() {
			_, err := GetConf("cid", "net1", dataDir)
			Expect(err).To(HaveOccurred(), "Using not existing config file should cause an error")
		})
	})
	Context("Checking Enabledpdkmode function", func() {
		It("Assuming dpdk mode enabled with correct config file", func() {
			dc.PCIaddr = "0000:af:09.0"
			execCommand = FakeExecCommand(true)
			defer func() { execCommand = exec.Command }()
			err := Enabledpdkmode(&dc, "net1", true)
			Expect(err).NotTo(HaveOccurred(), "Using correct config file should not cause an error")
		})
		It("Assuming dpdk mode disabled with correct config file", func() {
			execCommand = FakeExecCommand(true)
			defer func() { execCommand = exec.Command }()
			err := Enabledpdkmode(&dc, "net1", false)
			Expect(err).NotTo(HaveOccurred(), "Using correct config file should not cause an error")
		})
		It("Assuming dpdk mode enabled with incorrect config file - missing dpdk tool path", func() {
			dc.DPDKtool = ""
			execCommand = FakeExecCommand(false)
			defer func() { execCommand = exec.Command }()
			err := Enabledpdkmode(&dc, "net1", true)
			Expect(err).To(HaveOccurred(), "Using incorrect config file should cause an error")
		})
		It("Assuming dpdk mode disabled with incorrect config file - missing dpdk tool path", func() {
			execCommand = FakeExecCommand(false)
			defer func() { execCommand = exec.Command }()
			err := Enabledpdkmode(&dc, "net1", false)
			Expect(err).To(HaveOccurred(), "Using incorrect config file should cause an error")
		})
	})
})
