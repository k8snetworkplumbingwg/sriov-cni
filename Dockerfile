FROM golang:1.24-alpine as builder

COPY . /usr/src/sriov-cni

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

WORKDIR /usr/src/sriov-cni
RUN apk add --no-cache --virtual build-dependencies build-base=~0.5 && \
    make clean && \
    make build

FROM alpine:3
COPY --from=builder /usr/src/sriov-cni/build/sriov /usr/bin/
WORKDIR /

LABEL io.k8s.display-name="SR-IOV CNI"

COPY ./images/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
