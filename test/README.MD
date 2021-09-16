## SR-IOV CNI e2e test with KinD

### How to test e2e

To be able to execute Go tests it is necessary to create configuration files that will provide information about interface to be used in tests and configuration of device plugin. Based on that information scripts will execute tests. It is in responsibility of vendor that provides host to create those configuration files and prepare network interfaces with correct driver and sufficient number of Virtual Functions (VF). Test suite will NOT create any VFs on its own. After KinD initialization each VFs driver is checked against supported drivers array. It is not allowed to mix kernel space drivers with user space drivers on one Physical Function (PF). Vendors can update the driver array if needed by providing pull request with change. The array is defined inside **sriov-cni/test/e2e/e2e_tests_suite_test.go**. Test suite based on configuration tries to automatically detect the driver space and run tests suitable only for those drivers.

```
var (
	supportedKenrelDrivers = []string{"iavf"}
	supportedVfioDrivers   = []string{"vfio-pci"}
)
```
If VF is bind to unknown driver tests are going to failed.

The interfaces used in tests are taken from configuration file - **config.json** that should be available for the scripts at least with read permission inside **/usr/share/e2e/** folder. This file should contain an array with PF interface name and the path where SRIOV Device Plugin configuration is stored. Example below
```
[terminal]# cat /usr/share/e2e/config.json
{
  "interfaces": [
    {
      "interfaceName": "ens1000",
      "dpConfigFileName": "/usr/share/e2e/ens1000_config"
    },
    {
      "interfaceName": "ens2000",
      "dpConfigFileName": "/usr/share/e2e/ens2000_config"
    }
  ]
}
```
Storing configuration on host instead of hard coding it within test code make those tests more flexible. At the code level we do not need to take into account different Device Plugin configurations - please bear in mind that we should focus on SRIOV CNI functionality testing.

Example of the configuration file defined at **dpConfigFileName**
```
{
  "resourceList": [{
          "resourceName": "pool",
          "resourcePrefix": "vendor.com",
          "selectors": {
                  "vendors": ["8086"],
                  "devices": ["154c"],
                  "driver": ["iavf"]
          }
  }]
}
```
Please be aware that tests expects **vendor.com/pool**, so prefix and name of resource should be not changed.

It depends from the vendor what kind of host and network interfaces will be provided to execute tests.


Example steps to run test suite

```
$ git clone https://github.com/k8snetworkplumbingwg/sriov-cni
$ cd sriov-cni/
$ source scripts/e2e_get_tools.sh
$ source scripts/e2e_setup_cluster.sh
$ go run ./test/cmd/start.go
```

Scripts that creates cluster has to be sourced because they are exporting environment variables (path, network namespace) that are used by the test suite.

### How to teardown cluster

```
$ ./scripts/e2e_teardown.sh
```

### Current test cases
Test aims to verify SR-IOV CNI features on kernel Virtual Function drivers and with user-space drivers.

**Test with kernel Virtual Function driver**
* sriov_smoke_test - implements basic set of tests that should verify all functionality in one go for instance verifies if all VFs can be consumed, second network interface is available inside POD or it is possible to configure specific settings on link.


**VFIO-PCI**
* To be defined

### Troubleshooting
#### bash: echo: write error: No such device
It is possible that VFIO driver is not installed in system. Instruction: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/tree/master/docs/dpdk
