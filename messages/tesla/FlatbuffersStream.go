// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tesla

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type FlatbuffersStream struct {
	_tab flatbuffers.Table
}

func GetRootAsFlatbuffersStream(buf []byte, offset flatbuffers.UOffsetT) *FlatbuffersStream {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &FlatbuffersStream{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *FlatbuffersStream) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *FlatbuffersStream) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *FlatbuffersStream) CreatedAt() uint32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetUint32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *FlatbuffersStream) MutateCreatedAt(n uint32) bool {
	return rcv._tab.MutateUint32Slot(4, n)
}

// sender_id: device identity coming from the certificate - DEPRECATED: use combination of device_type + device_id
func (rcv *FlatbuffersStream) SenderId(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *FlatbuffersStream) SenderIdLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *FlatbuffersStream) SenderIdBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// sender_id: device identity coming from the certificate - DEPRECATED: use combination of device_type + device_id
func (rcv *FlatbuffersStream) MutateSenderId(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

func (rcv *FlatbuffersStream) Payload(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *FlatbuffersStream) PayloadLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *FlatbuffersStream) PayloadBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *FlatbuffersStream) MutatePayload(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

// device_type: device identity coming from the certificate derived of Issuer CN
func (rcv *FlatbuffersStream) DeviceType(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *FlatbuffersStream) DeviceTypeLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *FlatbuffersStream) DeviceTypeBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// device_type: device identity coming from the certificate derived of Issuer CN
func (rcv *FlatbuffersStream) MutateDeviceType(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

// device_id: device identity coming from the certificate derived of Subject CN
func (rcv *FlatbuffersStream) DeviceId(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *FlatbuffersStream) DeviceIdLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *FlatbuffersStream) DeviceIdBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// device_id: device identity coming from the certificate derived of Subject CN
func (rcv *FlatbuffersStream) MutateDeviceId(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

// delivered_at_epoch_ms: timestamp (in milliseconds) at which the message reached the backend
func (rcv *FlatbuffersStream) DeliveredAtEpochMs() uint64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.GetUint64(o + rcv._tab.Pos)
	}
	return 0
}

// delivered_at_epoch_ms: timestamp (in milliseconds) at which the message reached the backend
func (rcv *FlatbuffersStream) MutateDeliveredAtEpochMs(n uint64) bool {
	return rcv._tab.MutateUint64Slot(14, n)
}

func FlatbuffersStreamStart(builder *flatbuffers.Builder) {
	builder.StartObject(6)
}
func FlatbuffersStreamAddCreatedAt(builder *flatbuffers.Builder, createdAt uint32) {
	builder.PrependUint32Slot(0, createdAt, 0)
}
func FlatbuffersStreamAddSenderId(builder *flatbuffers.Builder, senderId flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(senderId), 0)
}
func FlatbuffersStreamStartSenderIdVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func FlatbuffersStreamAddPayload(builder *flatbuffers.Builder, payload flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, flatbuffers.UOffsetT(payload), 0)
}
func FlatbuffersStreamStartPayloadVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func FlatbuffersStreamAddDeviceType(builder *flatbuffers.Builder, deviceType flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(3, flatbuffers.UOffsetT(deviceType), 0)
}
func FlatbuffersStreamStartDeviceTypeVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func FlatbuffersStreamAddDeviceId(builder *flatbuffers.Builder, deviceId flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(4, flatbuffers.UOffsetT(deviceId), 0)
}
func FlatbuffersStreamStartDeviceIdVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func FlatbuffersStreamAddDeliveredAtEpochMs(builder *flatbuffers.Builder, DeliveredAtEpochMs uint64) {
	builder.PrependUint64Slot(5, DeliveredAtEpochMs, 0)
}
func FlatbuffersStreamEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
