package config

import (
	"testing"
)

func TestLoadArgs(t *testing.T) {
	s := "VF=1;VLAN=10;MAC=AA:BB:CC:DD:EE:FF;IP=192.168.1.2"
	args := &NetArgs{}
	err := LoadSriovArgs(s, args)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if args.IP.String() != "192.168.1.2" {
		t.Errorf("failed to parse IP")
	}
	if args.VF != 1 {
		t.Errorf("failed to parse VF")
	}
	if args.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("failed to parse MAC")
	}
	if args.VLAN != 10 {
		t.Errorf("failed to parse VLAN")
	}

	//fmt.Printf("%#v\n%s\n", args, args.IP.String())
}

// go test -run LoadArgs
