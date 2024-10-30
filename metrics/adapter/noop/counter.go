package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Counter for noop
type Counter struct {
}

// Add (noop)
func (c *Counter) Add(_ int64, _ adapter.Labels) {
}

// Inc (noop)
func (c *Counter) Inc(_ adapter.Labels) {
}
