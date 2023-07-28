package config

import (
	"os"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/prometheus"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Test application config initialization", func() {
	It("loads the config properly", func() {
		expectedConfig := &Config{
			Host:               "127.0.0.1",
			Port:               443,
			StatusPort:         8080,
			Namespace:          "tesla_telemetry",
			TLS:                &TLS{CAFile: "tesla.ca", ServerCert: "your_own_cert.crt", ServerKey: "your_own_key.key"},
			RateLimit:          &RateLimit{Enabled: true, MessageLimit: 1000, MessageInterval: 30},
			ReliableAck:        true,
			ReliableAckWorkers: 15,
			Kafka: &confluent.ConfigMap{
				"bootstrap.servers":            "some.broker1:9093,some.broker1:9093",
				"ssl.ca.location":              "kafka.ca",
				"ssl.certificate.location":     "kafka.crt",
				"ssl.key.location":             "kafka.key",
				"queue.buffering.max.messages": float64(1000000),
			},
			Monitoring:      &metrics.MonitoringConfig{PrometheusMetricsPort: 9090, ProfilerPort: 4269, ProfilingPath: "/tmp/fleet-telemetry/profile"},
			MetricCollector: prometheus.NewCollector(),
			LogLevel:        "info",
			JSONLogEnable:   true,
			Records:         map[string][]telemetry.Dispatcher{"FS": {"kafka"}},
		}

		loadedConfig, err := loadTestApplicationConfig(TestConfig)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig.MetricCollector = loadedConfig.MetricCollector
		expectedConfig.AckChan = loadedConfig.AckChan
		Expect(loadedConfig).To(Equal(expectedConfig))
	})

	It("loads small config properly", func() {
		expectedConfig := &Config{
			Host:       "127.0.0.1",
			Port:       443,
			StatusPort: 8080,
			Namespace:  "tesla_telemetry",
			TLS:        &TLS{CAFile: "tesla.ca", ServerCert: "your_own_cert.crt", ServerKey: "your_own_key.key"},
			Kafka: &confluent.ConfigMap{
				"bootstrap.servers":            "some.broker1:9093,some.broker1:9093",
				"ssl.ca.location":              "kafka.ca",
				"ssl.certificate.location":     "kafka.crt",
				"ssl.key.location":             "kafka.key",
				"queue.buffering.max.messages": float64(1000000),
			},
			MetricCollector: noop.NewCollector(),
			Records:         map[string][]telemetry.Dispatcher{"FS": {"kafka"}},
		}

		loadedConfig, err := loadTestApplicationConfig(TestSmallConfig)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig.MetricCollector = loadedConfig.MetricCollector
		expectedConfig.AckChan = loadedConfig.AckChan
		Expect(loadedConfig).To(Equal(expectedConfig))
	})

	It("returns an error if config is not appropriate", func() {
		_, err := loadTestApplicationConfig(BadTopicConfig)
		Expect(err).To(MatchError("invalid character '}' looking for beginning of object key string"))
	})
})

func loadTestApplicationConfig(configStr string) (*Config, error) {
	appConfig, err := os.CreateTemp(os.TempDir(), "config")
	Expect(err).NotTo(HaveOccurred())

	_, err = appConfig.Write([]byte(configStr))
	Expect(err).NotTo(HaveOccurred())
	Expect(appConfig.Close()).To(BeNil())

	return loadApplicationConfig(appConfig.Name())
}
