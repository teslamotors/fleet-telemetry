package metrics

import (
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/alexcesaro/statsd.v2"
)

// NewStatsdCollectorWithFlushAndSampleRate returns a statsd collector with sample/flush options
func NewStatsdCollectorWithFlushAndSampleRate(addr, prefix string, logger *logrus.Logger,
	sampleRate float32, flushPeriod time.Duration) (*statsd.Client, error) {

	client, err := statsd.New(statsd.Address(addr), statsd.Prefix(prefix), statsd.SampleRate(sampleRate), statsd.FlushPeriod(flushPeriod))
	if err != nil {
		return nil, err
	}

	logger.Infof("new_statsd_client address: %v, sampleRate:%v, flushPeriod: %v", addr, sampleRate, flushPeriod)
	return client, nil
}
