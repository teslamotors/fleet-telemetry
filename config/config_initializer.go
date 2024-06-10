package config

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/sirupsen/logrus/hooks/test"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
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
		logger.ErrorLog("read_application_configuration_error", err, nil)
		return nil, nil, err
	}

	config.configureLogger(logger)
	config.configureMetricsCollector(logger)
	config.configureDataConnector(logger)
	return config, logger, nil
}

func loadApplicationConfig(configFilePath string) (*Config, error) {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return nil, err
	}

	log, _ := test.NewNullLogger()
	logger, err := logrus.NewLogrusLogger("null_logger", map[string]interface{}{}, log.WithField("context", "metrics"))
	if err != nil {
		return nil, err
	}
	config.MetricCollector = metrics.NewCollector(config.Monitoring, logger)
	config.AckChan = make(chan *telemetry.Record)
	return config, err
}

func loadConfigFlags() string {
	applicationConfig := ""
	flag.StringVar(&applicationConfig, "config", "config.json", "application configuration file")

	flag.Parse()
	return applicationConfig
}
