package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Timer for Prometheus
type Timer struct {
	timer *prometheus.SummaryVec
}

// Observe records a new timing
func (c *Timer) Observe(n int64, labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.timer.With(l).Observe(float64(n))
}
