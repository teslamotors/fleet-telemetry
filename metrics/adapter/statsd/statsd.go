package statsd

import (
	"time"

	"github.com/sirupsen/logrus"
	sd "github.com/smira/go-statsd"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

// Collector for Statsd
type Collector struct {
	client *sd.Client
}

// NewCollector creates a metric collector which sends data to Statsd
func NewCollector(addr, prefix string, logger *logrus.Logger, flushPeriod time.Duration) *Collector {

	client := sd.NewClient(addr, sd.MetricPrefix(prefix), sd.FlushInterval(flushPeriod))

	logger.Infof("new_statsd_client address: %v, flushPeriod: %v", addr, flushPeriod)

	return &Collector{
		client,
	}
}

// RegisterTimer creates a new timer for Statsd
func (c *Collector) RegisterTimer(options adapter.CollectorOptions) adapter.Timer {
	return &Timer{
		name:   options.Name,
		client: c.client,
	}
}

// RegisterCounter creates a new counter for Statsd
func (c *Collector) RegisterCounter(options adapter.CollectorOptions) adapter.Counter {
	return &Counter{
		name:   options.Name,
		client: c.client,
	}
}

// RegisterGauge creates a new gauge for Statsd
func (c *Collector) RegisterGauge(options adapter.CollectorOptions) adapter.Gauge {
	return &Gauge{
		name:   options.Name,
		client: c.client,
	}
}

// Shutdown closes the current Statsd connection
func (c *Collector) Shutdown() {
	_ = c.client.Close()
}
