package sqs

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

const fixtureQueueURL = "http://local/vehicles"

type producerSettings struct {
	timeout  time.Duration
	queues   map[string]string
	reliable map[string]interface{}
}

func defaultProducerSettings() producerSettings {
	return producerSettings{
		timeout:  time.Second,
		queues:   map[string]string{"V": "vehicles"},
		reliable: map[string]interface{}{"V": true},
	}
}

func makeTestProducer(client sqsiface.SQSAPI, ack chan *telemetry.Record, settings producerSettings) *Producer {
	logger, _ := logrus.NoOpLogger()
	return newProducer(client, settings.queues, settings.timeout,
		metrics.NewCollector(nil, logger), airbrake.NewAirbrakeHandler(nil), ack, settings.reliable, logger)
}

func record() *telemetry.Record {
	return &telemetry.Record{
		TxType: "V", Txid: "tx", Vin: "synthetic", DeviceClientVersion: "1.0",
		PayloadBytes: []byte{0, 255, 128, 3},
	}
}

type fakeClient struct {
	sqsiface.SQSAPI
	mu           sync.Mutex
	lookupInputs []*awssqs.GetQueueUrlInput
	sendInputs   []*awssqs.SendMessageInput
	lookup       func(aws.Context, *awssqs.GetQueueUrlInput) (*awssqs.GetQueueUrlOutput, error)
	send         func(aws.Context, *awssqs.SendMessageInput) (*awssqs.SendMessageOutput, error)
}

//nolint:revive // AWS SDK interface spelling.
func (f *fakeClient) GetQueueUrlWithContext(ctx aws.Context, in *awssqs.GetQueueUrlInput, _ ...request.Option) (*awssqs.GetQueueUrlOutput, error) {
	f.mu.Lock()
	f.lookupInputs = append(f.lookupInputs, in)
	f.mu.Unlock()

	if f.lookup != nil {
		return f.lookup(ctx, in)
	}
	return &awssqs.GetQueueUrlOutput{QueueUrl: aws.String(fixtureQueueURL)}, nil
}

func (f *fakeClient) SendMessageWithContext(ctx aws.Context, in *awssqs.SendMessageInput, _ ...request.Option) (*awssqs.SendMessageOutput, error) {
	f.mu.Lock()
	f.sendInputs = append(f.sendInputs, in)
	f.mu.Unlock()

	if f.send != nil {
		return f.send(ctx, in)
	}
	return &awssqs.SendMessageOutput{MessageId: aws.String("id")}, nil
}

type countingCounter struct {
	value  int64
	labels adapter.Labels
}

func (c *countingCounter) Add(value int64, labels adapter.Labels) {
	c.value += value
	c.labels = labels
}

func (c *countingCounter) Inc(labels adapter.Labels) {
	c.Add(1, labels)
}
