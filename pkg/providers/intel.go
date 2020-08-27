package providers

import (
	"fmt"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

//IntelTrunkProviderConfig stores name of the provider
type IntelTrunkProviderConfig struct {
	ProviderName string
}

//NewIntelTrunkProviderConfig creates new Intel provider configuraton
func NewIntelTrunkProviderConfig() VlanTrunkProviderConfig {
	return &IntelTrunkProviderConfig{
		ProviderName: "Intel",
	}
}

//InitConfig initilizes provider configuration for given trunking ranges
func (p *IntelTrunkProviderConfig) InitConfig(vlanRanges *VlanTrunkRangeArray) {
	fmt.Println("Option not supported yet")
	return
}

//ApplyConfig applies provider configuration
func (p *IntelTrunkProviderConfig) ApplyConfig(conf *sriovtypes.NetConf) error {
	fmt.Println("Option not supported yet")
	return nil
}

//RemoveConfig removes configuration
func (p *IntelTrunkProviderConfig) RemoveConfig(conf *sriovtypes.NetConf) error {
	fmt.Println("Option not supported yet")
	return nil
}
