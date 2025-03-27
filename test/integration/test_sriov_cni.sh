#!/bin/bash

this_folder="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
export CNI_PATH="${this_folder}/test_utils"
export CNI_CONTAINERID=stub_container

setup() {
  ip netns del test_root_ns || true
  ip netns add test_root_ns

  # See pkg/utils/testing.go
  ip netns exec test_root_ns ip link add enp175s0f1 type dummy
  ip netns exec test_root_ns ip link add enp175s6 type dummy
  ip netns exec test_root_ns ip link add enp175s7 type dummy

  DEFAULT_CNI_DIR=$(mktemp -d)
  export DEFAULT_CNI_DIR
}

teardown() {
  # Double check the variable points to something created by the setup() function.
  if [[ $DEFAULT_CNI_DIR == "/tmp/tmp."* ]]; then
    rm -rf "$DEFAULT_CNI_DIR"
  fi
}

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

invoke_sriov_cni() {
  echo "$CNI_INPUT" | ip netns exec test_root_ns go run "${this_folder}/sriov_mocked.go"
}

create_network_ns() {
  name=$1
  delete_network_ns "$name"

  ip netns add "${name}"

  export CNI_NETNS=/run/netns/${name}
}

delete_network_ns() {
  name=$1
  ip netns del "${name}" 2>/dev/null
}

assert_file_contains() {
  file=$1
  substr=$2
  if ! grep -q "$substr" "$file"; then
    fail "File [$file] does not contains [$substr], contents: \n $(cat "$file")"
  fi
}
