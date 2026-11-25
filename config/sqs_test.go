package config

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("SQS configuration", func() {
	It("uses default retries and timeout when omitted", func() {
		var s SQS

		retries, timeout, err := s.options()

		Expect(err).NotTo(HaveOccurred())
		Expect(retries).To(Equal(1))
		Expect(timeout).To(Equal(10 * time.Second))
	})

	It("uses explicit options and permits disabling retries", func() {
		var s SQS
		Expect(json.Unmarshal([]byte(`{"max_retries":0,"publish_timeout_ms":500}`), &s)).To(Succeed())

		retries, timeout, err := s.options()

		Expect(err).NotTo(HaveOccurred())
		Expect(retries).To(BeZero())
		Expect(timeout).To(Equal(500 * time.Millisecond))
	})

	DescribeTable("rejects invalid explicit numeric settings", func(input string) {
		var s SQS
		Expect(json.Unmarshal([]byte(input), &s)).To(Succeed())

		_, _, err := s.options()

		Expect(err).To(HaveOccurred())
	},
		Entry("negative retries", `{"max_retries":-1}`),
		Entry("zero timeout", `{"publish_timeout_ms":0}`),
		Entry("negative timeout", `{"publish_timeout_ms":-1}`),
		Entry("overflow timeout", `{"publish_timeout_ms":9223372036855}`),
	)

	It("maps only requested types with environment, config, then namespace precedence", func() {
		GinkgoT().Setenv("SQS_QUEUE_V", "override")
		GinkgoT().Setenv("SQS_QUEUE_ALERTS", "")
		GinkgoT().Setenv("SQS_QUEUE_CONNECTIVITY", "")
		c := &Config{
			Namespace: "fleet",
			SQS: &SQS{Queues: map[string]string{
				"V": "configured", "alerts": "alert-queue", "unused": "unused.fifo",
			}},
		}

		queues, err := c.CreateSQSQueueMapping([]string{"V", "alerts", "connectivity"})

		Expect(err).NotTo(HaveOccurred())
		Expect(queues).To(Equal(map[string]string{"V": "override", "alerts": "alert-queue", "connectivity": "fleet_connectivity"}))
	})

	DescribeTable("rejects unusable queue names", func(name string) {
		GinkgoT().Setenv("SQS_QUEUE_V", "")
		c := &Config{SQS: &SQS{Queues: map[string]string{"V": name}}}

		_, err := c.CreateSQSQueueMapping([]string{"V"})

		Expect(err).To(HaveOccurred())
	},
		Entry("empty", ""),
		Entry("whitespace", " "),
		Entry("FIFO", "queue.fifo"),
		Entry("URL instead of queue name", "http://queue"),
	)

	It("validates overridden queue names", func() {
		GinkgoT().Setenv("SQS_QUEUE_V", "queue.fifo")
		c := &Config{SQS: &SQS{Queues: map[string]string{"V": "valid"}}}

		_, err := c.CreateSQSQueueMapping([]string{"V"})

		Expect(err).To(HaveOccurred())
	})

	It("requires configuration when SQS is selected", func() {
		logger, _ := logrus.NoOpLogger()
		c := &Config{Records: map[string][]telemetry.Dispatcher{"V": {telemetry.SQS}}}

		_, _, err := c.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), logger, true)

		Expect(err).To(MatchError("expected SQS to be configured"))
	})

	It("registers SQS and a reliable ACK source without contacting AWS", func() {
		// Explicit dummy credentials prevent host credential or metadata discovery.
		GinkgoT().Setenv("AWS_ACCESS_KEY_ID", "test")
		GinkgoT().Setenv("AWS_SECRET_ACCESS_KEY", "test")
		GinkgoT().Setenv("AWS_REGION", "us-west-2")
		GinkgoT().Setenv("AWS_EC2_METADATA_DISABLED", "true")
		GinkgoT().Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
		GinkgoT().Setenv("AWS_CONFIG_FILE", "/dev/null")
		GinkgoT().Setenv("SQS_QUEUE_V", "")
		logger, _ := logrus.NoOpLogger()
		c := &Config{
			Namespace:          "test",
			SQS:                &SQS{OverrideHost: "http://127.0.0.1:1"},
			MetricCollector:    metrics.NewCollector(nil, logger),
			AckChan:            make(chan *telemetry.Record),
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.SQS}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SQS},
		}

		sources, err := c.configureReliableAckSources()
		Expect(err).NotTo(HaveOccurred())
		Expect(sources[telemetry.SQS]).To(HaveKey("V"))

		producers, rules, err := c.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), logger, false)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(producers[telemetry.SQS].Close)
		Expect(rules["V"]).To(ConsistOf(producers[telemetry.SQS]))
	})

	It("rejects SQS as ACK source when it is not a record destination", func() {
		c := &Config{
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.Kafka}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SQS},
		}

		_, err := c.configureReliableAckSources()

		Expect(err).To(HaveOccurred())
	})
})
