package utils

import (
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
)

var _ = Describe("Packets", func() {

	Context("WaitForCarrier", func() {
		It("should wait until the link has IFF_UP flag", func() {
			DeferCleanup(func(old NetlinkManager) { netLinkLib = old }, netLinkLib)

			mockedNetLink := &mocks_utils.NetlinkManager{}
			netLinkLib = mockedNetLink

			rawFlagsAtomic := new(uint32)
			*rawFlagsAtomic = unix.IFF_UP

			fakeLink := &FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:    1000,
				Name:     "dummylink",
				RawFlags: atomic.LoadUint32(rawFlagsAtomic),
			}}

			mockedNetLink.On("LinkByName", "dummylink").Return(fakeLink, nil).Run(func(_ mock.Arguments) {
				fakeLink.RawFlags = atomic.LoadUint32(rawFlagsAtomic)
			})

			hasCarrier := make(chan bool)
			go func() {
				hasCarrier <- WaitForCarrier("dummylink", 5*time.Second)
			}()

			Consistently(hasCarrier, "100ms").ShouldNot(Receive())

			go func() {
				atomic.StoreUint32(rawFlagsAtomic, unix.IFF_UP|unix.IFF_RUNNING)
			}()

			Eventually(hasCarrier, "300ms").Should(Receive())
		})
	})
})
