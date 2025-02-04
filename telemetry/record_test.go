package telemetry_test

import (
	"crypto/rand"
	"fmt"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Socket handler test", func() {
	var (
		logger     *logrus.Logger
		serializer *telemetry.BinarySerializer
	)

	BeforeEach(func() {
		logger, _ = logrus.NoOpLogger()
		serializer = telemetry.NewBinarySerializer(
			&telemetry.RequestIdentity{
				DeviceID: "42",
				SenderID: "vehicle_device.42",
			},
			map[string][]telemetry.Producer{"D4": nil},
			logger,
		)
	})

	It("validates the message size", func() {
		raw := make([]byte, telemetry.SizeLimit+1)
		_, _ = rand.Read(raw)

		record, err := telemetry.NewRecord(serializer, raw, "", false)
		Expect(err).To(HaveOccurred())
		Expect(record).NotTo(BeNil())
		Expect(record.Serializer).NotTo(BeNil())
	})

	It("includes vin in body", func() {
		message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil)}
		recordMsg, err := message.ToBytes()
		Expect(err).NotTo(HaveOccurred())

		record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(record).NotTo(BeNil())
		Expect(record.Serializer).NotTo(BeNil())

		data := &protos.Payload{}
		err = proto.Unmarshal(record.Payload(), data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Vin).To(Equal("42"))
	})

	It("transforms a valid string location", func() {
		loc := stringDatum(protos.Field_Location, "(37.412374 N, 122.145867 W)")
		expected := &protos.LocationValue{Latitude: 37.412374, Longitude: -122.145867}

		message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil, loc)}
		recordMsg, err := message.ToBytes()
		Expect(err).NotTo(HaveOccurred())

		record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(record).NotTo(BeNil())

		data := &protos.Payload{}
		err = proto.Unmarshal(record.Payload(), data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Data).To(HaveLen(2))

		// Give some predictability to the test
		sort.Slice(data.Data, func(i, j int) bool {
			return data.Data[i].Key < data.Data[j].Key
		})

		first := data.Data[0]
		Expect(first.Key).To(Equal(protos.Field_Location))
		Expect(first.Value.GetStringValue()).To(BeEmpty())
		Expect(first.Value.GetLocationValue()).To(Equal(clone(expected)))

		second := data.Data[1]
		Expect(second.Key).To(Equal(protos.Field_VehicleName))
	})

	It("does not transform a bogus string location", func() {
		expected := "(37.412374 X, 122.145867 W)"
		loc := stringDatum(protos.Field_Location, expected)

		message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil, loc)}
		recordMsg, err := message.ToBytes()
		Expect(err).NotTo(HaveOccurred())

		record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(record).NotTo(BeNil())

		data := &protos.Payload{}
		err = proto.Unmarshal(record.Payload(), data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Data).To(HaveLen(2))

		sort.Slice(data.Data, func(i, j int) bool {
			return data.Data[i].Key < data.Data[j].Key
		})

		first := data.Data[0]
		Expect(first.Key).To(Equal(protos.Field_Location))
		Expect(first.Value.GetStringValue()).To(Equal(expected))
		Expect(first.Value.GetLocationValue()).To(BeNil())

		second := data.Data[1]
		Expect(second.Key).To(Equal(protos.Field_VehicleName))
	})

	It("passes through a valid Location location", func() {
		expected := &protos.LocationValue{Latitude: 37.412374, Longitude: -122.145867}
		loc := locationDatum(protos.Field_Location, expected)

		message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil, loc)}
		recordMsg, err := message.ToBytes()
		Expect(err).NotTo(HaveOccurred())

		record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(record).NotTo(BeNil())

		data := &protos.Payload{}
		err = proto.Unmarshal(record.Payload(), data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Data).To(HaveLen(2))

		sort.Slice(data.Data, func(i, j int) bool {
			return data.Data[i].Key < data.Data[j].Key
		})

		first := data.Data[0]
		Expect(first.Key).To(Equal(protos.Field_Location))
		Expect(first.Value.GetStringValue()).To(BeEmpty())
		Expect(first.Value.GetLocationValue()).To(Equal(clone(expected)))

		second := data.Data[1]
		Expect(second.Key).To(Equal(protos.Field_VehicleName))
	})

	DescribeTable("number formatting fixes",
		func(in string, expected string) {
			brakePedalPos := stringDatum(protos.Field_BrakePedalPos, in)

			message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil, brakePedalPos)}
			recordMsg, err := message.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(record).NotTo(BeNil())

			data := &protos.Payload{}
			err = proto.Unmarshal(record.Payload(), data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data.Data).To(HaveLen(2))

			// Give some predictability to the test
			sort.Slice(data.Data, func(i, j int) bool {
				return data.Data[i].Key < data.Data[j].Key
			})

			first := data.Data[0]
			Expect(first.Key).To(Equal(protos.Field_VehicleName))

			second := data.Data[1]
			Expect(second.Key).To(Equal(protos.Field_BrakePedalPos))
			Expect(second.Value.GetStringValue()).To(Equal(expected))
		},
		Entry("scientific notation is reported as regular float", "1.1920928955078125e-05", "0.00001"),
		Entry("scientific notation close to zero turns to zero", "1.23e-9", "0.00000"),
		Entry("long floats are not modified", "0.00000012", "0.00000012"),
		Entry("regular floats are not modified", "0.03", "0.03"),
		Entry("integers are not modified", "3", "3"),
	)

	DescribeTable("handleAlerts",
		func(payloadTimestamp *timestamppb.Timestamp, expectedTimestamp *timestamppb.Timestamp, isActive bool) {
			alert := &protos.VehicleAlert{
				Name:      "name1",
				StartedAt: payloadTimestamp,
			}
			expectedAlert := &protos.VehicleAlert{
				Name:      "name1",
				StartedAt: expectedTimestamp,
			}

			if !isActive {
				alert.EndedAt = payloadTimestamp
				expectedAlert.EndedAt = expectedTimestamp
			}

			alerts := &protos.VehicleAlerts{
				Vin: "42",
				Alerts: []*protos.VehicleAlert{
					alert,
				}}

			expected := &protos.VehicleAlerts{
				Vin: "42",
				Alerts: []*protos.VehicleAlert{
					expectedAlert,
				}}

			msg, err := proto.Marshal(alerts)
			Expect(err).NotTo(HaveOccurred())

			message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("alerts"), Payload: msg}
			recordMsg, err := message.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(record).NotTo(BeNil())

			data := &protos.VehicleAlerts{}
			_ = proto.Unmarshal(record.Payload(), data)
			Expect(proto.Equal(data, expected)).To(BeTrue())
		},
		Entry("for active alert with microsecond timestamp", timestamppb.New(time.Unix(1692044886337, 0)), timestamppb.New(time.Unix(1692044886, 337000000)), true),
		Entry("for inactive alert with microsecond timestamp", timestamppb.New(time.Unix(1692044886337, 0)), timestamppb.New(time.Unix(1692044886, 337000000)), false),
		Entry("for active alert with regular timestamp", timestamppb.New(time.Unix(1600000000, 337000000)), timestamppb.New(time.Unix(1600000000, 337000000)), true),
		Entry("for inactive alert with regular timestamp", timestamppb.New(time.Unix(1600000000, 337000000)), timestamppb.New(time.Unix(1600000000, 337000000)), false),
	)

	DescribeTable("ParseLocation",
		func(locStr string, expected *protos.LocationValue, errRegex string) {
			loc, err := telemetry.ParseLocation(locStr)
			if errRegex == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp(errRegex))
			}
			Expect(loc).To(Equal(clone(expected)))
		},
		Entry("for a bogus string", "Abhishek is NOT a location", nil, "input does not match format"),
		Entry("for a broken location", "(37.412374 Q, 122.145867 W)", nil, "invalid location format.*"),
		Entry("for a valid loc, NW", "(37.412374 N, 122.145867 W)", &protos.LocationValue{Latitude: 37.412374, Longitude: -122.145867}, ""),
		Entry("for a valid loc, NE", "(37.412374 N, 122.145867 E)", &protos.LocationValue{Latitude: 37.412374, Longitude: 122.145867}, ""),
		Entry("for a valid loc, SW", "(37.412374 S, 122.145867 W)", &protos.LocationValue{Latitude: -37.412374, Longitude: -122.145867}, ""),
		Entry("for a valid loc, SE", "(37.412374 S, 122.145867 E)", &protos.LocationValue{Latitude: -37.412374, Longitude: 122.145867}, ""),
	)

	Describe("GetProtoMessage", func() {
		DescribeTable("valid alert types",
			func(txType string, vin string, input proto.Message, verifyOutput func(proto.Message) bool) {
				payloadBytes, err := proto.Marshal(input)
				Expect(err).NotTo(HaveOccurred())

				message := messages.StreamMessage{TXID: []byte("1234"), DeviceID: []byte(vin), SenderID: []byte(fmt.Sprintf("vehicle_device.%s", vin)), MessageTopic: []byte(txType), Payload: payloadBytes}
				recordMsg, err := message.ToBytes()
				Expect(err).NotTo(HaveOccurred())

				serializer = telemetry.NewBinarySerializer(
					&telemetry.RequestIdentity{
						DeviceID: vin,
						SenderID: fmt.Sprintf("vehicle_device.%s", vin),
					},
					map[string][]telemetry.Producer{"D4": nil},
					logger,
				)

				record, err := telemetry.NewRecord(serializer, recordMsg, "1", true)
				Expect(err).NotTo(HaveOccurred())
				output := record.GetProtoMessage()
				Expect(verifyOutput(output)).To(BeTrue())
			},
			Entry("for txType alerts", "alerts", "testAlertVin", &protos.VehicleAlerts{Vin: "testAlertVin"}, func(msg proto.Message) bool {
				myMsg, ok := msg.(*protos.VehicleAlerts)
				if !ok {
					return false
				}
				return myMsg.GetVin() == "testAlertVin"
			}),
			Entry("for txType connectivity", "connectivity", "testConnectivityVin", &protos.VehicleConnectivity{Vin: "testConnectivityVin"}, func(msg proto.Message) bool {
				myMsg, ok := msg.(*protos.VehicleConnectivity)
				if !ok {
					return false
				}
				return myMsg.GetVin() == "testConnectivityVin"
			}),
			Entry("for txType V", "V", "testPayloadVIN", &protos.Payload{Vin: "testPayloadVIN"}, func(msg proto.Message) bool {
				myMsg, ok := msg.(*protos.Payload)
				if !ok {
					return false
				}
				return myMsg.GetVin() == "testPayloadVIN"
			}),
		)

		It("json payload returns valid data when transmitDecodedRecords is false", func() {
			message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil)}
			recordMsg, err := message.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			record, err := telemetry.NewRecord(serializer, recordMsg, "1", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(record).NotTo(BeNil())

			expectedJSON := "{\"data\":[{\"key\":\"VehicleName\",\"value\":{\"stringValue\":\"cybertruck\"}}],\"createdAt\":null,\"vin\":\"42\"}"
			data, err := record.GetJSONPayload()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(MatchJSON(expectedJSON))
		})
	})

	Describe("json record", func() {
		It("outputs json with all data", func() {
			message := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: generatePayload("cybertruck", "42", nil)}
			recordMsg, err := message.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			record, err := telemetry.NewRecord(serializer, recordMsg, "1", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(record).NotTo(BeNil())

			expectedJSON := "{\"data\":[{\"key\":\"VehicleName\",\"value\":{\"stringValue\":\"cybertruck\"}}],\"createdAt\":null,\"vin\":\"42\"}"
			Expect(string(record.Payload())).To(MatchJSON(expectedJSON))

			data, err := record.GetJSONPayload()
			Expect(err).NotTo(HaveOccurred())
			Expect(record.Payload()).To(Equal(data))
		})
	})
})

func generatePayload(vehicleName string, vin string, timestamp *timestamppb.Timestamp, extraData ...*protos.Datum) []byte {
	var data []*protos.Datum
	data = append(data, stringDatum(protos.Field_VehicleName, vehicleName))
	data = append(data, extraData...)

	payload, err := proto.Marshal(&protos.Payload{
		Vin:       vin,
		Data:      data,
		CreatedAt: timestamp,
	})
	Expect(err).NotTo(HaveOccurred())
	return payload
}

func stringDatum(field protos.Field, value string) *protos.Datum {
	return &protos.Datum{
		Key: field,
		Value: &protos.Value{
			Value: &protos.Value_StringValue{
				StringValue: value,
			},
		},
	}
}

func locationDatum(field protos.Field, location *protos.LocationValue) *protos.Datum {
	return &protos.Datum{
		Key: field,
		Value: &protos.Value{
			Value: &protos.Value_LocationValue{
				LocationValue: location,
			},
		},
	}
}

// clone creates a "clean" clone of the given proto.LocationValue so we can use DeepEqual freely.
func clone(o *protos.LocationValue) *protos.LocationValue {
	if o == nil {
		return nil
	}
	return &protos.LocationValue{Latitude: o.Latitude, Longitude: o.Longitude}
}
