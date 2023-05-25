package telemetry

import (
	"fmt"
	"time"
)

// SizeLimit maximum incoming payload size from the vehicle
const SizeLimit = 1000000 // 1mb

// Record is a structs that represents the telemetry records vehicles send to the backend
// vin is used as kafka produce partitioning key by default, can be configured to random
type Record struct {
	ProduceTime       time.Time
	ReceivedTimestamp int64
	Serializer        *BinarySerializer
	SocketID          string
	Timestamp         int64
	Txid              string
	TxType            string
	TripID            string
	Version           int
	Vin               string
	PayloadBytes      []byte
	RawBytes          []byte
}

// NewRecord Sanitizes and instantiates a Record from a message
// !! caller expect *Record to not be nil !!
func NewRecord(ts *BinarySerializer, msg []byte, socketID string) (*Record, error) {
	if len(msg) > SizeLimit {
		return &Record{Serializer: ts}, ErrMessageTooBig
	}

	rec, err := ts.Deserialize(msg, socketID)
	if err != nil {
		return rec, err
	}
	return rec, nil
}

// Ack returns a ack response from the serializer
func (record *Record) Ack() []byte {
	return record.Serializer.Ack(record)
}

// Error returns an error response from the serializer
func (record *Record) Error(err error) []byte {
	return record.Serializer.Error(err, record)
}

// Metadata converts record to metadata map
func (record *Record) Metadata() map[string]string {
	metadata := make(map[string]string)
	metadata["vin"] = record.Vin
	metadata["recievedat"] = fmt.Sprint(record.ReceivedTimestamp)
	metadata["timestamp"] = fmt.Sprint(record.Timestamp)
	metadata["txid"] = record.Txid
	metadata["txtype"] = record.TxType
	metadata["version"] = fmt.Sprint(record.Version)
	return metadata
}

// Payload returns the bytes of the telemetry record gdata
func (record *Record) Payload() []byte {
	return record.PayloadBytes
}

// Raw returns the raw telemetry record
func (record *Record) Raw() []byte {
	return record.RawBytes
}

// Length gets the records byte size
func (record *Record) Length() int {
	record.ensureEncoded()
	return len(record.RawBytes)
}

// Encode encodes the records into bytes
func (record *Record) Encode() ([]byte, error) {
	record.ensureEncoded()
	return record.RawBytes, nil
}

// Dispatch uses the configuration to send records to the list of backends/data stores they belong
func (record *Record) Dispatch() {
	logger := record.Serializer.Logger()
	logger.Debugf("socketID=\"%s\" message=\"dispatching Message: %#v\"", record.SocketID, record.Raw())
	record.Serializer.Dispatch(record)
}

func (record *Record) ensureEncoded() {
	if record.RawBytes == nil && record.Serializer != nil && record.Serializer.Logger() != nil {
		record.Serializer.Logger().Error("record_RawBytes_blank")
	}
}
