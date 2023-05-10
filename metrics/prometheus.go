package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var labels = []string{"bucket"}
var summaryObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

// PrometheusCollector is a prometheus based implementation of the stats collector
type PrometheusCollector struct {
	logger      *logrus.Logger
	application string
	newCounters *prometheus.CounterVec
	newGauges   *prometheus.GaugeVec
	newTimings  *prometheus.SummaryVec
	stopChan    chan struct{}

	countersSM sync.Map
	guagesSM   sync.Map
	timingsSM  sync.Map
}

// NewPrometheusCollector returns a prometheus collector
func NewPrometheusCollector(logger *logrus.Logger, application string) *PrometheusCollector {
	newCounters := newCounter("counter_totals", application)
	prometheus.MustRegister(newCounters)

	newGauges := newGauge("gauge_values", application)
	prometheus.MustRegister(newGauges)

	newTimings := newSummary("timing_milliseconds", application)
	prometheus.MustRegister(newTimings)

	return &PrometheusCollector{
		logger:      logger,
		application: application,
		newCounters: newCounters,
		newGauges:   newGauges,
		newTimings:  newTimings,
		stopChan:    make(chan struct{}),
	}
}

// Increment adds 1 to a bucket value
func (s *PrometheusCollector) Increment(bucket string) {
	s.newCounters.WithLabelValues(bucket).Inc()
}

// Count adds n to a bucket value
func (s *PrometheusCollector) Count(bucket string, n interface{}) {
	s.newCounters.WithLabelValues(bucket).Add(s.cast(n))
}

// Gauge sets a gauge value on a bucket
func (s *PrometheusCollector) Gauge(bucket string, n interface{}) {
	s.newGauges.WithLabelValues(bucket).Set(s.cast(n))
}

// Timing add a timing value to a bucket
func (s *PrometheusCollector) Timing(bucket string, n interface{}) {
	s.newTimings.WithLabelValues(bucket).Observe(s.cast(n))
}

// Close closes the collector
func (s *PrometheusCollector) Close() {
	close(s.stopChan)
	prometheus.Unregister(s.newCounters)
	prometheus.Unregister(s.newGauges)
	prometheus.Unregister(s.newTimings)

	s.countersSM.Range(func(collector, value interface{}) bool {
		prometheus.Unregister(value.(*prometheus.CounterVec))
		return true
	})

	s.guagesSM.Range(func(collector, value interface{}) bool {
		prometheus.Unregister(value.(*prometheus.GaugeVec))
		return true
	})

	s.timingsSM.Range(func(collector, value interface{}) bool {
		prometheus.Unregister(value.(*prometheus.SummaryVec))
		return true
	})
}

func (s *PrometheusCollector) cast(n interface{}) float64 {
	switch n := n.(type) {
	case int:
		return float64(n)
	case uint:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case int32:
		return float64(n)
	case uint32:
		return float64(n)
	case int16:
		return float64(n)
	case uint16:
		return float64(n)
	case int8:
		return float64(n)
	case uint8:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	default:
		s.logger.Errorf("unable_to_cast type: %v", fmt.Sprintf("%v, %T", n, n))
	}

	return 0
}

func newCounter(name string, application string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: fmt.Sprintf("%s counter values", application),
		},
		labels,
	)
}

func newGauge(name string, application string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: fmt.Sprintf("%s counter values", application),
		},
		labels,
	)
}

func newSummary(name string, application string) *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       name,
			Help:       fmt.Sprintf("%s timing latency distributions.", application),
			Objectives: summaryObjectives,
		},
		labels,
	)
}
