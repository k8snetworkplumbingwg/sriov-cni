package providers

import (
	"fmt"
	"os"
	"strconv"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

var (
	// TrunkFileDirectory trunk file directoy
	TrunkFileDirectory = "/sys/class/net/%s/device/sriov/%d/trunk"
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
	p.VlanData = GetVlanDataString(vlanRanges)
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

//GetVlanDataString converts vlanRanges.VlanTrunkRanges into string
func GetVlanDataString(vlanRanges *sriovtypes.VlanTrunkRangeData) string {
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

	return vlanData
}

//AddVlanFiltering writes "add [trunking ranges]" to trunk file
func AddVlanFiltering(vlanData, pfName string, vfid int) error {
	addTrunk := "add " + vlanData
	trunkFile := fmt.Sprintf(TrunkFileDirectory, pfName, strconv.Itoa(vfid))

	f, err := os.OpenFile(trunkFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %q", err)
	}
	defer f.Close()

	ret, errwrite := f.Write([]byte(addTrunk))
	if errwrite != nil {
		return fmt.Errorf("f.Write: %q", errwrite)
	}

	if ret != 1 {
		return fmt.Errorf("Failed to write to %q", trunkFile)
	}

	return nil
}

//RemoveVlanFiltering writes "rem [trunking ranges]"  to trunk file
func RemoveVlanFiltering(vlanData, pfName string, vfid int) error {
	removeTrunk := "rem " + vlanData
	trunkFile := fmt.Sprintf(TrunkFileDirectory, pfName, strconv.Itoa(vfid))

	f, err := os.OpenFile(trunkFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %q", err)
	}
	defer f.Close()

	ret, errwrite := f.Write([]byte(removeTrunk))
	if errwrite != nil {
		return fmt.Errorf("f.Write: %q", errwrite)
	}

	if ret != 1 {
		return fmt.Errorf("Failed to write to %q", trunkFile)
	}

	return nil
}
