#!/usr/bin/env bash
set -e

## Build docker image
docker build -t sriov-cni -f ./Dockerfile  ../
