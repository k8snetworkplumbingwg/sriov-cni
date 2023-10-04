## Configuration reference - SR-IOV CNI

The SR-IOV CNI configures networks through a CNI spec configuration object. In a Kubernetes cluster set up with Multus this object is most often delivered as a Network Attachment Definition. 


### Parameters
* `name` (string, required): the name of the network
* `type` (string, required): "sriov"
* `ipam` (dictionary, optional): IPAM configuration to be used for this network.
* `deviceID` (string, required): A valid pci address of an SRIOV NIC's VF. e.g. "0000:03:02.3"
* `vlan` (int, optional): VLAN ID to assign for the VF. Value must be in the range 0-4094 (0 for disabled, 1-4094 for valid VLAN IDs).
* `vlanQoS` (int, optional): VLAN QoS to assign for the VF. Value must be in the range 0-7. This option requires `vlan` field to be set to a non-zero value. Otherwise, the error will be returned.
* `vlanProto` (string, optional): VLAN protocol to assign for the VF. Allowed values: "802.1ad", "802.1q" (default).
* `mac` (string, optional): MAC address to assign for the VF
* `spoofchk` (string, optional): turn packet spoof checking on or off for the VF
* `trust` (string, optional): turn trust setting on or off for the VF
* `link_state` (string, optional): enforce link state for the VF. Allowed values: auto, enable, disable. Note that driver support may differ for this feature. For example, `i40e` is known to work but `igb` doesn't.
* `min_tx_rate` (int, optional): change the allowed minimum transmit bandwidth, in Mbps, for the VF. Setting this to 0 disables rate limiting. The min_tx_rate value should be <= max_tx_rate. Support of this feature depends on NICs and drivers.
* `max_tx_rate` (int, optional): change the allowed maximum transmit bandwidth, in Mbps, for the VF.
Setting this to 0 disables rate limiting.


An SR-IOV CNI config with each field filled out looks like: 

```json
{
    "cniVersion": "0.3.1",
    "name": "sriov-dpdk",
    "type": "sriovi-net",
    "deviceID": "0000:03:02.0",
    "mac": "CA:FE:C0:FF:EE:00",
    "vlan": 1000,
    "vlanQoS": 4,
    "vlanProto": "802.1ad",
    "min_tx_rate": 100,
    "max_tx_rate": 200,
    "spoofchk": "off",
    "trust": "on",
    "link_state": "enable"
}
```

### Runtime Configuration

The SR-IOV CNI accepts a MAC address when passed as a runtime configuration - that is as part of a Kubernetes Pod spec. An example pod with a runtime configuration is:

```
apiVersion: v1
kind: Pod
metadata:
  name: samplepod
  annotations:
    k8s.v1.cni.cncf.io/networks: '[
      {
        "name": "sriov-net",
        "mac": "CA:FE:C0:FF:EE:00"
      }
    ]'
spec:
  containers:
  - name: runTimeConfig
    command: ["/bin/bash", "-c", "sleep 300"]
    image: centos/tools 

```

The above config will configure a VF of type "sriov-net" with the MAC address configured as the value supplied under the 'k8s.v1.cni.cncf.io/networks'. Where the MAC address supplied is invalid the container may be created with an unexpected address.

To avoid this it's key to ensure the supplied MAC is valid for the specified interface. On some systems setting a Multicast MAC address (Where the least significant bit of the first octet is '1') results in failure to set the MAC address.
