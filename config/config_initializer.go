package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	maxVinsToTrack = 20
)

// LoadApplicationConfiguration loads the configuration from args and config files
func LoadApplicationConfiguration() (config *Config, logger *logrus.Logger, err error) {

	logger, err = logrus.NewBasicLogrusLogger("fleet-telemetry")
	if err != nil {
		return nil, nil, err
	}
	log.SetOutput(logger)

	configFilePath := loadConfigFlags()

	config, err = loadApplicationConfig(configFilePath)
	if err != nil {
		return nil, nil, err
	}

	config.configureLogger(logger)
	config.configureMetricsCollector(logger)
	return config, logger, nil
}

func loadApplicationConfig(configFilePath string) (*Config, error) {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}

	config := &Config{
		LoggerConfig: &simple.Config{},
	}
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return nil, err
	}

	l, _ := test.NewNullLogger()
	logger, err := logrus.NewLogrusLogger("null_logger", map[string]interface{}{}, l.WithField("context", "metrics"))
	if err != nil {
		return nil, err
	}

	if err := validateConfig(config); err != nil {
		return nil, err
	}
	config.MetricCollector = metrics.NewCollector(config.Monitoring, logger)
	config.AckChan = make(chan *telemetry.Record)
	return config, err
}

func validateConfig(config *Config) error {
	if len(config.VinsToTrack()) > maxVinsToTrack {
		return fmt.Errorf("set the value of `vins_signal_tracking_enabled` less than %d unique vins", maxVinsToTrack)
	}
	return nil
}

func loadConfigFlags() string {
	applicationConfig := ""
	flag.StringVar(&applicationConfig, "config", "config.json", "application configuration file")

	flag.Parse()
	return applicationConfig
}
