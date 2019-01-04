FROM openshift/origin-release:golang-1.10 as builder

ADD . /usr/src/sriov-cni

WORKDIR /usr/src/sriov-cni
RUN ./build

FROM openshift/origin-base
COPY --from=builder /usr/src/sriov-cni/bin/sriov /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="SR-IOV CNI"

ADD ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
