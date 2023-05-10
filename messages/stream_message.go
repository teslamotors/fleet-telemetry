package messages

import (
	"fmt"
	"time"

	"github.com/teslamotors/fleet-telemetry/messages/tesla"
)

// StreamMessage is struct that warps up information used for streaming communication with Tesla Cloud
type StreamMessage struct {
	fsm                *tesla.FlatbuffersStream
	envelope           *tesla.FlatbuffersEnvelope
	MessageTopic       []byte
	TXID               []byte
	SenderID           []byte
	DeviceType         []byte
	DeviceID           []byte
	DeliveredAtEpochMs uint64
	CreatedAt          uint32
	Payload            []byte
	EnvMessageID       []byte // unique id of the message
}

// StreamMessageFromBytes serializes a message from a slice of bytes
func StreamMessageFromBytes(value []byte) (*StreamMessage, error) {
	envelope, unionTable, err := tesla.FlatbuffersEnvelopeFromBytes(value)
	if err != nil {
		return nil, err
	}

	if envelope.MessageType() != tesla.MessageFlatbuffersStream {
		return nil, fmt.Errorf("unable to load stream message(%v) from topic: %v, type: %v", string(envelope.TxidBytes()), string(envelope.TopicBytes()), envelope.MessageType())
	}

	fsm := new(tesla.FlatbuffersStream)
	fsm.Init(unionTable.Bytes, unionTable.Pos)

	return &StreamMessage{
		fsm:                fsm,
		envelope:           envelope,
		MessageTopic:       envelope.TopicBytes(),
		TXID:               envelope.TxidBytes(),
		SenderID:           fsm.SenderIdBytes(),
		DeviceType:         fsm.DeviceTypeBytes(),
		DeviceID:           fsm.DeviceIdBytes(),
		DeliveredAtEpochMs: fsm.DeliveredAtEpochMs(),
		CreatedAt:          fsm.CreatedAt(),
		Payload:            fsm.PayloadBytes(),
		EnvMessageID:       envelope.MessageIdBytes(),
	}, nil
}

// ToBytes deserializes a message into a slice of bytes
func (m *StreamMessage) ToBytes() ([]byte, error) {
	if m.fsm == nil {
		b := tesla.FlatbuffersStreamToBytes(m.SenderID, m.MessageTopic, m.TXID, m.Payload, m.CreatedAt, m.EnvMessageID, m.DeviceType, m.DeviceID, m.DeliveredAtEpochMs)
		return b, nil
	}

	return m.fsm.ToBytes(m.envelope), nil
}

// MsgType returns the type of message
func (m *StreamMessage) MsgType() byte {
	if m.envelope == nil {
		return tesla.MessageFlatbuffersStream
	}
	return m.envelope.MessageType()
}

// Topic returns the topic of the message
func (m *StreamMessage) Topic() string {
	return string(m.MessageTopic)
}

// Txid returns the type of message
func (m *StreamMessage) Txid() []byte {
	return m.TXID
}

// IsExpired returns always returns false
func (m *StreamMessage) IsExpired() bool {
	return false
}

// SetSenderID is meant to be called only by hermesClient before sending the message --  Use SetIdentity instead
// Set skip to true for sending anonymized data
func (m *StreamMessage) SetSenderID(senderID string) {
	m.SenderID = []byte(senderID)

	deviceType, deviceID := ParseSenderID(senderID)
	m.DeviceType = []byte(deviceType)
	m.DeviceID = []byte(deviceID)
	m.fsm = nil
}

// SetIdentity use to set the message identity. It is meant to be called only by hermesClient before sending the message
func (m *StreamMessage) SetIdentity(deviceType, deviceID string) {
	m.DeviceType = []byte(deviceType)
	m.DeviceID = []byte(deviceID)
	m.SenderID = []byte(BuildClientID(deviceType, deviceID)) // deprecated but set it for backward compatibility reasons
	m.fsm = nil
}

// MessageID returns the type of message
func (m *StreamMessage) MessageID() []byte {
	return m.EnvMessageID
}

// SetMessageID is meant to be called only by hermesClient before sending the message
func (m *StreamMessage) SetMessageID(messageID []byte) {
	m.EnvMessageID = messageID
	m.fsm = nil
}

// SetDeliveredAt is meant to be called only by hermes server when receiving the message
func (m *StreamMessage) SetDeliveredAt(deliveredTime time.Time) {
	m.DeliveredAtEpochMs = uint64(deliveredTime.UnixNano() / int64(time.Millisecond))
	m.fsm = nil
}

// ExtraLogInfo provides extra log info
func (m *StreamMessage) ExtraLogInfo() map[string]interface{} {
	logInfo := map[string]interface{}{}
	logInfo["message_topic"] = string(m.MessageTopic)
	logInfo["sender_id"] = string(m.SenderID)
	return logInfo
}
