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
			result := transformers.PayloadToMap(payload, false, logger)
			Expect(result["Vin"]).To(Equal("TEST123"))
			Expect(result["CreatedAt"]).To(Equal(now.AsTime().Format(time.RFC3339)))
		})

		It("handles nil data", func() {
			now := timestamppb.Now()
			payload := &protos.Payload{
				Data: []*protos.Datum{
					nil,
					&protos.Datum{
						Value: nil,
					},
					&protos.Datum{
						Key: protos.Field_BatteryHeaterOn,
						Value: &protos.Value{
							Value: &protos.Value_BooleanValue{BooleanValue: true},
						},
					},
				},
				Vin:       "TEST123",
				CreatedAt: now,
			}
			result := transformers.PayloadToMap(payload, false, logger)
			Expect(result["Vin"]).To(Equal("TEST123"))
			Expect(result["CreatedAt"]).To(Equal(now.AsTime().Format(time.RFC3339)))
			Expect(result["BatteryHeaterOn"]).To(Equal(true))
		})

		DescribeTable("converting datum to key-value pairs",
			func(datum *protos.Datum, includeTypes bool, expectedKey string, expectedValue interface{}) {
				payload := &protos.Payload{
					Data:      []*protos.Datum{datum},
					Vin:       "TEST123",
					CreatedAt: timestamppb.Now(),
				}
				result := transformers.PayloadToMap(payload, includeTypes, logger)
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
