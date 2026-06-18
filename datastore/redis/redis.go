package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	defaultTimeout = 5 * time.Second
)

// Producer publishes telemetry records to Redis pub/sub channels.
type Producer struct {
	client              redis.UniversalClient
	timeout             time.Duration
	namespace           string
	publishVINTopics    bool
	subscriberSetPrefix string
	prometheusEnabled   bool
	metricsCollector    metrics.MetricCollector
	logger              *logrus.Logger
	airbrakeHandler     *airbrake.Handler
	ackChan             chan (*telemetry.Record)
	reliableAckTxTypes  map[string]interface{}
}

// Metrics stores metrics reported from this package
type Metrics struct {
	publishCount      adapter.Counter
	publishBytesTotal adapter.Counter
	errorCount        adapter.Counter
	reliableAckCount  adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer creates a new Redis producer.
func NewProducer(options *redis.UniversalOptions, publishTimeout time.Duration, publishVINTopics bool, subscriberSetPrefix string, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	if options == nil || len(options.Addrs) == 0 {
		return nil, errors.New("redis addrs cannot be empty")
	}

	if !publishVINTopics && subscriberSetPrefix == "" {
		return nil, errors.New("redis requires publish_vin_topics or subscriber_set_prefix to be set, otherwise no records are published")
	}

	client := redis.NewUniversalClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis_connect_error %w", err)
	}
	timeout := defaultTimeout
	if publishTimeout != 0 {
		timeout = publishTimeout
	}

	producer := &Producer{
		client:              client,
		namespace:           namespace,
		publishVINTopics:    publishVINTopics,
		subscriberSetPrefix: subscriberSetPrefix,
		prometheusEnabled:   prometheusEnabled,
		metricsCollector:    metricsCollector,
		logger:              logger,
		airbrakeHandler:     airbrakeHandler,
		ackChan:             ackChan,
		reliableAckTxTypes:  reliableAckTxTypes,
		timeout:             timeout,
	}

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis_connect_error %w", err)
	}
	producer.logger.ActivityLog("redis_registered", logrus.LogInfo{"namespace": namespace, "publish_vin_topics": publishVINTopics, "subscriber_set_prefix": subscriberSetPrefix})
	return producer, nil
}

// Produce publishes the record payload to its Redis pub/sub channels.
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()

	payload := entry.Payload()
	channels, err := p.channelsForRecord(entry)
	if err != nil {
		p.ReportError("redis_err", err, logrus.LogInfo{"vin": entry.Vin, "tx_type": entry.TxType, "txid": entry.Txid})
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}
	for _, channel := range channels {
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()
		if err := p.client.Publish(ctx, channel, payload).Err(); err != nil {
			p.ReportError("redis_err", err, logrus.LogInfo{"channel": channel, "vin": entry.Vin, "tx_type": entry.TxType, "txid": entry.Txid})
			metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
			return
		}
		metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})
		metricsRegistry.publishBytesTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
	}

	p.ProcessReliableAck(entry)
}

// ProcessReliableAck sends to ackChan if reliable ack is configured
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	if _, ok := p.reliableAckTxTypes[entry.TxType]; ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// ReportError to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

// Close the producer
func (p *Producer) Close() error {
	return p.client.Close()
}

func (p *Producer) channelsForRecord(entry *telemetry.Record) ([]string, error) {
	var channels []string

	if p.subscriberSetPrefix != "" {
		members, err := p.subscriberChannels(entry)
		if err != nil {
			return nil, err
		}
		channels = append(channels, members...)
	}

	if p.publishVINTopics {
		channels = append(channels, p.vinChannel(entry))
	}

	return channels, nil
}

// subscriberChannels returns the live subscriber channels registered for the
// record's VIN in the sorted set, purging entries whose lease expiry (epoch
// seconds) has already passed.
func (p *Producer) subscriberChannels(entry *telemetry.Record) ([]string, error) {
	setKey := fmt.Sprintf("%s_%s", p.subscriberSetPrefix, p.vinChannel(entry))
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	now := time.Now().Unix()
	if err := p.client.ZRemRangeByScore(ctx, setKey, "-inf", fmt.Sprintf("(%d", now)).Err(); err != nil {
		return nil, err
	}
	return p.client.ZRange(ctx, setKey, 0, -1).Result()
}

func (p *Producer) vinChannel(entry *telemetry.Record) string {
	return fmt.Sprintf("%s_{%s}", telemetry.BuildTopicName(p.namespace, entry.TxType), entry.Vin)
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "redis_publish_total",
		Help:   "The number of messages published to Redis.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishBytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "redis_publish_total_bytes",
		Help:   "The number of bytes published to Redis.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "redis_err",
		Help:   "The number of errors while publishing to Redis.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "redis_reliable_ack_total",
		Help:   "The number of records published to Redis for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})
}
