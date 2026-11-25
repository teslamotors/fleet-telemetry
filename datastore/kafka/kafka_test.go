package kafka_test

import (
	"time"

	confluentkafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/kafka"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("KafkaProducer", func() {
	It("reports client-level errors from the producer events channel", func() {
		logger, hook := logrus.NoOpLogger()
		// Nothing listens on port 1, so librdkafka raises broker errors on Events().
		config := &confluentkafka.ConfigMap{
			"bootstrap.servers":    "127.0.0.1:1",
			"reconnect.backoff.ms": 100,
		}

		producer, err := kafka.NewProducer(config, "test", false, metrics.NewCollector(nil, logger), airbrake.NewAirbrakeHandler(nil), make(chan *telemetry.Record), nil, logger)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(producer.Close)

		Eventually(func() []string {
			var messages []string
			for _, entry := range hook.AllEntries() {
				messages = append(messages, entry.Message)
			}
			return messages
		}, 10*time.Second, 100*time.Millisecond).Should(ContainElement(ContainSubstring("kafka_err")))
	})
})
