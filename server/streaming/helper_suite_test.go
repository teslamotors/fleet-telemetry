package streaming_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
)

func TestConfigs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite Tests")
}

func CreateTestConfig() *config.Config {
	conf := &config.Config{}

	logger, _ := logrus.NoOpLogger()
	conf.MetricCollector = metrics.NewCollector(nil, logger)
	return conf
}
