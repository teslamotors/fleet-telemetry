package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Counter for noop
type Counter struct {
}

// Add (noop)
func (c *Counter) Add(n int64, labels adapter.Labels) {
}

// Inc (noop)
func (c *Counter) Inc(labels adapter.Labels) {
}
