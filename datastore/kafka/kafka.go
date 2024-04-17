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
	deliveryChan      chan kafka.Event
}

// Metrics stores metrics reported from this package
type Metrics struct {
	producerCount     adapter.Counter
	bytesTotal        adapter.Counter
	producerAckCount  adapter.Counter
	bytesAckTotal     adapter.Counter
	errorCount        adapter.Counter
	producerQueueSize adapter.Gauge
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer establishes the kafka connection and define the dispatch method
func NewProducer(config *kafka.ConfigMap, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.AirbrakeHandler, logger *logrus.Logger) (telemetry.Producer, error) {
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
		deliveryChan:      make(chan kafka.Event),
	}

	go producer.handleProducerEvents()
	go producer.reportProducerMetrics()
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
	if err := p.kafkaProducer.Produce(msg, p.deliveryChan); err != nil {
		p.logError(err)
		return
	}
	metricsRegistry.producerCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.bytesTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
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

func (p *Producer) handleProducerEvents() {
	for e := range p.deliveryChan {
		switch ev := e.(type) {
		case kafka.Error:
			p.logError(fmt.Errorf("producer_error %v", ev))
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logError(fmt.Errorf("topic_partition_error %v", ev))
				continue
			}
			entry, ok := ev.Opaque.(*telemetry.Record)
			if !ok {
				p.logError(fmt.Errorf("opaque_record_missing %v", ev))
				continue
			}
			metricsRegistry.producerAckCount.Inc(map[string]string{"record_type": entry.TxType})
			metricsRegistry.bytesAckTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
		default:
			p.logger.ActivityLog("kafka_event_ignored", logrus.LogInfo{"event": ev.String()})
		}
	}
}

func (p *Producer) logError(err error) {
	p.ReportError("kafka_err", err, nil)
	metricsRegistry.errorCount.Inc(map[string]string{})
}

func (p *Producer) reportProducerMetrics() {
	interval := 5 * time.Second
	t := time.NewTicker(interval)
	for range t.C {
		total := p.kafkaProducer.Len()
		eventsCount := len(p.kafkaProducer.Events())
		metricsRegistry.producerQueueSize.Set(int64(total), map[string]string{"type": "total"})
		metricsRegistry.producerQueueSize.Set(int64(eventsCount), map[string]string{"type": "events"})
		metricsRegistry.producerQueueSize.Set(int64(total-eventsCount), map[string]string{"type": "buffer"})
	}
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.producerCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_total",
		Help:   "The number of records produced to Kafka.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.bytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_total_bytes",
		Help:   "The number of bytes produced to Kafka.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.producerAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_ack_total",
		Help:   "The number of records produced to Kafka for which we got an ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.bytesAckTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_produce_ack_total_bytes",
		Help:   "The number of bytes produced to Kafka for which we got an ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kafka_err",
		Help:   "The number of errors while producing to Kafka.",
		Labels: []string{},
	})

	metricsRegistry.producerQueueSize = metricsCollector.RegisterGauge(adapter.CollectorOptions{
		Name:   "kafka_produce_queue_size",
		Help:   "Total pending messages to produce",
		Labels: []string{"type"},
	})
}
