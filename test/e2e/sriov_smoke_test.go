package e2e

import (
	"fmt"
	"strings"

	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"

	"github.com/k8snetworkplumbingwg/sriov-cni/test/util"
	net "github.com/k8snetworkplumbingwg/sriov-cni/test/util/net"
	pod "github.com/k8snetworkplumbingwg/sriov-cni/test/util/pod"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SR-IOV CNI test", func() {
	Context("Test with kernel Virtual Function driver", func() {
		var err error
		var podObj, podObj2 *corev1.Pod
		var stdoutString, stderrString, net1Mac string
		var hostLinksBefore, hostLinksAfter []netlink.Link

		BeforeEach(func() {
			if testDriverKind != "kernel" {
				Skip("Tests are not suitable for non kernel drivers.")
			}

			By("Store host links before tests")
			hostLinksBefore, err = net.GetHostLinkList()
			Expect(err).To(BeNil())

			err, netnsErr := net.SetVfsMAC(testPfName, containerNsPath)
			Expect(err).Should(BeNil())
			Expect(netnsErr).Should(BeNil())
		})

		AfterEach(func() {
			By("Get links after test and move those which were moved from Docker to host, once again to Docker netNs")
			hostLinksAfter, err = net.GetHostLinkList()
			Expect(err).To(BeNil())

			err = net.MoveLinksToDocker(containerNsPath, hostLinksBefore, hostLinksAfter)
			Expect(err).To(BeNil())
		})

		Context("POD is able to consume SR-IOV interfaces", func() {
			It("Second interface is available within POD", func() {
				nadObj := createNAD(*testNs, testNetworkName, testNetworkResName)
				defer deleteNAD(testNetworkName, nadObj)

				By("Create POD")
				podObj, err = pod.TryToCreateRunningPod(cs.CoreV1Interface, "test-pod-vf-go", *testNs, testNetworkName, testNetworkResName, util.Timeout)
				defer deletePod(podObj)
				Expect(err).To(BeNil())

				By("Verification - Check second network interfaces")
				stdoutString, stderrString, err = pod.ExecuteCommand(cs.CoreV1Interface, kubeConfig, podObj.Name, *testNs, "test", "ethtool -i eth0")
				Expect(err).Should(BeNil())
				Expect(stderrString).Should(Equal(""))
				Expect(stdoutString).Should(ContainSubstring("driver: veth"))

				stdoutString, stderrString, err = pod.ExecuteCommand(cs.CoreV1Interface, kubeConfig, podObj.Name, *testNs, "test", "ethtool -i net1")
				Expect(err).Should(BeNil())
				Expect(stderrString).Should(Equal(""))

				var foundDriver bool
				for _, driver := range supportedKernelDrivers {
					if strings.Contains(stdoutString, "driver: "+driver) {
						foundDriver = true
						break
					}
				}
				Expect(foundDriver).Should(BeTrue())

				stdoutString, stderrString, err = pod.ExecuteCommand(cs.CoreV1Interface, kubeConfig, podObj.Name, *testNs, "test", "ethtool -i net2")
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(Equal("command terminated with exit code 71"))
				Expect(stderrString).Should(ContainSubstring("Cannot get driver information: No such device"))
				Expect(stdoutString).Should(Equal(""))
			})

			It("Verify if all available VFs can be used by POD with second interface", func() {
				By("Verify that number of VFs is equal to the requested number of VFs")
				vfsInfo, err := net.GetVfsLinksInfoList(testPfName, containerNsPath)
				Expect(err).Should(BeNil())
				Expect(len(vfsInfo)).ShouldNot(Equal(0))

				nadObj := createNAD(*testNs, testNetworkName, testNetworkResName)
				defer deleteNAD(testNetworkName, nadObj)

				By("Create list of PODs - one for each VF available on PF")
				podList, err := pod.CreateListOfPods(cs.CoreV1Interface, len(vfsInfo), "test-pod-vf", *testNs, testNetworkName, testNetworkResName)
				defer deletePods(podList)
				Expect(err).To(BeNil())

				By("Try to create another POD, should fail - no more interfaces")
				podLast, err := pod.TryToCreateRunningPod(cs.CoreV1Interface, "test-pod-last", *testNs, testNetworkName, testNetworkResName, util.ShortTimeout)
				defer deletePod(podLast)
				Expect(err).ToNot(BeNil())

				events, err := pod.GetPodEventsList(cs, podLast, timeout)
				Expect(err).Should(BeNil())

				var found bool
				for _, event := range events.Items {
					if event.Type == "Warning" && strings.Contains(event.Message, fmt.Sprintf("0/1 nodes are available: 1 Insufficient %s", testNetworkResName)) {
						found = true
						break
					}
				}
				Expect(found).To(Equal(true))
			})
		})

		Context("Smoke tests", func() {
			It("Check all features in one smoke test - set not default states", func() {
				By("Set all links to enable state on VFs and PF to down to have well know state before test execution")
				err = net.SetAllVfOnLinkState(testPfName, containerNsPath, net.LinkStateAuto)
				Expect(err).To(BeNil())

				nad0 := NADConfiguration{
					linkState: "auto",
					vlanID:    0,
					vlanQoS:   0,
					spoofchk:  "on",
					minTxRate: 0,
					maxTxRate: 0,
				}

				nad1 := NADConfiguration{
					linkState: "enable",
					vlanID:    1259,
					vlanQoS:   5,
					spoofchk:  "off",
					minTxRate: 0,
					maxTxRate: 40,
				}

				nad2 := NADConfiguration{
					linkState: "disable",
					vlanID:    250,
					vlanQoS:   1,
					spoofchk:  "off",
					minTxRate: 0,
					maxTxRate: 20,
				}

				nadObj := createNADwithConfig(*testNs, testNetworkName, testNetworkResName, nad1)
				defer deleteNAD(testNetworkName, nadObj)
				Expect(err).To(BeNil())

				nadObj2 := createNADwithConfig(*testNs, fmt.Sprintf("%s-2", testNetworkName), testNetworkResName, nad2)
				defer deleteNAD(fmt.Sprintf("%s-2", testNetworkName), nadObj2)
				Expect(err).To(BeNil())

				By("Create PODs")
				podObj, err = pod.TryToCreateRunningPod(cs.CoreV1Interface, "test-pod-smoke-1", *testNs, testNetworkName, testNetworkResName, util.Timeout)
				defer deletePod(podObj)
				Expect(err).To(BeNil())

				podObj2, err = pod.TryToCreateRunningPod(cs.CoreV1Interface, "test-pod-smoke-2", *testNs, fmt.Sprintf("%s-2", testNetworkName), testNetworkResName, util.Timeout)
				defer deletePod(podObj2)
				Expect(err).To(BeNil())

				By("Verify that PODs MAC was not changed")
				net1Mac, stderrString, err = pod.ExecuteCommand(cs.CoreV1Interface, kubeConfig, podObj.Name, *testNs, "test", "ip add show net1 | grep ether | awk '{print $2}'")
				net1Mac = strings.TrimSuffix(net1Mac, "\n")
				Expect(err).Should(BeNil())
				Expect(stderrString).Should(Equal(""))
				Expect(net1Mac).ShouldNot(Equal(""))

				net2Mac, stderrString, err := pod.ExecuteCommand(cs.CoreV1Interface, kubeConfig, podObj2.Name, *testNs, "test", "ip add show net1 | grep ether | awk '{print $2}'")
				net2Mac = strings.TrimSuffix(net2Mac, "\n")
				Expect(err).Should(BeNil())
				Expect(stderrString).Should(Equal(""))
				Expect(net2Mac).ShouldNot(Equal(""))

				By("Verify that VF has MAC defined in POD specification")
				vfsInfo, err := net.GetVfsLinksInfoList(testPfName, containerNsPath)
				Expect(err).Should(BeNil())
				for index, vf := range vfsInfo {
					By(fmt.Sprintf("VF index %d and MAC %s", index, vf.Mac.String()))
					if vf.Mac.String() == net1Mac {
						nad1.compare(vf)
					} else if vf.Mac.String() == net2Mac {
						nad2.compare(vf)
					} else {
						nad0.compare(vf)
					}
				}
			})
		})
	})
})
