package sqs

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// MaxMessageBytes is the maximum size AWS accepts for a single SQS message, counting
// the body and the message attributes together.
const MaxMessageBytes = 262144

// Producer client to handle SQS interactions
type Producer struct {
	sqs                sqsiface.SQSAPI
	logger             *logrus.Logger
	prometheusEnabled  bool
	metricsCollector   metrics.MetricCollector
	queueNames         map[string]string
	airbrakeHandler    *airbrake.Handler
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}

	// SendMessage addresses a queue by URL, not by name. URLs are resolved on first use
	// and cached here rather than at startup, so the server does not require its queues to
	// exist before it boots. Produce runs on a goroutine per connection, hence the mutex.
	urlMutex  sync.RWMutex
	queueURLs map[string]string
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

// NewProducer creates an SQS client and verifies it can reach SQS before returning.
func NewProducer(maxRetries int, queues map[string]string, overrideHost string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
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

	return NewProducerWithClient(sqs.New(sess, config), queues, prometheusEnabled, metricsCollector, airbrakeHandler, ackChan, reliableAckTxTypes, logger)
}

// NewProducerWithClient builds a Producer around an existing SQS client. It is used by
// NewProducer and is exported for callers that need to supply a customized client.
func NewProducerWithClient(client sqsiface.SQSAPI, queues map[string]string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	// Mirrors the Kinesis producer: prove credentials and connectivity work, without
	// requiring the individual queues to exist yet.
	if _, err := client.ListQueues(&sqs.ListQueuesInput{MaxResults: aws.Int64(1)}); err != nil {
		return nil, fmt.Errorf("failed to list queues (test connection): %v", err)
	}

	return &Producer{
		sqs:                client,
		logger:             logger,
		prometheusEnabled:  prometheusEnabled,
		metricsCollector:   metricsCollector,
		queueNames:         queues,
		queueURLs:          make(map[string]string, len(queues)),
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}, nil
}

// queueURL returns the cached URL for a record type, resolving it on first use.
func (p *Producer) queueURL(txType string) (string, error) {
	p.urlMutex.RLock()
	url, cached := p.queueURLs[txType]
	p.urlMutex.RUnlock()
	if cached {
		return url, nil
	}

	queueName, ok := p.queueNames[txType]
	if !ok {
		return "", fmt.Errorf("no sqs queue configured for record type %q", txType)
	}

	out, err := p.sqs.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: aws.String(queueName)})
	if err != nil {
		return "", fmt.Errorf("failed to resolve sqs queue %q for record type %q: %v", queueName, txType, err)
	}

	url = aws.StringValue(out.QueueUrl)
	p.urlMutex.Lock()
	p.queueURLs[txType] = url
	p.urlMutex.Unlock()
	return url, nil
}

// Produce sends the record payload to the SQS queue configured for its record type.
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()
	queueURL, err := p.queueURL(entry.TxType)
	if err != nil {
		p.ReportError("sqs_produce_queue_unavailable", err, logrus.LogInfo{"record_type": entry.TxType})
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	// SQS message bodies must be valid UTF-8 XML character data, so the protobuf payload
	// cannot be sent as-is. Base64 keeps the bytes intact at the cost of ~33% inflation.
	body := base64.StdEncoding.EncodeToString(entry.Payload())
	attributes := messageAttributes(entry)

	if size := messageSize(body, attributes); size > MaxMessageBytes {
		// Vehicles may send records up to telemetry.SizeLimit (1MB), which can exceed the
		// SQS limit once encoded. Reporting it here gives an actionable error instead of
		// an opaque InvalidParameterValue from AWS.
		metricsRegistry.oversizeCount.Inc(map[string]string{"record_type": entry.TxType})
		p.ReportError("sqs_message_too_large", nil, logrus.LogInfo{
			"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid,
			"encoded_bytes": size, "limit_bytes": MaxMessageBytes,
		})
		return
	}

	output, sendErr := p.sqs.SendMessage(&sqs.SendMessageInput{
		QueueUrl:          aws.String(queueURL),
		MessageBody:       aws.String(body),
		MessageAttributes: attributes,
	})
	if sendErr != nil {
		p.ReportError("sqs_err", sendErr, logrus.LogInfo{"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid})
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	p.ProcessReliableAck(entry)
	p.logger.Log(logrus.DEBUG, "sqs_message_dispatched", logrus.LogInfo{"vin": entry.Vin, "record_type": entry.TxType, "txid": entry.Txid, "message_id": aws.StringValue(output.MessageId)})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.byteTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})
}

// messageAttributes exposes routing metadata so consumers, and SNS subscription filter
// policies, can dispatch without base64-decoding the body first.
func messageAttributes(entry *telemetry.Record) map[string]*sqs.MessageAttributeValue {
	return map[string]*sqs.MessageAttributeValue{
		"record_type": {DataType: aws.String("String"), StringValue: aws.String(entry.TxType)},
		"vin":         {DataType: aws.String("String"), StringValue: aws.String(entry.Vin)},
		"txid":        {DataType: aws.String("String"), StringValue: aws.String(entry.Txid)},
	}
}

// messageSize reports the billed size of a message. AWS counts the body and every
// attribute name, type and value toward the same limit.
func messageSize(body string, attributes map[string]*sqs.MessageAttributeValue) int {
	size := len(body)
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
		Name:   "sqs_err",
		Help:   "The number of errors while producing to SQS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sqs_publish_total",
		Help:   "The number of messages published to SQS.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sqs_publish_total_bytes",
		Help:   "The number of bytes published to SQS, measured before base64 encoding.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sqs_reliable_ack_total",
		Help:   "The number of records produced to SQS for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.oversizeCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "sqs_message_too_large_total",
		Help:   "The number of records dropped because they exceeded the SQS message size limit.",
		Labels: []string{"record_type"},
	})
}
