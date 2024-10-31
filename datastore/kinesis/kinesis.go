package kinesis

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle kinesis interactions
type Producer struct {
	kinesis            *kinesis.Kinesis
	logger             *logrus.Logger
	prometheusEnabled  bool
	metricsCollector   metrics.MetricCollector
	streams            map[string]string
	airbrakeHandler    *airbrake.Handler
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
}

// Metrics stores metrics reported from this package
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

// NewProducer configures and tests the kinesis connection
func NewProducer(maxRetries int, streams map[string]string, overrideHost string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
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
		kinesis:            service,
		logger:             logger,
		prometheusEnabled:  prometheusEnabled,
		metricsCollector:   metricsCollector,
		streams:            streams,
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}, nil
}

// Produce asynchronously sends the record payload to kineses
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()
	stream, ok := p.streams[entry.TxType]
	if !ok {
		p.ReportError("kinesis_produce_stream_not_configured", nil, logrus.LogInfo{"record_type": entry.TxType})
		return
	}
	kinesisRecord := &kinesis.PutRecordInput{
		Data:         entry.Payload(),
		StreamName:   aws.String(stream),
		PartitionKey: aws.String(entry.Vin),
	}

	kinesisRecordOutput, err := p.kinesis.PutRecord(kinesisRecord)
	if err != nil {
		p.ReportError("kinesis_err", err, nil)
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}
	p.ProcessReliableAck(entry)
	p.logger.Log(logrus.DEBUG, "kinesis_message_dispatched", logrus.LogInfo{"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid, "shard_id": *kinesisRecordOutput.ShardId, "sequence_number": *kinesisRecordOutput.SequenceNumber})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.byteTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

// Close the producer
func (p *Producer) Close() error {
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

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "kinesis_reliable_ack_total",
		Help:   "The number of records produced to Kinesis for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})
}
