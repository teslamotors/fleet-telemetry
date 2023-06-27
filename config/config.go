package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed" //Used for default CAs
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/sirupsen/logrus"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/teslamotors/fleet-telemetry/datastore/googlepubsub"
	"github.com/teslamotors/fleet-telemetry/datastore/kafka"
	"github.com/teslamotors/fleet-telemetry/datastore/kinesis"
	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Config object for server
type Config struct {
	// Host is the telemetry server hostname
	Host string `json:"host,omitempty"`

	// Port is the telemetry server port
	Port int `json:"port,omitempty"`

	// Status Port is used to check whether service is live or not
	StatusPort int `json:"status_port,omitempty"`

	// TLS contains certificates & CA info for the webserver
	TLS *TLS `json:"tls,omitempty"`

	// UseDefaultEngCA overrides default CA to eng
	UseDefaultEngCA bool `json:"use_default_eng_ca"`

	// RateLimit is a configuration for the ratelimit
	RateLimit *RateLimit `json:"rate_limit,omitempty"`

	// ReliableAck if true, the server will send an ack back to the client only when the message has been stored in a datastore
	ReliableAck bool `json:"reliable_ack,omitempty"`

	// ReliableAckWorkers is the number of workers that will handle the acknoledgment
	ReliableAckWorkers int `json:"reliable_ack_workers,omitempty"`

	// Kafka is a configuration for the standard librdkafka configuration properties
	// seen here: https://raw.githubusercontent.com/confluentinc/librdkafka/master/CONFIGURATION.md
	// we extract the "topic" key as the default topic for the producer
	Kafka *confluent.ConfigMap `json:"kafka,omitempty"`

	// Kinesis is a configuration for AWS Kinesis
	Kinesis *Kinesis `json:"kinesis,omitempty"`

	// Pubsub is a configuration for the Google Pubsub
	Pubsub *Pubsub `json:"pubsub,omitempty"`

	// Namespace defines a prefix for the kafka/pubsub topic
	Namespace string `json:"namespace,omitempty"`

	Monitoring *Monitoring `json:"monitoring,omitempty"`

	// LogLevel set the log-level
	LogLevel string `json:"log_level,omitempty"`

	// JSONLogEnable if true log in json format
	JSONLogEnable bool `json:"json_log_enable,omitempty"`

	// Records is a mapping of topics (records type) to a reference dispatch implementation (i,e: kafka)
	Records map[string][]telemetry.Dispatcher `json:"records,omitempty"`

	// MetricCollector collects metrics for the application
	MetricCollector metrics.MetricCollector

	// AckChan is a channel used to push acknoledgment from the datastore to connected clients
	AckChan chan (*telemetry.Record)
}

// RateLimit config for the service to handle ratelimiting incoming requests
type RateLimit struct {
	// MessageRateLimiterEnabled skip messages if it exceeds the limit
	Enabled bool `json:"enabled,omitempty"`

	// MessageLimit is a rate limiting of the number of messages per client
	MessageLimit int `json:"message_limit,omitempty"`

	// MessageInterval is the rate limit time interval
	MessageInterval int `json:"message_interval_time,omitempty"`

	// MessageIntervalTimeSecond is the rate limit time interval as a duration in second
	MessageIntervalTimeSecond time.Duration
}

// Pubsub config for the Google pubsub
type Pubsub struct {
	// GCP Project ID
	ProjectID string `json:"gcp_project_id,omitempty"`

	Publisher *pubsub.Client
}

// Kinesis is a configuration for aws Kinesis.
type Kinesis struct {
	MaxRetries   *int              `json:"max_retries,omitempty"`
	OverrideHost string            `json:"override_host"`
	Streams      map[string]string `json:"streams,omitempty"`
}

//go:embed files/eng_ca.crt
var defaultEngCA []byte

//go:embed files/prod_ca.crt
var defaultProdCA []byte

// TLS config for the server
type TLS struct {
	CAFile     string `json:"ca_file"`
	ServerCert string `json:"server_cert"`
	ServerKey  string `json:"server_key"`
}

// Monitoring config for profiler and prometheus
type Monitoring struct {
	// PrometheusMetricsPort port to run prometheus on
	PrometheusMetricsPort int `json:"prometheus_metrics_port,omitempty"`

	// Statsd metrics if you are not using prometheus
	Statsd *Statsd `json:"statsd,omitempty"`

	// ProfilerPort if non zero enable http profiler om this port
	ProfilerPort int `json:"profiler_port,omitempty"`

	// ProfilingPath is the variable that enable deep profiling is set
	ProfilingPath string `json:"profiling_path,omitempty"`

	ProfilerFile *os.File
}

// Statsd config for metrics
type Statsd struct {
	// HostPort host:port of the statsd server
	HostPort string `json:"host,omitempty"`

	// StatsdPrefix prefix for statsd metrics
	Prefix string `json:"prefix,omitempty"`

	// StatsSampleRate 0 to 100 percentage to sample stats
	SampleRate int `json:"sample_rate,omitempty"`

	// StatsFlushPeriod in ms
	FlushPeriod int `json:"flush_period,omitempty"`
}

// ExtractServiceTLSConfig return the TLS config needed for stating the mTLS Server
func (c *Config) ExtractServiceTLSConfig() (*tls.Config, error) {
	if c.TLS == nil {
		return nil, errors.New("tls config is empty - telemetry server is mTLS only, make sure to provide certificates in the config")
	}

	var err error
	var caFileBytes []byte
	if c.TLS.CAFile == "" {
		if c.UseDefaultEngCA {
			caFileBytes = make([]byte, len(defaultEngCA))
			copy(caFileBytes, defaultEngCA)
		} else {
			caFileBytes = make([]byte, len(defaultProdCA))
			copy(caFileBytes, defaultProdCA)
		}
	} else {
		caFileBytes, err = os.ReadFile(c.TLS.CAFile)
		if err != nil {
			return nil, err
		}
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caFileBytes)
	if !ok {
		return nil, fmt.Errorf("tls ca not properly loaded: %s", c.TLS.CAFile)
	}

	return &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

func (c *Config) configureLogger(logger *logrus.Logger) {
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		logger.Errorf("Invalid level: %s\n", err)
	} else {
		logrus.SetLevel(level)
	}

	// Log as JSON instead of the default ASCII formatter.
	if c.JSONLogEnable {
		logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	}
}

func (c *Config) configureStatsCollector(logger *logrus.Logger) {
	if c.Monitoring == nil || (c.Monitoring.PrometheusMetricsPort == 0 && c.Monitoring.Statsd == nil) {
		logger.Infoln("config_skipping_empty_metrics_provider")
		return
	}

	var err error
	if c.Monitoring.PrometheusMetricsPort > 0 {
		c.MetricCollector = metrics.NewPrometheusCollector(logger, "")
	} else if c.Monitoring.Statsd != nil {
		sample, flushDuration := float32(1.0), 100*time.Millisecond

		if c.Monitoring.Statsd.SampleRate != 100 {
			sample = float32(c.Monitoring.Statsd.SampleRate) / 100
			flushDuration = time.Duration(c.Monitoring.Statsd.FlushPeriod) * time.Millisecond
		}

		c.MetricCollector, err = metrics.NewStatsdCollectorWithFlushAndSampleRate(c.Monitoring.Statsd.HostPort, c.Monitoring.Statsd.Prefix, logger, sample, flushDuration)
		if err != nil {
			logger.Errorf("skipping_metrics_error error: %v", err)
		}
	}
}

func (c *Config) prometheusEnabled() bool {
	if c.Monitoring != nil && c.Monitoring.PrometheusMetricsPort > 0 {
		return true
	}
	return false
}

// ConfigureProducers validates and estabilishes connections to the producers (kakfa/pubsub/logger)
func (c *Config) ConfigureProducers(logger *logrus.Logger) (map[string][]telemetry.Producer, error) {
	producers := make(map[telemetry.Dispatcher]telemetry.Producer)
	producers[telemetry.Logger] = simple.NewProtoLogger(logger)

	requiredDispatchers := make(map[telemetry.Dispatcher][]string)
	for recordName, dispatchRules := range c.Records {
		for _, dispatchRule := range dispatchRules {
			requiredDispatchers[dispatchRule] = append(requiredDispatchers[dispatchRule], recordName)
		}
	}

	if _, ok := requiredDispatchers[telemetry.Kafka]; ok {
		if c.Kafka == nil {
			return nil, errors.New("Expected Kafka to be configured")
		}
		convertKafkaConfig(c.Kafka)
		kafkaProducer, err := kafka.NewProducer(c.Kafka, c.Namespace, c.ReliableAckWorkers, c.AckChan, c.prometheusEnabled(), c.MetricCollector, logger)
		if err != nil {
			return nil, err
		}
		producers[telemetry.Kafka] = kafkaProducer
	}

	if _, ok := requiredDispatchers[telemetry.Pubsub]; ok {
		if c.Pubsub == nil {
			return nil, errors.New("Expected Pubsub to be configured")
		}
		googleProducer, err := googlepubsub.NewProducer(context.Background(), c.prometheusEnabled(), c.Pubsub.ProjectID, c.Namespace, c.MetricCollector, logger)
		if err != nil {
			return nil, err
		}
		producers[telemetry.Pubsub] = googleProducer
	}

	if recordNames, ok := requiredDispatchers[telemetry.Kinesis]; ok {
		if c.Kinesis == nil {
			return nil, errors.New("Expected Kinesis to be configured")
		}
		maxRetries := 1
		if c.Kinesis.MaxRetries != nil {
			maxRetries = *c.Kinesis.MaxRetries
		}
		streamMapping := c.CreateKinesisStreamMapping(recordNames)
		kinesis, err := kinesis.NewProducer(maxRetries, streamMapping, c.Kinesis.OverrideHost, c.prometheusEnabled(), c.MetricCollector, logger)
		if err != nil {
			return nil, err
		}
		producers[telemetry.Kinesis] = kinesis
	}

	dispatchProducerRules := make(map[string][]telemetry.Producer)
	for recordName, dispatchRules := range c.Records {
		var dispatchFuncs []telemetry.Producer
		for _, dispatchRule := range dispatchRules {
			dispatchFuncs = append(dispatchFuncs, producers[dispatchRule])
		}
		dispatchProducerRules[recordName] = dispatchFuncs

		if len(dispatchProducerRules[recordName]) == 0 {
			return nil, fmt.Errorf("unknown_dispatch_rule record: %v, dispatchRule:%v", recordName, dispatchRules)
		}
	}

	return dispatchProducerRules, nil
}

// convertKafkaConfig will prioritize int over float
// see: https://github.com/confluentinc/confluent-kafka-go/blob/cde2827bc49655eca0f9ce3fc1cda13cb6cdabc9/kafka/config.go#L108-L125
func convertKafkaConfig(input *confluent.ConfigMap) {
	for key, val := range *input {
		if i, ok := val.(float64); ok {
			(*input)[key] = int(i)
		}
	}
}

// CreateKinesisStreamMapping uses the config, overrides with ENV variable names, and finally falls back to namespace based names
func (c *Config) CreateKinesisStreamMapping(recordNames []string) map[string]string {
	streamMapping := make(map[string]string)
	for _, recordName := range recordNames {
		if c.Kinesis != nil {
			streamMapping[recordName] = c.Kinesis.Streams[recordName]
		}
		envVarStreamName := os.Getenv(fmt.Sprintf("KINESIS_STREAM_%s", strings.ToUpper(recordName)))
		if envVarStreamName != "" {
			streamMapping[recordName] = envVarStreamName
		}
		if streamMapping[recordName] == "" {
			streamMapping[recordName] = telemetry.BuildTopicName(c.Namespace, recordName)
		}
	}
	return streamMapping
}
