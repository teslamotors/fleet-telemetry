package simple

import (
	logrus "github.com/teslamotors/fleet-telemetry/logger"
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
		p.logger.ErrorLog("json_unmarshal_error", err, logrus.LogInfo{"vin": entry.Vin, "metadata": entry.Metadata()})
		return
	}
	p.logger.ActivityLog("logger_json_unmarshal", logrus.LogInfo{"vin": entry.Vin, "metadata": entry.Metadata(), "data": string(data)})
}

// ReportError noop method
func (p *ProtoLogger) ReportError(message string, err error, logInfo logrus.LogInfo) {
	return
}
