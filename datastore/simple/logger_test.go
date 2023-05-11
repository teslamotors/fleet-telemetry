package simple_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("Proto Logger", func() {
	var (
		protoLogger *simple.ProtoLogger
	)

	BeforeEach(func() {
		logger, _ := test.NewNullLogger()
		protoLogger, _ = simple.NewProtoLogger(logger).(*simple.ProtoLogger)
	})

	DescribeTable("Proto Parsing",
		func(txType string, input proto.Message, verifyOutput func(proto.Message) bool) {
			payloadBytes, err := proto.Marshal(input)
			Expect(err).To(BeNil())
			output, err := protoLogger.GetProtoMessage(&telemetry.Record{
				TxType:       txType,
				PayloadBytes: payloadBytes,
			})
			Expect(err).To(BeNil())
			Expect(verifyOutput(output)).To(BeTrue())
		},
		Entry("for txType alerts", "alerts", &protos.VehicleAlerts{Vin: "testAlertVin"}, func(msg proto.Message) bool {
			myMsg, ok := msg.(*protos.VehicleAlerts)
			if !ok {
				return false
			}
			return myMsg.GetVin() == "testAlertVin"
		}),
		Entry("for txType errors", "errors", &protos.VehicleErrors{Vin: "testErrorVin"}, func(msg proto.Message) bool {
			myMsg, ok := msg.(*protos.VehicleErrors)
			if !ok {
				return false
			}
			return myMsg.GetVin() == "testErrorVin"
		}),
		Entry("for txType V", "V", &protos.Payload{Vin: "testPayloadVIN"}, func(msg proto.Message) bool {
			myMsg, ok := msg.(*protos.Payload)
			if !ok {
				return false
			}
			return myMsg.GetVin() == "testPayloadVIN"
		}),
	)

	It("Doesn't process unknown txtype", func() {
		_, err := protoLogger.GetProtoMessage(&telemetry.Record{
			TxType: "badTxType",
		})
		Expect(err).To(MatchError("no mapping for txType: badTxType"))
	})
})
