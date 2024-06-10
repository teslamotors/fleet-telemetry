package connector

import (
	"fmt"

	"github.com/teslamotors/fleet-telemetry/connector/adapter/file"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/grpc"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/http"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/redis"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
)

type Connector interface {
	VinAllowed(vin string) (bool, error)
	Close() error
}

type Config struct {
	File  *file.Config  `json:"file,omitempty"`
	Grpc  *grpc.Config  `json:"grpc,omitempty"`
	Http  *http.Config  `json:"http,omitempty"`
	Redis *redis.Config `json:"redis,omitempty"`
}

type Connectors struct {
	File  *file.Connector
	Grpc  *grpc.Connector
	Http  *http.Connector
	Redis *redis.Connector
}

type ConnectorProvider struct {
	VinAllowedConnector Connector
	Connectors          Connectors

	config Config
	logger *logrus.Logger
}

func NewConnectorProvider(config Config, metricsCollector metrics.MetricCollector, logger *logrus.Logger) *ConnectorProvider {
	provider := &ConnectorProvider{
		logger: logger,
		config: config,
	}

	err := provider.configure(metricsCollector, logger)
	if err != nil {
		logger.ErrorLog("connector_provider_configure_sources_error", err, nil)
		return nil
	}

	return provider
}

func (c *ConnectorProvider) VinAllowed(vin string) (bool, error) {
	if c.VinAllowedConnector == nil {
		return true, nil
	}

	return c.VinAllowedConnector.VinAllowed(vin)
}

func (c *ConnectorProvider) Close() {
	if c.Connectors.File != nil {
		c.Connectors.File.Close()
	}
	if c.Connectors.Http != nil {
		c.Connectors.Http.Close()
	}
	if c.Connectors.Redis != nil {
		c.Connectors.Redis.Close()
	}
	if c.Connectors.Grpc != nil {
		c.Connectors.Grpc.Close()
	}
}

func (c *ConnectorProvider) configure(metricsCollector metrics.MetricCollector, logger *logrus.Logger) error {
	if c.config.File != nil && len(c.config.File.Capabilities) > 0 {
		connector, err := file.NewConnector(*c.config.File, metricsCollector, logger)
		if err == nil {
			c.Connectors.File = connector
			c.configureConnectorCapabilities(connector, c.config.File.Capabilities)
		} else {
			logger.ErrorLog("connector_provider_configure_sources_file_error", err, nil)
		}
	}

	if c.config.Grpc != nil && len(c.config.Grpc.Capabilities) > 0 {
		connector, err := grpc.NewConnector(*c.config.Grpc, metricsCollector, logger)
		if err == nil {
			c.Connectors.Grpc = connector
			c.configureConnectorCapabilities(connector, c.config.Grpc.Capabilities)

		} else {
			logger.ErrorLog("connector_provider_configure_sources_grpc_error", err, nil)
		}
	}

	if c.config.Http != nil && len(c.config.Http.Capabilities) > 0 {
		connector, err := http.NewConnector(c.config.Http, metricsCollector, logger)
		if err == nil {
			c.Connectors.Http = connector
			c.configureConnectorCapabilities(connector, c.config.Http.Capabilities)
		} else {
			logger.ErrorLog("connector_provider_configure_sources_http_error", err, nil)
		}
	}

	if c.config.Redis != nil && len(c.config.Redis.Capabilities) > 0 {
		connector, err := redis.NewConnector(*c.config.Redis, metricsCollector, logger)
		if err == nil {
			c.Connectors.Redis = connector
			c.configureConnectorCapabilities(connector, c.config.Redis.Capabilities)
		} else {
			logger.ErrorLog("connector_provider_configure_sources_redis_error", err, nil)
		}
	}

	return nil
}

func (c *ConnectorProvider) configureConnectorCapabilities(connector Connector, capabilities []string) {
	for _, capability := range capabilities {
		c.setCapabilityByName(capability, connector)
	}
}

func (c *ConnectorProvider) setCapabilityByName(name string, connector Connector) error {
	switch name {
	case "vin_allowed":
		if c.VinAllowedConnector != nil {
			c.logger.Log(logrus.WARN, "vin_allowed capability specified multiple times", logrus.LogInfo{})
		}
		c.VinAllowedConnector = connector
	default:
		c.logger.Log(logrus.WARN, fmt.Sprintf("unknown capability %s", name), logrus.LogInfo{})
	}
	return nil
}
