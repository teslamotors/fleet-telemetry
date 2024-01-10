package simple

import (
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// ProtoLogger is a simple protobuf logger
type ProtoLogger struct {
	logger *logrus.Logger
}

// NewProtoLogger initializes the parameters for protobuf payload logging
func NewProtoLogger(logger *logrus.Logger) telemetry.Producer {
	return &ProtoLogger{logger: logger}
}

// Produce sends the data to the logger
func (p *ProtoLogger) Produce(entry *telemetry.Record) {
	p.logger.Infof("logger_json_unmarshal %s %v %s\n", entry.Vin, entry.Metadata(), string(entry.Payload()))
}
