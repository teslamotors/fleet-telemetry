package messages

import (
	"fmt"

	"github.com/teslamotors/fleet-telemetry/messages/tesla"
)

// StreamAckMessage is struct that warps up information used for streaming response coming from Tesla Cloud
type StreamAckMessage struct {
	envelope     *tesla.FlatbuffersEnvelope
	TXID         []byte
	MessageTopic []byte
	EnvMessageID []byte // unique id of the message
}

// StreamAckMessageFromBytes serializes a message from a slice of bytes
func StreamAckMessageFromBytes(value []byte) (*StreamAckMessage, error) {
	envelope := tesla.GetRootAsFlatbuffersEnvelope(value, 0)

	if envelope.MessageType() != tesla.MessageFlatbuffersStreamAck {
		return nil, fmt.Errorf("unable to load stream ack message(%v) from topic: %v, type: %v", string(envelope.TxidBytes()), string(envelope.TopicBytes()), envelope.MessageType())
	}

	return &StreamAckMessage{
		envelope:     envelope,
		MessageTopic: envelope.TopicBytes(),
		TXID:         envelope.TxidBytes(),
		EnvMessageID: envelope.MessageIdBytes(),
	}, nil
}

// ToBytes deserializes a message into a slice of bytes
func (m *StreamAckMessage) ToBytes() ([]byte, error) {
	b := tesla.FlatbuffersStreamAckToBytes(m.TXID, m.MessageTopic, m.EnvMessageID)
	return b, nil
}

// MsgType returns the type of message
func (m *StreamAckMessage) MsgType() byte {
	if m.envelope == nil {
		return tesla.MessageFlatbuffersStreamAck
	}
	return m.envelope.MessageType()
}

// Topic returns the type of message
func (m *StreamAckMessage) Topic() string {
	return string(m.MessageTopic)
}

// Txid returns the type of message
func (m *StreamAckMessage) Txid() []byte {
	return m.TXID
}

// IsExpired returns always returns false
func (m *StreamAckMessage) IsExpired() bool {
	return false
}

// SetSenderID is not implemented for StreamAckMessage
func (m *StreamAckMessage) SetSenderID(_ string) {
	// nothing to do
}

// MessageID returns the uniq id of the message
func (m *StreamAckMessage) MessageID() []byte {
	return m.EnvMessageID
}

// SetMessageID is meant to be called only by hermesClient before sending the message
func (m *StreamAckMessage) SetMessageID(messageID []byte) {
	m.EnvMessageID = messageID
}

// ExtraLogInfo provides extra log info
func (m *StreamAckMessage) ExtraLogInfo() map[string]interface{} {
	logInfo := map[string]interface{}{}
	logInfo["message_topic"] = string(m.MessageTopic)
	return logInfo
}
