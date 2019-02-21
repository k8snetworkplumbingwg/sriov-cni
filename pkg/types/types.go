package types

import (
	"github.com/containernetworking/cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/dpdk"
)

// VfInformation holds VF specific informaiton
type VfInformation struct {
	PCIaddr string `json:"pci_addr"`
	Pfname  string `json:"pfname"`
	Vfid    int    `json:"vfid"`
}

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	DPDKMode   bool
	Sharedvf   bool
	DPDKConf   *dpdk.Conf     `json:"dpdk,omitempty"`
	CNIDir     string         `json:"cniDir"`
	Master     string         `json:"master"`
	L2Mode     bool           `json:"l2enable"`
	Vlan       int            `json:"vlan"`
	DeviceID   string         `json:"deviceID"`
	DeviceInfo *VfInformation `json:"deviceinfo,omitempty"`
}
