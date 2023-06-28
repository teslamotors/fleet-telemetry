package statsd

import (
	sd "github.com/smira/go-statsd"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Counter for noop
type Counter struct {
	client *sd.Client
	name   string
}

// Add to the Counter
func (s *Counter) Add(n int64, labels adapter.Labels) {
	tags := getTags(labels)
	s.client.Incr(s.name, n, tags...)
}

// Inc the Counter
func (s *Counter) Inc(labels adapter.Labels) {
	tags := getTags(labels)
	s.client.Incr(s.name, 1, tags...)
}
