package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/teslamotors/fleet-telemetry/connector/util"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
)

const (
	cacheCleanupInterval = 5 * time.Minute
)

var (
	serverMetricsRegistry ServerMetrics
	serverMetricsOnce     sync.Once
)

type ServerMetrics struct {
	requestErrorCount adapter.Counter
	requestCount      adapter.Counter
	cacheHitCount     adapter.Counter
}

type VinAllowed struct {
	AllowOnFailure  bool `json:"allow_on_failure,omitempty"`
	CacheResults    bool `json:"cache_results,omitempty"`
	CacheTTLMinutes int  `json:"cache_ttl_minutes,omitempty"`
	cacheTTL        time.Duration
}

type Config struct {
	Host           string          `json:"host"`
	TimeoutSeconds int             `json:"timeout_seconds,omitempty"`
	Transport      *http.Transport `json:"transport,omitempty"`
	Capabilities   []string        `json:"capabilities"`
	VinAllowed     VinAllowed      `json:"vin_allowed,omitempty"`
}

type Connector struct {
	Config          *Config
	client          *http.Client
	vinAllowedCache *util.Cache[bool]

	logger           *logrus.Logger
	metricsCollector metrics.MetricCollector
}

type VinAllowedResponse struct {
	Allowed bool `json:"allowed"`
}

func NewConnector(config *Config, metricsCollector metrics.MetricCollector, logger *logrus.Logger) (*Connector, error) {
	registerMetricsOnce(metricsCollector)
	populateDefaults(config)

	config.VinAllowed.cacheTTL = time.Duration(config.VinAllowed.CacheTTLMinutes) * time.Minute

	return &Connector{
		Config:          config,
		vinAllowedCache: util.NewCache[bool](cacheCleanupInterval),
		client: &http.Client{
			Timeout:   time.Duration(config.TimeoutSeconds) * time.Second,
			Transport: config.Transport,
		},
		logger:           logger,
		metricsCollector: metricsCollector,
	}, nil
}

func (c *Connector) VinAllowed(vin string) (bool, error) {
	serverMetricsRegistry.requestCount.Inc(adapter.Labels{"capability": util.VinAllowed})
	if allowed, ok := c.vinAllowedCache.Get(vin); ok {
		serverMetricsRegistry.cacheHitCount.Inc(adapter.Labels{"capability": util.VinAllowed})
		return *allowed, nil
	}

	var vinAllowedResponse VinAllowedResponse
	res, err := c.get(fmt.Sprintf("%s/allowed", vin), &vinAllowedResponse)
	if err != nil {
		c.logger.ErrorLog("http_connector_vin_allowed_error", err, nil)
		serverMetricsRegistry.requestErrorCount.Inc(adapter.Labels{"capability": util.VinAllowed, "status": "0"})
		return c.Config.VinAllowed.AllowOnFailure, err
	}
	if res.StatusCode != http.StatusOK {
		c.logger.ErrorLog("http_connector_vin_allowed_error", fmt.Errorf("unexpected status code: %d", res.StatusCode), logrus.LogInfo{"status": res.StatusCode})
		serverMetricsRegistry.requestErrorCount.Inc(adapter.Labels{"capability": "vin_allowed", "status": fmt.Sprintf("%d", res.StatusCode)})
		return c.Config.VinAllowed.AllowOnFailure, nil
	}

	if c.Config.VinAllowed.CacheResults {
		c.vinAllowedCache.Set(vin, vinAllowedResponse.Allowed, c.Config.VinAllowed.cacheTTL)
	}

	return vinAllowedResponse.Allowed, nil
}

func (c *Connector) Close() error {
	return nil
}

func (c *Connector) get(path string, resultStruct interface{}) (*http.Response, error) {
	res, err := c.client.Get(fmt.Sprintf("%s/%s", c.Config.Host, path))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		err = json.NewDecoder(res.Body).Decode(resultStruct)
	}

	return res, err
}

func populateDefaults(config *Config) {
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 10
	}
	if config.Transport == nil {
		config.Transport = http.DefaultTransport.(*http.Transport)
	}
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	serverMetricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	serverMetricsRegistry.requestCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_http_request_count",
		Help:   "The number requests handled by http connector.",
		Labels: []string{"capability"},
	})

	serverMetricsRegistry.requestErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_http_request_error_count",
		Help:   "The number of errors encountered by http connector.",
		Labels: []string{"capability", "status"},
	})

	serverMetricsRegistry.cacheHitCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_http_request_cache_hit_count",
		Help:   "The number of requests served by http cache.",
		Labels: []string{"capability"},
	})
}
