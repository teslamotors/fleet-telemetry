package sqs

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
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

// Producer sends telemetry to standard SQS queues.
type Producer struct {
	client   sqsiface.SQSAPI
	queues   map[string]string
	urls     map[string]string
	timeout  time.Duration
	logger   *logrus.Logger
	airbrake *airbrake.Handler
	ackChan  chan *telemetry.Record
	reliable map[string]interface{}
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	pending  sync.WaitGroup
}

type producerMetrics struct{ errors, published, bytes, acks adapter.Counter }

var metricRegistry producerMetrics
var metricOnce sync.Once

// NewProducer configures SQS without requiring account-wide queue listing permissions.
func NewProducer(maxRetries int, queues map[string]string, overrideHost string, publishTimeout time.Duration, collector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan *telemetry.Record, reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	if maxRetries < 0 {
		return nil, fmt.Errorf("sqs max_retries must not be negative")
	}
	if err := validate(queues, publishTimeout); err != nil {
		return nil, err
	}
	cfg := aws.NewConfig().WithMaxRetries(maxRetries)
	if overrideHost != "" {
		cfg = cfg.WithEndpoint(overrideHost)
	}
	sess, err := session.NewSessionWithOptions(session.Options{Config: *cfg, SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		return nil, err
	}
	return newProducer(sqs.New(sess), queues, publishTimeout, collector, airbrakeHandler, ackChan, reliableAckTxTypes, logger), nil
}

func validate(queues map[string]string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("sqs publish timeout must be positive")
	}
	for recordType, name := range queues {
		if err := ValidateQueueName(name); err != nil {
			return fmt.Errorf("sqs queue for record type %q: %w", recordType, err)
		}
	}
	return nil
}

// ValidateQueueName rejects empty names, queue URLs and unsupported FIFO queues.
func ValidateQueueName(name string) error {
	if strings.TrimSpace(name) == "" || strings.Contains(name, "/") || strings.HasSuffix(name, ".fifo") {
		return fmt.Errorf("must be a standard queue name")
	}
	return nil
}

func newProducer(client sqsiface.SQSAPI, queues map[string]string, timeout time.Duration, collector metrics.MetricCollector, handler *airbrake.Handler, ack chan *telemetry.Record, reliable map[string]interface{}, logger *logrus.Logger) *Producer {
	metricOnce.Do(func() {
		counter := func(name, help string) adapter.Counter {
			return collector.RegisterCounter(adapter.CollectorOptions{Name: name, Help: help, Labels: []string{"record_type"}})
		}
		metricRegistry = producerMetrics{
			errors:    counter("sqs_err", "SQS publish errors."),
			published: counter("sqs_publish_total", "Records accepted by SQS."),
			bytes:     counter("sqs_publish_total_bytes", "Payload bytes accepted by SQS before base64 encoding."),
			acks:      counter("sqs_reliable_ack_total", "Acknowledgements sent after SQS acceptance."),
		}
	})
	queueCopy := make(map[string]string, len(queues))
	for k, v := range queues {
		queueCopy[k] = v
	}
	reliableCopy := make(map[string]interface{}, len(reliable))
	for k, v := range reliable {
		reliableCopy[k] = v
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Producer{
		client: client, queues: queueCopy, urls: make(map[string]string),
		timeout: timeout,
		logger:  logger, airbrake: handler,
		ackChan: ack, reliable: reliableCopy,
		ctx: ctx, cancel: cancel,
	}
}

func (p *Producer) begin() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.pending.Add(1)
	return true
}

func (p *Producer) queueURL(ctx context.Context, recordType string) (string, error) {
	p.mu.Lock()
	cached, ok := p.urls[recordType]
	p.mu.Unlock()
	if ok {
		return cached, nil
	}
	name, ok := p.queues[recordType]
	if !ok {
		return "", fmt.Errorf("no sqs queue configured for record type %q", recordType)
	}
	out, err := p.client.GetQueueUrlWithContext(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		return "", err
	}
	if out == nil || aws.StringValue(out.QueueUrl) == "" {
		return "", fmt.Errorf("sqs returned an empty queue URL")
	}
	p.mu.Lock()
	p.urls[recordType] = *out.QueueUrl
	p.mu.Unlock()
	return *out.QueueUrl, nil
}

// Produce sends a base64 payload and acknowledges only successful SQS acceptance.
func (p *Producer) Produce(entry *telemetry.Record) {
	if !p.begin() {
		return
	}
	defer p.pending.Done()
	entry.ProduceTime = time.Now()
	labels := map[string]string{"record_type": entry.TxType}
	body := base64.StdEncoding.EncodeToString(entry.Payload())
	attributes := messageAttributes(entry)
	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()
	queueURL, err := p.queueURL(ctx, entry.TxType)
	if err == nil {
		_, err = p.client.SendMessageWithContext(ctx, &sqs.SendMessageInput{QueueUrl: aws.String(queueURL), MessageBody: aws.String(body), MessageAttributes: attributes})
	}
	if err != nil {
		metricRegistry.errors.Inc(labels)
		p.ReportError("sqs_err", err, logrus.LogInfo{"record_type": entry.TxType})
		return
	}
	metricRegistry.published.Inc(labels)
	metricRegistry.bytes.Add(int64(len(entry.Payload())), labels)
	p.sendAck(entry)
}

func messageAttributes(entry *telemetry.Record) map[string]*sqs.MessageAttributeValue {
	values := map[string]string{"vin": entry.Vin, "txid": entry.Txid, "txtype": entry.TxType, "device_client_version": entry.DeviceClientVersion}
	attributes := make(map[string]*sqs.MessageAttributeValue, len(values))
	for key, value := range values {
		if value != "" {
			attributes[key] = &sqs.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(value)}
		}
	}
	return attributes
}

// ProcessReliableAck sends an acknowledgement unless this producer is closing.
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	if !p.begin() {
		return
	}
	defer p.pending.Done()
	p.sendAck(entry)
}
func (p *Producer) sendAck(entry *telemetry.Record) {
	if _, ok := p.reliable[entry.TxType]; !ok {
		return
	}
	if p.ctx.Err() != nil {
		return
	}
	select {
	case <-p.ctx.Done():
	case p.ackChan <- entry:
		metricRegistry.acks.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// Close cancels requests and waits for registered operations, including blocked acknowledgements.
func (p *Producer) Close() error {
	p.mu.Lock()
	p.closed = true
	p.cancel()
	p.mu.Unlock()
	p.pending.Wait()
	return nil
}

// ReportError reports a dispatcher error without logging the payload.
func (p *Producer) ReportError(message string, err error, info logrus.LogInfo) {
	p.airbrake.ReportLogMessage(logrus.ERROR, message, err, info)
	p.logger.ErrorLog(message, err, info)
}
