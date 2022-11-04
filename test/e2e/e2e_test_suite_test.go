package e2e

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	networkCoreClient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	configmap "github.com/k8snetworkplumbingwg/sriov-cni/test/util/configmap"
	daemonset "github.com/k8snetworkplumbingwg/sriov-cni/test/util/daemonset"
	net "github.com/k8snetworkplumbingwg/sriov-cni/test/util/net"
	pod "github.com/k8snetworkplumbingwg/sriov-cni/test/util/pod"
	serviceaccount "github.com/k8snetworkplumbingwg/sriov-cni/test/util/serviceaccount"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	testNetworkName    = "sriov-network"
	testNetworkResName = "vendor.com/pool"
	interval           = time.Second * 5
	timeout            = time.Second * 60
	imageName          = "ghcr.io/k8snetworkplumbingwg/sriov-network-device-plugin"
	dsName             = "kube-sriov-device-plugin"
	serviceAccountName = "sriov-device-plugin"
	defaultImageTag    = "latest"
	testConfigMapName  = "sriovdp-config"
	// SysBusPci is sysfs pci device directory
	SysBusPci = "/sys/bus/pci/devices"
)

type ClientSet struct {
	coreclient.CoreV1Interface
}

type NetworkClientSet struct {
	networkCoreClient.K8sCniCncfIoV1Interface
}

type NADConfiguration struct {
	linkState string
	vlanID    int
	vlanQoS   int
	spoofchk  string
	minTxRate uint32
	maxTxRate uint32
}

var (
	master         *string
	kubeConfigPath *string
	testNs         *string
	cs             *ClientSet
	ac             *appsclient.AppsV1Client
	networkClient  *NetworkClientSet
	kubeConfig     *restclient.Config

	containerNsPath string
	testDriverKind  string
	testPfName      string
	testDpFilePath  string

	supportedKernelDrivers    = []string{"iavf"}
	supportedUserSpaceDrivers = []string{"vfio-pci"}
)

func init() {
	if home := homedir.HomeDir(); home != "" {
		kubeConfigPath = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "path to your kubeconfig file")
	} else {
		kubeConfigPath = flag.String("kubeconfig", "", "require absolute path to your kubeconfig file")
	}
	master = flag.String("master", "", "Address of Kubernetes API server")
	testNs = flag.String("testnamespace", "default", "namespace for testing")

	testPfName = os.Getenv("TEST_PF_NAME")
	if testPfName == "" {
		log.Fatalln("physical function was not defined - no place to execute tests")
	}

	var err error
	testDriverKind, err = determineDriverSpace(testPfName)
	if err != nil || testDriverKind == "" {
		log.Fatalf("Unable to find driver kind (user-space or kernel). Error: %s\n", err)
	}

	testDpFilePath = os.Getenv("TEST_DP_CONFIG_FILE")
	if testDpFilePath == "" {
		log.Fatalln("Missing Device Plugin configuration file.")
	}

	// get test target netns path. This path is the target netns for the PCI device.
	containerNsPath = os.Getenv("TEST_NETNS_PATH")
	if containerNsPath == "" {
		log.Fatalln("kind network namespace is not defined. Please check.")
	}

	fmt.Println("Tests are going to be run with PF", testPfName, "interfaces within KinD network namespace:", containerNsPath, "on driver:", testDriverKind)
	fmt.Println("Device Plugin configuration read from", testDpFilePath)
}

func TestSriovTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SR-IOV CNI E2E suite")
}

var _ = BeforeSuite(func(done Done) {
	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeConfigPath)
	Expect(err).Should(BeNil())

	kubeConfig = cfg

	cs = &ClientSet{}
	cs.CoreV1Interface = coreclient.NewForConfigOrDie(cfg)

	ac = &appsclient.AppsV1Client{}
	ac = appsclient.NewForConfigOrDie(cfg)

	networkClient = &NetworkClientSet{}
	networkClient.K8sCniCncfIoV1Interface = networkCoreClient.NewForConfigOrDie(cfg)

	interfaceConfigDp, err := readDpConfigurationFile(testDpFilePath)
	Expect(err).To(BeNil())
	configMapDp := configmap.CreateDevicePluginCM(testConfigMapName, *testNs, interfaceConfigDp)
	err = configmap.Apply(cs, configMapDp, timeout)
	Expect(err).To(BeNil())

	err = serviceaccount.Create(cs, serviceAccountName, *testNs)
	Expect(err).To(BeNil())

	// Will move VFs with PF to the KinD network namespace
	err = net.SetTestInterfaceNetworkNamespace(containerNsPath, testPfName)
	Expect(err).To(BeNil())

	_, err = daemonset.CreateDpDaemonset(ac, dsName, *testNs, imageName, defaultImageTag)
	Expect(err).To(BeNil())

	err = pod.WaitForPodStateRunningLabel(cs, "name=sriov-device-plugin", *testNs, timeout, interval)
	Expect(err).To(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	// Move PF with Vfs back to the Host network namespace
	err := net.SetInterfaceNamespace(containerNsPath, testPfName)
	Expect(err).To(BeNil())

	err = daemonset.Delete(ac, dsName, *testNs)
	Expect(err).To(BeNil())

	err = serviceaccount.Delete(cs, serviceAccountName, *testNs)
	Expect(err).To(BeNil())

	err = configmap.Delete(cs, testConfigMapName, *testNs, timeout)
	Expect(err).To(BeNil())
})
