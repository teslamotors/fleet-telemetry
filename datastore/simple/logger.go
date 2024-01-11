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
	data, err := entry.GetJSONPayload()
	if err != nil {
		p.logger.Errorf("json_unmarshal_error %s %v %s\n", entry.Vin, entry.Metadata(), err.Error())
		return
	}
	p.logger.Infof("logger_json_unmarshal %s %v %s\n", entry.Vin, entry.Metadata(), string(data))
}
