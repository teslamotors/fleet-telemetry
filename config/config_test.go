package config

import (
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	githublogrus "github.com/sirupsen/logrus"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Test full application config", func() {

	var (
		config    *Config
		producers map[string][]telemetry.Producer
		log       *logrus.Logger
	)

	BeforeEach(func() {
		log, _ = logrus.NoOpLogger()
		config = &Config{
			Host:       "127.0.0.1",
			Port:       443,
			StatusPort: 8080,
			Namespace:  "tesla_telemetry",
			TLS:        &TLS{CAFile: "tesla.ca", ServerCert: "your_own_cert.crt", ServerKey: "your_own_key.key"},
			RateLimit:  &RateLimit{Enabled: true, MessageLimit: 1000, MessageInterval: 30},
			Kafka: &confluent.ConfigMap{
				"bootstrap.servers":        "some.broker:9093",
				"ssl.ca.location":          "kafka.ca",
				"ssl.certificate.location": "kafka.crt",
				"ssl.key.location":         "kafka.key",
			},
			Monitoring:    &metrics.MonitoringConfig{PrometheusMetricsPort: 9090, ProfilerPort: 4269, ProfilingPath: "/tmp/fleet-telemetry/profile/"},
			LogLevel:      "info",
			JSONLogEnable: true,
			Records:       map[string][]telemetry.Dispatcher{"V": {"kafka"}},
		}
	})

	AfterEach(func() {
		os.Clearenv()
		type Closer interface {
			Close() error
		}
		for _, typeProducers := range producers {
			for _, producer := range typeProducers {
				if closer, ok := producer.(Closer); ok {
					err := closer.Close()
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}
	})

	Context("ExtractServiceTLSConfig", func() {
		It("fails when TLS is nil ", func() {
			config = &Config{}
			_, err := config.ExtractServiceTLSConfig(log)
			Expect(err).To(MatchError("tls config is empty - telemetry server is mTLS only, make sure to provide certificates in the config"))
		})

		It("fails when files are missing", func() {
			_, err := config.ExtractServiceTLSConfig(log)
			Expect(err).To(MatchError("open tesla.ca: no such file or directory"))
		})

		It("fails when pem file is invalid", func() {
			tmpCA, err := os.CreateTemp(GinkgoT().TempDir(), "tmpCA")
			Expect(err).NotTo(HaveOccurred())

			_, err = io.WriteString(tmpCA, "-----BEGIN CERTIFICATE-----\nFAKECA\n-----END CERTIFICATE-----")
			Expect(err).NotTo(HaveOccurred())
			config.TLS.CAFile = tmpCA.Name()

			_, err = config.ExtractServiceTLSConfig(log)
			Expect(err).To(MatchError(MatchRegexp("custom ca not properly loaded: .*tmpCA.*")))
		})

		It("uses prod CA", func() {
			config.TLS.CAFile = ""

			tls, err := config.ExtractServiceTLSConfig(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(tls).NotTo(BeNil())
			Expect(tls.ClientCAs).NotTo(BeNil())
			Expect(tls.ClientCAs.Subjects()).To(HaveLen(14)) //nolint:staticcheck
		})

		It("uses eng CA", func() {
			config.TLS.CAFile = ""
			config.UseDefaultEngCA = true

			tls, err := config.ExtractServiceTLSConfig(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(tls).NotTo(BeNil())
			Expect(tls.ClientCAs).NotTo(BeNil())
			Expect(tls.ClientCAs.Subjects()).To(HaveLen(8)) //nolint:staticcheck
		})
	})

	Context("basic config", func() {
		It("use correct ports", func() {
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Port).To(BeEquivalentTo(443))
			Expect(config.StatusPort).To(BeEquivalentTo(8080))
		})

		It("transmitrecords disabled by default", func() {
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.TransmitDecodedRecords).To(BeFalse())
		})

		It("transmitrecords enabled", func() {
			config, err := loadTestApplicationConfig(TestTransmitDecodedRecords)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.TransmitDecodedRecords).To(BeTrue())
		})
	})

	Context("configure kafka", func() {
		It("converts floats to int", func() {
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).NotTo(HaveOccurred())

			_, producers, err = config.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(producers["V"]).To(HaveLen(1))

			value, err := config.Kafka.Get("queue.buffering.max.messages", 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(value.(int)).To(Equal(1000000))
		})
	})

	Context("configure airbrake", func() {
		It("gets config from file", func() {
			config, err := loadTestApplicationConfig(TestAirbrakeConfig)
			Expect(err).NotTo(HaveOccurred())

			_, options, err := config.CreateAirbrakeNotifier(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(options.ProjectKey).To(Equal("test1"))
		})

		It("gets config from env variable", func() {
			projectKey := "environmentProjectKey"
			err := os.Setenv("AIRBRAKE_PROJECT_KEY", projectKey)
			Expect(err).NotTo(HaveOccurred())
			config, err := loadTestApplicationConfig(TestAirbrakeConfig)
			Expect(err).NotTo(HaveOccurred())

			_, options, err := config.CreateAirbrakeNotifier(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(options.ProjectKey).To(Equal(projectKey))
		})
	})

	Context("configure reliable acks", func() {

		DescribeTable("fails",
			func(configInput string, errMessage string) {

				config, err := loadTestApplicationConfig(configInput)
				Expect(err).NotTo(HaveOccurred())

				_, producers, err = config.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
				Expect(err).To(MatchError(errMessage))
				Expect(producers).To(BeNil())
			},
			Entry("when reliable ack is mapped incorrectly", TestBadReliableAckConfig, "pubsub cannot be configured as reliable ack for record: V. Valid datastores configured [kafka]"),
			Entry("when logger is configured as reliable ack", TestLoggerAsReliableAckConfig, "logger cannot be configured as reliable ack for record: V"),
			Entry("when reliable ack is configured for unmapped txtype", TestUnusedTxTypeAsReliableAckConfig, "kafka cannot be configured as reliable ack for record: error since no record mapping exists"),
			Entry("when reliable ack is mapped with unsupported txtype", TestBadTxTypeReliableAckConfig, "reliable ack not needed for txType: connectivity"),
		)

	})

	Context("configure kinesis", func() {
		It("returns an error if kinesis isn't included", func() {
			log, _ := logrus.NoOpLogger()
			config.Records = map[string][]telemetry.Dispatcher{"V": {"kinesis"}}

			var err error
			_, producers, err = config.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).To(MatchError("expected Kinesis to be configured"))
			Expect(producers).To(BeNil())
		})

		It("returns a map", func() {
			config.Kinesis = &Kinesis{Streams: map[string]string{"V": "mystream_V", "errors": "mystream_errors"}}
			err := os.Setenv("KINESIS_STREAM_ERRORS", "test_errors")
			Expect(err).NotTo(HaveOccurred())

			streamMapping := config.CreateKinesisStreamMapping([]string{"V", "errors", "alerts"})
			Expect(streamMapping).To(Equal(map[string]string{
				"V":      "mystream_V",
				"errors": "test_errors",
				"alerts": "tesla_telemetry_alerts",
			}))
		})
	})

	Context("VinsToTrack", func() {

		AfterEach(func() {
			maxVinsToTrack = 20
		})

		It("empty vins to track", func() {
			config, err := loadTestApplicationConfig(TestSmallConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.VinsToTrack()).To(BeEmpty())
		})

		It("valid vins to track", func() {
			config, err := loadTestApplicationConfig(TestVinsToTrackConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.VinsToTrack()).To(HaveLen(2))
		})

		It("returns an error when `vins_signal_tracking_enabled` exceeds limit", func() {
			maxVinsToTrack = 2
			_, err := loadTestApplicationConfig(BadVinsConfig)
			Expect(err).To(MatchError("set the value of `vins_signal_tracking_enabled` less than 2 unique vins"))
		})
	})

	Context("configure pubsub", func() {
		var (
			pubsubConfig *Config
		)

		BeforeEach(func() {
			var err error
			pubsubConfig, err = loadTestApplicationConfig(TestPubsubConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("pubsub does not work when both the environment variables are set", func() {
			log, _ := logrus.NoOpLogger()
			_ = os.Setenv("PUBSUB_EMULATOR_HOST", "some_url")
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "some_service_account_path")
			_, _, err := pubsubConfig.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).To(MatchError("pubsub_connect_error pubsub cannot initialize with both emulator and GCP resource"))
		})

		It("pubsub config works", func() {
			log, _ := logrus.NoOpLogger()
			_ = os.Setenv("PUBSUB_EMULATOR_HOST", "some_url")
			var err error
			_, producers, err = pubsubConfig.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(producers["V"]).NotTo(BeNil())
		})
	})

	Context("configure zmq", func() {
		var zmqConfig *Config

		BeforeEach(func() {
			var err error
			zmqConfig, err = loadTestApplicationConfig(TestZMQConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if zmq isn't included", func() {
			log, _ := logrus.NoOpLogger()
			config.Records = map[string][]telemetry.Dispatcher{"V": {"zmq"}}
			var err error
			_, producers, err = config.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).To(MatchError("expected ZMQ to be configured"))
			Expect(producers).To(BeNil())
			_, producers, err = zmqConfig.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("zmq config works", func() {
			// ZMQ close is async, this removes the need to sync between tests.
			zmqConfig.ZMQ.Addr = "tcp://127.0.0.1:5285"
			log, _ := logrus.NoOpLogger()
			var err error
			_, producers, err = zmqConfig.ConfigureProducers(airbrake.NewAirbrakeHandler(nil), log, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(producers["V"]).NotTo(BeNil())
		})
	})

	Context("configureMetricsCollector", func() {
		It("does not fail when TLS is nil ", func() {
			log, _ := logrus.NoOpLogger()
			config = &Config{}
			config.configureMetricsCollector(log)

			Expect(config.Monitoring).To(BeNil())
		})

		It("fails if not reachable", func() {
			log, _ := logrus.NoOpLogger()
			config.configureMetricsCollector(log)
			Expect(config.MetricCollector).NotTo(BeNil())
		})
	})

	Context("configureLogger", func() {
		It("Should properly configure logger", func() {
			log, _ := logrus.NoOpLogger()
			config.configureLogger(log)

			Expect(githublogrus.GetLevel().String()).To(Equal("info"))
		})
	})
})
