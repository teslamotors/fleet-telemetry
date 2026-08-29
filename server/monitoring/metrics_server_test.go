package monitoring

import (
	"fmt"
	"net"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

var _ = Describe("Metrics server", func() {
	Describe("listenAddress", func() {
		It("defaults to localhost when no host is configured", func() {
			Expect(listenAddress("", 9273)).To(Equal("127.0.0.1:9273"))
		})

		It("uses the configured host", func() {
			Expect(listenAddress("0.0.0.0", 9273)).To(Equal("0.0.0.0:9273"))
			Expect(listenAddress("10.48.1.5", 4269)).To(Equal("10.48.1.5:4269"))
		})
	})

	Describe("StartServerMetrics", func() {
		It("serves prometheus metrics on the configured address", func() {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())
			port := listener.Addr().(*net.TCPAddr).Port
			Expect(listener.Close()).To(Succeed())

			logger, _ := logrus.NoOpLogger()
			conf := &config.Config{
				MetricCollector: noop.NewCollector(),
				Monitoring: &metrics.MonitoringConfig{
					PrometheusMetricsHost: "127.0.0.1",
					PrometheusMetricsPort: port,
				},
			}
			StartServerMetrics(conf, logger, streaming.NewSocketRegistry())

			Eventually(func() error {
				resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
				if err != nil {
					return err
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
				return nil
			}).Should(Succeed())
		})
	})
})
