package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
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

// monitoringListenAddr builds host:port for metrics/profiler servers.
// An empty host defaults to 0.0.0.0 so Kubernetes probes that hit the pod IP
// succeed (see #458). Set prometheus_metrics_host / profiler_host to 127.0.0.1
// to keep the server localhost-only.
func monitoringListenAddr(host string, port int) string {
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// ListenAddrForTest exports monitoringListenAddr for unit tests.
func ListenAddrForTest(host string, port int) string {
	return monitoringListenAddr(host, port)
}

// StartServerMetrics initializes the metrics server on http
func StartServerMetrics(config *config.Config, logger *logrus.Logger, registry *streaming.SocketRegistry) {
	registerMetricsOnce(config.MetricCollector)

	if config.Monitoring.PrometheusMetricsPort > 0 {
		promMux := http.NewServeMux()
		promMux.Handle("/metrics", promhttp.Handler())
		go func() {
			addr := monitoringListenAddr(config.Monitoring.PrometheusMetricsHost, config.Monitoring.PrometheusMetricsPort)
			logger.ActivityLog("metrics_server_configured", logrus.LogInfo{"addr": addr})
			if err := http.ListenAndServe(addr, promMux); err != nil {
				logger.ErrorLog("metrics_server_err", err, nil)
			}
		}()
	}

	if config.Monitoring.ProfilerPort > 0 {
		go func() {
			profilerMux := http.NewServeMux()
			profilerMux.HandleFunc("/debug/pprof/", pprof.Index)
			profilerMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			profilerMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			profilerMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			profilerMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

			StartProfilerServer(config, profilerMux, logger)

			addr := monitoringListenAddr(config.Monitoring.ProfilerHost, config.Monitoring.ProfilerPort)
			logger.ActivityLog("profiler_server_configured", logrus.LogInfo{"addr": addr})
			if err := http.ListenAndServe(addr, profilerMux); err != nil {
				logger.ErrorLog("profiler_listen_error", err, nil)
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
