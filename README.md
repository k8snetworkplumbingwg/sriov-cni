[![Build Status](https://travis-ci.org/k8snetworkplumbingwg/sriov-cni.svg?branch=master)](https://travis-ci.org/k8snetworkplumbingwg/sriov-cni) [![Go Report Card](https://goreportcard.com/badge/github.com/k8snetworkplumbingwg/sriov-cni)](https://goreportcard.com/report/github.com/k8snetworkplumbingwg/sriov-cni) [![Weekly minutes](https://img.shields.io/badge/Weekly%20Meeting%20Minutes-Mon%203pm%20GMT-blue.svg?style=plastic)](https://docs.google.com/document/d/1sJQMHbxZdeYJPgAWK1aSt6yzZ4K_8es7woVIrwinVwI)

   * [SR-IOV CNI plugin](#sr-iov-cni-plugin)
      * [Build](#build)
      * [Kubernetes Quick Start](#kubernetes-quick-start)
      * [Usage](#usage)
         * [Basic configuration parameters](#basic-configuration-parameters)
         * [Example configurations](#example-configurations)
            * [Kernel driver config](#kernel-driver-config)
            * [Advanced kernel driver config](#advanced-kernel-driver-config)
            * [DPDK userspace driver config](#dpdk-userspace-driver-config)
         * [Advanced configuration](#advanced-configuration)
      * [Contributing](#contributing)

# SR-IOV CNI plugin
This plugin enables the configuration and usage of SR-IOV VF networks in containers and orchestrators like Kubernetes.

Network Interface Cards (NICs) with [SR-IOV](http://blog.scottlowe.org/2009/12/02/what-is-sr-iov/) capabilities are managed through physical functions (PFs) and virtual functions (VFs). A PF is used by the host and usually represents a single NIC port. VF configurations are applied through the PF. With SR-IOV CNI each VF can be treated as a separate network interface, assigned to a container, and configured with it's own MAC, VLAN, IP and more.

SR-IOV CNI plugin works with [SR-IOV device plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin) for VF allocation in Kubernetes. A metaplugin such as [Multus](https://github.com/intel/multus-cni) gets the allocated VF's `deviceID`(PCI address) and is responsible for invoking the SR-IOV CNI plugin with that `deviceID`.

## Build

This plugin uses Go modules for dependency management and requires Go 1.13+ to build.

To build the plugin binary:

``
make
``

Upon successful build the plugin binary will be available in `build/sriov`.

## Kubernetes Quick Start
A full guide on orchestrating SR-IOV virtual functions in Kubernetes can be found at the [SR-IOV Device Plugin project.](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#quick-start)

Creating VFs is outside the scope of the SR-IOV CNI plugin. [More information about allocating VFs on different NICs can be found here](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/master/docs/vf-setup.md)

To deploy SR-IOV CNI by itself on a Kubernetes 1.16+ cluster:

`kubectl apply -f images/k8s-v1.16/sriov-cni-daemonset.yaml`

**Note** The above deployment is not sufficient to manage and configure SR-IOV virtual functions. [See the full orchestration guide for more information.](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#sr-iov-network-device-plugin)


## Usage
SR-IOV CNI networks are commonly configured using Multus and SR-IOV Device Plugin using Network Attachment Definitions. More information about configuring Kubernetes networks using this pattern can be found in the [Multus configuration reference document.](https://intel.github.io/multus-cni/docs/configuration.html)

A Network Attachment Definition for SR-IOV CNI takes the form:

```
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: sriov-net1
  annotations:
    k8s.v1.cni.cncf.io/resourceName: intel.com/intel_sriov_netdevice
spec:
  config: '{
  "type": "sriov",
  "cniVersion": "0.3.1",
  "name": "sriov-network",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}'
```

The `.spec.config` field contains the configuration information used by the SR-IOV CNI.

### Basic configuration parameters

The following parameters are generic parameters which are not specific to the SR-IOV CNI configuration, though (with the exception of ipam) they need to be included in the config.

* `cniVersion` : the version of the CNI spec used.
* `type` : CNI plugin used. "sriov" corresponds to SR-IOV CNI.
* `name` : the name of the network created.
* `ipam` (optional) : the configuration of the IP Address Management plugin. Required to designate an IP for a kernel interface.

### Example configurations
The following examples show the config needed to set up basic SR-IOV networking in a container. Each of the json config objects below can be placed in the `.spec.config` field of a Network Attachment Definition to integrate with Multus.

#### Kernel driver config
This is the minimum configuration for a working kernel driver interface using an SR-IOV Virtual Function. It applies an IP address using the host-local IPAM plugin in the range of the subnet provided.

```json
{
  "type": "sriov",
  "cniVersion": "0.3.1",
  "name": "sriov-network",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}
```

#### Extended kernel driver config
This configuration sets a number of extra parameters that may be key for SR-IOV networks including a vlan tag, disabled spoof checking and enabled trust mode. These parameters are commonly set in more advanced SR-IOV VF based networks.

```json
{
  "cniVersion": "0.3.1",
  "name": "sriov-advanced",
  "type": "sriov",
  "vlan": 1000,
  "spoofchk": "off",
  "trust": "on",
  "ipam": {
    "type": "host-local",
    "subnet": "10.56.217.0/24",
    "routes": [{
      "dst": "0.0.0.0/0"
    }],
    "gateway": "10.56.217.1"
  }
}
```

#### DPDK userspace driver config

The below config will configure a VF using a userspace driver (uio/vfio) for use in a container. If this plugin is used with a VF bound to a dpdk driver then the IPAM configuration will still be respected, but it will only allocate IP address(es) using the specified IPAM plugin, not apply the IP address(es) to container interface. Other config parameters should be applicable but implementation may be driver specific.

```json
{
    "cniVersion": "0.3.1",
    "name": "sriov-dpdk",
    "type": "sriov",
    "vlan": 1000
}
```

**Note** [DHCP](https://github.com/containernetworking/plugins/tree/master/plugins/ipam/dhcp) IPAM plugin can not be used for VF bound to a dpdk driver (uio/vfio).

**Note** When VLAN is not specified in the Network-Attachment-Definition, or when it is given a value of 0,
VFs connected to this network will have no vlan tag.


### Advanced Configuration

SR-IOV CNI allows the setting of other SR-IOV options such as link-state and quality of service parameters. To learn more about how these parameters are set consult the [SR-IOV CNI configuration reference guide](docs/configuration-reference.md)

## Contributing
To report a bug or request a feature, open an issue on this repo using one of the available templates.
