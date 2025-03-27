# Integration tests

This folder contains tests and related tools to run integration test suite. These tests leverages a mocked version of the cni, which runs with a 
fake, programmed version of the `/sys` filesystem and a mocked version of `NetlinkLib`. Both are implemented in `pkg/utils/testing.go`.

The following diagram describes the interactions between the components. 

```mermaid
graph TD
  
  
    subgraph "pkg/utils/testing.go"
        CreateTmpSysFs
        MockNetlinkLib
    end
  
    sriovmocked["test/integration/sriov_mocked.go"]
    subgraph "sriov CNI"

        sriov["cmd/sriov/main.go"]
        cnicommands_pkg["CmdAdd | CmdDel"]
        sriov --- cnicommands_pkg
        
    end

    sriovmocked --- cnicommands_pkg
    sriovmocked -.setup.- CreateTmpSysFs
    sriovmocked -.setup.- MockNetlinkLib

    subgraph "System"
        calls_file[(/tmp/x/< pf_name >.calls)]
        PF{{PF << dummy >>}}
        VF1{{VF1 << dummy >>}}
        VF2{{VF2 << dummy >>}}
    end

    test_sriov_cni.sh

    test_sriov_cni.sh --> sriovmocked 
  
    cnicommands_pkg --> CreateTmpSysFs
    cnicommands_pkg --> MockNetlinkLib

    MockNetlinkLib -.write.- calls_file
    MockNetlinkLib -..- PF
    MockNetlinkLib -..- VF1
    MockNetlinkLib -..- VF2

    test_sriov_cni.sh -.read.- calls_file

    linkStyle default stroke-width:2px
    linkStyle 1,4,5,6 stroke:green,stroke-width:4px
```

Test cases in this directory are based on the https://github.com/pgrange/bash_unit framework.
