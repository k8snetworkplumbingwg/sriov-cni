package providers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"

	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

var execCommand = exec.Command

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
	if trunkingSupported := CheckTrunkSupport(); trunkingSupported == false {
		return fmt.Errorf("Vlan trunking supported only by i40e version 2.7.11 and higher, please upgrade your driver")
	}

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
	trunkFile := fmt.Sprintf(utils.TrunkFileDirectory, pfName, vfid)

	errwrite := ioutil.WriteFile(trunkFile, []byte(removeTrunk), 0644)
	if errwrite != nil {
		return fmt.Errorf("f.Write: %q", errwrite)
	}

	return nil
}

// CheckTrunkSupport checks installed driver version; trunking is supported for version 2.7.11 and higher
func CheckTrunkSupport() bool {
	var stdout bytes.Buffer
	modinfoCmd := "modinfo -F version i40e"
	cmd := execCommand("sh", "-c", modinfoCmd)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		fmt.Printf("modinfo returned error: %v %s", err, stdout.String())
		return false
	}

	stdoutSplit := strings.Split(stdout.String(), "\n")
	if len(stdoutSplit) == 0 {
		fmt.Printf("unexpected output after parsing driver version: %s", stdout.String())
		return false
	}
	driverVersion := stdoutSplit[0]
	numDots := strings.Count(driverVersion, ".")
	if numDots < 2 {
		fmt.Printf("unexpected driver version: %s", driverVersion)
		return false
	}
	//truncate driver version to only major.minor.patch version format length to ensure semver compatibility
	if numDots > 2 {
		truncVersion := strings.Split(driverVersion, ".")[:3]
		driverVersion = strings.Join(truncVersion, ".")
	}

	v1, _ := semver.NewVersion("2.7.11")
	v2, err := semver.NewVersion(driverVersion)
	if err != nil {
		fmt.Printf("invalid version error: %v %s", err, driverVersion)
		return false
	}

	if v2.Compare(v1) >= 0 {
		return true
	}

	return false
}
