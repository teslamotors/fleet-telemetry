package grpc

import (
	"context"
	"sync"
	"time"

	"github.com/teslamotors/fleet-telemetry/connector/util"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	pb "github.com/teslamotors/fleet-telemetry/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	cacheCleanupInterval = 5 * time.Minute
)

var (
	serverMetricsRegistry ServerMetrics
	serverMetricsOnce     sync.Once
)

type ServerMetrics struct {
	cacheHitCount     adapter.Counter
	requestErrorCount adapter.Counter
	requestCount      adapter.Counter
}

type VinAllowed struct {
	AllowOnFailure  bool `json:"allow_on_failure,omitempty"`
	CacheResults    bool `json:"cache_results,omitempty"`
	CacheTTLMinutes int  `json:"cache_ttl_minutes,omitempty"`
	cacheTTL        time.Duration
}

type Config struct {
	Host         string     `json:"host"`
	Tls          *TlsConfig `json:"tls,omitempty"`
	Capabilities []string   `json:"capabilities"`
	VinAllowed   VinAllowed `json:"vin_allowed,omitempty"`
}

type TlsConfig struct {
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type Connector struct {
	Client          pb.VehicleServiceClient
	Config          Config
	vinAllowedCache *util.Cache[bool]

	conn             *grpc.ClientConn
	logger           *logrus.Logger
	metricsCollector metrics.MetricCollector
}

func NewConnector(config Config, metricsCollector metrics.MetricCollector, logger *logrus.Logger) (*Connector, error) {
	registerMetricsOnce(metricsCollector)

	var opts []grpc.DialOption
	if config.Tls != nil {
		creds, err := credentials.NewClientTLSFromFile(config.Tls.CertFile, config.Tls.KeyFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	conn, err := grpc.NewClient(config.Host, opts...)
	if err != nil {
		return nil, err
	}

	config.VinAllowed.cacheTTL = time.Duration(config.VinAllowed.CacheTTLMinutes) * time.Minute

	return &Connector{
		Client:           pb.NewVehicleServiceClient(conn),
		conn:             conn,
		vinAllowedCache:  util.NewCache[bool](cacheCleanupInterval),
		Config:           config,
		logger:           logger,
		metricsCollector: metricsCollector,
	}, nil
}

func NewTestGrpcConnector(client pb.VehicleServiceClient, config Config, metricsCollector metrics.MetricCollector, logger *logrus.Logger) *Connector {
	registerMetricsOnce(metricsCollector)
	return &Connector{
		Client:          client,
		vinAllowedCache: util.NewCache[bool](cacheCleanupInterval),
		Config:          config,
		logger:          logger,
	}
}

func (c *Connector) VinAllowed(vin string) (bool, error) {
	serverMetricsRegistry.requestCount.Inc(adapter.Labels{"capability": util.VinAllowed})

	if allowed, ok := c.vinAllowedCache.Get(vin); ok {
		serverMetricsRegistry.cacheHitCount.Inc(adapter.Labels{"capability": util.VinAllowed})
		return *allowed, nil
	}

	req := &pb.VinAllowedRequest{Vin: vin}
	res, err := c.Client.VinAllowed(context.Background(), req)
	if err != nil {
		c.logger.ErrorLog("grpc_connector_vin_allowed_error", err, nil)
		serverMetricsRegistry.requestErrorCount.Inc(adapter.Labels{"capability": util.VinAllowed})
		return c.Config.VinAllowed.AllowOnFailure, err
	}

	if c.Config.VinAllowed.CacheResults {
		c.vinAllowedCache.Set(vin, res.Allowed, c.Config.VinAllowed.cacheTTL)
	}

	return res.Allowed, nil
}

func (c *Connector) Close() error {
	return c.conn.Close()
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	serverMetricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	serverMetricsRegistry.requestCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_grpc_request_count",
		Help:   "The number requests hanlded by grpc connector.",
		Labels: []string{"capability"},
	})

	serverMetricsRegistry.requestErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_grpc_request_error_count",
		Help:   "The number of errors encountered by grpc connector.",
		Labels: []string{"capability"},
	})

	serverMetricsRegistry.cacheHitCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "data_connector_grpc_request_cache_hit_count",
		Help:   "The number of requests served by grpc cache.",
		Labels: []string{"capability"},
	})
}
