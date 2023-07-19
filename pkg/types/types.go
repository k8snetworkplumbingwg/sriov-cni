package types

import (
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

// VfState represents the state of the VF
type VfState struct {
	HostIFName   string
	SpoofChk     bool
	Trust        bool
	AdminMAC     string
	EffectiveMAC string
	Vlan         int
	VlanQoS      int
	MinTxRate    int
	MaxTxRate    int
	LinkState    uint32
}

// FillFromVfInfo - Fill attributes according to the provided netlink.VfInfo struct
func (vs *VfState) FillFromVfInfo(info *netlink.VfInfo) {
	vs.AdminMAC = info.Mac.String()
	vs.LinkState = info.LinkState
	vs.MaxTxRate = int(info.MaxTxRate)
	vs.MinTxRate = int(info.MinTxRate)
	vs.Vlan = info.Vlan
	vs.VlanQoS = info.Qos
	vs.SpoofChk = info.Spoofchk
	vs.Trust = info.Trust != 0
}

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	OrigVfState   VfState // Stores the original VF state as it was prior to any operations done during cmdAdd flow
	DPDKMode      bool    `json:"-"`
	Master        string
	MAC           string
	Vlan          *int   `json:"vlan"`
	VlanQoS       *int   `json:"vlanQoS"`
	DeviceID      string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID          int
	MinTxRate     *int   `json:"min_tx_rate"`          // Mbps, 0 = disable rate limiting
	MaxTxRate     *int   `json:"max_tx_rate"`          // Mbps, 0 = disable rate limiting
	SpoofChk      string `json:"spoofchk,omitempty"`   // on|off
	Trust         string `json:"trust,omitempty"`      // on|off
	LinkState     string `json:"link_state,omitempty"` // auto|enable|disable
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
}
