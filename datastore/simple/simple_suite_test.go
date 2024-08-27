package simple_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSimple(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Simple Suite Tests")
}
