package telemetry

import (
	"fmt"
	"strings"
	"time"

	"github.com/teslamotors/fleet-telemetry/protos"
	"google.golang.org/protobuf/proto"
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
	err = rec.applyRecordTransforms()
	return rec, err
}

// Ack returns an ack response from the serializer
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
	metadata["receivedat"] = fmt.Sprint(record.ReceivedTimestamp)
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

func (record *Record) applyRecordTransforms() error {
	switch record.TxType {
	case "alerts":
		message := &protos.VehicleAlerts{}
		err := proto.Unmarshal(record.Payload(), message)
		if err != nil {
			return err
		}
		message.Vin = record.Vin
		record.PayloadBytes, err = proto.Marshal(message)
		return err
	case "errors":
		message := &protos.VehicleErrors{}
		err := proto.Unmarshal(record.Payload(), message)
		if err != nil {
			return err
		}
		message.Vin = record.Vin
		record.PayloadBytes, err = proto.Marshal(message)
		return err
	case "V":
		message := &protos.Payload{}
		err := proto.Unmarshal(record.Payload(), message)
		if err != nil {
			return err
		}
		message.Vin = record.Vin
		transformLocation(message)
		record.PayloadBytes, err = proto.Marshal(message)
		return err
	default:
		return nil
	}
}

// transformLocation does a best-effort attempt to convert the Location field to a proper protos.Location
// type if what we receive is a string that can be parsed. This should make the transition from strings to
// Locations easier to handle downstream.
func transformLocation(message *protos.Payload) {
	for _, datum := range message.Data {
		if datum.GetKey() == protos.Field_Location {
			if strVal := datum.GetValue().GetStringValue(); strVal != "" {
				if loc, err := ParseLocation(strVal); err == nil {
					datum.Value = &protos.Value{Value: &protos.Value_LocationValue{LocationValue: loc}}
				}
			}
			// There can be only one Field_Location Datum in the proto; abort once we've seen it.
			return
		}
	}
}

// ParseLocation parses a location string (such as "(37.412374 N, 122.145867 W)") into a *proto.Location type.
func ParseLocation(s string) (*protos.LocationValue, error) {
	var lat, lon float64
	var latQ, lonQ string
	count, err := fmt.Sscanf(s, "(%f %1s, %f %1s)", &lat, &latQ, &lon, &lonQ)
	if err != nil {
		return nil, err
	}
	if count != 4 || !strings.Contains("NS", latQ) || !strings.Contains("EW", lonQ) {
		return nil, fmt.Errorf("invalid location format: %s", s)
	}
	if latQ == "S" {
		lat = -lat
	}
	if lonQ == "W" {
		lon = -lon
	}
	return &protos.LocationValue{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}
