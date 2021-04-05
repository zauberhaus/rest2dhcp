package tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRest2dhcp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rest2dhcp Suite")
}
