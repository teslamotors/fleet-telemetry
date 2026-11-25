package sns

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

const vehicleTopicARN = "arn:aws:sns:us-west-2:123456789012:vehicles"

type fakeClient struct {
	snsiface.SNSAPI
	send     func(aws.Context, *awssns.PublishInput) (*awssns.PublishOutput, error)
	mu       sync.Mutex
	requests []*awssns.PublishInput
}

func (f *fakeClient) PublishWithContext(ctx aws.Context, in *awssns.PublishInput, _ ...request.Option) (*awssns.PublishOutput, error) {
	f.mu.Lock()
	f.requests = append(f.requests, in)
	f.mu.Unlock()
	if f.send != nil {
		return f.send(ctx, in)
	}
	return &awssns.PublishOutput{MessageId: aws.String("id")}, nil
}

func (f *fakeClient) publishedRequests() []*awssns.PublishInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*awssns.PublishInput(nil), f.requests...)
}

func record() *telemetry.Record {
	return &telemetry.Record{TxType: "V", Txid: "tx", Vin: "synthetic", DeviceClientVersion: "1.0", PayloadBytes: []byte{0, 255, 128, 3}}
}

type producerSettings struct {
	timeout  time.Duration
	topics   map[string]string
	reliable map[string]interface{}
}

func defaultProducerSettings() producerSettings {
	return producerSettings{
		timeout:  time.Second,
		topics:   map[string]string{"V": vehicleTopicARN},
		reliable: map[string]interface{}{"V": true},
	}
}

func testProducer(t interface{ Cleanup(func()) }, client snsiface.SNSAPI, ack chan *telemetry.Record, settings producerSettings) *Producer {
	logger, _ := logrus.NoOpLogger()
	p := newProducer(client, settings.topics, settings.timeout, metrics.NewCollector(nil, logger), airbrake.NewAirbrakeHandler(nil), ack, settings.reliable, logger)
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// Package tests run sequentially; these counters observe a scoped registry replacement.
type countingCounter struct {
	value  int64
	labels adapter.Labels
}

func (c *countingCounter) Add(value int64, labels adapter.Labels) {
	c.value += value
	c.labels = labels
}
func (c *countingCounter) Inc(labels adapter.Labels) { c.Add(1, labels) }
