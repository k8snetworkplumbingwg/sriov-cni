package config

import (
	"net"

	"github.com/containernetworking/cni/pkg/types"
)

type NetConf struct {
	types.NetConf
	Master string `json:"master"`
}

type NetArgs struct {
	types.CommonArgs
	VF   int    `json:"vf,omitempty"`
	VLAN int    `json:"vlan,omitempty"`
	MAC  string `json:"mac,omitempty"`
	IP   net.IP `json:"ip,omitempty"`
}

type SriovConf struct {
	Net  *NetConf
	Args *NetArgs
}
