package logging

import (
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "Logging Suite")
}

var _ = g.BeforeSuite(func() {
})

var _ = g.AfterSuite(func() {
})
