package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Timer for noop
type Timer struct {
}

// Observe (noop)
func (c *Timer) Observe(_ int64, _ adapter.Labels) {
}
