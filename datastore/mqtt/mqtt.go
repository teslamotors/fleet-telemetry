package mqtt

import (
	"context"
	"fmt"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	defaultTimeout = 5 * time.Second
)

// Producer is a telemetry.Producer that sends records to an MQTT broker.
type Producer struct {
	client             pahomqtt.Client
	config             *Config
	logger             *logrus.Logger
	airbrakeHandler    *airbrake.Handler
	namespace          string
	ctx                context.Context
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
}

// Config holds the configuration for the MQTT producer.
type Config struct {
	Broker               string `json:"broker"`
	ClientID             string `json:"client_id"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	TopicBase            string `json:"topic_base"`
	QoS                  byte   `json:"qos"`
	Retained             bool   `json:"retained"`
	ConnectTimeout       int    `json:"connect_timeout_ms"`
	PublishTimeout       int    `json:"publish_timeout_ms"`
	DisconnectTimeout    int    `json:"disconnect_timeout_ms"`
	ConnectRetryInterval int    `json:"connect_retry_interval_ms"`
	KeepAlive            int    `json:"keep_alive_seconds"`
}

// Metrics holds the metrics for the MQTT producer.
type Metrics struct {
	errorCount       adapter.Counter
	publishCount     adapter.Counter
	byteTotal        adapter.Counter
	reliableAckCount adapter.Counter
}

// Default values for the MQTT producer configuration options.
const (
	DefaultPublishTimeout       = 2500
	DefaultConnectTimeout       = 30000
	DefaultConnectRetryInterval = 10000
	DefaultDisconnectTimeout    = 250
	DefaultKeepAlive            = 30
	DefaultQoS                  = 0
)

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// PahoNewClient allows mocking the mqtt.NewClient function for testing
var PahoNewClient = pahomqtt.NewClient

// NewProducer creates a new MQTT producer.
func NewProducer(ctx context.Context, config *Config, metrics metrics.MetricCollector, namespace string, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metrics)

	// Set default values
	if config.PublishTimeout == 0 {
		config.PublishTimeout = DefaultPublishTimeout
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = DefaultConnectTimeout
	}
	if config.DisconnectTimeout == 0 {
		config.DisconnectTimeout = DefaultDisconnectTimeout
	}
	if config.ConnectRetryInterval == 0 {
		config.ConnectRetryInterval = DefaultConnectRetryInterval
	}
	if config.QoS == 0 {
		config.QoS = DefaultQoS
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = DefaultKeepAlive
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientID).
		SetUsername(config.Username).
		SetPassword(config.Password).
		SetConnectRetry(true).
		SetAutoReconnect(true).
		SetConnectRetryInterval(time.Duration(config.ConnectRetryInterval) * time.Millisecond).
		SetConnectTimeout(time.Duration(config.ConnectTimeout) * time.Millisecond).
		SetOrderMatters(false).
		SetKeepAlive(time.Duration(config.KeepAlive) * time.Second)

	client := PahoNewClient(opts)

	return &Producer{
		client:             client,
		config:             config,
		logger:             logger,
		airbrakeHandler:    airbrakeHandler,
		namespace:          namespace,
		ctx:                ctx,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}, nil
}

// Connect performs health check and returns error if connection is not established
func (p *Producer) Connect() error {
	token := p.client.Connect()
	if !token.WaitTimeout(defaultTimeout) {
		return fmt.Errorf("connection attempt timed out after %v", defaultTimeout)
	}
	if err := token.Error(); err != nil {
		return err
	}
	return nil
}

// Produce sends a record to the MQTT broker.
func (p *Producer) Produce(rec *telemetry.Record) {
	if p.ctx.Err() != nil {
		return
	}

	payload := rec.GetProtoMessage()

	var tokens []pahomqtt.Token
	var err error

	switch payload := payload.(type) {
	case *protos.Payload:
		tokens, err = p.processVehicleFields(rec, payload)
	case *protos.VehicleAlerts:
		tokens, err = p.processVehicleAlerts(rec, payload)
	case *protos.VehicleErrors:
		tokens, err = p.processVehicleErrors(rec, payload)
	case *protos.VehicleConnectivity:
		tokens, err = p.processVehicleConnectivity(rec, payload)
	default:
		p.ReportError("mqtt_unknown_payload_type", nil, p.createLogInfo(rec))
		return
	}
	if err != nil {
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
		p.ReportError("mqtt_process_payload_error", err, p.createLogInfo(rec))
		return
	}

	// Wait for all topics to be published
	var publishError bool
	startTime := time.Now()
	timeout := time.Duration(p.config.PublishTimeout) * time.Millisecond
	for _, token := range tokens {
		remainingTimeout := timeout - time.Since(startTime)
		if remainingTimeout < 0 {
			remainingTimeout = 0
		}
		if err := waitTokenTimeout(token, remainingTimeout); err != nil {
			metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
			p.ReportError("mqtt_publish_error", err, p.createLogInfo(rec))
			publishError = true
		}
	}

	// Only process reliable ACK if no token errors were reported
	if !publishError {
		p.ProcessReliableAck(rec)
	}
}

// waitTokenTimeout waits for a token to complete or timeout.
// It also handles the edge case where the wait time is 0.
func waitTokenTimeout(t pahomqtt.Token, d time.Duration) error {
	if d == 0 {
		select {
		case <-t.Done():
			return t.Error()
		default:
			return pahomqtt.TimedOut
		}
	}
	if !t.WaitTimeout(d) {
		return pahomqtt.TimedOut
	}
	return t.Error()
}

func (p *Producer) updateMetrics(txType string, byteCount int) {
	metricsRegistry.byteTotal.Add(int64(byteCount), map[string]string{"record_type": txType})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": txType})
}

func (p *Producer) createLogInfo(rec *telemetry.Record) logrus.LogInfo {
	logInfo := logrus.LogInfo{
		"topic_name": telemetry.BuildTopicName(p.namespace, rec.TxType),
		"txid":       rec.TxId,
		"vin":        rec.Vin,
	}
	return logInfo
}

// ProcessReliableAck marks a message as successfully published.
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	_, ok := p.reliableAckTxTypes[entry.TxType]
	if ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// ReportError logs an error and sends it to Airbrake.
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

// Close disconnects from the MQTT client.
func (p *Producer) Close() error {
	p.client.Disconnect(uint(p.config.DisconnectTimeout))
	return nil
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "mqtt_err",
		Help:   "The number of errors while publishing to MQTT.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "mqtt_publish_total",
		Help:   "The number of values published to MQTT.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "mqtt_publish_total_bytes",
		Help:   "The number of JSON bytes published to MQTT.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "mqtt_reliable_ack_total",
		Help:   "The number of records published to MQTT topics for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}
