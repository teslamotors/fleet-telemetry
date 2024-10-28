package simple

import (
	"fmt"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

type Config struct {
	// Verbose controls whether types are explicitly shown in the logs. Only applicable for record type 'V'.
	Verbose bool `json:"verbose"`
}

// ProtoLogger is a simple protobuf logger
type ProtoLogger struct {
	Config *Config
	logger *logrus.Logger
}

// NewProtoLogger initializes the parameters for protobuf payload logging
func NewProtoLogger(config *Config, logger *logrus.Logger) telemetry.Producer {
	return &ProtoLogger{Config: config, logger: logger}
}

// SetReliableAckTxType no-op for logger datastore
func (p *ProtoLogger) ProcessReliableAck(entry *telemetry.Record) {
}

// Produce sends the data to the logger
func (p *ProtoLogger) Produce(entry *telemetry.Record) {
	data, err := p.recordToLogMap(entry)
	if err != nil {
		p.logger.ErrorLog("record_logging_error", err, logrus.LogInfo{"vin": entry.Vin, "metadata": entry.Metadata()})
		return
	}
	p.logger.ActivityLog("record_payload", logrus.LogInfo{"vin": entry.Vin, "metadata": entry.Metadata(), "data": data})
}

// ReportError noop method
func (p *ProtoLogger) ReportError(message string, err error, logInfo logrus.LogInfo) {
}

// recordToLogMap converts the data of a record to a map or slice of maps
func (p *ProtoLogger) recordToLogMap(record *telemetry.Record) (interface{}, error) {
	payload, err := record.GetProtoMessage()
	if err != nil {
		return nil, err
	}

	switch payload := payload.(type) {
	case *protos.Payload:
		return transformers.PayloadToMap(payload, p.Config.Verbose, p.logger), nil
	case *protos.VehicleAlerts:
		alertMaps := make([]map[string]interface{}, len(payload.Alerts))
		for i, alert := range payload.Alerts {
			alertMaps[i] = transformers.VehicleAlertToMap(alert)
		}
		return alertMaps, nil
	case *protos.VehicleErrors:
		errorMaps := make([]map[string]interface{}, len(payload.Errors))
		for i, vehicleError := range payload.Errors {
			errorMaps[i] = transformers.VehicleErrorToMap(vehicleError)
		}
		return errorMaps, nil
	case *protos.VehicleConnectivity:
		return transformers.VehicleConnectivityToMap(payload), nil
	default:
		return nil, fmt.Errorf("unknown txType: %s", record.TxType)
	}
}
