package main

import (
	"fmt"
	"os"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/testutils"

	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const MASTER_NAME = "eth1"

var _ = Describe("sriov Operations", func() {
	var originalNS ns.NetNS

	BeforeEach(func() {
		var err error
		originalNS, err = ns.GetCurrentNS()
		Expect(err).NotTo(HaveOccurred())

		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			// Make sure master exist
			_, err = netlink.LinkByName(MASTER_NAME)
			Expect(err).NotTo(HaveOccurred())

			// Make sure SR-IOV enabled
			vf0Dir := fmt.Sprintf("/sys/class/net/%s/device/virtfn0/net", MASTER_NAME)
			_, err = os.Lstat(vf0Dir)
			Expect(err).NotTo(HaveOccurred())

			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(originalNS.Close()).To(Succeed())
	})

	It("configures and deconfigures a sriov link with ADD/DEL", func() {
		const IFNAME = "sriovl0"

		conf := fmt.Sprintf(`{
    "name": "mynet",
    "type": "sriov",
    "master": "%s",
    "mac":"66:77:88:99:aa:bb",
    "vf": 1,
    "ipam": {
        "type": "fixipam",
        "subnet": "192.168.1.0/24",
        "gateway": "192.168.1.1"
    }
}`, MASTER_NAME)

		targetNs, err := ns.NewNS()
		Expect(err).NotTo(HaveOccurred())
		defer targetNs.Close()

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNs.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}

		// Make sure sriov link exists in the target namespace
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			os.Setenv("CNI_ARGS", "IP=192.168.1.3")
			defer func() {
				os.Unsetenv("CNI_ARGS")
			}()
			_, err := testutils.CmdAddWithResult(targetNs.Path(), IFNAME, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure sriov link exists in the target namespace
		err = targetNs.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			link, err := netlink.LinkByName(IFNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(IFNAME))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err := testutils.CmdDelWithResult(targetNs.Path(), IFNAME, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure sriov link has been deleted
		err = targetNs.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			link, err := netlink.LinkByName(IFNAME)
			Expect(err).To(HaveOccurred())
			Expect(link).To(BeNil())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
