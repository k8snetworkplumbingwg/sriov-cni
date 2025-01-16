package main

import (
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/cnicommands"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/config"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {
	customCNIDir, ok := os.LookupEnv("DEFAULT_CNI_DIR")
	if ok {
		config.DefaultCNIDir = customCNIDir
	}

	err := utils.CreateTmpSysFs()
	if err != nil {
		panic(err)
	}

	defer func() {
		err := utils.RemoveTmpSysFs()
		if err != nil {
			panic(err)
		}
	}()

	cancel, err := utils.MockNetlinkLib(config.DefaultCNIDir)
	if err != nil {
		panic(err)
	}
	defer cancel()

	cniFuncs := skel.CNIFuncs{
		Add:   cnicommands.CmdAdd,
		Del:   cnicommands.CmdDel,
		Check: cnicommands.CmdCheck,
	}
	skel.PluginMainFuncs(cniFuncs, version.All, "")
}
