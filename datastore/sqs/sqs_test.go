package sqs_test

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/sqs"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// fakeSQS records calls instead of reaching AWS. Embedding the interface means only the
// methods the producer actually uses need implementing.
type fakeSQS struct {
	sqsiface.SQSAPI

	knownQueues   map[string]string
	sent          []*awssqs.SendMessageInput
	sendErr       error
	listErr       error
	getQueueCalls int
}

func (f *fakeSQS) ListQueues(*awssqs.ListQueuesInput) (*awssqs.ListQueuesOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &awssqs.ListQueuesOutput{}, nil
}

// GetQueueUrl keeps the AWS SDK's spelling because the name has to match sqsiface.SQSAPI.
func (f *fakeSQS) GetQueueUrl(input *awssqs.GetQueueUrlInput) (*awssqs.GetQueueUrlOutput, error) { //nolint:revive
	f.getQueueCalls++
	name := aws.StringValue(input.QueueName)
	url, ok := f.knownQueues[name]
	if !ok {
		return nil, fmt.Errorf("AWS.SimpleQueueService.NonExistentQueue: %s", name)
	}
	return &awssqs.GetQueueUrlOutput{QueueUrl: aws.String(url)}, nil
}

func (f *fakeSQS) SendMessage(input *awssqs.SendMessageInput) (*awssqs.SendMessageOutput, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	f.sent = append(f.sent, input)
	return &awssqs.SendMessageOutput{MessageId: aws.String("message-id-1")}, nil
}

var _ = Describe("SQSProducer", func() {
	var (
		mockLogger    *logrus.Logger
		mockCollector metrics.MetricCollector
		mockAirbrake  *airbrake.Handler
		client        *fakeSQS
		ackChan       chan *telemetry.Record
	)

	BeforeEach(func() {
		mockLogger, _ = logrus.NoOpLogger()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockAirbrake = airbrake.NewAirbrakeHandler(nil)
		ackChan = make(chan *telemetry.Record, 8)
		client = &fakeSQS{knownQueues: map[string]string{
			"telemetry_V": "https://sqs.us-east-1.amazonaws.com/1234567890/telemetry_V",
		}}
	})

	newProducer := func(queues map[string]string, reliableAckTxTypes map[string]interface{}) (telemetry.Producer, error) {
		return sqs.NewProducerWithClient(client, queues, false, mockCollector, mockAirbrake, ackChan, reliableAckTxTypes, mockLogger)
	}

	record := func(payload []byte) *telemetry.Record {
		return &telemetry.Record{TxType: "V", Vin: "TEST123", Txid: "txid-1", PayloadBytes: payload}
	}

	Describe("NewProducerWithClient", func() {
		It("succeeds without requiring the queues to exist yet", func() {
			// Queues are resolved lazily so the server can start before its queues are
			// created, matching how the Kinesis producer only tests connectivity.
			producer, err := newProducer(map[string]string{"V": "not_created_yet"}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(producer).NotTo(BeNil())
			Expect(client.getQueueCalls).To(Equal(0))
		})

		It("fails when SQS is unreachable", func() {
			client.listErr = errors.New("no credentials")
			_, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test connection"))
		})
	})

	Describe("Produce", func() {
		It("sends the payload base64 encoded to the mapped queue", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			payload := []byte{0x00, 0x01, 0x02, 0xff, 0xfe}
			producer.Produce(record(payload))

			Expect(client.sent).To(HaveLen(1))
			sent := client.sent[0]
			Expect(aws.StringValue(sent.QueueUrl)).To(Equal("https://sqs.us-east-1.amazonaws.com/1234567890/telemetry_V"))

			// The raw payload is not valid UTF-8, so it must not be sent verbatim.
			Expect(aws.StringValue(sent.MessageBody)).To(Equal(base64.StdEncoding.EncodeToString(payload)))
			decoded, decodeErr := base64.StdEncoding.DecodeString(aws.StringValue(sent.MessageBody))
			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(payload))
		})

		It("attaches routing attributes so consumers need not decode the body", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())
			producer.Produce(record([]byte("payload-bytes")))

			attributes := client.sent[0].MessageAttributes
			Expect(aws.StringValue(attributes["record_type"].StringValue)).To(Equal("V"))
			Expect(aws.StringValue(attributes["vin"].StringValue)).To(Equal("TEST123"))
			Expect(aws.StringValue(attributes["txid"].StringValue)).To(Equal("txid-1"))
		})

		It("drops records that exceed the SQS size limit once encoded", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			// Under the 1MB telemetry.SizeLimit, but base64 inflation pushes it past the
			// 256KiB SQS limit. AWS would reject this with an opaque error.
			oversize := make([]byte, 200000)
			Expect(len(oversize)).To(BeNumerically("<", telemetry.SizeLimit))
			Expect(base64.StdEncoding.EncodedLen(len(oversize))).To(BeNumerically(">", sqs.MaxMessageBytes))

			producer.Produce(record(oversize))
			Expect(client.sent).To(BeEmpty())
		})

		It("sends records that stay under the limit once encoded", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			payload := make([]byte, 150000)
			Expect(base64.StdEncoding.EncodedLen(len(payload))).To(BeNumerically("<", sqs.MaxMessageBytes))

			producer.Produce(record(payload))
			Expect(client.sent).To(HaveLen(1))
		})

		It("resolves the queue URL once and caches it", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			producer.Produce(record([]byte("one")))
			producer.Produce(record([]byte("two")))

			Expect(client.sent).To(HaveLen(2))
			Expect(client.getQueueCalls).To(Equal(1))
		})

		It("does not send when the queue cannot be resolved", func() {
			producer, err := newProducer(map[string]string{"V": "missing_queue"}, nil)
			Expect(err).NotTo(HaveOccurred())

			producer.Produce(record([]byte("payload-bytes")))
			Expect(client.sent).To(BeEmpty())
		})

		It("does not send when the record type has no configured queue", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			unmapped := record([]byte("payload-bytes"))
			unmapped.TxType = "alerts"
			producer.Produce(unmapped)
			Expect(client.sent).To(BeEmpty())
		})
	})

	Describe("ProcessReliableAck", func() {
		It("acks configured record types after a successful send", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, map[string]interface{}{"V": nil})
			Expect(err).NotTo(HaveOccurred())

			entry := record([]byte("payload-bytes"))
			producer.Produce(entry)
			Expect(ackChan).To(Receive(Equal(entry)))
		})

		It("does not ack when the send fails", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, map[string]interface{}{"V": nil})
			Expect(err).NotTo(HaveOccurred())

			client.sendErr = errors.New("throttled")
			producer.Produce(record([]byte("payload-bytes")))
			Expect(ackChan).NotTo(Receive())
		})

		It("does not ack record types that are not configured for reliable ack", func() {
			producer, err := newProducer(map[string]string{"V": "telemetry_V"}, nil)
			Expect(err).NotTo(HaveOccurred())

			producer.Produce(record([]byte("payload-bytes")))
			Expect(ackChan).NotTo(Receive())
		})
	})
})
