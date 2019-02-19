FROM openshift/origin-release:golang-1.11 as builder

# Add everything
ADD . /usr/src/sriov-cni

RUN cd /usr/src/sriov-cni && make build

FROM openshift/origin-base
COPY --from=builder /usr/src/sriov-cni/build/sriov /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="SR-IOV CNI"

ADD ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
