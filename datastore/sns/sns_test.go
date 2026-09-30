package sns_test

import (
	"encoding/base64"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/sns"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

const topicARN = "arn:aws:sns:us-east-1:1234567890:telemetry_V"

// fakeSNS records calls instead of reaching AWS. Embedding the interface means only the
// methods the producer actually uses need implementing.
type fakeSNS struct {
	snsiface.SNSAPI

	published  []*awssns.PublishInput
	publishErr error
	listErr    error
}

func (f *fakeSNS) ListTopics(*awssns.ListTopicsInput) (*awssns.ListTopicsOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &awssns.ListTopicsOutput{}, nil
}

func (f *fakeSNS) Publish(input *awssns.PublishInput) (*awssns.PublishOutput, error) {
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	f.published = append(f.published, input)
	return &awssns.PublishOutput{MessageId: aws.String("message-id-1")}, nil
}

var _ = Describe("SNSProducer", func() {
	var (
		mockLogger    *logrus.Logger
		mockCollector metrics.MetricCollector
		mockAirbrake  *airbrake.Handler
		client        *fakeSNS
		ackChan       chan *telemetry.Record
	)

	BeforeEach(func() {
		mockLogger, _ = logrus.NoOpLogger()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockAirbrake = airbrake.NewAirbrakeHandler(nil)
		ackChan = make(chan *telemetry.Record, 8)
		client = &fakeSNS{}
	})

	newProducer := func(topics map[string]string, reliableAckTxTypes map[string]interface{}) (telemetry.Producer, error) {
		return sns.NewProducerWithClient(client, topics, false, mockCollector, mockAirbrake, ackChan, reliableAckTxTypes, mockLogger)
	}

	record := func(payload []byte) *telemetry.Record {
		return &telemetry.Record{TxType: "V", Vin: "TEST123", Txid: "txid-1", PayloadBytes: payload}
	}

	Describe("NewProducerWithClient", func() {
		It("succeeds without requiring the topics to exist yet", func() {
			// Topics are addressed by full ARN, so there is nothing to resolve and no
			// reason to couple server startup to topic creation.
			producer, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(producer).NotTo(BeNil())
		})

		It("fails when SNS is unreachable", func() {
			client.listErr = errors.New("no credentials")
			_, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test connection"))
		})
	})

	Describe("Produce", func() {
		It("publishes the payload base64 encoded to the mapped topic", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).NotTo(HaveOccurred())

			payload := []byte{0x00, 0x01, 0x02, 0xff, 0xfe}
			producer.Produce(record(payload))

			Expect(client.published).To(HaveLen(1))
			published := client.published[0]
			Expect(aws.StringValue(published.TopicArn)).To(Equal(topicARN))

			// The raw payload is not valid UTF-8, so it must not be published verbatim.
			Expect(aws.StringValue(published.Message)).To(Equal(base64.StdEncoding.EncodeToString(payload)))
			decoded, decodeErr := base64.StdEncoding.DecodeString(aws.StringValue(published.Message))
			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(payload))
		})

		It("attaches attributes usable by subscription filter policies", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).NotTo(HaveOccurred())
			producer.Produce(record([]byte("payload-bytes")))

			attributes := client.published[0].MessageAttributes
			Expect(aws.StringValue(attributes["record_type"].StringValue)).To(Equal("V"))
			Expect(aws.StringValue(attributes["vin"].StringValue)).To(Equal("TEST123"))
			Expect(aws.StringValue(attributes["txid"].StringValue)).To(Equal("txid-1"))
		})

		It("drops records that exceed the SNS size limit once encoded", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).NotTo(HaveOccurred())

			oversize := make([]byte, 200000)
			Expect(len(oversize)).To(BeNumerically("<", telemetry.SizeLimit))
			Expect(base64.StdEncoding.EncodedLen(len(oversize))).To(BeNumerically(">", sns.MaxMessageBytes))

			producer.Produce(record(oversize))
			Expect(client.published).To(BeEmpty())
		})

		It("does not publish when the record type has no configured topic", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, nil)
			Expect(err).NotTo(HaveOccurred())

			unmapped := record([]byte("payload-bytes"))
			unmapped.TxType = "alerts"
			producer.Produce(unmapped)
			Expect(client.published).To(BeEmpty())
		})
	})

	Describe("ProcessReliableAck", func() {
		It("acks configured record types after a successful publish", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, map[string]interface{}{"V": nil})
			Expect(err).NotTo(HaveOccurred())

			entry := record([]byte("payload-bytes"))
			producer.Produce(entry)
			Expect(ackChan).To(Receive(Equal(entry)))
		})

		It("does not ack when the publish fails", func() {
			producer, err := newProducer(map[string]string{"V": topicARN}, map[string]interface{}{"V": nil})
			Expect(err).NotTo(HaveOccurred())

			client.publishErr = errors.New("throttled")
			producer.Produce(record([]byte("payload-bytes")))
			Expect(ackChan).NotTo(Receive())
		})
	})
})
