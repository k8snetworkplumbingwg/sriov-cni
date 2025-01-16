package main

import (
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/cnicommands"
)


func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}


func main() {
	cniFuncs := skel.CNIFuncs{
		Add:   cnicommands.CmdAdd,
		Del:   cnicommands.CmdDel,
		Check: cnicommands.CmdCheck,
	}
	skel.PluginMainFuncs(cniFuncs, version.All, "")
}
