package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Timer for noop
type Timer struct {
}

// Observe (noop)
func (c *Timer) Observe(n int64, labels adapter.Labels) {
}
