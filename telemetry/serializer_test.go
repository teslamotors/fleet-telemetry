package telemetry_test

import (
	"errors"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

type CallbackTester struct {
	counter     int
	errors      int
	reliableAck int
}

func (c *CallbackTester) Produce(entry *telemetry.Record) {
	c.counter++
}

func (c *CallbackTester) ProcessReliableAck(entry *telemetry.Record) {
	c.reliableAck++
}

func (c *CallbackTester) ReportError(message string, err error, logInfo logrus.LogInfo) {
	c.errors++
}

var _ = Describe("BinarySerializer", func() {
	DispatchKafkaGlobal := &CallbackTester{counter: 0, errors: 0, reliableAck: 0}
	DispatchRules := map[string][]telemetry.Producer{
		"T":   {DispatchKafkaGlobal},
		"D7":  {DispatchKafkaGlobal},
		"D3":  {DispatchKafkaGlobal},
		"D":   {DispatchKafkaGlobal},
		"C":   {DispatchKafkaGlobal},
		"D10": {DispatchKafkaGlobal},
		"":    {DispatchKafkaGlobal},
	}

	msg := messages.StreamMessage{
		MessageTopic: []byte("T"),
		TXID:         []byte("test-42"),
		Payload:      []byte("disiz a test"),
		SenderID:     []byte("client_type.VIN42"),
		DeviceID:     []byte("VIN42"),
	}

	type fields struct {
		DispatchRules map[string][]telemetry.Producer
		SenderID      string
		DeviceID      string
	}
	type args struct {
		msg      messages.StreamMessage
		socketID string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantRecord *telemetry.Record
		wantErr    bool
	}{
		{
			name: "T record",
			fields: fields{
				DispatchRules: DispatchRules,
				SenderID:      "client_type.VIN42",
				DeviceID:      "VIN42",
			},
			args: args{
				msg:      msg,
				socketID: "Socket-42",
			},
			wantRecord: &telemetry.Record{
				Serializer: nil,
				RawBytes:   nil,
				TxType:     "T",
				Txid:       "test-42",
				SocketID:   "Socket-42",
				Vin:        "VIN42",
			},
			wantErr: false,
		},
		{
			name: "unauthorized serializer",
			fields: fields{
				DispatchRules: make(map[string][]telemetry.Producer),
				SenderID:      "client_type.VIN42",
				DeviceID:      "VIN42",
			},
			args: args{
				msg:      msg,
				socketID: "Socket-42",
			},
			wantRecord: &telemetry.Record{
				Serializer: nil,
				RawBytes:   nil,
				TxType:     "T",
				Txid:       "test-42",
				SocketID:   "Socket-42",
				Vin:        "VIN42",
			},
			wantErr: true,
		},
		{
			name: "client verification, SenderID matches DeviceID",
			fields: fields{
				DispatchRules: DispatchRules,
				SenderID:      "client_type.VIN43",
				DeviceID:      "VIN43",
			},
			args: args{
				msg: messages.StreamMessage{
					MessageTopic: []byte("T1"),
					TXID:         []byte("test-42"),
					Payload:      []byte("disiz a test"),
					SenderID:     []byte("client_type.VIN43"),
					DeviceID:     []byte("VIN43"),
				},
				socketID: "Socket-42",
			},
			wantRecord: &telemetry.Record{
				Serializer: nil,
				RawBytes:   nil,
				TxType:     "T1",
				Txid:       "test-42",
				SocketID:   "Socket-42",
				Vin:        "VIN43",
			},
			wantErr: false,
		},
		{
			name: "client verification, SenderID matches serializer SenderID",
			fields: fields{
				DispatchRules: DispatchRules,
				SenderID:      "client_type.VIN43",
				DeviceID:      "VIN43",
			},
			args: args{
				msg: messages.StreamMessage{
					MessageTopic: []byte("T1"),
					TXID:         []byte("test-42"),
					Payload:      []byte("disiz a test"),
					SenderID:     []byte("client_type.VIN43"),
					DeviceID:     []byte("VIN43"),
				},
				socketID: "Socket-42",
			},
			wantRecord: &telemetry.Record{
				Serializer: nil,
				RawBytes:   nil,
				TxType:     "T1",
				Txid:       "test-42",
				SocketID:   "Socket-42",
				Vin:        "VIN43",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		It(tt.name, func() {
			logger, _ := logrus.NoOpLogger()
			bs := telemetry.NewBinarySerializer(&telemetry.RequestIdentity{DeviceID: tt.fields.DeviceID, SenderID: tt.fields.SenderID}, tt.fields.DispatchRules, logger)

			msgBytes, err := tt.args.msg.ToBytes()
			Expect(err).NotTo(HaveOccurred())

			gotRecord, err := bs.Deserialize(msgBytes, tt.args.socketID)
			Expect(err).NotTo(HaveOccurred())

			Expect(gotRecord.ReceivedTimestamp).NotTo(Equal(0))

			gotRecord.ReceivedTimestamp = 0
			tt.wantRecord.Serializer = bs
			Expect(gotRecord.RawBytes).NotTo(BeEmpty())
			Expect(reflect.DeepEqual(gotRecord.RawBytes, msgBytes)).To(BeFalse())

			tt.wantRecord.RawBytes = gotRecord.RawBytes
			tt.wantRecord.PayloadBytes = gotRecord.PayloadBytes

			Expect(gotRecord).To(Equal(tt.wantRecord))
		})
	}

	It("Dispatches", func() {
		var CallbackTester = &CallbackTester{counter: 0, errors: 0}

		dispatchRules := map[string][]telemetry.Producer{"T": {CallbackTester}}
		bs := &telemetry.BinarySerializer{DispatchRules: dispatchRules, RequestIdentity: &telemetry.RequestIdentity{DeviceID: "42", SenderID: "vehicle_device.42"}}
		msg := messages.StreamMessage{
			MessageTopic: []byte("T1"),
			TXID:         []byte("test-42"),
			Payload:      []byte("disiz a test"),
			SenderID:     []byte("VIN42"),
		}

		msgBytes, e := msg.ToBytes()
		Expect(e).To(BeNil())
		result, _ := bs.Deserialize(msgBytes, "Socket-42")
		bs.Dispatch(result)
		Expect(CallbackTester.counter).To(Equal(0))
		Expect(CallbackTester.errors).To(Equal(0))

		msg = messages.StreamMessage{
			MessageTopic: []byte("T"),
			TXID:         []byte("test-42"),
			Payload:      []byte("disiz a test"),
			SenderID:     []byte("VIN42"),
		}

		msgBytes, e = msg.ToBytes()
		Expect(e).To(BeNil())
		result, _ = bs.Deserialize(msgBytes, "Socket-42")
		bs.Dispatch(result)
		Expect(CallbackTester.counter).To(Equal(1))
		Expect(CallbackTester.errors).To(Equal(0))
	})

	It("Detects unknown types", func() {
		bs := &telemetry.BinarySerializer{DispatchRules: DispatchRules}

		var unknownError *telemetry.UnknownMessageType
		_, err := bs.Deserialize([]byte("test,1234,type"), "Socket-42")
		Expect(err).To(HaveOccurred())
		Expect(errors.As(err, &unknownError))
	})

	It("Serializer Acks", func() {
		bs := &telemetry.BinarySerializer{DispatchRules: DispatchRules}
		msg := &telemetry.Record{Txid: "1234", TxType: "test-topic"}

		ackBytes := bs.Ack(msg)
		result, err := messages.StreamAckMessageFromBytes(ackBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(result.TXID)).To(Equal("1234"))
		Expect(string(result.MessageTopic)).To(Equal("test-topic"))
	})

	It("Serializer Errors", func() {
		bs := &telemetry.BinarySerializer{DispatchRules: DispatchRules}
		msg := &telemetry.Record{Txid: "1234"}
		e := errors.New("a bug")

		errBytes := bs.Error(e, msg)
		result, err := messages.StreamMessageFromBytes(errBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(result.TXID)).To(Equal("1234"))
		Expect(string(result.Payload)).To(Equal("a bug"))
	})

})
