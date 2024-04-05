package googlepubsub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle google pubsub interactions
type Producer struct {
	pubsubClient      *pubsub.Client
	projectID         string
	namespace         string
	metricsCollector  metrics.MetricCollector
	prometheusEnabled bool
	logger            *logrus.Logger
	airbrakeHandler   *airbrake.AirbrakeHandler
}

// Metrics stores metrics reported from this package
type Metrics struct {
	notConnectedTotal adapter.Counter
	publishCount      adapter.Counter
	publishBytesTotal adapter.Counter
	errorCount        adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

func configurePubsub(projectID string) (*pubsub.Client, error) {
	if projectID == "" {
		return nil, errors.New("GCP Project ID cannot be empty")
	}
	_, useEmulator := os.LookupEnv("PUBSUB_EMULATOR_HOST")
	_, useGcpPubsub := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if useEmulator && useGcpPubsub {
		return nil, errors.New("pubsub cannot initialize with both emulator and GCP resource")
	}
	return pubsub.NewClient(context.Background(), projectID)
}

// NewProducer establishes the pubsub connection and define the dispatch method
func NewProducer(ctx context.Context, prometheusEnabled bool, projectID string, namespace string, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.AirbrakeHandler, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)
	pubsubClient, err := configurePubsub(projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub_connect_error %s", err)
	}

	p := &Producer{
		projectID:         projectID,
		namespace:         namespace,
		pubsubClient:      pubsubClient,
		prometheusEnabled: prometheusEnabled,
		metricsCollector:  metricsCollector,
		logger:            logger,
		airbrakeHandler:   airbrakeHandler,
	}
	p.logger.ActivityLog("pubsub_registerd", logrus.LogInfo{"project": projectID, "namespace": namespace})
	return p, nil
}

// Produce sends the record payload to pubsub
func (p *Producer) Produce(entry *telemetry.Record) {
	ctx := context.Background()

	topicName := telemetry.BuildTopicName(p.namespace, entry.TxType)
	logInfo := logrus.LogInfo{"topic_name": topicName, "txid": entry.Txid}
	pubsubTopic, err := p.createTopicIfNotExists(ctx, topicName)

	if err != nil {
		p.ReportError("pubsub_topic_creation_error", err, logInfo)
		metricsRegistry.notConnectedTotal.Inc(map[string]string{})
		return
	}

	if exists, err := pubsubTopic.Exists(ctx); !exists || err != nil {
		p.ReportError("pubsub_topic_check_error", err, logInfo)
		metricsRegistry.notConnectedTotal.Inc(map[string]string{})
		return
	}

	entry.ProduceTime = time.Now()
	result := pubsubTopic.Publish(ctx, &pubsub.Message{
		Data:       entry.Payload(),
		Attributes: entry.Metadata(),
	})
	if _, err = result.Get(ctx); err != nil {
		p.ReportError("pubsub_err", err, logInfo)
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}
	metricsRegistry.publishBytesTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})

}

func (p *Producer) createTopicIfNotExists(ctx context.Context, topic string) (*pubsub.Topic, error) {
	pubsubTopic := p.pubsubClient.Topic(topic)
	exists, err := pubsubTopic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return pubsubTopic, nil
	}

	return p.pubsubClient.CreateTopic(ctx, topic)
}

// ReportError to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.notConnectedTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "pubsub_not_connected_total",
		Help:   "The number of times pubsub has not been connected when attempting to produce.",
		Labels: []string{},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "pubsub_publish_total",
		Help:   "The number of messages published to pubsub.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishBytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "pubsub_publish_total_bytes",
		Help:   "The number of bytes published to pubsub.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "pubsub_err",
		Help:   "The number of errors while publishing to pubsub.",
		Labels: []string{"record_type"},
	})
}
