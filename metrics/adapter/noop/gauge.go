package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Gauge for noop
type Gauge struct {
}

// Add (noop)
func (c *Gauge) Add(_ int64, _ adapter.Labels) {
}

// Sub (noop)
func (c *Gauge) Sub(_ int64, _ adapter.Labels) {
}

// Inc (noop)
func (c *Gauge) Inc(_ adapter.Labels) {
}

// Set (noop)
func (c *Gauge) Set(_ int64, _ adapter.Labels) {
}
