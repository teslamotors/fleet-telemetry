package metrics_test

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
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/metrics"
)

func dclose(closer io.Closer) {
	err := closer.Close()
	Expect(err).To(BeNil())
}

var _ = Describe("Prometheus", Ordered, func() {
	var (
		client     metrics.MetricCollector
		pClient    *metrics.PrometheusCollector
		logger     *logrus.Logger
		port       int
		server     *httptest.Server
		httpClient http.Client
		getMetrics func() string
	)

	BeforeAll(func() {
		logger, _ = test.NewNullLogger()
		pClient = metrics.NewPrometheusCollector(logger, "app")
		client = pClient

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		server = httptest.NewServer(mux)
		port = server.Listener.Addr().(*net.TCPAddr).Port
		httpClient = http.Client{Timeout: time.Second}

		getMetrics = func() string {
			resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
			Expect(err).ToNot(HaveOccurred())

			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			defer dclose(resp.Body)

			return string(body)
		}
	})

	AfterAll(func() {
		server.Close()
		pClient.Close()
	})

	Context("Prometheus", func() {
		It("increments a bucket", func() {
			client.Increment("incr")
			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring(`counter_totals{bucket="incr"} 1`))

			client.Increment("incr")
			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring(`counter_totals{bucket="incr"} 2`))
		})

		It("counts a bucket", func() {
			client.Count("count", 5)
			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring(`counter_totals{bucket="count"} 5`))

			client.Count("count", 5.5)
			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring(`counter_totals{bucket="count"} 10.5`))
		})

		It("gauges a bucket", func() {
			client.Gauge("gauge", 23)
			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring(`gauge_values{bucket="gauge"} 23`))

			client.Gauge("gauge", 23.5)
			metrics = getMetrics()
			Expect(metrics).To(ContainSubstring(`gauge_values{bucket="gauge"} 23.5`))
		})

		It("times a bucket", func() {
			client.Timing("timer", 1.5)
			metrics := getMetrics()
			Expect(metrics).To(ContainSubstring(`timing_milliseconds_sum{bucket="timer"} 1.5`))
		})
	})
})
