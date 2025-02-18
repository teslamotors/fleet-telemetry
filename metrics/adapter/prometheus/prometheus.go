package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Collector is a prometheus based implementation of the stats collector
type Collector struct {
	collectors []prometheus.Collector
	stopChan   chan struct{}
}

// NewCollector returns a Prometheus metrics collector
func NewCollector() *Collector {
	return &Collector{
		stopChan:   make(chan struct{}),
		collectors: []prometheus.Collector{},
	}
}

func (c *Collector) register(collector prometheus.Collector) {
	prometheus.MustRegister(collector)
	c.collectors = append(c.collectors, prometheus.Collector(collector))
}

// RegisterCounter registers a new counter with Prometheus
func (c *Collector) RegisterCounter(options adapter.CollectorOptions) adapter.Counter {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: options.Name,
			Help: options.Help,
		},
		options.Labels,
	)

	c.register(counter)

	if len(options.Labels) == 0 {
		counter.With(nil).Add(0)
	}

	return &Counter{
		counter,
	}
}

// RegisterGauge registers a new gauge with Prometheus
func (c *Collector) RegisterGauge(options adapter.CollectorOptions) adapter.Gauge {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: options.Name,
			Help: options.Help,
		},
		options.Labels,
	)

	c.register(gauge)

	return &Gauge{
		gauge,
	}
}

// RegisterTimer registers a new timer with Prometheus
func (c *Collector) RegisterTimer(options adapter.CollectorOptions) adapter.Timer {
	timer := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: options.Name,
			Help: options.Help,
		},
		options.Labels,
	)

	c.register(timer)

	return &Timer{
		timer,
	}
}

// Shutdown unregisters and safely shuts down
func (c *Collector) Shutdown() {
	close(c.stopChan)
	unregisterAll(c.collectors)
}

func unregisterAll(collectors []prometheus.Collector) {
	for _, c := range collectors {
		prometheus.Unregister(c)
	}
}
