package util

import (
	"context"
	"strconv"
	"strings"
	"time"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	networkCoreClient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNetworkAttachmentDefinition returns network attachment definition
func GetNetworkAttachmentDefinition(networkName, ns string) *cniv1.NetworkAttachmentDefinition {
	return &cniv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      networkName,
			Namespace: ns,
		},
		Spec: cniv1.NetworkAttachmentDefinitionSpec{
			Config: getNetworkSpecConfig(networkName),
		},
	}
}

func getNetworkSpecConfig(networkName string) string {
	config := `{
  "type": "sriov",
  "cniVersion": "0.3.1",
  "name": "` + networkName + `",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}
	`
	return config
}

// AddNADSpecConfigProperty adds a property to Spec.Config field. New property is going to be injected before 'ipam' config
func AddNADSpecConfigProperty(nad *cniv1.NetworkAttachmentDefinition, key, value string) *cniv1.NetworkAttachmentDefinition {
	// do not modify empty configuration
	if nad.Spec.Config == "" {
		return nad
	}

	tempConfig := nad.Spec.Config
	index := strings.Index(tempConfig, "\"ipam")
	if index == -1 {
		return nad
	}

	tempConfig = tempConfig[:index] + "\"" + key + "\": \"" + value + "\",\n  " + tempConfig[index:]

	nad.Spec.Config = tempConfig

	return nad
}

// ReplaceNADIpamConfiguration replace IPAM section inside CRD configuration
func ReplaceNADIpamConfiguration(nad *cniv1.NetworkAttachmentDefinition, config string) *cniv1.NetworkAttachmentDefinition {
	// do not modify empty configuration
	if nad.Spec.Config == "" {
		return nad
	}

	tempConfig := nad.Spec.Config
	index := strings.Index(tempConfig, "\"ipam")
	if index == -1 {
		return nad
	}

	// replace ipam configuration with the one provided by the user
	tempConfig = tempConfig[:index] + config + "}"

	nad.Spec.Config = tempConfig

	return nad
}

// AddNADIntSpecConfigProperty adds a property to Spec.Config field. New property is going to be injected before 'ipam' config
func AddNADIntSpecConfigProperty(nad *cniv1.NetworkAttachmentDefinition, key string, value int) *cniv1.NetworkAttachmentDefinition {
	// do not modify empty configuration
	if nad.Spec.Config == "" {
		return nad
	}

	tempConfig := nad.Spec.Config
	index := strings.Index(tempConfig, "\"ipam")
	if index == -1 {
		return nad
	}

	tempConfig = tempConfig[:index] + "\"" + key + "\": " + strconv.Itoa(value) + ",\n  " + tempConfig[index:]

	nad.Spec.Config = tempConfig

	return nad
}

// ReplaceDefaultIP - replaces default IP (10.56.217) with the provided one
func ReplaceDefaultIP(nad *cniv1.NetworkAttachmentDefinition, network string) *cniv1.NetworkAttachmentDefinition {
	// do not modify empty configuration
	if nad.Spec.Config == "" {
		return nad
	}

	tempConfig := nad.Spec.Config
	tempConfig = strings.ReplaceAll(tempConfig, "10.56.217", network)
	nad.Spec.Config = tempConfig

	return nad
}

// AddNADAnnotation adds to given NAD a key, value pair to the annotations map
func AddNADAnnotation(nad *cniv1.NetworkAttachmentDefinition, key, value string) *cniv1.NetworkAttachmentDefinition {
	if nil == nad.Annotations {
		nad.Annotations = make(map[string]string)
	}

	nad.Annotations[key] = value

	return nad
}

// ApplyNetworkAttachmentDefinition applay network attachment definition
func ApplyNetworkAttachmentDefinition(ci networkCoreClient.K8sCniCncfIoV1Interface, nad *cniv1.NetworkAttachmentDefinition, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	_, err := ci.NetworkAttachmentDefinitions(nad.Namespace).Create(ctx, nad, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

// DeleteNetworkAttachmentDefinition delete network attachment definition
func DeleteNetworkAttachmentDefinition(ci networkCoreClient.K8sCniCncfIoV1Interface, testNetworkName string, nad *cniv1.NetworkAttachmentDefinition, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	err := ci.NetworkAttachmentDefinitions(nad.Namespace).Delete(ctx, testNetworkName, metav1.DeleteOptions{})

	if err != nil {
		return err
	}

	nad = nil

	return nil
}
