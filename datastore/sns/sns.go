package sns

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer sends telemetry to standard SNS topics.
type Producer struct {
	client   snsiface.SNSAPI
	topics   map[string]string
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

// NewProducer configures SNS without making startup API calls.
func NewProducer(maxRetries int, topics map[string]string, overrideHost string, publishTimeout time.Duration, collector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan *telemetry.Record, reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	if maxRetries < 0 {
		return nil, fmt.Errorf("sns max_retries must not be negative")
	}
	if err := validate(topics, publishTimeout); err != nil {
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
	return newProducer(sns.New(sess), topics, publishTimeout, collector, airbrakeHandler, ackChan, reliableAckTxTypes, logger), nil
}

func validate(topics map[string]string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("sns publish timeout must be positive")
	}
	for recordType, name := range topics {
		if err := ValidateTopicARN(name); err != nil {
			return fmt.Errorf("sns topic for record type %q: %w", recordType, err)
		}
	}
	return nil
}

func newProducer(client snsiface.SNSAPI, topics map[string]string, timeout time.Duration, collector metrics.MetricCollector, handler *airbrake.Handler, ack chan *telemetry.Record, reliable map[string]interface{}, logger *logrus.Logger) *Producer {
	metricOnce.Do(func() {
		counter := func(name, help string) adapter.Counter {
			return collector.RegisterCounter(adapter.CollectorOptions{Name: name, Help: help, Labels: []string{"record_type"}})
		}
		metricRegistry = producerMetrics{
			errors:    counter("sns_err", "SNS publish errors."),
			published: counter("sns_publish_total", "Records accepted by SNS."),
			bytes:     counter("sns_publish_total_bytes", "Payload bytes accepted by SNS before base64 encoding."),
			acks:      counter("sns_reliable_ack_total", "Acknowledgements sent after SNS acceptance."),
		}
	})
	topicCopy := make(map[string]string, len(topics))
	for k, v := range topics {
		topicCopy[k] = v
	}
	reliableCopy := make(map[string]interface{}, len(reliable))
	for k, v := range reliable {
		reliableCopy[k] = v
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Producer{
		client: client, topics: topicCopy,
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

// ValidateTopicARN checks that value identifies a Standard SNS topic.
func ValidateTopicARN(value string) error {
	parsed, err := arn.Parse(value)
	if err != nil || parsed.Partition == "" || parsed.Service != "sns" || parsed.Region == "" || parsed.AccountID == "" || parsed.Resource == "" || strings.ContainsAny(parsed.Resource, ":/ \t\r\n") {
		return fmt.Errorf("must be a topic ARN")
	}
	if strings.HasSuffix(parsed.Resource, ".fifo") {
		return fmt.Errorf("FIFO topics are not supported")
	}
	return nil
}

// Produce sends a base64 payload and acknowledges only successful SNS acceptance.
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
	topic, ok := p.topics[entry.TxType]
	if !ok {
		metricRegistry.errors.Inc(labels)
		p.ReportError("sns_err", fmt.Errorf("no sns topic configured for record type %q", entry.TxType), logrus.LogInfo{"record_type": entry.TxType})
		return
	}
	_, err := p.client.PublishWithContext(ctx, &sns.PublishInput{TopicArn: aws.String(topic), Message: aws.String(body), MessageAttributes: attributes})
	if err != nil {
		metricRegistry.errors.Inc(labels)
		p.ReportError("sns_err", err, logrus.LogInfo{"record_type": entry.TxType})
		return
	}
	metricRegistry.published.Inc(labels)
	metricRegistry.bytes.Add(int64(len(entry.Payload())), labels)
	p.sendAck(entry)
}

func messageAttributes(entry *telemetry.Record) map[string]*sns.MessageAttributeValue {
	values := map[string]string{"vin": entry.Vin, "txid": entry.Txid, "txtype": entry.TxType, "device_client_version": entry.DeviceClientVersion}
	attributes := make(map[string]*sns.MessageAttributeValue, len(values))
	for key, value := range values {
		if value != "" {
			attributes[key] = &sns.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(value)}
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
