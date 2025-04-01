#!/bin/bash

# shellcheck source-path=test/integration
. libtest.sh

test_macaddress() {

  create_network_ns "container_1"

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
    "mac": "60:00:00:00:00:E1",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log",
    "logLevel": "debug" 
  }
EOM

  export CNI_COMMAND=ADD
  assert invoke_sriov_cni
  assert 'ip netns exec container_1 ip link | grep -i 60:00:00:00:00:E1'

  export CNI_COMMAND=DEL
  assert 'invoke_sriov_cni'
  assert 'ip netns exec test_root_ns ip link show enp175s6'
}


test_vlan() {

  create_network_ns "container_1"

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
    "mac": "60:00:00:00:00:E1",
    "logLevel": "debug"
  }
EOM

  export CNI_COMMAND=ADD
  assert invoke_sriov_cni
  assert_file_contains "${DEFAULT_CNI_DIR}/enp175s0f1.calls" "LinkSetVfVlanQosProto enp175s0f1 0 1234 0 33024"

  export CNI_COMMAND=DEL
  assert invoke_sriov_cni
  assert 'ip netns exec test_root_ns ip link show enp175s6'
  assert_file_contains "${DEFAULT_CNI_DIR}/enp175s0f1.calls" "LinkSetVfVlanQosProto enp175s0f1 0 0 0 33024"
}


test_mtu_reset() {

  create_network_ns "container_1"

  assert 'ip netns exec test_root_ns ip link set mtu 3333 dev enp175s6'

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
    "mac": "60:00:00:00:00:E1",
    "logFile": "${DEFAULT_CNI_DIR}/sriov.log",
    "logLevel": "debug"
  }
EOM

  export CNI_COMMAND=ADD
  assert invoke_sriov_cni

  # Verify the VF has the correct MTU inside the container
  assert 'ip netns exec container_1 ip link | grep -i 3333'

  # Simulate an application modifying the MTU value
  assert 'ip netns exec container_1 ip link set mtu 4444 dev net1'
  assert 'ip netns exec container_1 ip link | grep -i 4444'

  export CNI_COMMAND=DEL
  assert invoke_sriov_cni
  assert 'ip netns exec test_root_ns ip link show enp175s6 | grep 3333'
}
