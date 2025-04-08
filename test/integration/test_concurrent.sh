#!/bin/bash

# shellcheck source-path=test/integration
. libtest.sh

test_concurrent_add_calls() {
  export CNI_IFNAME=net1

  create_network_ns "container_1"
  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "ipam": { "type": "test-ipam-cni" },
    "deviceID": "0000:af:06.0",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log",
    "logLevel": "debug"
  }
EOM

  # Simulate a long CNI Add operation
  IPAM_MOCK_SLEEP=3 CNI_COMMAND=ADD invoke_sriov_cni > /dev/null &
  wait_for_file_to_exist "${DEFAULT_CNI_DIR}/pci/vf_lock/0000:af:06.0.lock"

  # Call CNI Add on the same PCI address
  create_network_ns "container_2"
  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "ipam": { "type": "test-ipam-cni" },
    "deviceID": "0000:af:06.0",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log",
    "logLevel": "debug"
  }
EOM

  export CNI_COMMAND=ADD
  output=$(invoke_sriov_cni 2>/dev/null)
  assert_matches ".*pci address 0000:af:06.0 is already allocated.*" "$output"
  wait

 }


# This test simulates a heavy load on the IPAM plugin, which takes a long time to finish. In this case, 
# the Kubelet can decide to remove the container with its network namespace while a CNI DEL command is still running.
test_long_running_ipam() {

  create_network_ns "container_1"
  export CNI_CONTAINERID=container_1
  export CNI_IFNAME=net1

  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "ipam": {
      "type": "test-ipam-cni"
    },
    "deviceID": "0000:af:06.0",
    "vlan": 0,
    "logLevel": "debug",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log"
  }
EOM

  export CNI_COMMAND=ADD
  assert invoke_sriov_cni

  # Start a long live CNI delete
  IPAM_MOCK_SLEEP=3 CNI_COMMAND=DEL invoke_sriov_cni &

  # Simulate the kubelet deleting the container and the network namespace after a timeout
  # The VF goes back to the root network namespace
  sleep 1
  ip netns exec container_1 ip link set net1 netns test_root_ns name enp175s6
  delete_network_ns container_1

  # Spawn a new container that tries to use the same device
  create_network_ns "container_2"
  export CNI_IFNAME=net1

  read -r -d '' CNI_INPUT <<- EOM
  {
    "type": "sriov",
    "cniVersion": "0.3.1",
    "name": "sriov-network",
    "vlan": 1234,
    "ipam": {
      "type": "test-ipam-cni"
    },
    "deviceID": "0000:af:06.0",
    "logLevel": "debug",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log"
  }
EOM

  export CNI_COMMAND=ADD
  assert invoke_sriov_cni
  assert_file_contains "${DEFAULT_CNI_DIR}/enp175s0f1.calls" "LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024"

  wait

  expected_vlan_set_calls=$(cat <<EOM
LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024
LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024
LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024
EOM
)
  
  assert_equals "$expected_vlan_set_calls" "$(grep LinkSetVfVlanQosProto "${DEFAULT_CNI_DIR}/enp175s0f1.calls")"
}
