package telemetry_test

import (
	"crypto/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Socket handler test", func() {
	It("validates the message size", func() {
		logger, _ := test.NewNullLogger()
		serializer := telemetry.NewBinarySerializer(
			&telemetry.RequestIdentity{
				DeviceID: "42",
				SenderID: "vehicle_device.42",
			},
			map[string][]telemetry.Producer{"D4": nil},
			false,
			logger,
		)
		raw := make([]byte, telemetry.SizeLimit+1)
		_, _ = rand.Read(raw)

		record, err := telemetry.NewRecord(serializer, raw, "")
		Expect(record).To(Not(BeNil()))
		Expect(err).To(Not(BeNil()))
		Expect(record.Serializer).To(Not(BeNil()))
	})
})
