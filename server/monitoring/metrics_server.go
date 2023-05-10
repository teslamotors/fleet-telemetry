package monitoring

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

// StartServerMetrics initilizes metrics server on http
func StartServerMetrics(config *config.Config, logger *logrus.Logger, registry *streaming.SocketRegistry) {
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
	go metrics.ReportServerUsage(config.MetricCollector, make(chan struct{}), appMetrics(startTime, registry))
}

func appMetrics(startTime int64, registry *streaming.SocketRegistry) func(stats metrics.MetricCollector) {
	return func(stats metrics.MetricCollector) {
		stats.Gauge("uptime_sec", time.Now().Unix()-startTime)
		stats.Gauge("num_connections", registry.NumConnectedSockets())
	}
}
