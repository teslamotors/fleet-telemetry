package simple_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"

	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("ProtoLogger", func() {
	var (
		protoLogger *simple.Producer
		testLogger  *logrus.Logger
		hook        *test.Hook
		config      *simple.Config
	)

	BeforeEach(func() {
		testLogger, hook = logrus.NoOpLogger()
		config = &simple.Config{Verbose: false}
		protoLogger = simple.NewProtoLogger(config, testLogger).(*simple.Producer)
	})

	Describe("NewProtoLogger", func() {
		It("creates a new ProtoLogger", func() {
			Expect(protoLogger).NotTo(BeNil())
			Expect(protoLogger.Config).To(Equal(config))
		})
	})

	Describe("ProcessReliableAck", func() {
		It("does not panic", func() {
			entry := &telemetry.Record{}
			Expect(func() { protoLogger.ProcessReliableAck(entry) }).NotTo(Panic())
		})
	})

	Describe("Produce", func() {
		var (
			streamMessageBytes []byte
			serializer         *telemetry.BinarySerializer
		)

		BeforeEach(func() {
			payload := &protos.Payload{
				Vin:       "TEST123",
				CreatedAt: timestamppb.New(time.Unix(0, 0)),
				Data: []*protos.Datum{
					{
						Key: protos.Field_VehicleName,
						Value: &protos.Value{
							Value: &protos.Value_StringValue{StringValue: "TestVehicle"},
						},
					},
					{
						Key: protos.Field_Gear,
						Value: &protos.Value{
							Value: &protos.Value_ShiftStateValue{ShiftStateValue: protos.ShiftState_ShiftStateD},
						},
					},
				},
			}
			payloadBytes, err := proto.Marshal(payload)
			Expect(err).NotTo(HaveOccurred())

			logger, _ := logrus.NoOpLogger()
			serializer = telemetry.NewBinarySerializer(
				&telemetry.RequestIdentity{
					DeviceID: "TEST123",
					SenderID: "vehicle_device.TEST123",
				},
				map[string][]telemetry.Producer{},
				logger,
			)
			message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.TEST123"), MessageTopic: []byte("V"), Payload: payloadBytes}
			streamMessageBytes, err = message.ToBytes()
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("logs data",
			func(useDecoded bool) {
				record, err := telemetry.NewRecord(serializer, streamMessageBytes, "1", useDecoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(record).NotTo(BeNil())

				protoLogger.Produce(record)

				lastLog := hook.LastEntry()
				Expect(lastLog.Message).To(Equal("record_payload"))
				Expect(lastLog.Data).To(HaveKeyWithValue("vin", "TEST123"))
				Expect(lastLog.Data).To(HaveKey("data"))

				data, ok := lastLog.Data["data"].(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(data).To(Equal(map[string]interface{}{
					"VehicleName": "TestVehicle",
					"Gear":        "ShiftStateD",
					"Vin":         "TEST123",
					"CreatedAt":   "1970-01-01T00:00:00Z",
				}))
			},
			Entry("record", true),
			Entry("decoded record", false),
		)

		Context("when verbose set to true", func() {
			BeforeEach(func() {
				config.Verbose = true
				protoLogger = simple.NewProtoLogger(config, testLogger).(*simple.Producer)
			})

			It("does not include types in the data", func() {
				record, err := telemetry.NewRecord(serializer, streamMessageBytes, "1", true)
				Expect(err).NotTo(HaveOccurred())
				Expect(record).NotTo(BeNil())

				protoLogger.Produce(record)

				data, ok := hook.LastEntry().Data["data"].(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(data).To(Equal(map[string]interface{}{
					"VehicleName": map[string]interface{}{"stringValue": "TestVehicle"},
					"Gear":        map[string]interface{}{"shiftStateValue": "ShiftStateD"},
					"Vin":         "TEST123",
					"CreatedAt":   "1970-01-01T00:00:00Z",
				}))
			})
		})
	})

	Describe("ReportError", func() {
		It("succeeds", func() {
			Expect(func() {
				protoLogger.ReportError("test error", nil, logrus.LogInfo{})
			}).NotTo(Panic())
		})
	})
})
