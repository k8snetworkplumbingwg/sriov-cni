# Setting up Virtual Functions

The below gives an overview for setting up virtual functions on a host with either Intel or Mellanox NICs.


## Intel cards
Given Intel ixgbe NIC on CentOS, Fedora or RHEL:

```
# vi /etc/modprobe.conf
options ixgbe max_vfs=8,8
```

## Mellanox cards
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

