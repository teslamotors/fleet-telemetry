package sns

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("SNS producer", func() {
	Context("payload and routing", func() {
		It("publishes binary bytes to the configured topic with all attributes and acknowledges the record", func() {
			client := &fakeClient{}
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), client, ack, defaultProducerSettings())
			entry := record()

			p.Produce(entry)

			requests := client.publishedRequests()
			Expect(requests).To(HaveLen(1))
			request := requests[0]
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(request.Message))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(aws.StringValue(request.TopicArn)).To(Equal(vehicleTopicARN))
			expected := map[string]string{"vin": "synthetic", "txid": "tx", "txtype": "V", "device_client_version": "1.0"}
			Expect(request.MessageAttributes).To(HaveLen(len(expected)))
			for key, value := range expected {
				Expect(request.MessageAttributes).To(HaveKey(key))
				attribute := request.MessageAttributes[key]
				Expect(attribute).NotTo(BeNil(), "attribute %s", key)
				Expect(aws.StringValue(attribute.StringValue)).To(Equal(value), "attribute %s", key)
				Expect(aws.StringValue(attribute.DataType)).To(Equal("String"), "attribute %s", key)
			}
			Expect(ack).To(Receive(BeIdenticalTo(entry)))
		})

		It("returns without an ACK receiver when reliable acknowledgements are disabled", func() {
			settings := defaultProducerSettings()
			settings.reliable = map[string]interface{}{}
			client := &fakeClient{}
			ack := make(chan *telemetry.Record)
			p := testProducer(GinkgoT(), client, ack, settings)
			done := make(chan struct{})

			go func() { p.Produce(record()); close(done) }()

			Eventually(done, 2*time.Second).Should(BeClosed())
			Expect(client.publishedRequests()).To(HaveLen(1))
			Expect(ack).NotTo(Receive())
		})

		It("preserves Unicode JSON payload bytes", func() {
			entry := record()
			entry.PayloadBytes = []byte(`{"value":"한글"}`)
			client := &fakeClient{}
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), client, ack, defaultProducerSettings())

			p.Produce(entry)

			requests := client.publishedRequests()
			Expect(requests).To(HaveLen(1))
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(requests[0].Message))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(ack).To(Receive(BeIdenticalTo(entry)))
		})
	})

	Context("publish failures", func() {

		It("lets SNS enforce message size and does not acknowledge its rejection", func() {
			client := &fakeClient{send: func(aws.Context, *awssns.PublishInput) (*awssns.PublishOutput, error) {
				return nil, awserr.New("InvalidParameter", "Message too long", nil)
			}}
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), client, ack, defaultProducerSettings())
			entry := record()
			entry.PayloadBytes = []byte(strings.Repeat("x", 300000))

			p.Produce(entry)

			requests := client.publishedRequests()
			Expect(requests).To(HaveLen(1))
			decoded, err := base64.StdEncoding.DecodeString(aws.StringValue(requests[0].Message))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(Equal(entry.PayloadBytes))
			Expect(ack).To(BeEmpty())
		})

		It("neither calls the SDK nor acknowledges a reliable record without its topic mapping", func() {
			settings := defaultProducerSettings()
			settings.topics = map[string]string{}
			client := &fakeClient{}
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), client, ack, settings)

			p.Produce(record())

			Expect(client.publishedRequests()).To(BeEmpty())
			Expect(ack).To(BeEmpty())
		})
	})

	Context("configuration validation", func() {
		DescribeTable("rejects invalid topic destinations", func(topic string) {
			err := validate(map[string]string{"V": topic}, time.Second)

			Expect(err).To(HaveOccurred())
		},
			Entry("empty", ""),
			Entry("plain name", "queue"),
			Entry("URL", "https://topic"),
			Entry("FIFO", "arn:aws:sns:us-west-2:123456789012:vehicles.fifo"),
			Entry("wrong service", "arn:aws:sqs:us-west-2:123456789012:vehicles"),
			Entry("subscription ARN", "arn:aws:sns:us-west-2:123456789012:vehicles:subscription"),
			Entry("path", "arn:aws:sns:us-west-2:123456789012:path/topic"),
			Entry("whitespace", "arn:aws:sns:us-west-2:123456789012:bad topic"),
			Entry("missing partition", "arn::sns:us-west-2:123456789012:topic"),
			Entry("missing region", "arn:aws:sns::123456789012:topic"),
			Entry("missing account", "arn:aws:sns:us-west-2::topic"),
			Entry("missing resource", "arn:aws:sns:us-west-2:123456789012:"),
		)

		DescribeTable("rejects invalid timeouts", func(timeout time.Duration) {
			err := validate(nil, timeout)

			Expect(err).To(HaveOccurred())
		},
			Entry("zero timeout", time.Duration(0)),
			Entry("negative timeout", -time.Second),
		)
	})

	Context("AWS SDK HTTP behavior", func() {
		It("cancels an in-flight HTTP request at the publish deadline without acknowledging", func() {
			parseErrors := make(chan error, 1)
			cancelled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				parseErrors <- r.ParseForm()
				<-r.Context().Done()
				close(cancelled)
			}))
			defer server.Close()
			sess := session.Must(session.NewSession(aws.NewConfig().WithRegion("us-west-2").WithEndpoint(server.URL).WithCredentials(credentials.NewStaticCredentials("fake", "fake", "")).WithMaxRetries(0)))
			settings := defaultProducerSettings()
			settings.timeout = 100 * time.Millisecond
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), awssns.New(sess), ack, settings)
			done := make(chan struct{})

			go func() { p.Produce(record()); close(done) }()

			Eventually(parseErrors, 2*time.Second).Should(Receive(BeNil()))
			Eventually(done, 2*time.Second).Should(BeClosed())
			Eventually(cancelled, 2*time.Second).Should(BeClosed())
			Expect(ack).To(BeEmpty())
		})

		It("retries a transient HTTP failure and acknowledges after acceptance", func() {
			var attempts atomic.Int32
			parseErrors := make(chan error, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				parseErrors <- r.ParseForm()
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`<ErrorResponse><Error><Code>ServiceUnavailable</Code><Message>retry</Message></Error></ErrorResponse>`))
					return
				}
				_, _ = w.Write([]byte(`<PublishResponse><PublishResult><MessageId>accepted</MessageId></PublishResult></PublishResponse>`))
			}))
			defer server.Close()
			sess := session.Must(session.NewSession(aws.NewConfig().WithRegion("us-west-2").WithEndpoint(server.URL).WithCredentials(credentials.NewStaticCredentials("fake", "fake", "")).WithMaxRetries(1)))
			settings := defaultProducerSettings()
			settings.timeout = 3 * time.Second
			ack := make(chan *telemetry.Record, 1)
			p := testProducer(GinkgoT(), awssns.New(sess), ack, settings)
			entry := record()

			p.Produce(entry)

			Expect(attempts.Load()).To(Equal(int32(2)))
			Expect(parseErrors).To(Receive(BeNil()))
			Expect(parseErrors).To(Receive(BeNil()))
			Expect(ack).To(Receive(BeIdenticalTo(entry)))
		})
	})

	Context("metrics", func() {
		var (
			client                           *fakeClient
			ack                              chan *telemetry.Record
			p                                *Producer
			published, bytes, failures, acks *countingCounter
		)

		BeforeEach(func() {
			client = &fakeClient{}
			ack = make(chan *telemetry.Record, 1)
			p = testProducer(GinkgoT(), client, ack, defaultProducerSettings())
			saved := metricRegistry
			DeferCleanup(func() { metricRegistry = saved })
			published, bytes, failures, acks = &countingCounter{}, &countingCounter{}, &countingCounter{}, &countingCounter{}
			metricRegistry = producerMetrics{published: published, bytes: bytes, errors: failures, acks: acks}
		})

		It("counts accepted payload bytes and the delivered acknowledgement", func() {
			p.Produce(record())

			Expect(client.publishedRequests()).To(HaveLen(1))
			Expect(published.value).To(Equal(int64(1)))
			Expect(bytes.value).To(Equal(int64(4)))
			Expect(failures.value).To(BeZero())
			Expect(acks.value).To(Equal(int64(1)))
			Expect(ack).To(HaveLen(1))
			Expect(published.labels).To(Equal(adapter.Labels{"record_type": "V"}))
			Expect(bytes.labels).To(Equal(adapter.Labels{"record_type": "V"}))
			Expect(acks.labels).To(Equal(adapter.Labels{"record_type": "V"}))
		})

		It("counts a failed API call without counting publication or acknowledgement", func() {
			client.send = func(aws.Context, *awssns.PublishInput) (*awssns.PublishOutput, error) {
				return nil, errors.New("denied")
			}

			p.Produce(record())

			Expect(client.publishedRequests()).To(HaveLen(1))
			Expect(failures.value).To(Equal(int64(1)))
			Expect(published.value).To(BeZero())
			Expect(bytes.value).To(BeZero())
			Expect(acks.value).To(BeZero())
			Expect(ack).To(BeEmpty())
			Expect(failures.labels).To(Equal(adapter.Labels{"record_type": "V"}))
		})
	})
})
