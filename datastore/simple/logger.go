package simple

import (
	"fmt"
	"sync"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// Metrics holds the metrics for the Logger producer.
type Metrics struct {
	reliableAckCount adapter.Counter
}

// Config for the protobuf logger
type Config struct {
	// Verbose controls whether types are explicitly shown in the logs. Only applicable for record type 'V'.
	Verbose bool `json:"verbose"`
}

// Producer is a simple protobuf logger
type Producer struct {
	Config             *Config
	logger             *logrus.Logger
	reliableAckTxTypes map[string]interface{}
	ackChan            chan (*telemetry.Record)
}

// NewProtoLogger initializes the parameters for protobuf payload logging
func NewProtoLogger(config *Config, metrics metrics.MetricCollector, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) telemetry.Producer {
	registerMetricsOnce(metrics)
	return &Producer{Config: config, ackChan: ackChan, logger: logger, reliableAckTxTypes: reliableAckTxTypes}
}

// Close the producer
func (p *Producer) Close() error {
	return nil
}

// ProcessReliableAck noop method
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	_, ok := p.reliableAckTxTypes[entry.TxType]
	if ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
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

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "logger_reliable_ack_total",
		Help:   "The number of records published to Logger topics for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})
}
