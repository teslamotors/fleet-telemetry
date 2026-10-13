package streaming_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/prometheus"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func TestConfigs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite Tests")
}

// The streaming package registers its metrics once per process, with the
// collector of the first SocketManager created. Register them with a real
// prometheus collector before any spec runs so tests can observe emitted
// metrics through the default prometheus gatherer.
var _ = BeforeSuite(func() {
	conf := CreateTestConfig()
	conf.MetricCollector = prometheus.NewCollector()
	logger, _ := logrus.NoOpLogger()
	streaming.NewSocketManager(context.Background(), &telemetry.RequestIdentity{}, nil, conf, logger)
})

func CreateTestConfig() *config.Config {
	conf := &config.Config{}

	logger, _ := logrus.NoOpLogger()
	conf.MetricCollector = metrics.NewCollector(nil, logger)
	return conf
}
