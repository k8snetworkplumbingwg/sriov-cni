#!/bin/bash

this_folder="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
export CNI_PATH="${this_folder}/test_utils"
export CNI_CONTAINERID=stub_container

setup() {
  ip netns del test_root_ns 2> /dev/null || true
  ip netns add test_root_ns

  # See pkg/utils/testing.go
  ip netns exec test_root_ns ip link add enp175s0f1 type dummy
  ip netns exec test_root_ns ip link add enp175s6 type dummy
  ip netns exec test_root_ns ip link add enp175s7 type dummy

  DEFAULT_CNI_DIR=$(mktemp -d "${this_folder}/tmp/default_cni_dir.XXXXX")
  export DEFAULT_CNI_DIR
}

teardown() {
  if [ -n "${INT_TEST_SKIP_CLEANUP}" ]; then
    return
  fi

  # Double check the variable points to something created by the setup() function.
  if [[ $DEFAULT_CNI_DIR == *"tmp/default_cni_dir."* ]]; then
    rm -rf "$DEFAULT_CNI_DIR"
  fi
}

assert_file_does_not_exists() {
  file=$1
  if [ -f "$file" ]; then
    fail "File [$file] exists"
  fi
}

invoke_sriov_cni() {
  echo "$CNI_INPUT" | ip netns exec test_root_ns go run -cover -covermode atomic "${this_folder}/sriov_mocked.go"
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

wait_for_file_to_exist() {
  file=$1
  
  SECONDS=0
  until [ -f "$file" ]
  do
    sleep 1
    if [[ $SECONDS -gt 20 ]]; then
      fail "File [$file] does not exists after [$SECONDS] seconds."
    fi
  done
}
