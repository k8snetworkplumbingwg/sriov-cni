package dpdk

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDpdk(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dpdk Suite")
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var tmpdir, dataDir string

var _ = BeforeSuite(func() {
	var err error
	// create test sys tree
	tmpdir, err = ioutil.TempDir("/tmp", "sriovplugin-testfiles-")
	check(err)
	// switch to test sys tree
	dataDir = filepath.Join(tmpdir, "var/lib/cni/sriov")
})

var _ = AfterSuite(func() {
	var err error
	err = os.RemoveAll(tmpdir)
	check(err)
})
