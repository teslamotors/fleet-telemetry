package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

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

	log, _ := test.NewNullLogger()
	logger, err := logrus.NewLogrusLogger("null_logger", map[string]interface{}{}, log.WithField("context", "metrics"))
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
	if err := validateRateLimit(config.RateLimit); err != nil {
		return err
	}
	return nil
}

// validateRateLimit maps message_interval_time onto MessageIntervalTimeSecond and
// rejects enabled limiters that cannot trip (zero limit or zero interval). See #545.
func validateRateLimit(rateLimit *RateLimit) error {
	if rateLimit == nil {
		return nil
	}
	if rateLimit.MessageInterval > 0 && rateLimit.MessageIntervalTimeSecond == 0 {
		rateLimit.MessageIntervalTimeSecond = time.Duration(rateLimit.MessageInterval) * time.Second
	}
	if !rateLimit.Enabled {
		return nil
	}
	if rateLimit.MessageLimit <= 0 {
		return fmt.Errorf("rate_limit: message_limit must be greater than 0")
	}
	if rateLimit.MessageIntervalTimeSecond <= 0 {
		return fmt.Errorf("rate_limit: message_interval_time must be greater than 0")
	}
	return nil
}

func loadConfigFlags() string {
	applicationConfig := ""
	flag.StringVar(&applicationConfig, "config", "config.json", "application configuration file")

	flag.Parse()
	return applicationConfig
}
