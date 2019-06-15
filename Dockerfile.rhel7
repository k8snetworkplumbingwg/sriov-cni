FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.10 AS builder

ADD . /usr/src/sriov-cni

WORKDIR /usr/src/sriov-cni
RUN make clean && \
    make build

FROM registry.svc.ci.openshift.org/ocp/4.0:base
COPY --from=builder /usr/src/sriov-cni/build/sriov /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="SR-IOV CNI"

ADD ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
