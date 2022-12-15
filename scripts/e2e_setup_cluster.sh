#!/bin/bash

here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
RETRY_MAX=10
INTERVAL=10
TIMEOUT=60
MULTUS_DAEMONSET_URL="https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset.yml"
MULTUS_NAME="multus"
CONFIG_FILE="config.json"
CONFIG_PATH="/usr/share/e2e"

retry() {
  local status=0
  local retries=${RETRY_MAX:=5}
  local delay=${INTERVAL:=5}
  local to=${TIMEOUT:=20}
  cmd="$*"

  while [ $retries -gt 0 ]; do
    status=0
    timeout $to bash -c "echo $cmd && $cmd" || status=$?
    if [ $status -eq 0 ]; then
      break
    fi
    echo "Exit code: '$status'. Sleeping '$delay' seconds before retrying"
    sleep $delay
    ((retries--)) || true
  done
  return $status
}

check_requirements() {
  for cmd in docker kind kubectl ip; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "$cmd is not available"
      return 1
    fi
  done

  echo "### Verify no existing KinD cluster is running"
  kind_clusters=$(kind get clusters)
  if [[ "$kind_clusters" == *"kind"* ]]; then
    echo "ERROR: Please teardown existing KinD cluster"
    return 2
  fi

  if [[ ! -d "$CONFIG_PATH" ]]; then
    echo "ERROR: E2E tests configuration directory does not exist"
    return 3
  fi

  if [[ ! -r "$CONFIG_PATH"/"$CONFIG_FILE" ]]; then
    echo "ERROR: E2E configuration file read permission is missing"
    return 3
  fi
}

echo "## Checking requirements"
check_requirements || return $?

echo '## Build SRIOV-CNI'
make build

echo "## Build Docker image with KinD custom kind cluster that contains our SRIOV CNI"
retry docker build . -f scripts/Dockerfile -t e2e/custom-kind:latest

echo "## Start custom KinD cluster"
kind create cluster --config="$here"/e2e_kindConfig.yaml --image e2e/custom-kind:latest

echo "## export kube config for utilising locally"
kind export kubeconfig

echo "## Wait for coredns"
retry kubectl -n kube-system wait --for=condition=available deploy/coredns --timeout=${TIMEOUT}s

echo "## Install multus"
retry kubectl create -f "${MULTUS_DAEMONSET_URL}"
# temporary fix as multus :stable image blocks the pod in "ConteinerCreating" state
kubectl patch ds kube-multus-ds -n kube-system --type='json' -p='[{"op": "replace","path": "/spec/template/spec/containers/0/image", "value":"ghcr.io/k8snetworkplumbingwg/multus-cni:latest"}]'
retry kubectl -n kube-system wait --for=condition=ready -l name="${MULTUS_NAME}" pod --timeout="${TIMEOUT}"s

echo "## find KinD container"
kind_container="$(docker ps -q --filter 'name=kind-control-plane')"

echo "## validate KinD cluster formed"
[ "$kind_container" == "" ] && echo "could not find a KinD container 'kind-control-plane'" && return

echo "## make KinD's sysfs writable (required to create VFs)"
docker exec "$kind_container" mount -o remount,rw /sys

echo "## retrieving netns path from container"
netns_path="$(docker inspect --format '{{ .NetworkSettings.SandboxKey }}' "${kind_container}")"

echo "## exporting test netns path '${netns_path}'"
export TEST_NETNS_PATH="${netns_path}"