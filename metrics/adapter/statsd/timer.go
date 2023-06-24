package statsd

import (
	sd "github.com/smira/go-statsd"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Timer for Statsd
type Timer struct {
	client *sd.Client
	name   string
}

// Observe records a new timing
func (s *Timer) Observe(n int64, labels adapter.Labels) {
	tags := getTags(labels)
	s.client.Timing(s.name, n, tags...)
}
