package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Gauge for Prometheus
type Gauge struct {
	gauge *prometheus.GaugeVec
}

// Add to the Gauge
func (c *Gauge) Add(n int64, labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.gauge.With(l).Add(float64(n))
}

// Sub from the Gauge
func (c *Gauge) Sub(n int64, labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.gauge.With(l).Sub(float64(n))
}

// Inc the Gauge
func (c *Gauge) Inc(labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.gauge.With(l).Inc()
}

// Set the Gauge
func (c *Gauge) Set(n int64, labels adapter.Labels) {
	l := prometheus.Labels(labels)
	c.gauge.With(l).Set(float64(n))
}
