package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Counter for Prometheus
type Counter struct {
	counter *prometheus.CounterVec
}

// Add to the counter
func (c *Counter) Add(n int64, labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.counter.With(l).Add(float64(n))
}

// Inc the counter
func (c *Counter) Inc(labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.counter.With(l).Inc()
}
