package factory

import (
	"fmt"

	"github.com/intel/sriov-cni/pkg/providers"
	"github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
)

const (
	//IntelProviderID Intel vendor ID
	IntelProviderID = "0x8086"
	//MellanoxProviderID Mellanox vendor ID
	MellanoxProviderID = "0x15b3"
)

// GetProviderConfig get Config for specific NIC
func GetProviderConfig(deviceID string) (types.VlanTrunkProviderConfig, error) {
	vendor, err := utils.GetVendorID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("GetProviderConfig Error: %q", err)
	}
	if vendor == IntelProviderID {
		return providers.NewIntelTrunkProviderConfig(), nil
	} else if vendor == MellanoxProviderID {
		// return NewMellanoxTrunkProviderConfig()
		return nil, fmt.Errorf("Mellanox is not supported")
	}

	return nil, fmt.Errorf("No supported vendor")
}
