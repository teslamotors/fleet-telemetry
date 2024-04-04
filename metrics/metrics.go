package metrics

import (
	"os"
	"runtime"
	"sync"
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/prometheus"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/statsd"
)

// MonitoringConfig for profiler and prometheus
type MonitoringConfig struct {
	// PrometheusMetricsPort port to run prometheus on
	PrometheusMetricsPort int `json:"prometheus_metrics_port,omitempty"`

	// Statsd metrics if you are not using prometheus
	Statsd *StatsdConfig `json:"statsd,omitempty"`

	// ProfilerPort if non-zero enable http profiler on this port
	ProfilerPort int `json:"profiler_port,omitempty"`

	// ProfilingPath is the variable that enable deep profiling is set
	ProfilingPath string `json:"profiling_path,omitempty"`

	ProfilerFile *os.File
}

// StatsdConfig for metrics
type StatsdConfig struct {
	// HostPort host:port of the statsd server
	HostPort string `json:"host,omitempty"`

	// StatsdPrefix prefix for statsd metrics
	Prefix string `json:"prefix,omitempty"`

	// StatsFlushPeriod in ms
	FlushPeriod int `json:"flush_period,omitempty"`
}

// MetricCollector provides means to create new collectors
type MetricCollector interface {
	RegisterCounter(adapter.CollectorOptions) adapter.Counter
	RegisterGauge(adapter.CollectorOptions) adapter.Gauge
	RegisterTimer(adapter.CollectorOptions) adapter.Timer
	Shutdown()
}

// NewCollector creates a collector based on monitoring configuration
func NewCollector(monitoringConfig *MonitoringConfig, logger *logrus.Logger) MetricCollector {
	isPrometheus := monitoringConfig != nil && monitoringConfig.PrometheusMetricsPort > 0
	isStatsd := monitoringConfig != nil && monitoringConfig.Statsd != nil

	if isPrometheus {
		return prometheus.NewCollector()
	}

	if isStatsd {
		flushDuration := 100 * time.Millisecond
		if monitoringConfig.Statsd.FlushPeriod > 0 {
			flushDuration = time.Duration(monitoringConfig.Statsd.FlushPeriod) * time.Millisecond
		}
		return statsd.NewCollector(monitoringConfig.Statsd.HostPort, monitoringConfig.Statsd.Prefix, logger, flushDuration)
	}

	logger.ActivityLog("config_skipping_empty_metrics_provider", nil)
	return noop.NewCollector()
}

// Metrics stores metrics reported from this package
type Metrics struct {
	goRoutineCount   adapter.Gauge
	memoryAllocated  adapter.Gauge
	memoryHeap       adapter.Gauge
	memoryStack      adapter.Gauge
	gcTotalPause     adapter.Gauge
	gcPausePerSecond adapter.Gauge
	gcPerSecond      adapter.Gauge
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// ReportServerUsage collects server usage data and reports metrics
func ReportServerUsage(stats MetricCollector, customStats func()) {
	registerMetricsOnce(stats)

	interval := time.Minute
	t := time.NewTicker(interval)
	var lastPauseNs uint64
	var lastNumGc uint32
	var secondsSinceLastSample uint64
	nsInMs := uint64(time.Millisecond)
	lastSampleTime := time.Now()

	for range t.C {
		now := time.Now()
		metricsRegistry.goRoutineCount.Set(int64(runtime.NumGoroutine()), map[string]string{})

		// memory stats
		memStats := &runtime.MemStats{}
		runtime.ReadMemStats(memStats)
		metricsRegistry.memoryAllocated.Set(int64(memStats.Alloc), map[string]string{})
		metricsRegistry.memoryHeap.Set(int64(memStats.HeapAlloc), map[string]string{})
		metricsRegistry.memoryStack.Set(int64(memStats.StackInuse), map[string]string{})

		// gc stats
		metricsRegistry.gcTotalPause.Set(int64(memStats.PauseTotalNs/nsInMs), map[string]string{})

		secondsSinceLastSample = uint64(now.Sub(lastSampleTime).Seconds())

		if lastPauseNs > 0 {
			pauseSinceLastSample := memStats.PauseTotalNs - lastPauseNs
			metricsRegistry.gcPausePerSecond.Set(int64(pauseSinceLastSample/nsInMs/secondsSinceLastSample), map[string]string{})
		}

		lastPauseNs = memStats.PauseTotalNs

		countGc := int(memStats.NumGC - lastNumGc)
		if lastNumGc > 0 {
			diff := float64(countGc)
			metricsRegistry.gcPerSecond.Set(int64(diff/float64(secondsSinceLastSample)), map[string]string{})
		}

		lastNumGc = memStats.NumGC
		lastSampleTime = now

		if customStats != nil {
			customStats()
		}
	}
}

func registerMetricsOnce(metricsCollector MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metrics MetricCollector) {
	metricsRegistry.goRoutineCount = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "num_goroutine",
		Help:   "The number of active Go routines.",
		Labels: []string{},
	})

	metricsRegistry.memoryAllocated = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "memory_allocated_bytes",
		Help:   "The number of bytes Go has allocated.",
		Labels: []string{},
	})

	metricsRegistry.memoryHeap = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "memory_heap",
		Help:   "The number of bytes of heap space Go has allocated.",
		Labels: []string{},
	})

	metricsRegistry.memoryStack = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "memory_stack",
		Help:   "The number of bytes of stack space Go has allocated.",
		Labels: []string{},
	})

	metricsRegistry.gcTotalPause = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "gc_total_pause",
		Help:   "The number of ms paused in total for garbage collection.",
		Labels: []string{},
	})

	metricsRegistry.gcPausePerSecond = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "gc_pause_per_second",
		Help:   "The number of ms paused per second for garbage collection.",
		Labels: []string{},
	})

	metricsRegistry.gcPerSecond = metrics.RegisterGauge(adapter.CollectorOptions{
		Name:   "gc_per_second",
		Help:   "The number of times the garbage collector runs per second.",
		Labels: []string{},
	})
}
