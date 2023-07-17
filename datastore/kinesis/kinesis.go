package kinesis

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle kinesis interactions
type Producer struct {
	kinesis           *kinesis.Kinesis
	logger            *logrus.Logger
	prometheusEnabled bool
	metricsCollector  metrics.MetricCollector
	streams           map[string]string
}

// Metrics stores metrics reported from this package
type Metrics struct {
	errorCount   adapter.Counter
	publishCount adapter.Counter
	byteTotal    adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer configures and tests the kinesis connection
func NewProducer(maxRetries int, streams map[string]string, overrideHost string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	config := &aws.Config{
		MaxRetries:                    aws.Int(maxRetries),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}
	if overrideHost != "" {
		config = config.WithEndpoint(overrideHost)
	}
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}

	service := kinesis.New(sess, config)
	if _, err := service.ListStreams(&kinesis.ListStreamsInput{Limit: aws.Int64(1)}); err != nil {
		return nil, fmt.Errorf("failed to list streams (test connection): %v", err)
	}

	return &Producer{
		kinesis:           service,
		logger:            logger,
		prometheusEnabled: prometheusEnabled,
		metricsCollector:  metricsCollector,
		streams:           streams,
	}, nil
}

// Produce asynchronously sends the record payload to kineses
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()
	stream, ok := p.streams[entry.TxType]
	if !ok {
		p.logger.Errorf("kinesis_produce_stream_not_configured: %s", entry.TxType)
		return
	}
	kinesisRecord := &kinesis.PutRecordInput{
		Data:         entry.Payload(),
		StreamName:   aws.String(stream),
		PartitionKey: aws.String(entry.Vin),
	}

	kinesisRecordOutput, err := p.kinesis.PutRecord(kinesisRecord)
	if err != nil {
		p.logger.Errorf("kinesis_err: %v", err)
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	p.logger.Debugf("kinesis_publish vin=%s,type=%s,txid=%s,shard_id=%s,sequence_number=%s", entry.Vin, entry.TxType, entry.Txid, *kinesisRecordOutput.ShardId, *kinesisRecordOutput.SequenceNumber)

	metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.byteTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kinesis_err",
		Help:   "The number of errors while producing to Kinesis.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kinesis_publish_total",
		Help:   "The number of messages published to Kinesis.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kinesis_publish_total_bytes",
		Help:   "The number of bytes published to Kinesis.",
		Labels: []string{"record_type"},
	})
}
