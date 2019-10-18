package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	DPDKMode      bool
	Master        string
	MAC           string
	AdminMAC      string `json:"adminMAC"`
	EffectiveMAC  string `json:"effectiveMAC"`
	Vlan          int    `json:"vlan"`
	VlanQoS       int    `json:"vlanQoS"`
	DeviceID      string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID          int
	HostIFNames   string // VF netdevice name(s)
	ContIFNames   string // VF names after in the container; used during deletion
	MinTxRate     *int   `json:"min_tx_rate"`          // Mbps, 0 = disable rate limiting
	MaxTxRate     *int   `json:"max_tx_rate"`          // Mbps, 0 = disable rate limiting
	SpoofChk      string `json:"spoofchk,omitempty"`   // on|off
	Trust         string `json:"trust,omitempty"`      // on|off
	LinkState     string `json:"link_state,omitempty"` // auto|enable|disable
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
}
