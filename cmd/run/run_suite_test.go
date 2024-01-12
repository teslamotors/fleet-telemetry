package run

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestRun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Run suite")
}
