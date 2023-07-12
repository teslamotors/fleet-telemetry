package telemetry_test

import (
	"crypto/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/protos"
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
	It("includes vin in body", func() {
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
		message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil)}
		recordMsg, err := message.ToBytes()
		Expect(err).To(BeNil())

		record, err := telemetry.NewRecord(serializer, recordMsg, "1")
		Expect(record).To(Not(BeNil()))
		Expect(err).To(BeNil())
		Expect(record.Serializer).To(Not(BeNil()))

		data := &protos.Payload{}
		err = proto.Unmarshal(record.Payload(), data)
		Expect(err).To(BeNil())
		Expect(data.Vin).To(Equal("42"))
	})
})

func generatePayload(vehicleName string, vin string, timestamp *timestamppb.Timestamp) []byte {
	var data []*protos.Datum
	data = append(data, &protos.Datum{
		Key: protos.Field_VehicleName,
		Value: &protos.Value{
			Value: &protos.Value_StringValue{
				StringValue: vehicleName,
			},
		},
	})
	payload, err := proto.Marshal(&protos.Payload{
		Vin:       vin,
		Data:      data,
		CreatedAt: timestamp,
	})
	Expect(err).To(BeNil())
	return payload
}
