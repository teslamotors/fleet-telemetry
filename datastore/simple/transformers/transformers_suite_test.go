package transformers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTransformers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Transformers Suite Tests")
}
