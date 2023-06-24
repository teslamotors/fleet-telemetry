package statsd

import (
	sd "github.com/smira/go-statsd"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Gauge for Statsd
type Gauge struct {
	client *sd.Client
	name   string
}

// Add to the Gauge
func (s *Gauge) Add(n int64, labels adapter.Labels) {
	tags := getTags(labels)
	s.client.GaugeDelta(s.name, n, tags...)
}

// Sub from the Gauge
func (s *Gauge) Sub(n int64, labels adapter.Labels) {
	tags := getTags(labels)
	s.client.GaugeDelta(s.name, -n, tags...)
}

// Inc the Gauge
func (s *Gauge) Inc(labels adapter.Labels) {
	tags := getTags(labels)
	s.client.GaugeDelta(s.name, 1, tags...)
}

// Set the Gauge
func (s *Gauge) Set(n int64, labels adapter.Labels) {
	tags := getTags(labels)
	s.client.Gauge(s.name, n, tags...)
}
