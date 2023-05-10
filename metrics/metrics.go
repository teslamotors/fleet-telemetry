package metrics

import (
	"fmt"
	"runtime"
	"time"
)

// MetricCollector interface to handle metric counters
type MetricCollector interface {
	Increment(bucket string)                 // increment a bucket
	Count(bucket string, n interface{})      // add a number to a bucket
	Gauge(bucket string, n interface{})      // gauge a bucket
	Timing(bucket string, value interface{}) // add a timing value
}

// StatsIncrement Deals with the statsd client to increment metric counters
func StatsIncrement(statsCollector MetricCollector, statKey string, value int64, labels map[string]string) {
	sendMetrics(statsCollector, statKey, value, labels, statsCollector.Count)
}

// StatsTiming Deals with the statsd client to log timings in ms
func StatsTiming(statsCollector MetricCollector, statKey string, value int64, labels map[string]string) {
	sendMetrics(statsCollector, statKey, value, labels, statsCollector.Timing)
}

// ReportServerUsage collects server usage data and reports to stats lib
func ReportServerUsage(stats MetricCollector, stopChan chan struct{}, customStats func(MetricCollector)) {
	interval := time.Minute
	t := time.NewTicker(interval)
	var lastPauseNs uint64
	var lastNumGc uint32
	nsInMs := uint64(time.Millisecond)
	lastSampleTime := time.Now()

	for {
		select {
		case <-t.C:
			now := time.Now()
			stats.Gauge("num_goroutine", runtime.NumGoroutine())

			// memory stats
			memStats := &runtime.MemStats{}
			runtime.ReadMemStats(memStats)
			stats.Gauge("memory_allocated", memStats.Alloc)
			stats.Gauge("memory_heap", memStats.HeapAlloc)
			stats.Gauge("memory_stack", memStats.StackInuse)

			// gc stats
			stats.Gauge("gc_total_pause", memStats.PauseTotalNs/nsInMs)

			if lastPauseNs > 0 {
				pauseSinceLastSample := memStats.PauseTotalNs - lastPauseNs
				stats.Gauge("gc_pause_per_second", pauseSinceLastSample/nsInMs/uint64(interval.Seconds()))
			}

			lastPauseNs = memStats.PauseTotalNs

			countGc := int(memStats.NumGC - lastNumGc)
			if lastNumGc > 0 {
				diff := float64(countGc)
				diffTime := now.Sub(lastSampleTime).Seconds()
				stats.Gauge("gc_per_second", diff/diffTime)
			}

			lastNumGc = memStats.NumGC
			lastSampleTime = now

			if customStats != nil {
				customStats(stats)
			}
		case <-stopChan:
			return
		}
	}
}

func sendMetrics(statsCollector MetricCollector, statKey string, value int64, labels map[string]string, sendMetricsFunc func(string, interface{})) {
	if statsCollector == nil {
		return
	}
	if value < 0 {
		value = 0
	}

	newBucket := statKey[:]
	for key, value := range labels {
		newBucket += fmt.Sprintf("_%s_%s", key, value)
	}

	sendMetricsFunc(newBucket, value)
}
