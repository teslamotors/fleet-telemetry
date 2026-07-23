package streaming_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/messages/tesla"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// gaugeValue reads a gauge from the default prometheus gatherer, returning 0
// when the metric or label combination has not been reported yet.
func gaugeValue(name string, labels map[string]string) float64 {
	families, err := prom.DefaultGatherer.Gather()
	Expect(err).NotTo(HaveOccurred())

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			matched := map[string]string{}
			for _, label := range metric.GetLabel() {
				matched[label.GetName()] = label.GetValue()
			}
			if len(matched) != len(labels) {
				continue
			}
			found := true
			for key, value := range labels {
				if matched[key] != value {
					found = false
					break
				}
			}
			if found {
				return metric.GetGauge().GetValue()
			}
		}
	}
	return 0
}

var _ = Describe("Socket test", func() {
	var (
		conf            *config.Config
		logger          *logrus.Logger
		hook            *test.Hook
		serializer      *telemetry.BinarySerializer
		sm              *streaming.SocketManager
		requestIdentity *telemetry.RequestIdentity
	)

	BeforeEach(func() {
		conf = CreateTestConfig()
		logger, hook = logrus.NoOpLogger()
		requestIdentity = &telemetry.RequestIdentity{
			DeviceID: "42",
			SenderID: "vehicle_device.42",
		}
		serializer = telemetry.NewBinarySerializer(
			requestIdentity,
			map[string][]telemetry.Producer{"D4": nil},
			logger,
		)
		sm = streaming.NewSocketManager(context.Background(), requestIdentity, nil, conf, logger)
	})

	It("TestRecordsStatsToString", func() {
		logInfo := sm.RecordsStatsToLogInfo()
		Expect(logInfo["total"]).To(Equal("0"))

		sm.RecordsStats["test"] = 42
		logInfo = sm.RecordsStatsToLogInfo()

		Expect(logInfo["total"]).To(Equal("42"))
		Expect(logInfo["test"]).To(Equal("42"))
		Expect(hook.AllEntries()).To(BeEmpty())
	})

	It("TestStatsBytesPerRecords", func() {
		sm.ReportMetricBytesPerRecords("test", 42)
		Expect(sm.RecordsStats["test"]).To(Equal(42))

		sm.ReportMetricBytesPerRecords("test", 42)
		Expect(sm.RecordsStats["test"]).To(Equal(84))
	})

	var _ = Describe("ParseAndProcessMessage", func() {
		It("rejects text as binary", func() {
			record := []byte("D4,test,1234,{\"mydata\":42}")
			sm.ParseAndProcessRecord(serializer, record)

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))
			strMsg, err := messages.StreamMessageFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(strMsg.Payload)).To(Equal("incorrect message format"))
		})

		It("unknown message type returns error", func() {
			sm.ParseAndProcessRecord(serializer, []byte{})

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))
			envelope, _, err := tesla.FlatbuffersEnvelopeFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(envelope.MessageType()).To(Equal(tesla.MessageFlatbuffersStreamAck))
			lastLogEntry := hook.LastEntry()
			Expect(lastLogEntry.Message).To(ContainSubstring("unknown_message_type_error"))
		})

		It("returns error for unknown topic and unmatched sender", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), MessageTopic: []byte("canlogs"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			sm.ParseAndProcessRecord(serializer, recordMsg)
			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))

			streamMessage, err := messages.StreamMessageFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(streamMessage.Payload)).To(Equal("incorrect message format"))
			firstEntry := hook.Entries[0]
			Expect(firstEntry.Message).To(ContainSubstring("unexpected_sender_id"))
			lastLogEntry := hook.LastEntry()
			Expect(lastLogEntry.Message).To(ContainSubstring("unexpected_record"))
		})

		It("topic is verified and no matching sender", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("SOMEOTHERVIN"), MessageTopic: []byte("D4"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			sm.ParseAndProcessRecord(serializer, recordMsg)
			Expect(hook.Entries).To(HaveLen(0))
		})

		It("topic is not verified, but sender matches", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("canlogs"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			sm.ParseAndProcessRecord(serializer, recordMsg)

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))

			streamMessage, err := messages.StreamAckMessageFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(hook.Entries).To(HaveLen(0))

			Expect(string(streamMessage.MessageTopic)).To(Equal("canlogs"))
		})

		It("tracks signal usage for processed records", func() {
			conf.VinsSignalTrackingEnabled = []string{"42"}
			sm := streaming.NewSocketManager(context.Background(), requestIdentity, nil, conf, logger)

			payload := &protos.Payload{
				Data: []*protos.Datum{
					{Key: protos.Field_VehicleName, Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: "car"}}},
					{Key: protos.Field_Odometer, Value: &protos.Value{Value: &protos.Value_DoubleValue{DoubleValue: 42000}}},
				},
			}
			payloadBytes, err := proto.Marshal(payload)
			Expect(err).NotTo(HaveOccurred())

			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: payloadBytes}
			recordMsg, err := record.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			signalLabels := map[string]string{"record_type": "V"}
			vinLabels := map[string]string{"vin": "42", "record_type": "V"}
			signalsBefore := gaugeValue("signal_count", signalLabels)
			vinSignalsBefore := gaugeValue("vin_signal_count", vinLabels)

			sm.ParseAndProcessRecord(serializer, recordMsg)

			msg := sm.ListenToWriteChannel()
			_, err = messages.StreamAckMessageFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())

			Expect(gaugeValue("signal_count", signalLabels) - signalsBefore).To(Equal(2.0))
			Expect(gaugeValue("vin_signal_count", vinLabels) - vinSignalsBefore).To(Equal(2.0))
		})

		It("does not track vin signal usage for untracked vins", func() {
			payload := &protos.Payload{
				Data: []*protos.Datum{
					{Key: protos.Field_Odometer, Value: &protos.Value{Value: &protos.Value_DoubleValue{DoubleValue: 42000}}},
				},
			}
			payloadBytes, err := proto.Marshal(payload)
			Expect(err).NotTo(HaveOccurred())

			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("V"), Payload: payloadBytes}
			recordMsg, err := record.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			vinLabels := map[string]string{"vin": "42", "record_type": "V"}
			vinSignalsBefore := gaugeValue("vin_signal_count", vinLabels)

			sm.ParseAndProcessRecord(serializer, recordMsg)

			Expect(gaugeValue("vin_signal_count", vinLabels)).To(Equal(vinSignalsBefore))
		})

		It("empty network interface", func() {
			Expect(sm.GetNetworkInterface()).To(BeEmpty())
		})

		It("with request in context", func() {
			req, err := http.NewRequest("GET", "/test", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("X-Network-Interface", "cellular")
			ctx := context.WithValue(context.Background(), streaming.SocketContext, map[string]interface{}{"request": req})
			sm := streaming.NewSocketManager(ctx, requestIdentity, nil, conf, logger)
			Expect(sm.GetNetworkInterface()).To(Equal("cellular"))
		})
	})
})
