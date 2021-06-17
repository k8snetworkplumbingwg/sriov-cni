## Dockerfile build

This is used for distribution of SR-IOV CNI binary in a Docker image.

Typically you'd build this from the root of your SR-IOV CNI clone, and you'd set the `-f` flag to specify the Dockerfile during build time. This allows the addition of the entirety of the SR-IOV CNI git clone as part of the Docker context. Use the `-f` flag with the root of the clone as the context (e.g. your current work directory would be root of git clone), such as:

```
$ docker build -t sriov-cni -f ./Dockerfile .
```

A `build_docker.sh` script is available for building the SR-IOV CNI docker image from the `./images` directory.

---

## Daemonset deployment

You may wish to deploy SR-IOV CNI as a daemonset, you can do so by starting with the example Daemonset shown here:

```
$ kubectl create -f ./images/sriov-cni-daemonset.yaml
```

Note: The likely best practice here is to build your own image given the Dockerfile, and then push it to your preferred registry, and change the `image` fields in the Daemonset YAML to reference that image.

---

### Development notes

Example docker run command:

```
$ docker run -it -v /opt/cni/bin/:/host/opt/cni/bin/ --entrypoint=/bin/bash ghcr.io/k8snetworkplumbingwg/sriov-cni
```

Originally inspired by and is a portmanteau of the [Flannel daemonset](https://github.com/coreos/flannel/blob/master/Documentation/kube-flannel.yml), the [Calico Daemonset](https://github.com/projectcalico/calico/blob/master/v2.0/getting-started/kubernetes/installation/hosted/k8s-backend-addon-manager/calico-daemonset.yaml), and the [Calico CNI install bash script](https://github.com/projectcalico/cni-plugin/blob/be4df4db2e47aa7378b1bdf6933724bac1f348d0/k8s-install/scripts/install-cni.sh#L104-L153).
