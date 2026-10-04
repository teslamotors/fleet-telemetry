package transformers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("Payload", func() {
	logger, _ := logrus.NoOpLogger()
	Describe("PayloadToMap", func() {
		It("includes the vin and createdAt fields", func() {
			now := timestamppb.Now()
			payload := &protos.Payload{
				Data:      []*protos.Datum{},
				Vin:       "TEST123",
				CreatedAt: now,
			}
			result := transformers.PayloadToMap(payload, false, "", logger)
			Expect(result["Vin"]).To(Equal("TEST123"))
			Expect(result["CreatedAt"]).To(Equal(now.AsTime().Format(time.RFC3339)))
		})

		It("handles nil data", func() {
			now := timestamppb.Now()
			payload := &protos.Payload{
				Data: []*protos.Datum{
					nil,
					{
						Value: nil,
					},
					{
						Key: protos.Field_BatteryHeaterOn,
						Value: &protos.Value{
							Value: &protos.Value_BooleanValue{BooleanValue: true},
						},
					},
				},
				Vin:       "TEST123",
				CreatedAt: now,
			}
			result := transformers.PayloadToMap(payload, false, "", logger)
			Expect(result["Vin"]).To(Equal("TEST123"))
			Expect(result["CreatedAt"]).To(Equal(now.AsTime().Format(time.RFC3339)))
			Expect(result["BatteryHeaterOn"]).To(Equal(true))
		})

		It("keeps unrecognized field numbers distinct instead of collapsing them", func() {
			// A vehicle running firmware newer than this build can report field numbers
			// that are absent from the compiled Field enum. Previously every such field
			// resolved to the empty name, so they all shared one map key and only the
			// last one survived.
			now := timestamppb.Now()
			payload := &protos.Payload{
				Data: []*protos.Datum{
					{
						Key:   protos.Field(64001),
						Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "first"}},
					},
					{
						Key:   protos.Field(64002),
						Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "second"}},
					},
					{
						Key:   protos.Field_VehicleSpeed,
						Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 42}},
					},
				},
				Vin:       "TEST123",
				CreatedAt: now,
			}
			result := transformers.PayloadToMap(payload, false, "", logger)

			Expect(result).ToNot(HaveKey(""), "unrecognized fields must not share the empty key")
			Expect(result[transformers.UnknownFieldNamePrefix+"64001"]).To(Equal("first"))
			Expect(result[transformers.UnknownFieldNamePrefix+"64002"]).To(Equal("second"))
			// Recognized fields are unaffected.
			Expect(result["VehicleSpeed"]).To(Equal(float32(42)))
		})

		It("still names recognized fields, including field number zero", func() {
			payload := &protos.Payload{
				Data: []*protos.Datum{
					{
						Key:   protos.Field_Unknown,
						Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "zero"}},
					},
				},
				Vin:       "TEST123",
				CreatedAt: timestamppb.Now(),
			}
			result := transformers.PayloadToMap(payload, false, "", logger)
			// Field 0 is a real enum member named "Unknown"; it must not be renamed.
			Expect(result["Unknown"]).To(Equal("zero"))
			Expect(result).ToNot(HaveKey(transformers.UnknownFieldNamePrefix + "0"))
		})

		DescribeTable("converting datum to key-value pairs",
			func(datum *protos.Datum, includeTypes bool, expectedKey string, expectedValue interface{}) {
				payload := &protos.Payload{
					Data:      []*protos.Datum{datum},
					Vin:       "TEST123",
					CreatedAt: timestamppb.Now(),
				}
				result := transformers.PayloadToMap(payload, includeTypes, "", logger)
				Expect(result[expectedKey]).To(Equal(expectedValue))
			},
			Entry("String value with types excluded",
				&protos.Datum{
					Key:   protos.Field_VehicleName,
					Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "CyberBeast"}},
				},
				excludeTypes,
				"VehicleName",
				"CyberBeast",
			),
			Entry("String value with types included",
				&protos.Datum{
					Key:   protos.Field_VehicleName,
					Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "CyberBeast"}},
				},
				includeTypes,
				"VehicleName",
				map[string]interface{}{
					"stringValue": "CyberBeast",
				},
			),
			Entry("Integer value with types excluded",
				&protos.Datum{
					Key:   protos.Field_Odometer,
					Value: &protos.Value{Value: &protos.Value_IntValue{IntValue: 50000}},
				},
				excludeTypes,
				"Odometer",
				int32(50000),
			),
			Entry("Integer value with types included",
				&protos.Datum{
					Key:   protos.Field_Odometer,
					Value: &protos.Value{Value: &protos.Value_IntValue{IntValue: 50000}},
				},
				includeTypes,
				"Odometer",
				map[string]interface{}{
					"intValue": int32(50000),
				},
			),
			Entry("Float value with types excluded",
				&protos.Datum{
					Key:   protos.Field_BatteryLevel,
					Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 75.5}},
				},
				excludeTypes,
				"BatteryLevel",
				float32(75.5),
			),
			Entry("Float value with types included",
				&protos.Datum{
					Key:   protos.Field_BatteryLevel,
					Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 75.5}},
				},
				includeTypes,
				"BatteryLevel",
				map[string]interface{}{
					"floatValue": float32(75.5),
				},
			),
			Entry("Boolean value with types excluded",
				&protos.Datum{
					Key:   protos.Field_SentryMode,
					Value: &protos.Value{Value: &protos.Value_BooleanValue{BooleanValue: true}},
				},
				excludeTypes,
				"SentryMode",
				true,
			),
			Entry("Boolean value with types included",
				&protos.Datum{
					Key:   protos.Field_SentryMode,
					Value: &protos.Value{Value: &protos.Value_BooleanValue{BooleanValue: true}},
				},
				includeTypes,
				"SentryMode",
				map[string]interface{}{
					"booleanValue": true,
				},
			),
			Entry("ShiftState with enums as strings and types excluded",
				&protos.Datum{
					Key:   protos.Field_Gear,
					Value: &protos.Value{Value: &protos.Value_ShiftStateValue{ShiftStateValue: protos.ShiftState_ShiftStateD}},
				},
				excludeTypes,
				"Gear",
				"ShiftStateD",
			),
			Entry("ShiftState with types included",
				&protos.Datum{
					Key:   protos.Field_Gear,
					Value: &protos.Value{Value: &protos.Value_ShiftStateValue{ShiftStateValue: protos.ShiftState_ShiftStateD}},
				},
				includeTypes,
				"Gear",
				map[string]interface{}{
					"shiftStateValue": "ShiftStateD",
				},
			),
			Entry("Invalid with types excluded",
				&protos.Datum{
					Key:   protos.Field_BMSState,
					Value: &protos.Value{Value: &protos.Value_Invalid{}},
				},
				excludeTypes,
				"BMSState",
				"<invalid>",
			),
			Entry("Invalid with types included",
				&protos.Datum{
					Key:   protos.Field_BMSState,
					Value: &protos.Value{Value: &protos.Value_Invalid{}},
				},
				includeTypes,
				"BMSState",
				map[string]interface{}{
					"invalid": true,
				},
			),
		)
	})
})
