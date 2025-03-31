package nats_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/teslamotors/fleet-telemetry/datastore/nats"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func TestNewProducer(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &nats.Config{
		URL: nats.DefaultURL,
	}

	producer, err := nats.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)
}

func TestProduce(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &nats.Config{
		URL: nats.DefaultURL,
	}

	producer, err := nats.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	record := &telemetry.Record{
		TxType:       "test_type",
		PayloadBytes: []byte("test_payload"),
	}

	producer.Produce(record)
}

func TestClose(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &nats.Config{
		URL: nats.DefaultURL,
	}

	producer, err := nats.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	err = producer.Close()
	assert.NoError(t, err)
}

func TestProcessReliableAck(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := map[string]interface{}{
		"test_type": true,
	}

	config := &nats.Config{
		URL: nats.DefaultURL,
	}

	producer, err := nats.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	record := &telemetry.Record{
		TxType: "test_type",
	}

	go producer.ProcessReliableAck(record)

	ackRecord := <-ackChan
	assert.Equal(t, record, ackRecord)
}

func TestReportError(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &nats.Config{
		URL: nats.DefaultURL,
	}

	producer, err := nats.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	logInfo := logrus.LogInfo{"key": "value"}
	producer.ReportError("test_error", assert.AnError, logInfo)
}
