package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

type interfacesArray struct {
	Interfaces []interfaceConfiguration `json:"interfaces"`
}

type interfaceConfiguration struct {
	// InterfaceName interface name used to tests
	InterfaceName string `json:"interfaceName"`
	// File that contains device plugin configuration associated with provided interface
	// content of file is going to be provided to device plugin
	DpConfigFile string `json:"dpConfigFileName"`
}

const (
	interfaceConfigPath = "/usr/share/e2e/config.json"
)

var (
	interfaceConfig interfacesArray
)

func main() {
	err := readConfigurationFile(interfaceConfigPath, &interfaceConfig)
	if err != nil {
		log.Fatalln("unable to read configuration file", err)
	}

	for _, pfInterface := range interfaceConfig.Interfaces {
		err = os.Setenv("TEST_PF_NAME", pfInterface.InterfaceName)
		defer os.Unsetenv("TEST_PF_NAME")
		if err != nil {
			log.Printf("Unable to set TEST_PF_NAME >%s<, ignoring entry", pfInterface.InterfaceName)
			break
		}

		err = os.Setenv("TEST_DP_CONFIG_FILE", pfInterface.DpConfigFile)
		defer os.Unsetenv("TEST_DP_CONFIG_FILE")
		if err != nil {
			log.Printf("Unable to set TEST_DP_CONFIG_FILE >%s<, ignoring entry", pfInterface.InterfaceName)
			break
		}

		fmt.Println("________________________________________________________________________________________________________")
		fmt.Println("___________________________________ START TEST WITH", pfInterface.InterfaceName, "___________________________________________")
		fmt.Println("________________________________________________________________________________________________________")

		cmd := exec.Command("go", "test", "-v", "-timeout", "40m", "./test/e2e/...")
		output, _ := cmd.CombinedOutput()
		fmt.Println(string(output))
	}
}

func readConfigurationFile(path string, config *interfacesArray) error {
	jsonFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	bytesVal, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytesVal, config)
	if err != nil {
		return err
	}

	return nil
}
