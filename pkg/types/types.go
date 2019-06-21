package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	DPDKMode     bool
	Master       string
	MAC          string
	AdminMAC     string `json:"adminMAC"`
	EffectiveMAC string `json:"effectiveMAC"`
	Vlan         int    `json:"vlan"`
	DeviceID     string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID         int
	HostIFNames  string // VF netdevice name(s)
	ContIFNames  string // VF names after in the container; used during deletion
}
