package sns

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// MaxMessageBytes is the maximum size AWS accepts for a single SNS message, counting
// the body and the message attributes together.
const MaxMessageBytes = 262144

// Producer client to handle SNS interactions
type Producer struct {
	sns                snsiface.SNSAPI
	logger             *logrus.Logger
	prometheusEnabled  bool
	metricsCollector   metrics.MetricCollector
	topicARNs          map[string]string
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
	oversizeCount    adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer creates an SNS client and verifies it can reach SNS before returning.
func NewProducer(maxRetries int, topics map[string]string, overrideHost string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
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

	return NewProducerWithClient(sns.New(sess, config), topics, prometheusEnabled, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
}

// NewProducerWithClient builds a Producer around an existing SNS client. It is used by
// NewProducer and is exported for callers that need to supply a customized client.
func NewProducerWithClient(client snsiface.SNSAPI, topics map[string]string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	// Mirrors the Kinesis producer: prove credentials and connectivity work. Topics are
	// addressed by full ARN, so there is nothing to resolve per record type and no reason
	// to require the topics to exist before the server boots.
	if _, err := client.ListTopics(&sns.ListTopicsInput{}); err != nil {
		return nil, fmt.Errorf("failed to list topics (test connection): %v", err)
	}

	return &Producer{
		sns:                client,
		logger:             logger,
		prometheusEnabled:  prometheusEnabled,
		metricsCollector:   metricsCollector,
		topicARNs:          topics,
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}, nil
}

// Produce publishes the record payload to the SNS topic configured for its record type.
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()
	topicARN, ok := p.topicARNs[entry.TxType]
	if !ok {
		p.ReportError("sns_produce_topic_not_configured", nil, logrus.LogInfo{"record_type": entry.TxType})
		return
	}

	// SNS messages must be valid UTF-8, so the protobuf payload cannot be published
	// as-is. Base64 keeps the bytes intact at the cost of ~33% inflation.
	message := base64.StdEncoding.EncodeToString(entry.Payload())
	attributes := messageAttributes(entry)

	if size := messageSize(message, attributes); size > MaxMessageBytes {
		// Vehicles may send records up to telemetry.SizeLimit (1MB), which can exceed the
		// SNS limit once encoded. Reporting it here gives an actionable error instead of
		// an opaque InvalidParameter from AWS.
		metricsRegistry.oversizeCount.Inc(map[string]string{"record_type": entry.TxType})
		p.ReportError("sns_message_too_large", nil, logrus.LogInfo{
			"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid,
			"encoded_bytes": size, "limit_bytes": MaxMessageBytes,
		})
		return
	}

	output, err := p.sns.Publish(&sns.PublishInput{
		TopicArn:          aws.String(topicARN),
		Message:           aws.String(message),
		MessageAttributes: attributes,
	})
	if err != nil {
		p.ReportError("sns_err", err, logrus.LogInfo{"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid})
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	p.ProcessReliableAck(entry)
	p.logger.Log(logrus.DEBUG, "sns_message_dispatched", logrus.LogInfo{"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid, "message_id": aws.StringValue(output.MessageId)})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.byteTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

// messageAttributes exposes routing metadata so subscription filter policies can route
// without base64-decoding the message first.
func messageAttributes(entry *telemetry.Record) map[string]*sns.MessageAttributeValue {
	return map[string]*sns.MessageAttributeValue{
		"record_type": {DataType: aws.String("String"), StringValue: aws.String(entry.TxType)},
		"vin":         {DataType: aws.String("String"), StringValue: aws.String(entry.Vin)},
		"txid":        {DataType: aws.String("String"), StringValue: aws.String(entry.Txid)},
	}
}

// messageSize reports the billed size of a message. AWS counts the body and every
// attribute name, type and value toward the same limit.
func messageSize(message string, attributes map[string]*sns.MessageAttributeValue) int {
	size := len(message)
	for name, attribute := range attributes {
		size += len(name) + len(aws.StringValue(attribute.DataType)) + len(aws.StringValue(attribute.StringValue))
	}
	return size
}

// Close the producer
func (p *Producer) Close() error {
	return nil
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

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sns_err",
		Help:   "The number of errors while publishing to SNS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sns_publish_total",
		Help:   "The number of messages published to SNS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sns_publish_total_bytes",
		Help:   "The number of bytes published to SNS, measured before base64 encoding.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sns_reliable_ack_total",
		Help:   "The number of records produced to SNS for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.oversizeCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sns_message_too_large_total",
		Help:   "The number of records dropped because they exceeded the SNS message size limit.",
		Labels: []string{"record_type"},
	})
}
