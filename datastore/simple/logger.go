package simple

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// ProtoLogger simple protobuf logger for incoming vitals payload
type ProtoLogger struct {
	logger  *logrus.Logger
	options protojson.MarshalOptions
}

var (
	protobufMap = map[string]func() proto.Message{
		"alerts": func() proto.Message {
			return &protos.VehicleAlerts{}
		},
		"errors": func() proto.Message {
			return &protos.VehicleErrors{}
		},
		"V": func() proto.Message {
			return &protos.Payload{}
		},
	}
)

// NewProtoLogger initializes the parameters for protobuf payload logging
func NewProtoLogger(logger *logrus.Logger) telemetry.Producer {
	return &ProtoLogger{
		logger: logger,
		options: protojson.MarshalOptions{
			UseEnumNumbers:  false,
			EmitUnpopulated: true,
			Indent:          ""}}

}

// GetProtoMessage converts telemetry record to protoMessage based on txType
func (p *ProtoLogger) GetProtoMessage(entry *telemetry.Record) (proto.Message, error) {
	msgFunc, ok := protobufMap[entry.TxType]
	if !ok {
		return nil, fmt.Errorf("no mapping for txType: %s", entry.TxType)
	}
	message := msgFunc()
	err := proto.Unmarshal(entry.Payload(), message)
	return message, err
}

// Produce sends the vitals data to the logger
func (p *ProtoLogger) Produce(entry *telemetry.Record) {
	payload, err := p.GetProtoMessage(entry)
	if err != nil {
		p.logger.Errorf("vitals_logger_proto_unmarshal_error %s %v %s\n", entry.Vin, entry.Metadata(), err.Error())
		return
	}
	output, err := p.options.Marshal(payload)
	if err != nil {
		p.logger.Errorf("vitals_logger_json_unmarshal_error %s %v %s\n", entry.Vin, entry.Metadata(), err.Error())
	} else {
		p.logger.Infof("vitals_logger_json_unmarshal %s %v %s\n", entry.Vin, entry.Metadata(), output)
	}
}
