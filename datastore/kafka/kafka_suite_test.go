package kafka

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKafka(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kafka Producer Suite Tests")
}
