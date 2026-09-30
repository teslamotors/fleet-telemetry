package sns_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSNS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SNS Suite")
}
