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

var _ = Describe("SNS configuration", func() {
	It("uses default retries and timeout when omitted", func() {
		var s SNS

		retries, timeout, err := s.options()

		Expect(err).NotTo(HaveOccurred())
		Expect(retries).To(Equal(1))
		Expect(timeout).To(Equal(10 * time.Second))
	})

	It("uses explicit options and permits disabling retries", func() {
		var s SNS
		Expect(json.Unmarshal([]byte(`{"max_retries":0,"publish_timeout_ms":500}`), &s)).To(Succeed())

		retries, timeout, err := s.options()

		Expect(err).NotTo(HaveOccurred())
		Expect(retries).To(BeZero())
		Expect(timeout).To(Equal(500 * time.Millisecond))
	})

	It("allows the minimum positive timeout", func() {
		var s SNS
		Expect(json.Unmarshal([]byte(`{"publish_timeout_ms":1}`), &s)).To(Succeed())

		_, timeout, err := s.options()

		Expect(err).NotTo(HaveOccurred())
		Expect(timeout).To(Equal(time.Millisecond))
	})

	DescribeTable("rejects invalid explicit numeric settings", func(input string) {
		var s SNS
		Expect(json.Unmarshal([]byte(input), &s)).To(Succeed())

		_, _, err := s.options()

		Expect(err).To(HaveOccurred())
	},
		Entry("negative retries", `{"max_retries":-1}`),
		Entry("zero timeout", `{"publish_timeout_ms":0}`),
		Entry("negative timeout", `{"publish_timeout_ms":-1}`),
		Entry("overflow timeout", `{"publish_timeout_ms":9223372036855}`),
	)

	It("maps only required records with environment overriding explicit ARNs", func() {
		override := "arn:aws:sns:us-west-2:123456789012:override"
		configured := "arn:aws:sns:us-west-2:123456789012:alerts"
		GinkgoT().Setenv("SNS_TOPIC_V", override)
		GinkgoT().Setenv("SNS_TOPIC_ALERTS", "")
		c := &Config{SNS: &SNS{Topics: map[string]string{"V": "invalid", "alerts": configured, "unused": "invalid"}}}

		topics, err := c.CreateSNSTopicMapping([]string{"V", "alerts"})

		Expect(err).NotTo(HaveOccurred())
		Expect(topics).To(Equal(map[string]string{"V": override, "alerts": configured}))
	})

	It("does not derive a topic ARN from namespace", func() {
		GinkgoT().Setenv("SNS_TOPIC_V", "")
		c := &Config{Namespace: "test", SNS: &SNS{}}

		_, err := c.CreateSNSTopicMapping([]string{"V"})

		Expect(err).To(HaveOccurred())
	})

	It("reports an invalid configured topic with its record type", func() {
		GinkgoT().Setenv("SNS_TOPIC_V", "")
		c := &Config{SNS: &SNS{Topics: map[string]string{"V": "topic"}}}

		_, err := c.CreateSNSTopicMapping([]string{"V"})

		Expect(err).To(MatchError(`sns topic for record type "V": must be a topic ARN`))
	})

	It("validates the environment override instead of the configured topic", func() {
		GinkgoT().Setenv("SNS_TOPIC_V", "invalid")
		c := &Config{SNS: &SNS{Topics: map[string]string{"V": "arn:aws:sns:us-west-2:123456789012:vehicles"}}}

		_, err := c.CreateSNSTopicMapping([]string{"V"})

		Expect(err).To(MatchError(`sns topic for record type "V": must be a topic ARN`))
	})

	It("requires config when SNS is selected", func() {
		logger, _ := logrus.NoOpLogger()
		c := &Config{Records: map[string][]telemetry.Dispatcher{"V": {telemetry.SNS}}}

		_, _, err := c.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), logger, true)

		Expect(err).To(MatchError("expected SNS to be configured"))
	})

	It("registers SNS and its ACK source without contacting AWS", func() {
		GinkgoT().Setenv("AWS_ACCESS_KEY_ID", "test")
		GinkgoT().Setenv("AWS_SECRET_ACCESS_KEY", "test")
		GinkgoT().Setenv("AWS_REGION", "us-west-2")
		GinkgoT().Setenv("AWS_EC2_METADATA_DISABLED", "true")
		GinkgoT().Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
		GinkgoT().Setenv("AWS_CONFIG_FILE", "/dev/null")
		GinkgoT().Setenv("SNS_TOPIC_V", "")
		logger, _ := logrus.NoOpLogger()
		c := &Config{
			SNS: &SNS{
				OverrideHost: "http://127.0.0.1:1",
				Topics:       map[string]string{"V": "arn:aws:sns:us-west-2:123456789012:topic"},
			},
			MetricCollector:    metrics.NewCollector(nil, logger),
			AckChan:            make(chan *telemetry.Record),
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.SNS}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SNS},
		}

		sources, err := c.configureReliableAckSources()
		Expect(err).NotTo(HaveOccurred())
		Expect(sources[telemetry.SNS]).To(HaveKey("V"))

		producers, rules, err := c.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), logger, false)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(producers[telemetry.SNS].Close)
		Expect(rules["V"]).To(ConsistOf(producers[telemetry.SNS]))
	})

	It("rejects SNS ACK source when absent from destinations", func() {
		c := &Config{
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.SQS}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SNS},
		}

		_, err := c.configureReliableAckSources()

		Expect(err).To(HaveOccurred())
	})
})
