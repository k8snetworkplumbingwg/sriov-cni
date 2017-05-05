package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/containernetworking/cni/pkg/types"
)

type UnmarshallableInt int

func (i *UnmarshallableInt) UnmarshalText(data []byte) error {
	s := string(data)
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("Int unmarshal error: %v", err)
	}

	*i = UnmarshallableInt(v)
	return nil
}

type NetConf struct {
	types.NetConf
	Master string `json:"master"`
}

type NetArgs struct {
	types.CommonArgs
	VF   UnmarshallableInt          `json:"vf,omitempty"`
	VLAN UnmarshallableInt          `json:"vlan,omitempty"`
	MAC  types.UnmarshallableString `json:"mac,omitempty"`
}

type SriovConf struct {
	Net  *NetConf
	Args *NetArgs
}

func LoadConf(bytes []byte, args string) (*SriovConf, error) {
	n := &NetConf{}
	a := &NetArgs{}

	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}
	if n.Master == "" {
		return nil, fmt.Errorf(`"master" field is required. It specifies the host interface name to virtualize`)
	}

	if args != "" {
		err := types.LoadArgs(args, a)
		if err != nil {
			return nil, fmt.Errorf("failed to parse args: %v", err)
		}
	}

	return &SriovConf{
		Net:  n,
		Args: a,
	}, nil
}
