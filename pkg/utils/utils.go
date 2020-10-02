package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/intel/sriov-cni/pkg/types"
)

var (
	sriovConfigured = "/sriov_numvfs"
	// NetDirectory sysfs net directory
	NetDirectory = "/sys/class/net"
	// DevSubDirectory device subdirectory
	DevSubDirectory = "/device/driver"
	// SysBusPci is sysfs pci device directory
	SysBusPci = "/sys/bus/pci/devices"
	// UserspaceDrivers is a list of driver names that don't have netlink representation for their devices
	UserspaceDrivers = []string{"vfio-pci", "uio_pci_generic", "igb_uio"}
	//ExecCommand used for os.exec
	execCommand = exec.Command
	// TrunkFileDirectory trunk file directoy
	TrunkFileDirectory = "/sys/class/net/%s/device/sriov/%d/trunk"
)

// GetSriovNumVfs takes in a PF name(ifName) as string and returns number of VF configured as int
func GetSriovNumVfs(ifName string) (int, error) {
	var vfTotal int

	sriovFile := filepath.Join(NetDirectory, ifName, "device", sriovConfigured)
	if _, err := os.Lstat(sriovFile); err != nil {
		return vfTotal, fmt.Errorf("failed to open the sriov_numfs of device %q: %v", ifName, err)
	}

	data, err := ioutil.ReadFile(sriovFile)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to read the sriov_numfs of device %q: %v", ifName, err)
	}

	if len(data) == 0 {
		return vfTotal, fmt.Errorf("no data in the file %q", sriovFile)
	}

	sriovNumfs := strings.TrimSpace(string(data))
	vfTotal, err = strconv.Atoi(sriovNumfs)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to convert sriov_numfs(byte value) to int of device %q: %v", ifName, err)
	}

	return vfTotal, nil
}

// GetVfid takes in VF's PCI address(addr) and pfName as string and returns VF's ID as int
func GetVfid(addr string, pfName string) (int, error) {
	var id int
	vfTotal, err := GetSriovNumVfs(pfName)
	if err != nil {
		return id, err
	}
	for vf := 0; vf <= vfTotal; vf++ {
		vfDir := filepath.Join(NetDirectory, pfName, "device", fmt.Sprintf("virtfn%d", vf))
		_, err := os.Lstat(vfDir)
		if err != nil {
			continue
		}
		pciinfo, err := os.Readlink(vfDir)
		if err != nil {
			continue
		}
		pciaddr := filepath.Base(pciinfo)
		if pciaddr == addr {
			return vf, nil
		}
	}
	return id, fmt.Errorf("unable to get VF ID with PF: %s and VF pci address %v", pfName, addr)
}

// GetPfName returns PF net device name of a given VF pci address
func GetPfName(vf string) (string, error) {
	pfSymLink := filepath.Join(SysBusPci, vf, "physfn", "net")
	_, err := os.Lstat(pfSymLink)
	if err != nil {
		return "", err
	}

	files, err := ioutil.ReadDir(pfSymLink)
	if err != nil {
		return "", err
	}

	if len(files) < 1 {
		return "", fmt.Errorf("PF network device not found")
	}

	return strings.TrimSpace(files[0].Name()), nil
}

// GetPciAddress takes in a interface(ifName) and VF id and returns returns its pci addr as string
func GetPciAddress(ifName string, vf int) (string, error) {
	var pciaddr string
	vfDir := filepath.Join(NetDirectory, ifName, "device", fmt.Sprintf("virtfn%d", vf))
	dirInfo, err := os.Lstat(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't get the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pciaddr, fmt.Errorf("No symbolic link for the virtfn%d dir of the device %q", vf, ifName)
	}

	pciinfo, err := os.Readlink(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't read the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	pciaddr = filepath.Base(pciinfo)
	return pciaddr, nil
}

// GetSharedPF takes in VF name(ifName) as string and returns the other VF name that shares same PCI address as string
func GetSharedPF(ifName string) (string, error) {
	pfName := ""
	pfDir := filepath.Join(NetDirectory, ifName)
	dirInfo, err := os.Lstat(pfDir)
	if err != nil {
		return pfName, fmt.Errorf("can't get the symbolic link of the device %q: %v", ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pfName, fmt.Errorf("No symbolic link for dir of the device %q", ifName)
	}

	fullpath, err := filepath.EvalSymlinks(pfDir)
	parentDir := fullpath[:len(fullpath)-len(ifName)]
	dirList, err := ioutil.ReadDir(parentDir)

	for _, file := range dirList {
		if file.Name() != ifName {
			pfName = file.Name()
			return pfName, nil
		}
	}

	return pfName, fmt.Errorf("Shared PF not found")
}

// GetVFLinkNames returns VF's network interface name given it's PCI addr
func GetVFLinkNames(pciAddr string) (string, error) {
	var names []string
	vfDir := filepath.Join(SysBusPci, pciAddr, "net")
	if _, err := os.Lstat(vfDir); err != nil {
		return "", err
	}

	fInfos, err := ioutil.ReadDir(vfDir)
	if err != nil {
		return "", fmt.Errorf("failed to read net dir of the device %s: %v", pciAddr, err)
	}

	if len(fInfos) == 0 {
		return "", fmt.Errorf("VF device %s sysfs path (%s) has no entries", pciAddr, vfDir)
	}

	names = make([]string, 0)
	for _, f := range fInfos {
		names = append(names, f.Name())
	}

	return names[0], nil
}

// GetVFLinkNamesFromVFID returns VF's network interface name given it's PF name as string and VF id as int
func GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	var names []string
	vfDir := filepath.Join(NetDirectory, pfName, "device", fmt.Sprintf("virtfn%d", vfID), "net")
	if _, err := os.Lstat(vfDir); err != nil {
		return nil, err
	}

	fInfos, err := ioutil.ReadDir(vfDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vfID, pfName, err)
	}

	names = make([]string, 0)
	for _, f := range fInfos {
		names = append(names, f.Name())
	}

	return names, nil
}

// HasDpdkDriver checks if a device is attached to dpdk supported driver
func HasDpdkDriver(pciAddr string) (bool, error) {
	driverLink := filepath.Join(SysBusPci, pciAddr, "driver")
	driverPath, err := filepath.EvalSymlinks(driverLink)
	if err != nil {
		return false, err
	}
	driverStat, err := os.Stat(driverPath)
	if err != nil {
		return false, err
	}
	driverName := driverStat.Name()
	for _, drv := range UserspaceDrivers {
		if driverName == drv {
			return true, nil
		}
	}
	return false, nil
}

// SaveNetConf takes in container ID, data dir and Pod interface name as string and a json encoded struct Conf
// and save this Conf in data dir
func SaveNetConf(cid, dataDir, podIfName string, conf interface{}) error {
	netConfBytes, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error serializing delegate netconf: %v", err)
	}

	s := []string{cid, podIfName}
	cRef := strings.Join(s, "-")

	// save the rendered netconf for cmdDel
	if err = saveScratchNetConf(cRef, dataDir, netConfBytes); err != nil {
		return err
	}

	return nil
}

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create the sriov data directory(%q): %v", dataDir, err)
	}

	path := filepath.Join(dataDir, containerID)

	err := ioutil.WriteFile(path, netconf, 0600)
	if err != nil {
		return fmt.Errorf("failed to write container data in the path(%q): %v", path, err)
	}

	return err
}

// ReadScratchNetConf takes in container ID, Pod interface name and data dir as string and returns a pointer to Conf
func ReadScratchNetConf(cRefPath string) ([]byte, error) {
	data, err := ioutil.ReadFile(cRefPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read container data in the path(%q): %v", cRefPath, err)
	}

	return data, err
}

// CleanCachedNetConf removed cached NetConf from disk
func CleanCachedNetConf(cRefPath string) error {
	if err := os.Remove(cRefPath); err != nil {
		return fmt.Errorf("error removing NetConf file %s: %q", cRefPath, err)
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
		fmt.Printf("modinfo returned an error: %v %s", err, stdout.String())
		return false
	}

	driverVersion := strings.Split(stdout.String(), "\n")
	v1, _ := version.NewVersion("2.7.11")
	v2, err := version.NewVersion(driverVersion[0])
	if err != nil {
		fmt.Printf("invalid version error: %v %s", err, driverVersion)
		return false
	}

	if v2.LessThan(v1) {
		return false
	}

	return true
}

//GetVlanTrunkRange creates VlanTrunkRangeData from vlanTrunkString
func GetVlanTrunkRange(vlanTrunkString string) (types.VlanTrunkRangeData, error) {

	var vlanRange = []types.VlanTrunkRange{}
	trunkingRanges := strings.Split(vlanTrunkString, ",")

	for _, r := range trunkingRanges {
		values := strings.Split(r, "-")
		v1, errconv1 := strconv.Atoi(values[0])
		v2, errconv2 := strconv.Atoi(values[len(values)-1])

		if errconv1 != nil || errconv2 != nil {
			return types.VlanTrunkRangeData{}, fmt.Errorf("Trunk range error: invalid values")
		}

		v := types.VlanTrunkRange{
			Start: uint(v1),
			End:   uint(v2),
		}

		vlanRange = append(vlanRange, v)
	}
	if err := ValidateVlanTrunkRange(vlanRange); err != nil {
		return types.VlanTrunkRangeData{}, err
	}

	vlanRanges := types.VlanTrunkRangeData{
		VlanTrunkRanges: vlanRange,
	}
	return vlanRanges, nil

}

//ValidateVlanTrunkRange checks if given vlan trunking ranges are of correct form
func ValidateVlanTrunkRange(vlanRanges []types.VlanTrunkRange) error {

	for i, r1 := range vlanRanges {
		if r1.Start > r1.End {
			return fmt.Errorf("Invalid VlanTrunk range values")
		}

		if r1.Start < 1 || r1.End > 4094 {
			return fmt.Errorf("Invalid VlanTrunk range values")
		}

		for j, r2 := range vlanRanges {
			if r1.End > r2.Start && i < j {
				return fmt.Errorf("Invalid VlanTrunk range values")
			}
		}

	}
	return nil
}

//GetVendorID returns ID of installed vendor
func GetVendorID(deviceID string) (string, error) {
	path := filepath.Join(SysBusPci, deviceID, "vendor")

	readVendor, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading vendor file %q, %q", path, err)
	}

	vendorCode := strings.Split(string(readVendor), "\n")[0]

	return vendorCode, nil
}
