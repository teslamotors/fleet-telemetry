package streaming_test

import (
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
)

func TestConfigs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite Tests")
}

func CreateTestConfig() *config.Config {
	conf := &config.Config{}
	conf.MetricCollector = TestStatter{Stats: make(map[string]int64), mutex: &sync.Mutex{}}
	return conf
}

type TestStatter struct {
	Stats map[string]int64
	mutex *sync.Mutex
}

func (t TestStatter) Increment(bucket string) {
}

func (t TestStatter) Count(bucket string, n interface{}) {
}

func (t TestStatter) Gauge(bucket string, n interface{}) {
}

func (t TestStatter) Timing(bucket string, value interface{}) {
}

func (t TestStatter) Histogram(bucket string, value interface{}) {
}

func (t TestStatter) Unique(bucket string, value string) {
}

func (t TestStatter) IncrementWithLabels(bucket string, labels map[string]string) {
	t.CountWithLabels(bucket, 1, labels)
}

func (t TestStatter) CountWithLabels(bucket string, value interface{}, labels map[string]string) {
	var n int64
	switch v := value.(type) {
	case int:
		n = int64(v)
	case int64:
		n = v
	default:
		panic("Wrong type for Count()")
	}
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Stats[bucket] += n
}

func (t TestStatter) GaugeWithLabels(bucket string, value interface{}, labels map[string]string) {
	t.CountWithLabels("gauge_"+bucket, value, labels)
}

func (t TestStatter) TimingWithLabels(bucket string, value interface{}, labels map[string]string) {
	t.CountWithLabels("timing_"+bucket, value, labels)
}

func (t TestStatter) HistogramWithLabels(bucket string, value interface{}, labels map[string]string) {
	t.CountWithLabels("timing_"+bucket, value, labels)
}

func (t TestStatter) Close() {}
