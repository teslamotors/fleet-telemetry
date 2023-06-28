package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

// Metrics stores metrics reported from this package
type Metrics struct {
	uptimeSeconds     adapter.Gauge
	numberConnections adapter.Gauge
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// StartServerMetrics initilizes metrics server on http
func StartServerMetrics(config *config.Config, logger *logrus.Logger, registry *streaming.SocketRegistry) {
	registerMetricsOnce(config.MetricCollector)

	if config.Monitoring.PrometheusMetricsPort > 0 {
		promMux := http.NewServeMux()
		promMux.Handle("/metrics", promhttp.Handler())
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Monitoring.PrometheusMetricsPort), promMux); err != nil {
				logger.Errorf("metrics %v", err)
			}
		}()
	}

	if config.Monitoring.ProfilerPort > 0 {
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Monitoring.ProfilerPort), nil); err != nil {
				logger.Errorf("profiler_listen_error: %v", err)
			}
		}()
	}

	startTime := time.Now().Unix()
	go metrics.ReportServerUsage(config.MetricCollector, appMetrics(startTime, registry))
}

func appMetrics(startTime int64, registry *streaming.SocketRegistry) func() {
	return func() {
		metricsRegistry.uptimeSeconds.Set(time.Now().Unix()-startTime, map[string]string{})
		metricsRegistry.numberConnections.Set(int64(registry.NumConnectedSockets()), map[string]string{})
	}
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.uptimeSeconds = metricsCollector.RegisterGauge(adapter.CollectorOptions{
		Name:   "uptime_sec",
		Help:   "The number of seconds the application has been running.",
		Labels: []string{},
	})

	metricsRegistry.numberConnections = metricsCollector.RegisterGauge(adapter.CollectorOptions{
		Name:   "num_connections",
		Help:   "The number of active websocket connections.",
		Labels: []string{},
	})
}
