package monitoring_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/monitoring"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

var _ = Describe("Monitoring listen address (#458)", func() {
	It("defaults empty host to 0.0.0.0 so pod-IP probes can connect", func() {
		Expect(monitoring.ListenAddrForTest("", 9273)).To(Equal("0.0.0.0:9273"))
	})

	It("honors an explicit localhost bind", func() {
		Expect(monitoring.ListenAddrForTest("127.0.0.1", 9273)).To(Equal("127.0.0.1:9273"))
	})

	It("serves /metrics when host is left empty so probes can reach the process", func() {
		port := freeTCPPort()
		logger, _ := logrus.NoOpLogger()
		conf := &config.Config{
			MetricCollector: noop.NewCollector(),
			Monitoring: &metrics.MonitoringConfig{
				PrometheusMetricsPort: port,
				// PrometheusMetricsHost intentionally empty → 0.0.0.0
			},
		}

		monitoring.StartServerMetrics(conf, logger, streaming.NewSocketRegistry())

		url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
		Eventually(func() int {
			resp, err := http.Get(url)
			if err != nil {
				return 0
			}
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.StatusCode
		}, 2*time.Second, 50*time.Millisecond).Should(Equal(http.StatusOK))
	})
})

func freeTCPPort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())
	port := ln.Addr().(*net.TCPAddr).Port
	Expect(ln.Close()).To(Succeed())
	return port
}
