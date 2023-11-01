package run

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/datastore/zmq"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Test app shutdown", func() {
	var (
		conf   *config.Config
		logger *logrus.Logger
	)

	BeforeEach(func() {
		conf = &config.Config{
			Host:       "127.0.0.1",
			Port:       443,
			StatusPort: 8080,
			Namespace:  "tesla_telemetry",
			TLS:        &config.TLS{CAFile: "tesla.ca", ServerCert: "your_own_cert.crt", ServerKey: "your_own_key.key"},
			ZMQ: &zmq.Config{
				Addr: "tcp://127.0.0.1:5284",
			},
			Monitoring:    &metrics.MonitoringConfig{PrometheusMetricsPort: 9090, ProfilerPort: 4269, ProfilingPath: "/tmp/fleet-telemetry/profile/"},
			LogLevel:      "info",
			JSONLogEnable: true,
			Records:       map[string][]telemetry.Dispatcher{"FS": {"kafka"}},
		}
		logger = logrus.New()
		conf.MetricCollector = metrics.NewCollector(conf.Monitoring, logger)
		conf.AckChan = make(chan *telemetry.Record)
	})

	It("fails when app doesn't shutdown before timeout", func() {
		// Run the server for 1s, then cancel it.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		beganRunning := time.Now()
		RunServer(ctx, conf, logger)
		Expect(time.Since(beganRunning) > ShutdownTimeout).To(BeFalse())
	})

	It("fails when app doesn't close its producer", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cancel()
		RunServer(ctx, conf, logger)
		_, err := net.Dial("tcp", "127.0.0.1:5284")
		Expect(err).ToNot(BeNil())
	})
})
