package mqtt_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMQTT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MQTT Suite")
}
