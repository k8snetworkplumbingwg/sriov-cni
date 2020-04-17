package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

/*
vlan trunk support requires OOT i40e driver for Intel 700 series NICs.

FROM - i40e README
- trunk
  Supports two operations: add and rem.
  - add: adds one or more VLAN id into VF VLAN filtering.
  - rem: removes VLAN ids from the VF VLAN filtering list.
  Example 1: add multiple VLAN tags, VLANs 2,4,5,10-20, by PF, p1p2, on
  a selected VF, 1, for filtering, with the sysfs support:
  #echo add 2,4,5,10-20 > /sys/class/net/p1p2/device/sriov/1/trunk
  Example 2: remove VLANs 5, 11-13 from PF p1p2 VF 1 with sysfs:
  #echo rem 5,11-13 > /sys/class/net/p1p2/device/sriov/1/trunk
  Note: for rem, if VLAN id is not on the VLAN filtering list, the
  VLAN id will be ignored.

REGEX vaildation string: ^[0-9]+([,\-][0-9]+)*$

tested on following inputs:
10
3,5
3,200
10-30
2,4,5
2,4,5,10-20
*/

// IsValidTrunkInput checks if trunk input format valid or not
func IsValidTrunkInput(inStr string) (bool, error) {

	// Sanity check with REGEX validation
	var validString = regexp.MustCompile(`^[0-9]+([,\-][0-9]+)*$`)
	if !validString.MatchString(inStr) {
		return false, fmt.Errorf("invalid input format")
	}

	// Extract vlan numbers and check range validation
	entries := strings.Split(inStr, ",")
	for i := 0; i < len(entries); i++ {
		if strings.Contains(entries[i], "-") {
			rng := strings.Split(entries[i], "-")
			if len(rng) != 2 {
				return false, fmt.Errorf("invalid range value %s", rng)
			}
			rngSt, err := strconv.Atoi(rng[0])
			if err != nil {
				return false, fmt.Errorf("failed to parse %s vlan id, start range is incorrect", rng[0])
			}
			rngEnd, err := strconv.Atoi(rng[1])
			if err != nil {
				return false, fmt.Errorf("failed to parse %s vlan id,, end range is incorrect", rng[1])
			}

			if !isValidVlan(rngSt) || !isValidVlan(rngEnd) || rngSt >= rngEnd {
				return false, fmt.Errorf("invalid vlan range %d-%d", rngSt, rngEnd)
			}
		} else {
			vlan, err := strconv.Atoi(entries[i])
			if err != nil {
				return false, fmt.Errorf("failed to parse vlan entry %s", entries[i])
			}

			if !isValidVlan(vlan) {
				return false, fmt.Errorf("invalid vlan %d", vlan)
			}

		}
	}

	return true, nil
}

func isValidVlan(vlan int) bool {
	return vlan > 0 && vlan < 4095
}

var trunkExecCommand = exec.Command
var trunkFileFormat = "/sys/class/net/%s/device/sriov/%d/trunk"

// AddVlanTrunk add vlan trunk for given VF(vfID) for a PF 'pfName' from value given in 'value' string
func AddVlanTrunk(pfName string, vfID int, value string) error {
	var stdout bytes.Buffer
	trunkFile := fmt.Sprintf(trunkFileFormat, pfName, vfID)

	if _, err := os.Stat(trunkFile); os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", trunkFile)
	}

	trunkAddCmd := fmt.Sprintf("echo add %s > %s", value, trunkFile)

	cmd := trunkExecCommand("sh", "-c", trunkAddCmd)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("addVlanTrunk() returned error: %v %s", err, stdout.String())
	}
	return nil
}

// RemoveVlanTrunk remove vlan trunk for given VF(vfID) for a PF 'pfName' from value given in 'value' string
func RemoveVlanTrunk(pfName string, vfID int, value string) error {
	var stdout bytes.Buffer
	trunkFile := fmt.Sprintf(trunkFileFormat, pfName, vfID)
	if _, err := os.Stat(trunkFile); os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", trunkFile)
	}

	trunkRemCmd := fmt.Sprintf("echo rem %s > %s", value, trunkFile)

	cmd := trunkExecCommand("sh", "-c", trunkRemCmd)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removeVlanTrunk() returned error: %v %s", err, stdout.String())
	}
	return nil
}
