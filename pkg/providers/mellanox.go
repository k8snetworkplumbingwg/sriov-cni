package providers

import (
	"bytes"
	"fmt"
	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	"io/ioutil"
	"strconv"
)

//MellanoxTrunkProviderConfig stores name of the provider
type MellanoxTrunkProviderConfig struct {
	ProviderName string
	VlanData     []string
}

//NewMellanoxTrunkProviderConfig creates new Mellanox provider configuraton
func NewMellanoxTrunkProviderConfig() sriovtypes.VlanTrunkProviderConfig {
	return &MellanoxTrunkProviderConfig{
		ProviderName: "Mellanox",
	}
}

//InitConfig initializes provider configuration for given trunking ranges
func (p *MellanoxTrunkProviderConfig) InitConfig(vlanRanges *sriovtypes.VlanTrunkRangeData) {
	p.GetVlanData(vlanRanges)
	return
}

//ApplyConfig applies provider configuration
func (p *MellanoxTrunkProviderConfig) ApplyConfig(conf *sriovtypes.NetConf) error {
	if trunkingSupported := CheckVgtPlusSupport(); trunkingSupported == false {
		return fmt.Errorf("Vlan trunking is only supported by mlx5_core")
	}

	if err := EnableVgtPlus(p.VlanData, conf.Master, conf.VFID); err != nil {
		return err
	}

	return nil
}

//RemoveConfig removes configuration
func (p *MellanoxTrunkProviderConfig) RemoveConfig(conf *sriovtypes.NetConf) error {
	if err := DisableVgtPlus(p.VlanData, conf.Master, conf.VFID); err != nil {
		return err
	}

	return nil
}

//GetVlanData converts vlanRanges.VlanTrunkRanges into string
func (p *MellanoxTrunkProviderConfig) GetVlanData(vlanRanges *sriovtypes.VlanTrunkRangeData) {
	var vlanData []string
	var start, end string

	for _, vlanRange := range vlanRanges.VlanTrunkRanges {
		start = strconv.Itoa(int(vlanRange.Start))
		end = strconv.Itoa(int(vlanRange.End))
		vlanData = append(vlanData, start+" "+end)
	}
	p.VlanData = vlanData
	return
}

//EnableVgtPlus writes "add <start_vid> <end_vid>" to trunk file
func EnableVgtPlus(vlanData []string, pfName string, vfid int) error {
	for _, vlans := range vlanData {
		addTrunk := "add " + vlans
		trunkFile := fmt.Sprintf(utils.TrunkFileDirectory, pfName, vfid)

		errwrite := ioutil.WriteFile(trunkFile, []byte(addTrunk), 0644)
		if errwrite != nil {
			return fmt.Errorf("f.Write: %q", errwrite)
		}
	}
	return nil
}

//DisableVgtPlus writes "rem <start_vid> <end_vid>"  to trunk file
func DisableVgtPlus(vlanData []string, pfName string, vfid int) error {
	for _, vlans := range vlanData {
		removeTrunk := "rem " + vlans
		trunkFile := fmt.Sprintf(utils.TrunkFileDirectory, pfName, vfid)

		errwrite := ioutil.WriteFile(trunkFile, []byte(removeTrunk), 0644)
		if errwrite != nil {
			return fmt.Errorf("f.Write: %q", errwrite)
		}
	}

	return nil
}

// CheckVgtPlusSupport checks mlx5_core is installed
func CheckVgtPlusSupport() bool {
	var stdout bytes.Buffer
	modinfoCmd := "modinfo -F version mlx5_core"
	cmd := execCommand("sh", "-c", modinfoCmd)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		fmt.Printf("modinfo returned error: %v %s", err, stdout.String())
		return false
	}

	return true
}
