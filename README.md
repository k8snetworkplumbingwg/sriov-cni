# SR-IOV CNI plugin

If you do not know CNI. Please read [here](https://github.com/containernetworking/cni) at first.

NIC with [SR-IOV](http://blog.scottlowe.org/2009/12/02/what-is-sr-iov/) capabilities works by introducing the idea of physical functions (PFs) and virtual functions (VFs). 

PF is used by host.Each VFs can be treated as a separate physical NIC and assigned to one container, and configured with separate MAC, VLAN and IP, etc.

## Build

This plugin requires Go 1.5+ to build.

Go 1.5 users will need to set `GO15VENDOREXPERIMENT=1` to get vendored dependencies. This flag is set by default in 1.6.

```
#./build
```

## Enable SR-IOV

Given Intel ixgbe NIC on CentOS, Fedora or RHEL:

```
# vi /etc/modprobe.conf
options ixgbe max_vfs=8,8
```

## Network configuration reference

* `name` (string, required): the name of the network
* `type` (string, required): "sriov"
* `master` (string, required): name of the PF
* `ipam` (dictionary, required): IPAM configuration to be used for this network.

## Extra arguments

* `vf` (int, optional): VF index. This plugin will allocate a free VF if not assigned
* `vlan` (int, optional): VLAN ID for VF device
* `mac` (string, optional): mac address for VF device

## Usage

Given the following network configuration:

```
# cat > /etc/cni/net.d/10-mynet.conf <<EOF
{
    "name": "mynet",
    "type": "sriov",
    "master": "eth1",
    "ipam": {
        "type": "fixipam",
        "subnet": "10.55.206.0/26",
        "routes": [
            { "dst": "0.0.0.0/0" }
        ],
        "gateway": "10.55.206.1"
    }
}
EOF
```

Add container to network:

```sh
# CNI_PATH=`pwd`/bin
# cd scripts
# CNI_PATH=$CNI_PATH CNI_ARGS="IgnoreUnknown=1;IP=10.55.206.46;VF=1;MAC=66:d8:02:77:aa:aa" ./priv-net-run.sh ifconfig
contid=148e21a85bcc7aaf
netnspath=/var/run/netns/148e21a85bcc7aaf
eth0      Link encap:Ethernet  HWaddr 66:D8:02:77:AA:AA  
          inet addr:10.55.206.46  Bcast:0.0.0.0  Mask:255.255.255.192
          inet6 addr: fe80::64d8:2ff:fe77:aaaa/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:7 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:0 (0.0 b)  TX bytes:558 (558.0 b)

lo        Link encap:Local Loopback  
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0 
          RX bytes:0 (0.0 b)  TX bytes:0 (0.0 b)
```

Remove container from network:

```sh
# CNI_PATH=$CNI_PATH ./exec-plugins.sh del $contid /var/run/netns/$contid
```

For example:

```sh
# CNI_PATH=$CNI_PATH ./exec-plugins.sh del 148e21a85bcc7aaf /var/run/netns/148e21a85bcc7aaf
```

[More info](https://github.com/containernetworking/cni/pull/259).
