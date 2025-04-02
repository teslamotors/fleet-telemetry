package nats_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	natspkg "github.com/teslamotors/fleet-telemetry/datastore/nats"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewProducer(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &natspkg.Config{
		URL: nats.DefaultURL,
	}

	producer, err := natspkg.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)
}

func TestProduce(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &natspkg.Config{
		URL: nats.DefaultURL,
	}

	producer, err := natspkg.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	payload := &protos.Payload{
		Vin: "TEST123",
		Data: []*protos.Datum{
			{
				Key: protos.Field_VehicleName,
				Value: &protos.Value{
					Value: &protos.Value_StringValue{StringValue: "My Tesla"},
				},
			},
		},
		CreatedAt: timestamppb.Now(),
	}

	payloadBytes, err := proto.Marshal(payload)
	assert.NoError(t, err)

	message := &telemetry.Record{
		TxType:       "test_type",
		PayloadBytes: payloadBytes,
	}

	producer.Produce(message)

	// Add tests for error handling
	producerWithError, err := natspkg.NewProducer(&natspkg.Config{URL: "invalid_url"}, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.Error(t, err)
	assert.Nil(t, producerWithError)

	// Add tests for metrics
	metricsRegistry := natspkg.Metrics{}
	metricsRegistry.ProducerCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_total",
		Help:   "The number of records produced to NATS.",
		Labels: []string{"record_type"},
	})
	metricsRegistry.BytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_produce_total_bytes",
		Help:   "The number of bytes produced to NATS.",
		Labels: []string{"record_type"},
	})
	metricsRegistry.ErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "nats_err",
		Help:   "The number of errors while producing to NATS.",
		Labels: []string{},
	})

	metricsRegistry.ProducerCount.Inc(map[string]string{"record_type": "test_type"})
	metricsRegistry.BytesTotal.Add(int64(len(payloadBytes)), map[string]string{"record_type": "test_type"})

	// Add tests for different payload types
	alerts := &protos.VehicleAlerts{
		Vin: "TEST123",
		Alerts: []*protos.VehicleAlert{
			{
				Name:      "TestAlert1",
				StartedAt: timestamppb.Now(),
				EndedAt:   nil,
				Audiences: []protos.Audience{protos.Audience_Customer, protos.Audience_Service},
			},
		},
		CreatedAt: timestamppb.Now(),
	}

	alertsBytes, err := proto.Marshal(alerts)
	assert.NoError(t, err)

	alertMessage := &telemetry.Record{
		TxType:       "alerts",
		PayloadBytes: alertsBytes,
	}

	producer.Produce(alertMessage)

	errors := &protos.VehicleErrors{
		Vin: "TEST123",
		Errors: []*protos.VehicleError{
			{
				Name:      "TestError1",
				Body:      "This is a test error",
				Tags:      map[string]string{"tag1": "value1", "tag2": "value2"},
				CreatedAt: timestamppb.Now(),
			},
		},
		CreatedAt: timestamppb.Now(),
	}

	errorsBytes, err := proto.Marshal(errors)
	assert.NoError(t, err)

	errorMessage := &telemetry.Record{
		TxType:       "errors",
		PayloadBytes: errorsBytes,
	}

	producer.Produce(errorMessage)
}

func TestClose(t *testing.T) {
	logger, _ := logrus.NewBasicLogrusLogger("test")
	metricsCollector := metrics.NewCollector(nil, logger)
	airbrakeHandler := airbrake.NewAirbrakeHandler(nil)
	ackChan := make(chan *telemetry.Record)
	reliableAckTxTypes := make(map[string]interface{})

	config := &natspkg.Config{
		URL: nats.DefaultURL,
	}

	producer, err := natspkg.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
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

	config := &natspkg.Config{
		URL: nats.DefaultURL,
	}

	producer, err := natspkg.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
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

	config := &natspkg.Config{
		URL: nats.DefaultURL,
	}

	producer, err := natspkg.NewProducer(config, "test_namespace", true, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
	assert.NoError(t, err)
	assert.NotNil(t, producer)

	logInfo := logrus.LogInfo{"key": "value"}
	producer.ReportError("test_error", assert.AnError, logInfo)
}
