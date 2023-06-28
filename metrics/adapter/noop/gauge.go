package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Gauge for noop
type Gauge struct {
}

// Add (noop)
func (c *Gauge) Add(n int64, labels adapter.Labels) {
}

// Sub (noop)
func (c *Gauge) Sub(n int64, labels adapter.Labels) {
}

// Inc (noop)
func (c *Gauge) Inc(labels adapter.Labels) {
}

// Set (noop)
func (c *Gauge) Set(n int64, labels adapter.Labels) {
}
