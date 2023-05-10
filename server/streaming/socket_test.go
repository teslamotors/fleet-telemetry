package streaming_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/messages/tesla"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Socket test", func() {
	var (
		conf       *config.Config
		logger     *logrus.Logger
		hook       *test.Hook
		serializer *telemetry.BinarySerializer
		sm         *streaming.SocketManager
	)

	BeforeEach(func() {
		conf = CreateTestConfig()
		logger, hook = test.NewNullLogger()
		requestIdentity := &telemetry.RequestIdentity{
			DeviceID: "42",
			SenderID: "vehicle_device.42",
		}
		serializer = telemetry.NewBinarySerializer(
			requestIdentity,
			map[string][]telemetry.Producer{"D4": nil},
			false,
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
		sm.StatsBytesPerRecords("test", 42)
		Expect(sm.RecordsStats["test"]).To(Equal(42))

		sm.StatsBytesPerRecords("test", 42)
		Expect(sm.RecordsStats["test"]).To(Equal(84))
	})

	var _ = Describe("ParseAndProcessMessage", func() {
		It("rejects text as binary", func() {
			record := []byte("D4,test,1234,{\"mydata\":42}")
			sm.ParseAndProcessRecord(serializer, record)

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))
			strMsg, err := messages.StreamMessageFromBytes(msg.Msg)
			Expect(err).To(BeNil())
			Expect(string(strMsg.Payload)).To(Equal("incorrect message format"))
		})

		It("unknown message type returns error", func() {
			sm.ParseAndProcessRecord(serializer, []byte{})

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))
			envelope, _, err := tesla.FlatbuffersEnvelopeFromBytes(msg.Msg)
			Expect(err).To(BeNil())
			Expect(envelope.MessageType()).To(Equal(tesla.MessageFlatbuffersStreamAck))
			lastLogEntry := hook.LastEntry()
			Expect(lastLogEntry.Message).To(ContainSubstring("unknown_message_type_error"))
		})

		It("returns error for unknown topic and unmatched sender", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), MessageTopic: []byte("canlogs"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).To(BeNil())

			sm.ParseAndProcessRecord(serializer, recordMsg)
			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))

			streamMessage, err := messages.StreamMessageFromBytes(msg.Msg)
			Expect(err).To(BeNil())
			Expect(string(streamMessage.Payload)).To(Equal("incorrect message format"))
			firstEntry := hook.Entries[0]
			Expect(firstEntry.Message).To(ContainSubstring("unexpected_sender_id"))
			lastLogEntry := hook.LastEntry()
			Expect(lastLogEntry.Message).To(ContainSubstring("unexpected_record"))
		})

		It("topic is verified and no matching sender", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("SOMEOTHERVIN"), MessageTopic: []byte("D4"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).To(BeNil())

			sm.ParseAndProcessRecord(serializer, recordMsg)
			Expect(hook.Entries).To(HaveLen(0))
		})

		It("topic is not verified, but sender matches", func() {
			record := messages.StreamMessage{TXID: []byte("1234"), SenderID: []byte("vehicle_device.42"), MessageTopic: []byte("canlogs"), Payload: []byte("data")}
			recordMsg, err := record.ToBytes()
			Expect(err).To(BeNil())

			sm.ParseAndProcessRecord(serializer, recordMsg)

			msg := sm.ListenToWriteChannel()
			Expect(msg.MsgType).To(Equal(2))

			streamMessage, err := messages.StreamAckMessageFromBytes(msg.Msg)
			Expect(err).To(BeNil())
			Expect(hook.Entries).To(HaveLen(0))

			Expect(string(streamMessage.MessageTopic)).To(Equal("canlogs"))
		})
	})
})
