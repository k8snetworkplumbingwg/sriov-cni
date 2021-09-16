FROM kindest/node:v1.22.0

# copy sriov cni plugin
COPY build/sriov /opt/cni/bin

RUN chmod +x /opt/cni/bin/sriov
