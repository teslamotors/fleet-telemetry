package simple_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"

	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("ProtoLogger", func() {
	var (
		protoLogger *simple.ProtoLogger
		testLogger  *logrus.Logger
		hook        *test.Hook
		config      *simple.Config
	)

	BeforeEach(func() {
		testLogger, hook = logrus.NoOpLogger()
		config = &simple.Config{Verbose: false}
		protoLogger = simple.NewProtoLogger(config, testLogger).(*simple.ProtoLogger)
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
			record *telemetry.Record
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

			record = &telemetry.Record{
				Vin:          "TEST123",
				PayloadBytes: payloadBytes,
				TxType:       "V",
			}
		})

		It("logs data", func() {
			protoLogger.Produce(record)

			lastLog := hook.LastEntry()
			Expect(lastLog.Message).To(Equal("logger_json_unmarshal"))
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
		})

		It("logs an error when unmarshaling fails", func() {
			record.PayloadBytes = []byte("invalid payload")
			protoLogger.Produce(record)

			lastLog := hook.LastEntry()
			Expect(lastLog.Message).To(Equal("json_unmarshal_error"))
			Expect(lastLog.Data).To(HaveKeyWithValue("vin", "TEST123"))
			Expect(lastLog.Data).To(HaveKey("metadata"))
		})

		Context("when verbose set to true", func() {
			BeforeEach(func() {
				config.Verbose = true
				protoLogger = simple.NewProtoLogger(config, testLogger).(*simple.ProtoLogger)
			})

			It("does not include types in the data", func() {
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
