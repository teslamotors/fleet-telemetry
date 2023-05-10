package telemetry_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Test dispatcher", func() {

	It("builds topic", func() {
		record := &telemetry.Record{
			TxType: "test_device",
		}
		Expect(telemetry.BuildTopic("some_namespace", record)).To(Equal("some_namespace_test_device"))
	})
})
