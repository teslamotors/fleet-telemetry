package telemetry

import (
	"fmt"
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
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

	logger *logrus.Logger
}

// NewBinarySerializer returns a dedicated serializer for a current socket connection
func NewBinarySerializer(requestIdentity *RequestIdentity, dispatchRules map[string][]Producer, logger *logrus.Logger) *BinarySerializer {
	return &BinarySerializer{
		DispatchRules:   dispatchRules,
		RequestIdentity: requestIdentity,
		logger:          logger,
	}
}

// Deserialize transforms a csv byte array into a Record
func (bs *BinarySerializer) Deserialize(msg []byte, socketID string) (record *Record, err error) {
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
		bs.logger.ErrorLog("set_delivered_at_bytes_error", err, logrus.LogInfo{"record_type": record.TxType, "txid": record.Txid})
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
		bs.logger.ErrorLog("unexpected_sender_id", err, logrus.LogInfo{"sender_id": string(streamMessage.SenderID), "expected_sender_id": bs.RequestIdentity.SenderID, "txid": record.Txid, "record_type": record.TxType})
		return record, fmt.Errorf("message SenderID: %s do not match vehicleID: %s", string(streamMessage.SenderID), bs.RequestIdentity.SenderID)
	}

	return record, err
}

// Ack returns an ack response
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
