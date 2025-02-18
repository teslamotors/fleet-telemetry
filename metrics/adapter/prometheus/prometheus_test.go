package prometheus_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/prometheus"
)

func dclose(closer io.Closer) {
	err := closer.Close()
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("Prometheus Metric Adapter", Ordered, func() {
	var (
		metricCollector metrics.MetricCollector
		port            int
		server          *httptest.Server
		httpClient      http.Client
		getMetrics      func() string
	)

	BeforeAll(func() {
		metricCollector = prometheus.NewCollector()

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		server = httptest.NewServer(mux)
		port = server.Listener.Addr().(*net.TCPAddr).Port
		httpClient = http.Client{Timeout: time.Second}

		getMetrics = func() string {
			resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
			Expect(err).NotTo(HaveOccurred())

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			defer dclose(resp.Body)

			return string(body)
		}
	})

	AfterAll(func() {
		server.Close()
	})

	Context("counter", func() {
		It("reports with no labels", func() {
			counter := metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "new_bucket",
				Help:   "help text",
				Labels: []string{},
			})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("new_bucket 0"))

			counter.Add(5, map[string]string{})
			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("new_bucket 5"))
		})

		It("reports with labels", func() {
			metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "new_bucket_with_labels",
				Help:   "help text",
				Labels: []string{"key"},
			}).Add(5, map[string]string{"key": "value"})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("new_bucket_with_labels{key=\"value\"} 5"))
		})

		It("adds", func() {
			counter := metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "adder_counter",
				Help:   "help text",
				Labels: []string{},
			})

			counter.Add(5, map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("adder_counter 5"))

			counter.Add(10, map[string]string{})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("adder_counter 15"))
		})

		It("increments", func() {
			counter := metricCollector.RegisterCounter(adapter.CollectorOptions{
				Name:   "increment_counter",
				Help:   "help text",
				Labels: []string{},
			})

			counter.Inc(map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("increment_counter 1"))

			counter.Inc(map[string]string{})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("increment_counter 2"))
		})
	})

	Context("gauge", func() {
		It("handles no labels", func() {
			metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "new_gauge_no_labels",
				Help:   "help text",
				Labels: []string{},
			}).Add(5, map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("new_gauge_no_labels 5"))
		})

		It("handles labels", func() {
			metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "new_gauge_with_labels",
				Help:   "help text",
				Labels: []string{"key"},
			}).Add(5, map[string]string{"key": "value"})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("new_gauge_with_labels{key=\"value\"} 5"))
		})

		It("adds", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "adder_gauge",
				Help:   "help text",
				Labels: []string{"key"},
			})

			gauge.Add(5, map[string]string{"key": "value"})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("adder_gauge{key=\"value\"} 5"))

			gauge.Add(10, map[string]string{"key": "value"})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("adder_gauge{key=\"value\"} 15"))
		})

		It("subtracts", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "subtractor_gauge",
				Help:   "help text",
				Labels: []string{"key"},
			})

			gauge.Sub(5, map[string]string{"key": "value"})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("subtractor_gauge{key=\"value\"} -5"))

			gauge.Sub(10, map[string]string{"key": "value"})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("subtractor_gauge{key=\"value\"} -15"))
		})

		It("sets", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "set_gauge",
				Help:   "help text",
				Labels: []string{},
			})

			gauge.Set(5, map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("set_gauge 5"))

			gauge.Set(6, map[string]string{})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("set_gauge 6"))
		})
		It("increments", func() {
			gauge := metricCollector.RegisterGauge(adapter.CollectorOptions{
				Name:   "increment_gauge",
				Help:   "help text",
				Labels: []string{},
			})

			gauge.Inc(map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("increment_gauge 1"))

			gauge.Inc(map[string]string{})

			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring("increment_gauge 2"))
		})
	})

	Context("timer", func() {
		It("reports with no labels", func() {
			metricCollector.RegisterTimer(adapter.CollectorOptions{
				Name:   "timer_no_label",
				Help:   "help text",
				Labels: []string{},
			}).Observe(5, map[string]string{})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("timer_no_label_count 1"))
			Expect(metrics).To(ContainSubstring("timer_no_label_sum 5"))
		})

		It("reports with labels", func() {
			metricCollector.RegisterTimer(adapter.CollectorOptions{
				Name:   "timer_with_label",
				Help:   "help text",
				Labels: []string{"key"},
			}).Observe(5, map[string]string{"key": "value"})

			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring("timer_with_label_count{key=\"value\"} 1"))
			Expect(metrics).To(ContainSubstring("timer_with_label_sum{key=\"value\"} 5"))
		})
	})

	Context("Shutdown", func() {
		It("shuts down", func() {
			metricCollector.Shutdown()
		})
	})
})
