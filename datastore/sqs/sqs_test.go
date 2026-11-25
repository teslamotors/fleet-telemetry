package sqs

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("SQS producer", func() {
	var (
		client   *fakeClient
		ack      chan *telemetry.Record
		settings producerSettings
	)

	BeforeEach(func() {
		client = &fakeClient{}
		ack = make(chan *telemetry.Record, 2)
		settings = defaultProducerSettings()
	})

	build := func(client sqsiface.SQSAPI, ack chan *telemetry.Record, settings producerSettings) *Producer {
		p := makeTestProducer(client, ack, settings)
		DeferCleanup(p.Close)
		return p
	}

	Describe("payload and routing", func() {
		It("preserves binary payload bytes and the four explicit attributes", func() {
			entry := record()
			p := build(client, ack, settings)

			p.Produce(entry)

			Expect(client.lookupInputs).To(HaveLen(1))
			Expect(aws.StringValue(client.lookupInputs[0].QueueName)).To(Equal("vehicles"))
			Expect(client.sendInputs).To(HaveLen(1))
			sent := client.sendInputs[0]
			Expect(aws.StringValue(sent.QueueUrl)).To(Equal(fixtureQueueURL))
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(sent.MessageBody))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(sent.MessageAttributes).To(Equal(map[string]*awssqs.MessageAttributeValue{
				"vin":                   {DataType: aws.String("String"), StringValue: aws.String("synthetic")},
				"txid":                  {DataType: aws.String("String"), StringValue: aws.String("tx")},
				"txtype":                {DataType: aws.String("String"), StringValue: aws.String("V")},
				"device_client_version": {DataType: aws.String("String"), StringValue: aws.String("1.0")},
			}))
			Expect(ack).To(Receive(BeIdenticalTo(entry)))
		})

		It("reuses a successfully resolved queue URL", func() {
			p := build(client, ack, settings)

			p.Produce(record())
			p.Produce(record())

			Expect(client.lookupInputs).To(HaveLen(1))
			Expect(client.sendInputs).To(HaveLen(2))
			Expect(aws.StringValue(client.sendInputs[0].QueueUrl)).To(Equal(fixtureQueueURL))
			Expect(aws.StringValue(client.sendInputs[1].QueueUrl)).To(Equal(fixtureQueueURL))
			Expect(ack).To(HaveLen(2))
		})

		It("returns without an ACK receiver when reliable acknowledgements are disabled", func() {
			settings.reliable = nil
			unbufferedAck := make(chan *telemetry.Record)
			p := build(client, unbufferedAck, settings)
			done := make(chan struct{})

			go func() { p.Produce(record()); close(done) }()

			Eventually(done, 2*time.Second).Should(BeClosed())
			Expect(client.sendInputs).To(HaveLen(1))
		})

		It("preserves Unicode JSON payload bytes", func() {
			entry := record()
			entry.PayloadBytes = []byte(`{"value":"한글"}`)
			p := build(client, ack, settings)

			p.Produce(entry)

			Expect(client.sendInputs).To(HaveLen(1))
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(client.sendInputs[0].MessageBody))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(ack).To(Receive(BeIdenticalTo(entry)))
		})
	})

	Describe("publish failures", func() {
		It("does not call AWS or acknowledge a reliable record with no configured queue", func() {
			settings.queues = nil
			p := build(client, ack, settings)

			p.Produce(record())

			Expect(client.lookupInputs).To(BeEmpty())
			Expect(client.sendInputs).To(BeEmpty())
			Expect(ack).To(BeEmpty())
		})

		It("does not send or acknowledge when queue lookup fails", func() {
			client.lookup = func(aws.Context, *awssqs.GetQueueUrlInput) (*awssqs.GetQueueUrlOutput, error) {
				return nil, errors.New("denied")
			}
			p := build(client, ack, settings)

			p.Produce(record())

			Expect(client.lookupInputs).To(HaveLen(1))
			Expect(client.sendInputs).To(BeEmpty())
			Expect(ack).To(BeEmpty())
		})

		It("does not send or acknowledge an empty queue URL", func() {
			client.lookup = func(aws.Context, *awssqs.GetQueueUrlInput) (*awssqs.GetQueueUrlOutput, error) {
				return &awssqs.GetQueueUrlOutput{}, nil
			}
			p := build(client, ack, settings)

			p.Produce(record())

			Expect(client.sendInputs).To(BeEmpty())
			Expect(ack).To(BeEmpty())
		})

		It("retries a failed lookup rather than caching its failure", func() {
			lookups := 0
			client.lookup = func(aws.Context, *awssqs.GetQueueUrlInput) (*awssqs.GetQueueUrlOutput, error) {
				lookups++
				if lookups == 1 {
					return nil, errors.New("unavailable")
				}
				return &awssqs.GetQueueUrlOutput{QueueUrl: aws.String(fixtureQueueURL)}, nil
			}
			p := build(client, ack, settings)

			p.Produce(record())
			p.Produce(record())

			Expect(client.lookupInputs).To(HaveLen(2))
			Expect(client.sendInputs).To(HaveLen(1))
			Expect(ack).To(HaveLen(1))
		})
	})

	Describe("configuration validation", func() {
		DescribeTable("requires a standard queue name", func(name string) {
			err := validate(map[string]string{"V": name}, time.Second)

			Expect(err).To(HaveOccurred())
		},
			Entry("empty queue name", ""),
			Entry("whitespace queue name", " \t\r\n"),
			Entry("FIFO queue", "queue.fifo"),
			Entry("URL instead of queue name", "http://queue"),
		)

		It("rejects a nonpositive publish timeout", func() {
			err := validate(map[string]string{"V": "vehicles"}, 0)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AWS SDK HTTP behavior", func() {
		sdkClient := func(endpoint string, retries int) *awssqs.SQS {
			cfg := aws.NewConfig().WithRegion("us-west-2").WithEndpoint(endpoint).
				WithCredentials(credentials.NewStaticCredentials("fake", "fake", "")).WithMaxRetries(retries)
			return awssqs.New(session.Must(session.NewSession(cfg)))
		}

		It("cancels an HTTP request at the publish deadline without acknowledging it", func() {
			entered := make(chan struct{})
			handlerErrors := make(chan error, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					handlerErrors <- err
					return
				}
				if r.Form.Get("Action") == "GetQueueUrl" {
					_, _ = fmt.Fprintf(w, `<GetQueueUrlResponse><GetQueueUrlResult><QueueUrl>%s</QueueUrl></GetQueueUrlResult></GetQueueUrlResponse>`, fixtureQueueURL)
					return
				}
				close(entered)
				<-r.Context().Done()
			}))
			DeferCleanup(server.Close)
			settings.timeout = 100 * time.Millisecond
			p := build(sdkClient(server.URL, 0), ack, settings)
			done := make(chan struct{})

			go func() { p.Produce(record()); close(done) }()

			Eventually(entered, 2*time.Second).Should(BeClosed())
			Eventually(done, 2*time.Second).Should(BeClosed())
			Expect(handlerErrors).To(BeEmpty())
			Expect(ack).To(BeEmpty())
		})

		It("retries a transient HTTP failure and acknowledges the successful response once", func() {
			var attempts atomic.Int32
			handlerErrors := make(chan error, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					handlerErrors <- err
					return
				}
				if r.Form.Get("Action") == "GetQueueUrl" {
					_, _ = fmt.Fprintf(w, `<GetQueueUrlResponse><GetQueueUrlResult><QueueUrl>%s</QueueUrl></GetQueueUrlResult></GetQueueUrlResponse>`, fixtureQueueURL)
					return
				}
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`<ErrorResponse><Error><Code>ServiceUnavailable</Code><Message>retry</Message></Error></ErrorResponse>`))
					return
				}
				checksum := md5.Sum([]byte(r.Form.Get("MessageBody")))
				_, _ = fmt.Fprintf(w, `<SendMessageResponse><SendMessageResult><MessageId>accepted</MessageId><MD5OfMessageBody>%x</MD5OfMessageBody></SendMessageResult></SendMessageResponse>`, checksum)
			}))
			DeferCleanup(server.Close)
			settings.timeout = 3 * time.Second
			p := build(sdkClient(server.URL, 1), ack, settings)

			p.Produce(record())

			Expect(handlerErrors).To(BeEmpty())
			Expect(attempts.Load()).To(Equal(int32(2)))
			Expect(ack).To(HaveLen(1))
		})
	})

	Describe("metrics", func() {
		var published, bytes, failures, acknowledgements *countingCounter

		BeforeEach(func() {
			// Initialize the production registry before replacing it for this spec.
			p := build(client, ack, settings)
			Expect(p.Close()).To(Succeed())
			saved := metricRegistry
			DeferCleanup(func() { metricRegistry = saved })
			published, bytes, failures = &countingCounter{}, &countingCounter{}, &countingCounter{}
			acknowledgements = &countingCounter{}
			metricRegistry = producerMetrics{
				published: published, bytes: bytes, errors: failures, acks: acknowledgements,
			}
		})

		It("counts accepted payload bytes and the delivered acknowledgement", func() {
			p := build(client, ack, settings)

			p.Produce(record())

			Expect(published.value).To(Equal(int64(1)))
			Expect(bytes.value).To(Equal(int64(4)))
			Expect(acknowledgements.value).To(Equal(int64(1)))
			Expect(failures.value).To(BeZero())
			Expect(ack).To(HaveLen(1))
			Expect(published.labels).To(Equal(adapter.Labels{"record_type": "V"}))
			Expect(bytes.labels).To(Equal(adapter.Labels{"record_type": "V"}))
			Expect(acknowledgements.labels).To(Equal(adapter.Labels{"record_type": "V"}))
		})

		It("counts a failed API call without counting publication or acknowledgement", func() {
			client.send = func(aws.Context, *awssqs.SendMessageInput) (*awssqs.SendMessageOutput, error) {
				return nil, errors.New("denied")
			}
			p := build(client, ack, settings)

			p.Produce(record())

			Expect(client.sendInputs).To(HaveLen(1))
			Expect(failures.value).To(Equal(int64(1)))
			Expect(published.value).To(BeZero())
			Expect(bytes.value).To(BeZero())
			Expect(acknowledgements.value).To(BeZero())
			Expect(ack).To(BeEmpty())
			Expect(failures.labels).To(Equal(adapter.Labels{"record_type": "V"}))
		})

		It("reports an AWS size rejection without acknowledging the record", func() {
			entry := record()
			entry.PayloadBytes = []byte(strings.Repeat("x", telemetry.SizeLimit))
			client.send = func(aws.Context, *awssqs.SendMessageInput) (*awssqs.SendMessageOutput, error) {
				return nil, awserr.New("InvalidParameterValue", "Message must be shorter than the queue limit", nil)
			}
			p := build(client, ack, settings)

			p.Produce(entry)

			Expect(client.sendInputs).To(HaveLen(1))
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(client.sendInputs[0].MessageBody))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(failures.value).To(Equal(int64(1)))
			Expect(published.value).To(BeZero())
			Expect(bytes.value).To(BeZero())
			Expect(acknowledgements.value).To(BeZero())
			Expect(ack).To(BeEmpty())
			Expect(failures.labels).To(Equal(adapter.Labels{"record_type": "V"}))
		})
	})
})
