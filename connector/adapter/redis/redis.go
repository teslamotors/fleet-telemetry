package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/teslamotors/fleet-telemetry/connector/util"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

var (
	serverMetricsRegistry ServerMetrics
	serverMetricsOnce     sync.Once
)

type ServerMetrics struct {
	requestCount      adapter.Counter
	requestErrorCount adapter.Counter
}

type VinAllowed struct {
	AllowOnFailure bool   `json:"allow_on_failure,omitempty"`
	Prefix         string `json:"prefix"`
}

type Config struct {
	Client       redis.Options `json:"client,omitempty"`
	Capabilities []string      `json:"capabilities"`

	VinAllowed VinAllowed `json:"vin_allowed"`
}

type Connector struct {
	Client *redis.Client
	Config Config

	logger           *logrus.Logger
	metricsCollector metrics.MetricCollector
}

func NewConnector(config Config, metricsCollector metrics.MetricCollector, logger *logrus.Logger) (*Connector, error) {
	registerMetricsOnce(metricsCollector)
	return &Connector{
		Client:           redis.NewClient(&config.Client),
		Config:           config,
		metricsCollector: metricsCollector,
		logger:           logger,
	}, nil
}

func NewTestRedisConnector(client *redis.Client, config Config) *Connector {
	return &Connector{
		Client: client,
		Config: config,
	}
}

func (c *Connector) VinAllowed(vin string) (bool, error) {
	serverMetricsRegistry.requestCount.Inc(adapter.Labels{"capability": util.VinAllowed})
	exists, err := c.Client.Exists(context.TODO(), c.createRedisKey(c.Config.VinAllowed.Prefix, vin)).Result()
	if err != nil {
		serverMetricsRegistry.requestErrorCount.Inc(adapter.Labels{"capability": util.VinAllowed})
		return c.Config.VinAllowed.AllowOnFailure, err
	}

	return exists == 1, err
}

func (c *Connector) Close() error {
	return c.Client.Close()
}

func (c *Connector) createRedisKey(prefix, vin string) string {
	return fmt.Sprintf("%s%s", prefix, vin)
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	serverMetricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	serverMetricsRegistry.requestCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_redis_request_count",
		Help:   "The number requests hanlded by redis connector.",
		Labels: []string{"capability"},
	})

	serverMetricsRegistry.requestErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_redis_request_error_count",
		Help:   "The number of errors encountered by redis connector.",
		Labels: []string{"capability"},
	})
}
