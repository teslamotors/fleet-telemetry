package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// NatsConnect is a variable that holds the function to create a NATS connection
// This allows for testing by replacing it with a mock
var NatsConnect = nats.Connect

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
	URL           string `json:"url"`
	Name          string `json:"name,omitempty"`
	RetryConnect  bool   `json:"retry_connect,omitempty"`
	ReconnectWait int    `json:"reconnect_wait,omitempty"`
}

// NewProducer establishes the NATS connection and define the dispatch method
func NewProducer(config *Config, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	// Use default values if not specified in config
	name := "Tesla Fleet Telemetry"
	if config.Name != "" {
		name = config.Name
	}

	retryConnect := true
	if !config.RetryConnect {
		retryConnect = false
	}

	reconnectWait := 0
	if config.ReconnectWait != 0 {
		reconnectWait = config.ReconnectWait
	}

	natsConn, err := NatsConnect(
		config.URL,
		nats.Name(name),
		nats.RetryOnFailedConnect(retryConnect),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Duration(reconnectWait)*time.Second),
		nats.ClosedHandler(func(conn *nats.Conn) {
			logger.ActivityLog("nats_closed", logrus.LogInfo{"reason": conn.LastError()})
		}),
		nats.ErrorHandler(func(conn *nats.Conn, sub *nats.Subscription, err error) {
			logger.ActivityLog("nats_error", logrus.LogInfo{"error": err, "subject": sub.Subject})
		}),
		nats.ConnectHandler(func(conn *nats.Conn) {
			logger.ActivityLog("nats_connected", logrus.LogInfo{"server": conn.ConnectedUrl()})
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			logger.ActivityLog("nats_reconnected", logrus.LogInfo{"server": conn.ConnectedUrl()})
		}),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			logger.ActivityLog("nats_disconnected", logrus.LogInfo{"error": err})
		}),
	)
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
	// Hardcode the namespace for now
	subject := fmt.Sprintf("%s.%s.%s", p.namespace, entry.Vin, entry.TxType)

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
