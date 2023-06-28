package statsd_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/statsd"
)

var _ = Describe("Statsd Metric Adapter", Ordered, func() {
	var (
		metricCollector metrics.MetricCollector
	)

	BeforeAll(func() {
		logger, _ := test.NewNullLogger()

		// create logger that will fail to connect forever
		metricCollector = statsd.NewCollector("", "", logger, time.Second)
	})

	Context("counter", func() {
		It("adds", func() {
			counter := metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "adder_counter",
				Help:   "help text",
				Labels: []string{},
			})

			counter.Add(5, map[string]string{})
		})

		It("increments", func() {
			counter := metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "increment_counter",
				Help:   "help text",
				Labels: []string{},
			})

			counter.Inc(map[string]string{})
		})
	})

	Context("gauge", func() {
		It("adds", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "adder_gauge",
				Help:   "help text",
				Labels: []string{"key"},
			})

			gauge.Add(5, map[string]string{"key": "value"})
		})

		It("subtracts", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "subtractor_gauge",
				Help:   "help text",
				Labels: []string{"key"},
			})

			gauge.Sub(5, map[string]string{"key": "value"})
		})

		It("sets", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "set_gauge",
				Help:   "help text",
				Labels: []string{},
			})

			gauge.Set(5, map[string]string{})
		})

		It("increments", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "increment_gauge",
				Help:   "help text",
				Labels: []string{},
			})

			gauge.Inc(map[string]string{})
		})
	})

	Context("timer", func() {
		It("Observe", func() {
			timer := metricCollector.RegisterTimer(adapter.CollectorOptions{
				Name:   "timer_with_label",
				Help:   "help text",
				Labels: []string{"key"},
			})

			timer.Observe(5, map[string]string{"key": "value"})
		})
	})

	Context("Shutdown", func() {
		It("shuts down", func() {
			metricCollector.Shutdown()
		})
	})
})
