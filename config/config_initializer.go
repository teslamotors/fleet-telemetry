package config

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// LoadApplicationConfiguration loads the configuration from args and config files
func LoadApplicationConfiguration() (config *Config, logger *logrus.Logger, err error) {
	logger = logrus.New()

	configFilePath := loadConfigFlags()

	config, err = loadApplicationConfig(configFilePath)
	if err != nil {
		logger.Errorf("read_application_configuration_error %s", err)
		return nil, nil, err
	}

	config.configureLogger(logger)
	config.configureStatsCollector(logger)
	return config, logger, nil
}

func loadApplicationConfig(configFilePath string) (*Config, error) {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = json.NewDecoder(configFile).Decode(&config)

	config.AckChan = make(chan *telemetry.Record)
	return config, err
}

func loadConfigFlags() string {
	applicationConfig := ""
	flag.StringVar(&applicationConfig, "config", "config.json", "application configuration file")

	flag.Parse()
	return applicationConfig
}
