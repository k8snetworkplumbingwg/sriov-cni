package providers

import (
	"fmt"
	"io/ioutil"
	"os"

	sriovtypes "github.com/intel/sriov-cni/pkg/types"
)

const (
	netDir    = "/sys/class/net/"
	devDir    = "/device/driver"
	vendorDir = "/device/vendor"
)

// VlanTrunkProviderConfig provdes methods for provider configuration
type VlanTrunkProviderConfig interface {
	InitConfig(vlanRanges *VlanTrunkRangeArray)
	ApplyConfig(conf *sriovtypes.NetConf) error
	RemoveConfig(conf *sriovtypes.NetConf) error
}

//VlanTrunkRange strores trunking range
type VlanTrunkRange struct {
	Start uint
	End   uint
}

//VlanTrunkRangeArray stores an array of VlanTrunkRange
type VlanTrunkRangeArray struct {
	VlanTrunkRanges []VlanTrunkRange
}

// GetProviderConfig get Config for specific NIC
func GetProviderConfig(deviceID string) VlanTrunkProviderConfig {

	if devices, err := ioutil.ReadDir(netDir); err == nil {

		for _, iface := range devices {
			if driver, err := ioutil.ReadDir(netDir + iface.Name() + devDir); err == nil {

				for _, devID := range driver {
					if devID.Name() == deviceID {
						if f, err := os.Open(netDir + iface.Name() + vendorDir); err == nil {
							defer f.Close()

							var buff [6]byte
							_, errread := f.Read(buff[:])
							if errread != nil {
								fmt.Println("Error reading vendor file")
								return nil
							}

							if string(buff[:]) == "0x8086" {
								return NewIntelTrunkProviderConfig()
							} else if string(buff[:]) == "0x15b3" {
								return NewMellanoxTrunkProviderConfig()
							} else {
								fmt.Println("No vendor info")
								return nil
							}

						} else {
							fmt.Println("Error opening vendor file")
							return nil
						}

					}

				}

			}

		}

	}

	return nil
}
