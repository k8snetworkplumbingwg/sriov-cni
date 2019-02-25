package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	DPDKMode    bool `json:"dpdkMode,omitempty"`
	Master      string
	Vlan        int    `json:"vlan"`
	DeviceID    string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID        int
	HostIFNames []string // VF netdevice name(s)
	ContIFNames []string // VF names after in the container; used during deletion
}
