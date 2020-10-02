package providers

import (
	"fmt"
	"io/ioutil"
	"strconv"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
	"github.com/intel/sriov-cni/pkg/utils"
)

//IntelTrunkProviderConfig stores name of the provider
type IntelTrunkProviderConfig struct {
	ProviderName string
	VlanData     string
}

//NewIntelTrunkProviderConfig creates new Intel provider configuraton
func NewIntelTrunkProviderConfig() sriovtypes.VlanTrunkProviderConfig {
	return &IntelTrunkProviderConfig{
		ProviderName: "Intel",
	}
}

//InitConfig initializes provider configuration for given trunking ranges
func (p *IntelTrunkProviderConfig) InitConfig(vlanRanges *sriovtypes.VlanTrunkRangeData) {
	p.GetVlanData(vlanRanges)
	return
}

//ApplyConfig applies provider configuration
func (p *IntelTrunkProviderConfig) ApplyConfig(conf *sriovtypes.NetConf) error {
	if err := AddVlanFiltering(p.VlanData, conf.Master, conf.VFID); err != nil {
		return err
	}

	return nil
}

//RemoveConfig removes configuration
func (p *IntelTrunkProviderConfig) RemoveConfig(conf *sriovtypes.NetConf) error {
	if err := RemoveVlanFiltering(p.VlanData, conf.Master, conf.VFID); err != nil {
		return err
	}

	return nil
}

//GetVlanData converts vlanRanges.VlanTrunkRanges into string
func (p *IntelTrunkProviderConfig) GetVlanData(vlanRanges *sriovtypes.VlanTrunkRangeData) {
	vlanData := ""
	var start, end string

	for i, vlanRange := range vlanRanges.VlanTrunkRanges {

		start = strconv.Itoa(int(vlanRange.Start))
		end = strconv.Itoa(int(vlanRange.End))
		vlanData = vlanData + start

		if start != end {
			vlanData = vlanData + "-" + end
		}
		if i < len(vlanRanges.VlanTrunkRanges)-1 {
			vlanData = vlanData + ","
		}

	}
	p.VlanData = vlanData
	return
}

//AddVlanFiltering writes "add [trunking ranges]" to trunk file
func AddVlanFiltering(vlanData, pfName string, vfid int) error {
	addTrunk := "add " + vlanData
	trunkFile := fmt.Sprintf(utils.TrunkFileDirectory, pfName, vfid)

	errwrite := ioutil.WriteFile(trunkFile, []byte(addTrunk), 0644)
	if errwrite != nil {
		return fmt.Errorf("f.Write: %q", errwrite)
	}

	return nil
}

//RemoveVlanFiltering writes "rem [trunking ranges]"  to trunk file
func RemoveVlanFiltering(vlanData, pfName string, vfid int) error {
	removeTrunk := "rem " + vlanData
	trunkFile := fmt.Sprintf(utils.TrunkFileDirectory, pfName, strconv.Itoa(vfid))

	errwrite := ioutil.WriteFile(trunkFile, []byte(removeTrunk), 0644)
	if errwrite != nil {
		return fmt.Errorf("f.Write: %q", errwrite)
	}

	return nil
}
