# Integration tests

This folder contains tests and related tools to run integration test suite. These tests leverages a mocked version of the cni, which runs with a 
fake, programmed version of the `/sys` filesystem and a mocked version of `NetlinkLib`. Both are implemented in `pkg/utils/testing.go`.

The following diagram describes the interactions between the component. 

```mermaid
graph TD
  
    subgraph "Go files"
        sriovmocked["test/integration/<b>sriov-mocked.go</b>"]
        sriov["cmd/sriov/<b>main.go</b>"]
        cnicommands_pkg["CmdAdd | CmdDel"]
        sriov --- cnicommands_pkg
        subgraph "pkg/utils/testing.go"
            CreateTmpSysFs
            MockNetlinkLib
        end

        netlinkLib["<small><< lib >></small><br>github.com/vishvananda/netlink"]

    end

    sriovmocked --- cnicommands_pkg

    subgraph "Test Harness"
        calls_file[(<small><< file >><br>/tmp/x/< pf_name >.calls)]
        PF{{<small><< dummy >></small><br>PF}}
        VF1{{<small><< dummy >></small><br>VF1}}
        VF2{{<small><< dummy >></small><br>VF2}}
    end

    test_sriov_cni.sh
    test_sriov_cni.sh --> sriovmocked 
  
    cnicommands_pkg --> CreateTmpSysFs
    cnicommands_pkg --> MockNetlinkLib

    MockNetlinkLib -.write.- calls_file
    MockNetlinkLib -..- netlinkLib
    netlinkLib -..- PF
    netlinkLib -..- VF1
    netlinkLib -..- VF2

    test_sriov_cni.sh -.read.- calls_file
    test_sriov_cni.sh -.assert.- PF
    test_sriov_cni.sh -.assert.- VF1
    test_sriov_cni.sh -.assert.- VF2

    linkStyle default stroke-width:2px
    linkStyle 1,2,3,4 stroke:green,stroke-width:4px
```
