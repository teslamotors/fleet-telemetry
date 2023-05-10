package telemetry

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/messages/tesla"
)

// RequestIdentity stores identifiers for the socket connection
type RequestIdentity struct {
	DeviceID string
	SenderID string
}

// BinarySerializer serializes records
type BinarySerializer struct {
	DispatchRules   map[string][]Producer
	RequestIdentity *RequestIdentity

	logger      *logrus.Logger
	reliableAck bool
}

// NewBinarySerializer returns a dedicated serializer for a current socket connection
func NewBinarySerializer(requestIdentity *RequestIdentity, dispatchRules map[string][]Producer, reliableAck bool, logger *logrus.Logger) *BinarySerializer {
	return &BinarySerializer{
		DispatchRules:   dispatchRules,
		RequestIdentity: requestIdentity,
		logger:          logger,
		reliableAck:     reliableAck,
	}
}

// Serialize transforms a csv byte array into a Record
func (bs *BinarySerializer) Serialize(msg []byte, socketID string) (record *Record, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while serializing binary hermes stream: %v", r)
		}
	}()

	record = &Record{Serializer: bs, RawBytes: msg, SocketID: socketID}
	streamMessage, err := messages.StreamMessageFromBytes(msg)
	if err != nil {
		return record, bs.guessError(record, msg)
	}

	streamMessage.SetDeliveredAt(time.Now())
	record.RawBytes, err = streamMessage.ToBytes()
	if err != nil {
		bs.Logger().Errorf("set_delivered_at_bytes_error error:%v, topic: %v, txid: %v", err, record.TxType, record.Txid)
	}

	record.TxType = streamMessage.Topic()
	record.Txid = string(streamMessage.TXID)
	record.Vin = string(bs.RequestIdentity.DeviceID)
	record.PayloadBytes = streamMessage.Payload
	record.ReceivedTimestamp = time.Now().Unix() * 1000

	if _, ok := bs.DispatchRules[streamMessage.Topic()]; ok {
		return record, nil
	}

	if string(streamMessage.SenderID) != bs.RequestIdentity.SenderID && string(streamMessage.SenderID) != bs.RequestIdentity.DeviceID {
		bs.logger.Errorf("unexpected_sender_id sender_id: %v, expected_sender_id: %v, txid: %v, topic: %v", string(streamMessage.SenderID), bs.RequestIdentity.SenderID, record.Txid, record.TxType)
		return record, fmt.Errorf("message SenderID: %s do not match vehicleID: %s", string(streamMessage.SenderID), bs.RequestIdentity.SenderID)
	}

	return record, err
}

// Ack returns a ack response
func (bs *BinarySerializer) Ack(record *Record) []byte {
	ackMessage := messages.StreamAckMessage{TXID: []byte(record.Txid), MessageTopic: []byte(record.TxType)}
	b, _ := ackMessage.ToBytes()
	return b
}

// Error returns an error response
func (bs *BinarySerializer) Error(err error, record *Record) []byte {
	ackMessage := messages.StreamMessage{TXID: []byte(record.Txid), Payload: []byte(err.Error())}
	b, _ := ackMessage.ToBytes()
	return b
}

// Dispatch pushes the record to kafka for every rule associated to it
func (bs *BinarySerializer) Dispatch(record *Record) {
	for _, producer := range bs.DispatchRules[record.TxType] {
		producer.Produce(record)
	}
}

// ReliableAck returns true if serializer supports reliable acks (only ack to car once datastore acked the data)
func (bs *BinarySerializer) ReliableAck() bool {
	return bs.reliableAck
}

// Logger returns logger for the serializer
func (bs *BinarySerializer) Logger() *logrus.Logger {
	return bs.logger
}

func (bs *BinarySerializer) guessError(record *Record, msg []byte) error {
	envelope, _, err := tesla.FlatbuffersEnvelopeFromBytes(msg)
	if err != nil {
		return &UnknownMessageType{Bytes: msg}
	}

	record.Txid = string(envelope.TxidBytes())
	return &UnknownMessageType{Txid: record.Txid, GuessedType: envelope.MessageType(), Bytes: msg}
}
