package noop

import (
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Collector for noop
type Collector struct {
}

// RegisterCounter returns a noop Counter
func (p *Collector) RegisterCounter(_ adapter.CollectorOptions) adapter.Counter {
	return &Counter{}
}

// RegisterGauge returns a noop Gauge
func (p *Collector) RegisterGauge(_ adapter.CollectorOptions) adapter.Gauge {
	return &Gauge{}
}

// RegisterTimer returns a noop Timer
func (p *Collector) RegisterTimer(_ adapter.CollectorOptions) adapter.Timer {
	return &Timer{}
}

// Shutdown (noop)
func (p *Collector) Shutdown() {
}

// NewCollector creates a new noop Collector
func NewCollector() *Collector {
	return &Collector{}
}
