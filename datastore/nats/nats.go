package nats

import (
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle NATS interactions
type Producer struct {
	natsConn           *nats.Conn
	namespace          string
	metricsCollector   metrics.MetricCollector
	logger             *logrus.Logger
	airbrakeHandler    *airbrake.Handler
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
}

// Metrics stores metrics reported from this package
type Metrics struct {
	producerCount     adapter.Counter
	bytesTotal        adapter.Counter
	producerAckCount  adapter.Counter
	bytesAckTotal     adapter.Counter
	errorCount        adapter.Counter
	reliableAckCount  adapter.Counter
	producerQueueSize adapter.Gauge
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// Config for NATS producer
type Config struct {
	URL string `json:"url"`
}

// NewProducer establishes the NATS connection and define the dispatch method
func NewProducer(config *Config, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	natsConn, err := nats.Connect(config.URL)
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		natsConn:           natsConn,
		namespace:          namespace,
		metricsCollector:   metricsCollector,
		logger:             logger,
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}

	producer.logger.ActivityLog("nats_registered", logrus.LogInfo{"namespace": namespace})
	return producer, nil
}

// Produce asynchronously sends the record payload to NATS
func (p *Producer) Produce(entry *telemetry.Record) {
	subject := telemetry.BuildTopicName(p.namespace, entry.TxType)

	err := p.natsConn.Publish(subject, entry.Payload())
	if err != nil {
		p.logError(err)
		return
	}
	p.ProcessReliableAck(entry)
	metricsRegistry.producerCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.bytesTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

// Close the producer
func (p *Producer) Close() error {
	p.natsConn.Close()
	return nil
}

// ProcessReliableAck sends to ackChan if reliable ack is configured
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	_, ok := p.reliableAckTxTypes[entry.TxType]
	if ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// ReportError to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

func (p *Producer) logError(err error) {
	p.ReportError("nats_err", err, nil)
	metricsRegistry.errorCount.Inc(map[string]string{})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.producerCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_total",
		Help:   "The number of records produced to NATS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.bytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_total_bytes",
		Help:   "The number of bytes produced to NATS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.producerAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_ack_total",
		Help:   "The number of records produced to NATS for which we got an ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_reliable_ack_total",
		Help:   "The number of records produced to NATS for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.bytesAckTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_ack_total_bytes",
		Help:   "The number of bytes produced to NATS for which we got an ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_err",
		Help:   "The number of errors while producing to NATS.",
		Labels: []string{},
	})

	metricsRegistry.producerQueueSize = metricsCollector.RegisterGauge(adapter.CollectorOptions{
		Name:   "nats_produce_queue_size",
		Help:   "Total pending messages to produce",
		Labels: []string{"type"},
	})
}
