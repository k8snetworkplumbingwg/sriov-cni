package ethtool

import (
	"github.com/safchain/ethtool"
)

// GetDriverInformation returns driver information for selected interfaceName
func GetDriverInformation(interfaceName string) (string, error) {
	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		return "", err
	}
	defer ethHandle.Close()

	return ethHandle.DriverName(interfaceName)
}
