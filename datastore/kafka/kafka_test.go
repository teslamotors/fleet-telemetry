package kafka_test

import (
	"bytes"
	"runtime/pprof"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kafka "github.com/teslamotors/fleet-telemetry/datastore/kafka"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// goroutineStacks returns the stacks of all live goroutines as text.
func goroutineStacks() string {
	var buf bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&buf, 1)
	return buf.String()
}

var _ = Describe("Producer.Close", func() {
	It("stops the goroutines started by NewProducer", func() {
		logger, _ := logrus.NoOpLogger()
		collector := metrics.NewCollector(nil, logger)
		// No broker is needed: the producer is created without connecting.
		cfg := &confluent.ConfigMap{"bootstrap.servers": "127.0.0.1:1"}
		p, err := kafka.NewProducer(cfg, "testns", false, collector, airbrake.NewAirbrakeHandler(nil), make(chan *telemetry.Record, 1), map[string]interface{}{}, logger)
		Expect(err).NotTo(HaveOccurred())

		Eventually(goroutineStacks, 2*time.Second, 10*time.Millisecond).Should(ContainSubstring("handleProducerEvents"))
		Eventually(goroutineStacks, 2*time.Second, 10*time.Millisecond).Should(ContainSubstring("reportProducerMetrics"))

		Expect(p.Close()).To(Succeed())
		Eventually(goroutineStacks, 2*time.Second, 10*time.Millisecond).ShouldNot(ContainSubstring("handleProducerEvents"))
		Eventually(goroutineStacks, 2*time.Second, 10*time.Millisecond).ShouldNot(ContainSubstring("reportProducerMetrics"))

		// Closing twice must not panic.
		Expect(p.Close()).To(Succeed())
	})
})
