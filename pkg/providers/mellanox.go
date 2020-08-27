package providers

import (
	"fmt"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

//MellanoxTrunkProviderConfig  stores name of the provider
type MellanoxTrunkProviderConfig struct {
	ProviderName string
}

//NewMellanoxTrunkProviderConfig creates new Mellanox provider configuraton
func NewMellanoxTrunkProviderConfig() VlanTrunkProviderConfig {
	return &MellanoxTrunkProviderConfig{
		ProviderName: "Mellanox",
	}
}

//InitConfig initilizes provider configuration for given trunking ranges
func (p *MellanoxTrunkProviderConfig) InitConfig(vlanRanges *VlanTrunkRangeArray) {
	fmt.Println("Option not supported yet")
	return
}

//ApplyConfig applies provider configuration
func (p *MellanoxTrunkProviderConfig) ApplyConfig(conf *sriovtypes.NetConf) error {
	fmt.Println("Option not supported yet")
	return nil
}

//RemoveConfig removes configuration
func (p *MellanoxTrunkProviderConfig) RemoveConfig(conf *sriovtypes.NetConf) error {
	fmt.Println("Option not supported yet")
	return nil
}
