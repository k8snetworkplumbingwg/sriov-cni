package dpdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conf defines configuration related to dpdk driver binding/unbinding
type Conf struct {
	PCIaddr    string `json:"pci_addr"`
	Ifname     string `json:"ifname"`
	KDriver    string `json:"kernel_driver"`
	DPDKDriver string `json:"dpdk_driver"`
	DPDKtool   string `json:"dpdk_tool"`
	VFID       int    `json:"vfid"`
}

// ValidateConf vaildates dpdk configuration for required fields
func ValidateConf() error {
	return nil
}

// GetConf takes in container ID, Pod interface name and data dir as string and returns a pointer to Conf
func GetConf(cid, podIfName, dataDir string) (*Conf, error) {
	s := []string{cid, podIfName}
	cRef := strings.Join(s, "-")

	dpdkconfBytes, err := consumeScratchNetConf(cRef, dataDir)
	if err != nil {
		return nil, err
	}
	dc := &Conf{}
	if err = json.Unmarshal(dpdkconfBytes, dc); err != nil {
		return nil, fmt.Errorf("failed to parse netconf: %v", err)
	}

	return dc, nil
}

// SaveDpdkConf takes in container ID, data dir as string and a pointer to Conf then save this Conf in data dir
func SaveDpdkConf(cid, dataDir string, dc *Conf) error {
	dpdkconfBytes, err := json.Marshal(dc)
	if err != nil {
		return fmt.Errorf("error serializing delegate netconf: %v", err)
	}

	s := []string{cid, dc.Ifname}
	cRef := strings.Join(s, "-")

	// save the rendered netconf for cmdDel
	if err = saveScratchNetConf(cRef, dataDir, dpdkconfBytes); err != nil {
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

func consumeScratchNetConf(containerID, dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, containerID)
	defer os.Remove(path)

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read container data in the path(%q): %v", path, err)
	}

	return data, err
}

//https://npf.io/2015/06/testing-exec-command
var execCommand = exec.Command

// Enabledpdkmode binds an interface given as ifname string to the dpdk driver in Conf
func Enabledpdkmode(dc *Conf, ifname string, dpdkmode bool) error {
	stdout := &bytes.Buffer{}
	var driver string
	var device string

	if dpdkmode != false {
		driver = dc.DPDKDriver
		device = ifname
	} else {
		driver = dc.KDriver
		device = dc.PCIaddr
	}

	cmd := execCommand(dc.DPDKtool, "-b", driver, device)
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error binding dpdk driver %q", stdout.String())
	}

	stdout.Reset()
	return nil
}
