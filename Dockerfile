FROM centos:centos7

# Add everything
ADD . /usr/src/sriov-cni

ENV INSTALL_PKGS "git golang"
RUN yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    cd /usr/src/sriov-cni && \
    ./build && \
    yum autoremove -y $INSTALL_PKGS && \
    yum clean all && \
    rm -rf /tmp/*

WORKDIR /

LABEL io.k8s.display-name="SR-IOV CNI"

ADD ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
