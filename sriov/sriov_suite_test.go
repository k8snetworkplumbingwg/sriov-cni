package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMacvlan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "sriov Suite")
}
