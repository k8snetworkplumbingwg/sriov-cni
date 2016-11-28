package main

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"net"
)

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}

func validateRangeIP(ip net.IP, ipnet *net.IPNet) error {
	if !ipnet.Contains(ip) {
		return fmt.Errorf("%s not in network: %s", ip, ipnet)
	}
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	ipamConf, err := LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	var requestIP net.IP
	if ipamConf.Args != nil {
		requestIP = ipamConf.Args.IP
	}

	if requestIP == nil {
		return fmt.Errorf("request IP can not be empty")
	}

	gw := ipamConf.Gateway

	if gw == nil {
		return fmt.Errorf("gateway can not be empty")
	}

	if gw != nil && gw.Equal(ipamConf.Args.IP) {
		return fmt.Errorf("requested IP must differ gateway IP")
	}

	subnet := net.IPNet{
		IP:   ipamConf.Subnet.IP,
		Mask: ipamConf.Subnet.Mask,
	}
	err = validateRangeIP(requestIP, &subnet)
	if err != nil {
		return err
	}

	ipConf := &types.IPConfig{
		IP:      net.IPNet{IP: requestIP, Mask: ipamConf.Subnet.Mask},
		Gateway: gw,
		Routes:  ipamConf.Routes,
	}
	r := &types.Result{
		IP4: ipConf,
	}
	return r.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	_, err := LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
	return nil
}
