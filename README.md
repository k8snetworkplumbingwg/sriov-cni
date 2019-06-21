[![Build Status](https://travis-ci.org/intel/sriov-cni.svg?branch=master)](https://travis-ci.org/intel/sriov-cni)

   * [SR-IOV CNI plugin](#sr-iov-cni-plugin)
      * [Build](#build)
      * [Enable SR-IOV](#enable-sr-iov)
         * [Intel Cards](#intel-cards)
         * [Mellanox Cards](#mellanox-cards)
      * [Configuration reference](#configuration-reference)
         * [Main parameters](#main-parameters)
         * [Using DPDK drivers:](#using-dpdk-drivers)
         * [DPDK parameters](#dpdk-parameters)
      * [Usage](#usage)
         * [Configuration with IPAM:](#configuration-with-ipam)
         * [Configuration with DPDK:](#configuration-with-dpdk)
      * [Contacts](#contacts)

# SR-IOV CNI plugin
NIC with [SR-IOV](http://blog.scottlowe.org/2009/12/02/what-is-sr-iov/) capabilities work by introducing the idea of physical functions (PFs) and virtual functions (VFs). 

A PF is used by host and VF configurations are applied through the PF. Each VF can be treated as a separate physical NIC and assigned to one container, and configured with it's own MAC, VLAN and IP, etc.

SR-IOV CNI plugin works with [SR-IOV device plugin](https://github.com/intel/sriov-network-device-plugin) for VF allocation for a container. A metaplugin such as [Multus](https://github.com/intel/multus-cni) gets the allocated VF's `deviceID`(PCI address) and responsible to invoke SR-IOV cni plugin with that `deviceID`.

## Build

This plugin requires Go 1.5+ to build.

Go 1.5 users will need to set `GO15VENDOREXPERIMENT=1` to get vendored dependencies. This flag is set by default in 1.6.

To build the plugin binary:

```
# make
```

Upon successful build the plugin binary will be available in `build/sriov`.

## Enable SR-IOV
### Intel cards
Given Intel ixgbe NIC on CentOS, Fedora or RHEL:

```
# vi /etc/modprobe.conf
options ixgbe max_vfs=8,8
```
### Mellanox cards
SRIOV-CNI support Mellanox ConnectX®-4 Lx and ConnectX®-5 adapter cards.
To enable SR-IOV functionality the following steps are required:

1- Enable SR-IOV in the NIC's Firmware.

> Installing Mellanox Management Tools (MFT) or mstflint is a pre-requisite, MFT can be downloaded from [here](http://www.mellanox.com/page/management_tools), mstflint package available in the various distros and can be downloaded from [here](https://github.com/Mellanox/mstflint).

Use Mellanox Firmware Tools package to enable and configure SR-IOV in firmware

```
# mst start
Starting MST (Mellanox Software Tools) driver set
Loading MST PCI module - Success
Loading MST PCI configuration module - Success
Create devices
```

Locate the HCA device on the desired PCI slot

```
# mst status
MST modules:
------------
    MST PCI module loaded
    MST PCI configuration module loaded
MST devices:
------------
/dev/mst/mt4115_pciconf0         - PCI configuration cycles access.
...
```

Enable SR-IOV

```
# mlxconfig -d /dev/mst/mt4115_pciconf0 q set SRIOV_EN=1 NUM_OF_VFS=8
...
Apply new Configuration? ? (y/n) [n] : y
Applying... Done!
-I- Please reboot machine to load new configurations.
```

Reboot the machine
```
# reboot
```

2- Enable SR-IOV in the NIC's Driver.

```
# ibdev2netdev
mlx5_0 port 1 ==> enp2s0f0 (Up)
mlx5_1 port 1 ==> enp2s0f1 (Up)

# echo 4 > /sys/class/net/enp2s0f0/device/sriov_numvfs
# ibdev2netdev -v
0000:02:00.0 mlx5_0 (MT4115 - MT1523X04353) CX456A - ConnectX-4 QSFP fw 12.23.1020 port 1 (ACTIVE) ==> enp2s0f0 (Up)
0000:02:00.1 mlx5_1 (MT4115 - MT1523X04353) CX456A - ConnectX-4 QSFP fw 12.23.1020 port 1 (ACTIVE) ==> enp2s0f1 (Up)
0000:02:00.5 mlx5_2 (MT4116 - NA)  fw 12.23.1020 port 1 (DOWN  ) ==> enp2s0f2 (Down)
0000:02:00.6 mlx5_3 (MT4116 - NA)  fw 12.23.1020 port 1 (DOWN  ) ==> enp2s0f3 (Down)
0000:02:00.7 mlx5_4 (MT4116 - NA)  fw 12.23.1020 port 1 (DOWN  ) ==> enp2s0f4 (Down)
0000:02:00.2 mlx5_5 (MT4116 - NA)  fw 12.23.1020 port 1 (DOWN  ) ==> enp2s0f5 (Down)

# lspci | grep Mellanox
02:00.0 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4]
02:00.1 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4]
02:00.2 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4 Virtual Function]
02:00.3 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4 Virtual Function]
02:00.4 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4 Virtual Function]
02:00.5 Ethernet controller: Mellanox Technologies MT27700 Family [ConnectX-4 Virtual Function]

# ip link show
...
enp2s0f2: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether c6:6d:7d:dd:2a:d5 brd ff:ff:ff:ff:ff:ff
enp2s0f3: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether 42:3e:07:68:da:fb brd ff:ff:ff:ff:ff:ff
enp2s0f4: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether 42:68:f2:aa:c2:27 brd ff:ff:ff:ff:ff:ff
enp2s0f5: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
...
```

To change the number of VFs reset the number to 0 then set the needed number

```
echo 0 > /sys/class/net/enp2s0f0/device/sriov_numvfs
echo 8 > /sys/class/net/enp2s0f0/device/sriov_numvfs
```

## Configuration reference
### Main parameters
* `name` (string, required): the name of the network
* `type` (string, required): "sriov"
* `deviceID` (string, required): A valid pci address of an SRIOV NIC's VF. e.g. "0000:03:02.3"
* `vlan` (int, optional): VLAN ID to assign for the VF
* `mac` (string, optional): MAC address to assign for the VF
* `ipam` (dictionary, optional): IPAM configuration to be used for this network.


### Using DPDK drivers:
If this plugin is used with a VF bound to a dpdk driver then the IPAM configuration will be ignored.

## Usage

### Configuration with IPAM:

```
# cat > /etc/cni/net.d/10-mynet.conf <<EOF
{
    "cniVersion": "0.3.1",
    "name": "sriov-net",
    "type": "sriov",
        "deviceID": "0000:03:02.0",
        "vlan": 2222,
        "ipam": {
                "type": "host-local",
                "subnet": "10.56.217.0/24",
                "rangeStart": "10.56.217.171",
                "rangeEnd": "10.56.217.181",
                "routes": [
                        { "dst": "0.0.0.0/0" }
                ],
                "gateway": "10.56.217.1"
        }
}

EOF
```

```
eth0      Link encap:Ethernet  HWaddr 66:D8:02:77:AA:AA  
          inet addr:10.55.206.46  Bcast:0.0.0.0  Mask:255.255.255.192
          inet6 addr: fe80::64d8:2ff:fe77:aaaa/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:7 errors:0 dropped:0 overruns:0 frame:0
          TX packets:14 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:530 (530.0 b)  TX bytes:988 (988.0 b)

lo        Link encap:Local Loopback  
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0 
          RX bytes:0 (0.0 b)  TX bytes:0 (0.0 b)
```

### Configuration with DPDK:

> Note: In dpdk mode IPAM configuration is ignored.

```
# cat > /etc/cni/net.d/20-mynet-dpdk.conf <<EOF
{
    "cniVersion": "0.3.1",
    "name": "sriov-dpdk",
    "type": "sriov",
    "deviceID": "0000:03:02.0",
    "vlan": 1000
}
EOF
```


[More info](https://github.com/containernetworking/cni/pull/259).

## Contacts
For any questions about Multus CNI, please reach out on github issue or feel free to contact the developers @kural OR @ahalim in our [Intel-Corp Slack](https://intel-corp.herokuapp.com/)
