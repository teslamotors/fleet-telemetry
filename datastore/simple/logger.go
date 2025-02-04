package simple

import (
	"fmt"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Config for the protobuf logger
type Config struct {
	// Verbose controls whether types are explicitly shown in the logs. Only applicable for record type 'V'.
	Verbose bool `json:"verbose"`
}

// Producer is a simple protobuf logger
type Producer struct {
	Config *Config
	logger *logrus.Logger
}

// NewProtoLogger initializes the parameters for protobuf payload logging
func NewProtoLogger(config *Config, logger *logrus.Logger) telemetry.Producer {
	return &Producer{Config: config, logger: logger}
}

// Close the producer
func (p *Producer) Close() error {
	return nil
}

// ProcessReliableAck noop method
func (p *Producer) ProcessReliableAck(_ *telemetry.Record) {
}

// Produce sends the data to the logger
func (p *Producer) Produce(entry *telemetry.Record) {
	data, err := p.recordToLogMap(entry, entry.Vin)
	if err != nil {
		p.logger.ErrorLog("record_logging_error", err, logrus.LogInfo{"vin": entry.Vin, "txtype": entry.TxType, "metadata": entry.Metadata()})
		return
	}
	p.logger.ActivityLog("record_payload", logrus.LogInfo{"vin": entry.Vin, "metadata": entry.Metadata(), "data": data})
}

// ReportError noop method
func (p *Producer) ReportError(_ string, _ error, _ logrus.LogInfo) {
}

// recordToLogMap converts the data of a record to a map or slice of maps
func (p *Producer) recordToLogMap(record *telemetry.Record, vin string) (interface{}, error) {
	switch payload := record.GetProtoMessage().(type) {
	case *protos.Payload:
		return transformers.PayloadToMap(payload, p.Config.Verbose, vin, p.logger), nil
	case *protos.VehicleAlerts:
		alertMaps := make([]map[string]interface{}, len(payload.Alerts))
		for i, alert := range payload.Alerts {
			alertMaps[i] = transformers.VehicleAlertToMap(alert)
		}
		return alertMaps, nil
	case *protos.VehicleConnectivity:
		return transformers.VehicleConnectivityToMap(payload), nil
	default:
		return nil, fmt.Errorf("unknown txType: %s", record.TxType)
	}
}
