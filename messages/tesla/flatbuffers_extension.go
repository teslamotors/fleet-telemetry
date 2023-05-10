package tesla

import (
	"fmt"

	flatbuffers "github.com/google/flatbuffers/go"
)

/******** FlatbuffersEnvelope extension ********/

// FlatbuffersEnvelopeToBytes dump all the field of a FlatbuffersCommand into a flatbuffers []byte
func FlatbuffersEnvelopeToBytes(b *flatbuffers.Builder, txid, topic []byte, msg flatbuffers.UOffsetT, messageID []byte, msgType byte) []byte {
	txidVector := b.CreateByteString(txid)
	topicVector := b.CreateByteString(topic)
	messageIDVector := b.CreateByteString(messageID)

	// write the User object:
	FlatbuffersEnvelopeStart(b)
	FlatbuffersEnvelopeAddTxid(b, txidVector)
	FlatbuffersEnvelopeAddTopic(b, topicVector)
	FlatbuffersEnvelopeAddMessageType(b, msgType)
	FlatbuffersEnvelopeAddMessage(b, msg)
	FlatbuffersEnvelopeAddMessageId(b, messageIDVector)
	msgPosition := FlatbuffersEnvelopeEnd(b)

	// finish the write operations by our User the root object:
	b.Finish(msgPosition)

	// return the byte slice containing encoded data:
	return b.Bytes[b.Head():]
}

// FlatbuffersEnvelopeFromBytes serializes a Flatbuffers Envelope from a slice of bytes
func FlatbuffersEnvelopeFromBytes(value []byte) (*FlatbuffersEnvelope, *flatbuffers.Table, error) {
	if len(value) == 0 {
		return nil, nil, fmt.Errorf("cannot serialize %#v into message(empty message)", value)
	}

	envelope := GetRootAsFlatbuffersEnvelope(value, 0)

	unionTable := new(flatbuffers.Table)
	if !envelope.Message(unionTable) {
		return nil, nil, fmt.Errorf("cannot serialize %#v into Message", value)
	}

	return envelope, unionTable, nil
}

/******** StreamMessage extension ********/

// NewFlatbuffersStream instantiate a new FlatbuffersStream according to its definition
func NewFlatbuffersStream(buf []byte, offset flatbuffers.UOffsetT) *FlatbuffersStream {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	fsm := &FlatbuffersStream{}
	fsm.Init(buf, n+offset)
	return fsm
}

// FlatbuffersStreamToBytes dump all the field of a FlatbuffersStream into a flatbuffers []byte
func FlatbuffersStreamToBytes(senderID, topic, txid, payload []byte, createdAt uint32, messageID, deviceType, deviceID []byte, DeliveredAtEpochMs uint64) []byte {
	b := flatbuffers.NewBuilder(0)

	senderIDVector := b.CreateByteString(senderID)
	payloadVector := b.CreateByteString(payload)
	deviceTypeVector := b.CreateByteString(deviceType)
	deviceIDVector := b.CreateByteString(deviceID)

	// write the User object:
	FlatbuffersStreamStart(b)
	FlatbuffersStreamAddSenderId(b, senderIDVector)
	FlatbuffersStreamAddCreatedAt(b, createdAt)
	FlatbuffersStreamAddPayload(b, payloadVector)

	FlatbuffersStreamAddDeviceType(b, deviceTypeVector)
	FlatbuffersStreamAddDeviceId(b, deviceIDVector)
	FlatbuffersStreamAddDeliveredAtEpochMs(b, DeliveredAtEpochMs)
	message := FlatbuffersStreamEnd(b)

	// return the byte slice containing encoded data:
	return FlatbuffersEnvelopeToBytes(b, txid, topic, message, messageID, MessageFlatbuffersStream)
}

// ToBytes dump all the field of a FlatbuffersCommand into a flatbuffers []byte
func (fsm *FlatbuffersStream) ToBytes(envelope *FlatbuffersEnvelope) []byte {
	return FlatbuffersStreamToBytes(fsm.SenderIdBytes(), envelope.TopicBytes(), envelope.TxidBytes(),
		fsm.PayloadBytes(), fsm.CreatedAt(), envelope.MessageIdBytes(), fsm.DeviceTypeBytes(), fsm.DeviceIdBytes(), fsm.DeliveredAtEpochMs())
}

/******** StreamAckMessage extension ********/

// NewFlatbuffersStreamAck instantiate a new FlatbuffersStream according to its definition
func NewFlatbuffersStreamAck(buf []byte, offset flatbuffers.UOffsetT) *FlatbuffersStreamAck {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	fsm := &FlatbuffersStreamAck{}
	fsm.Init(buf, n+offset)
	return fsm
}

// FlatbuffersStreamAckToBytes dump all the field of a FlatbuffersStream into a flatbuffers []byte
func FlatbuffersStreamAckToBytes(txid, topic, messageID []byte) []byte {
	b := flatbuffers.NewBuilder(0)
	return FlatbuffersEnvelopeToBytes(b, txid, topic, b.CreateByteString([]byte{}), messageID, MessageFlatbuffersStreamAck)
}
