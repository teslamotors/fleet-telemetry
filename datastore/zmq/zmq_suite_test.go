package zmq

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZMQ(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ZMQ Producer Suite Tests")
}
