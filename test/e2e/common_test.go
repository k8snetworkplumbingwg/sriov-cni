package e2e

// This file should contain functions that are common for all tests and make tests more readable by hiding
// implementation details
// For instance it does not make sense to have in each test four lines that are used to creat network-attachment definition

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"

	utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	nad "github.com/k8snetworkplumbingwg/sriov-cni/test/util/nad"
	net "github.com/k8snetworkplumbingwg/sriov-cni/test/util/net"
	pod "github.com/k8snetworkplumbingwg/sriov-cni/test/util/pod"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func createNAD(namespace, networkName, networkResName string) *cniv1.NetworkAttachmentDefinition {
	By("Create network attachment definition")
	nadObj := nad.GetNetworkAttachmentDefinition(networkName, namespace)
	nadObj = nad.AddNADAnnotation(nadObj, "k8s.v1.cni.cncf.io/resourceName", networkResName)
	err := nad.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nadObj, timeout)
	Expect(err).To(BeNil())

	return nadObj
}

func createNADwithConfig(namespace, networkName, networkResName string, config NADConfiguration) *cniv1.NetworkAttachmentDefinition {
	By("Create network attachment definition")
	nadObj := nad.GetNetworkAttachmentDefinition(networkName, namespace)
	nadObj = nad.AddNADAnnotation(nadObj, "k8s.v1.cni.cncf.io/resourceName", networkResName)
	nadObj = nad.AddNADSpecConfigProperty(nadObj, "link_state", config.linkState)
	nadObj = nad.AddNADIntSpecConfigProperty(nadObj, "vlan", config.vlanID)
	nadObj = nad.AddNADIntSpecConfigProperty(nadObj, "vlanQoS", config.vlanQoS)
	nadObj = nad.AddNADSpecConfigProperty(nadObj, "spoofchk", config.spoofchk)
	nadObj = nad.AddNADIntSpecConfigProperty(nadObj, "max_tx_rate", int(config.maxTxRate))
	err := nad.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nadObj, timeout)
	Expect(err).To(BeNil())

	return nadObj
}

func (expectedConfig *NADConfiguration) compare(vf netlink.VfInfo) {
	if expectedConfig.linkState == "enable" {
		Expect(vf.LinkState).Should(Equal(uint32(net.LinkStateEnable)))
	} else if expectedConfig.linkState == "disable" {
		Expect(vf.LinkState).Should(Equal(uint32(net.LinkStateDisable)))
	} else {
		Expect(vf.LinkState).Should(Equal(uint32(net.LinkStateAuto)))
	}

	if expectedConfig.spoofchk == "on" {
		Expect(vf.Spoofchk).Should(Equal(true))
	} else {
		Expect(vf.Spoofchk).Should(Equal(false))
	}

	Expect(vf.MaxTxRate).Should(Equal(expectedConfig.maxTxRate))
	Expect(vf.MinTxRate).Should(Equal(expectedConfig.minTxRate))
	Expect(vf.Vlan).Should(Equal(expectedConfig.vlanID))
	Expect(vf.Qos).Should(Equal(expectedConfig.vlanQoS))
}

func deleteNAD(networkName string, nadObj *cniv1.NetworkAttachmentDefinition) {
	By("Delete network attachment definition")
	err := nad.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, networkName, nadObj, timeout)
	Expect(err).To(BeNil())
}

func deletePod(podObj *corev1.Pod) {
	err := pod.Delete(cs.CoreV1Interface, podObj, timeout)
	Expect(err).To(BeNil())
}

func deletePods(podObj []*corev1.Pod) {
	err := pod.DeleteList(cs.CoreV1Interface, podObj, timeout)
	Expect(err).To(BeNil())
}

func determineDriverSpace(pfName string) (string, error) {
	number, err := utils.GetSriovNumVfs(pfName)
	if err != nil {
		return "", err
	}

	var user, kernel int
	for index := 0; index < number; index++ {
		pciAddr, err := utils.GetPciAddress(pfName, index)
		if err != nil {
			return "", err
		}

		supported, err := isDriverSupported(pciAddr, supportedUserSpaceDrivers)
		if err != nil {
			return "", err
		}

		if supported {
			user++
		}

		supported, err = isDriverSupported(pciAddr, supportedKenrelDrivers)
		if err != nil {
			return "", err
		}

		if supported {
			kernel++
		}
	}

	if kernel > 0 && user > 0 {
		return "", fmt.Errorf("both user space and kernel drivers at the same PF are not supported within tests")
	}

	if kernel > 0 {
		return "kernel", nil
	}

	if user > 0 {
		return "user-space", nil
	}

	return "", fmt.Errorf("PF have unsupported drivers")
}

// isDriverSupported checks if a device is attached to one of the supported drivers passed in slice
func isDriverSupported(pciAddr string, supportedDrivers []string) (bool, error) {
	driverLink := filepath.Join(SysBusPci, pciAddr, "driver")
	driverPath, err := filepath.EvalSymlinks(driverLink)
	if err != nil {
		return false, err
	}

	driverStat, err := os.Stat(driverPath)
	if err != nil {
		return false, err
	}

	driverName := driverStat.Name()
	for _, drv := range supportedDrivers {
		if driverName == drv {
			return true, nil
		}
	}

	return false, nil
}

func readDpConfigurationFile(path string) (string, error) {
	configFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer configFile.Close()

	bytesVal, err := ioutil.ReadAll(configFile)
	if err != nil {
		return "", err
	}

	return string(bytesVal), nil
}
