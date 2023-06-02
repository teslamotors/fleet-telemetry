package config

import (
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Test full application config", func() {

	var config *Config

	BeforeEach(func() {
		config = &Config{
			Host:               "127.0.0.1",
			Port:               443,
			StatusPort:         8080,
			Namespace:          "tesla_telemetry",
			TLS:                &TLS{CAFile: "tesla.ca", ServerCert: "your_own_cert.crt", ServerKey: "your_own_key.key"},
			RateLimit:          &RateLimit{Enabled: true, MessageLimit: 1000, MessageInterval: 30},
			ReliableAck:        true,
			ReliableAckWorkers: 15,
			Kafka: &confluent.ConfigMap{
				"bootstrap.servers":        "some.broker:9093",
				"ssl.ca.location":          "kafka.ca",
				"ssl.certificate.location": "kafka.crt",
				"ssl.key.location":         "kafka.key",
			},
			Monitoring:    &Monitoring{PrometheusMetricsPort: 9090, ProfilerPort: 4269, ProfilingPath: "/tmp/tesla-telemetry/profile/"},
			LogLevel:      "info",
			JSONLogEnable: true,
			Records:       map[string][]telemetry.Dispatcher{"FS": {"kafka"}},
		}
	})

	Context("ExtractServiceTLSConfig", func() {
		It("fails when TLS is nil ", func() {
			config = &Config{}
			_, err := config.ExtractServiceTLSConfig()
			Expect(err).To(MatchError("tls config is empty - telemetry server is mTLS only, make sure to provide certificates in the config"))
		})

		It("fails when files are missing", func() {
			_, err := config.ExtractServiceTLSConfig()
			Expect(err).To(MatchError("open tesla.ca: no such file or directory"))
		})

		It("fails when pem file is invalid", func() {
			tmpCA, err := os.CreateTemp(GinkgoT().TempDir(), "tmpCA")
			Expect(err).To(BeNil())

			_, err = io.WriteString(tmpCA, "-----BEGIN CERTIFICATE-----\nFAKECA\n-----END CERTIFICATE-----")
			Expect(err).To(BeNil())
			config.TLS.CAFile = tmpCA.Name()

			_, err = config.ExtractServiceTLSConfig()
			Expect(err).To(MatchError(MatchRegexp("tls ca not properly loaded: .*tmpCA.*")))
		})

		It("uses prod CA", func() {
			config.TLS.CAFile = ""

			tls, err := config.ExtractServiceTLSConfig()
			Expect(err).To(BeNil())
			Expect(tls).ToNot(BeNil())
			Expect(tls.ClientCAs).ToNot(BeNil())
			Expect(len(tls.ClientCAs.Subjects())).To(Equal(14)) //nolint:staticcheck
		})

		It("uses eng CA", func() {
			config.TLS.CAFile = ""
			config.UseDefaultEngCA = true

			tls, err := config.ExtractServiceTLSConfig()
			Expect(err).To(BeNil())
			Expect(tls).ToNot(BeNil())
			Expect(tls.ClientCAs).ToNot(BeNil())
			Expect(len(tls.ClientCAs.Subjects())).To(Equal(8)) //nolint:staticcheck
		})
	})

	Context("configure ports", func() {
		It("use correct ports", func() {
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).To(BeNil())
			Expect(config.Port).To(BeEquivalentTo(443))
			Expect(config.StatusPort).To(BeEquivalentTo(8080))
		})
	})

	Context("configure kafka", func() {
		It("converts floats to int", func() {
			log, _ := test.NewNullLogger()
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).To(BeNil())

			producers, err := config.ConfigureProducers(log)
			Expect(err).To(BeNil())
			Expect(producers["FS"]).To(HaveLen(1))
			value, err := config.Kafka.Get("queue.buffering.max.messages", 10)
			Expect(err).To(BeNil())
			Expect(value.(int)).To(Equal(1000000))
		})
	})

	Context("configurePubsub", func() {
		var (
			pubsubConfig *Config
		)
		BeforeEach(func() {
			var err error
			pubsubConfig, err = loadTestApplicationConfig(TestPubsubConfig)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			os.Clearenv()
		})

		It("pubsub does not work when environment variable is not set for emulator", func() {
			log, _ := test.NewNullLogger()
			_, err := pubsubConfig.ConfigureProducers(log)
			Expect(err).To(MatchError("pubsub_connect_error must set environment variable GOOGLE_APPLICATION_CREDENTIALS or PUBSUB_EMULATOR_HOST"))
		})

		It("pubsub does not work when both the environment variables are set", func() {
			log, _ := test.NewNullLogger()
			_ = os.Setenv("PUBSUB_EMULATOR_HOST", "some_url")
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "some_service_account_path")
			_, err := pubsubConfig.ConfigureProducers(log)
			Expect(err).To(MatchError("pubsub_connect_error pubsub cannot initialize with both emulator and GCP resource"))
		})

		It("pubsub config works", func() {
			log, _ := test.NewNullLogger()
			_ = os.Setenv("PUBSUB_EMULATOR_HOST", "some_url")
			producers, err := pubsubConfig.ConfigureProducers(log)
			Expect(err).To(BeNil())
			Expect(producers["FS"]).NotTo(BeNil())
		})
	})

	Context("configureStatsCollector", func() {
		It("does not fail when TLS is nil ", func() {
			log, _ := test.NewNullLogger()
			config = &Config{}
			config.configureStatsCollector(log)

			Expect(config.Monitoring).To(BeNil())
		})

		It("fails if not reachable", func() {
			log, _ := test.NewNullLogger()
			config.configureStatsCollector(log)
			Expect(config.MetricCollector).To(Not(BeNil()))
		})
	})

	Context("configureLogger", func() {
		It("Should properly configure logger", func() {
			log, _ := test.NewNullLogger()
			config.configureLogger(log)

			Expect(log.Level).To(Equal(logrus.InfoLevel))
		})
	})
})
