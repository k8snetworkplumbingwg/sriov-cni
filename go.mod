module github.com/k8snetworkplumbingwg/sriov-cni

go 1.22.4

require (
	github.com/containernetworking/cni v1.2.0-rc0.0.20240317203738-a448e71e9867
	github.com/containernetworking/plugins v1.4.2-0.20240312120516-c860b78de419
	github.com/k8snetworkplumbingwg/cni-log v0.0.0-20230801160229-b6e062c9e0f2
	github.com/onsi/ginkgo/v2 v2.16.0
	github.com/onsi/gomega v1.31.1
	github.com/stretchr/testify v1.8.2
	github.com/vishvananda/netlink v1.2.1-beta.2.0.20240221172127-ec7bcb248e94
	golang.org/x/net v0.33.0
	golang.org/x/sys v0.28.0
)

require (
	github.com/coreos/go-iptables v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20230323073829-e72429f035bd // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/safchain/ethtool v0.3.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/onsi/ginkgo/v2 => github.com/onsi/ginkgo/v2 v2.9.2
	github.com/onsi/gomega => github.com/onsi/gomega v1.27.5
	github.com/vishvananda/netlink => github.com/vishvananda/netlink v1.2.1-beta.2.0.20240806173335-3b7e16c5f836
)
