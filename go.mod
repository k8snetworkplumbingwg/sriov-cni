module github.com/k8snetworkplumbingwg/sriov-cni

go 1.23.0

toolchain go1.23.6

require (
	github.com/containernetworking/cni v1.3.0
	github.com/containernetworking/plugins v1.4.2-0.20240312120516-c860b78de419
	github.com/k8snetworkplumbingwg/cni-log v0.0.0-20230801160229-b6e062c9e0f2
	github.com/onsi/ginkgo/v2 v2.23.4
	github.com/onsi/gomega v1.36.3
	github.com/stretchr/testify v1.10.0
	github.com/vishvananda/netlink v1.2.1-beta.2.0.20240221172127-ec7bcb248e94
	golang.org/x/net v0.39.0
	golang.org/x/sys v0.32.0
)

require (
	github.com/coreos/go-iptables v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/safchain/ethtool v0.3.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.31.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/vishvananda/netlink => github.com/vishvananda/netlink v1.2.1-beta.2.0.20240806173335-3b7e16c5f836
