package kafka

import (
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle kafka interactions
type Producer struct {
	kafkaProducer     *kafka.Producer
	namespace         string
	prometheusEnabled bool
	metricsCollector  metrics.MetricCollector
	logger            *logrus.Logger
	airbrakeHandler   *airbrake.AirbrakeHandler
}

// Metrics stores metrics reported from this package
type Metrics struct {
	produceCount adapter.Counter
	byteTotal    adapter.Counter
	errorCount   adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer establishes the kafka connection and define the dispatch method
func NewProducer(config *kafka.ConfigMap, namespace string, reliableAckWorkers int,
	ackChan chan (*telemetry.Record), prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.AirbrakeHandler, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	kafkaProducer, err := kafka.NewProducer(config)
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		kafkaProducer:     kafkaProducer,
		namespace:         namespace,
		metricsCollector:  metricsCollector,
		prometheusEnabled: prometheusEnabled,
		logger:            logger,
		airbrakeHandler:   airbrakeHandler,
	}

	for i := 0; i < reliableAckWorkers; i++ {
		go producer.handleProducerEvents(ackChan)
	}
	producer.logger.ActivityLog("kafka_registered", logrus.LogInfo{"namespace": namespace})
	return producer, nil
}

// Produce asynchronously sends the record payload to kafka
func (p *Producer) Produce(entry *telemetry.Record) {
	topic := telemetry.BuildTopicName(p.namespace, entry.TxType)

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          entry.Payload(),
		Key:            []byte(entry.Vin),
		Headers:        headersFromRecord(entry),
		Timestamp:      time.Now(),
		Opaque:         entry,
	}

	// Note: confluent kafka supports the concept of one channel per connection, so we could add those here and get rid of reliableAckWorkers
	// ex.: https://github.com/confluentinc/confluent-kafka-go/blob/master/examples/producer_custom_channel_example/producer_custom_channel_example.go#L79
	entry.ProduceTime = time.Now()
	if err := p.kafkaProducer.Produce(msg, nil); err != nil {
		p.logError(err)
		return
	}

	metricsRegistry.produceCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.byteTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

// ReportError to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

func headersFromRecord(record *telemetry.Record) (headers []kafka.Header) {
	for key, val := range record.Metadata() {
		headers = append(headers, kafka.Header{
			Key:   key,
			Value: []byte(val),
		})
	}
	return
}

func (p *Producer) handleProducerEvents(ackChan chan (*telemetry.Record)) {
	for e := range p.kafkaProducer.Events() {
		switch ev := e.(type) {
		case kafka.Error:
			p.logError(fmt.Errorf("producer_error %v", ev))
		case *kafka.Message:
			record, ok := ev.Opaque.(*telemetry.Record)
			if ok {
				ackChan <- record
			}
		default:
			p.logger.ActivityLog("kafka_event_ignored", logrus.LogInfo{"event": ev.String()})
		}
	}
}

func (p *Producer) logError(err error) {
	p.ReportError("kafka_err", err, nil)
	metricsRegistry.errorCount.Inc(map[string]string{})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.produceCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_total",
		Help:   "The number of records produced to Kafka.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_total_bytes",
		Help:   "The number of bytes produced to Kafka.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_err",
		Help:   "The number of errors while producing to Kafka.",
		Labels: []string{},
	})
}
