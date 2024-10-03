package mqtt

import (
	"context"
	"sync"
	"time"

	"encoding/json"
	"fmt"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

type MQTTProducer struct {
	client             pahomqtt.Client
	config             *Config
	logger             *logrus.Logger
	airbrakeHandler    *airbrake.AirbrakeHandler
	namespace          string
	ctx                context.Context
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
}

type Config struct {
	Broker         string `json:"broker"`
	ClientID       string `json:"client_id"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TopicBase      string `json:"topic_base"`
	QoS            byte   `json:"qos"`
	Retained       bool   `json:"retained"`
	ConnectTimeout int    `json:"connect_timeout"`
	PublishTimeout int    `json:"publish_timeout"`
}

type Metrics struct {
	errorCount       adapter.Counter
	publishCount     adapter.Counter
	byteTotal        adapter.Counter
	reliableAckCount adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// Allow us to mock the mqtt.NewClient function for testing
var PahoNewClient = pahomqtt.NewClient

func NewProducer(ctx context.Context, config *Config, metrics metrics.MetricCollector, namespace string, airbrakeHandler *airbrake.AirbrakeHandler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metrics)

	if config.PublishTimeout == 0 {
		config.PublishTimeout = 1 // Default to 1 second
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 30 // Default to 30 seconds
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientID).
		SetUsername(config.Username).
		SetPassword(config.Password).
		SetConnectTimeout(time.Duration(config.ConnectTimeout) * time.Second)

	client := PahoNewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &MQTTProducer{
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

func (p *MQTTProducer) Produce(rec *telemetry.Record) {
	if p.ctx.Err() != nil {
		return
	}

	// Alerts and errors are not published to MQTT
	if rec.TxType == "alerts" || rec.TxType == "errors" {
		p.logger.Log(logrus.DEBUG, "mqtt_ignored_record", logrus.LogInfo{"tx_type": rec.TxType})
		return
	}

	payload, err := rec.GetProtoMessage()
	if err != nil {
		p.ReportError("mqtt_payload_unmarshal_error", err, nil)
		return
	}

	data, ok := payload.(*protos.Payload)
	if !ok {
		p.ReportError("mqtt_invalid_payload_type", fmt.Errorf("expected *protos.Payload, got %T", payload), nil)
		return
	}

	convertedPayload := transformers.PayloadToMap(data, false, p.logger)

	var tokens []pahomqtt.Token

	// Publish each field to the relevant topic on the MQTT broker
	for key, value := range convertedPayload {
		// Skip VIN and CreatedAt fields
		if key == "Vin" || key == "CreatedAt" {
			continue
		}

		topicName := fmt.Sprintf("%s/vin/%s/%s", p.config.TopicBase, rec.Vin, key)

		jsonValue, err := json.Marshal(map[string]interface{}{"value": value})
		if err != nil {
			p.ReportError("mqtt_json_marshal_error", err, logrus.LogInfo{"key": key})
			continue
		}

		token := p.client.Publish(topicName, p.config.QoS, p.config.Retained, jsonValue)
		tokens = append(tokens, token)

		metricsRegistry.byteTotal.Add(int64(len(jsonValue)), map[string]string{"record_type": rec.TxType})
		metricsRegistry.publishCount.Inc(map[string]string{"record_type": rec.TxType})
	}

	var publishError bool
	// Wait for all topics to be published
	for _, token := range tokens {
		if !token.WaitTimeout(time.Duration(p.config.PublishTimeout) * time.Second) {
			metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
			p.ReportError("mqtt_publish_timeout", fmt.Errorf("publish operation timed out"), logrus.LogInfo{"topic": "multiple"})
			publishError = true
		} else if token.Error() != nil {
			metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
			p.ReportError("mqtt_dispatch_error", token.Error(), logrus.LogInfo{"topic": "multiple"})
			publishError = true
		}
	}

	// Only process reliable ACK if no errors were reported
	if !publishError {
		p.ProcessReliableAck(rec)
	}
}

func (p *MQTTProducer) ProcessReliableAck(entry *telemetry.Record) {
	_, ok := p.reliableAckTxTypes[entry.TxType]
	if ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

func (p *MQTTProducer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

func (p *MQTTProducer) Close() error {
	p.client.Disconnect(250)
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
		Help:   "The number of jons bytes published to MQTT.",
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
