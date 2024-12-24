package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed" //Used for default CAs
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	githubairbrake "github.com/airbrake/gobrake/v5"

	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	githublogrus "github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/datastore/googlepubsub"
	"github.com/teslamotors/fleet-telemetry/datastore/kafka"
	"github.com/teslamotors/fleet-telemetry/datastore/kinesis"
	"github.com/teslamotors/fleet-telemetry/datastore/simple"
	"github.com/teslamotors/fleet-telemetry/datastore/zmq"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

const (
	airbrakeProjectKeyEnv = "AIRBRAKE_PROJECT_KEY"
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

	// DisableTLS indicates whether to disable mutual TLS (mTLS) for incoming connections.
	// This should only be set to true if there is a reverse proxy that is already handling
	// mTLS on behalf of this service. TLS will be ignored.
	DisableTLS bool `json:"disable_tls,omitempty"`

	// UseDefaultEngCA overrides default CA to eng
	UseDefaultEngCA bool `json:"use_default_eng_ca"`

	// RateLimit is a configuration for the ratelimit
	RateLimit *RateLimit `json:"rate_limit,omitempty"`

	// ReliableAckSources is a mapping of record types to a dispatcher that will be used for reliable ack
	ReliableAckSources map[string]telemetry.Dispatcher `json:"reliable_ack_sources,omitempty"`

	// Kafka is a configuration for the standard librdkafka configuration properties
	// seen here: https://raw.githubusercontent.com/confluentinc/librdkafka/master/CONFIGURATION.md
	// we extract the "topic" key as the default topic for the producer
	Kafka *confluent.ConfigMap `json:"kafka,omitempty"`

	// Kinesis is a configuration for AWS Kinesis
	Kinesis *Kinesis `json:"kinesis,omitempty"`

	// Pubsub is a configuration for the Google Pubsub
	Pubsub *Pubsub `json:"pubsub,omitempty"`

	// ZMQ configures a zeromq socket
	ZMQ *zmq.Config `json:"zmq,omitempty"`

	// Namespace defines a prefix for the kafka/pubsub topic
	Namespace string `json:"namespace,omitempty"`

	// Monitoring defines information for metrics
	Monitoring *metrics.MonitoringConfig `json:"monitoring,omitempty"`

	// LoggerConfig configures the simple logger
	LoggerConfig *simple.Config `json:"logger,omitempty"`

	// LogLevel set the log-level
	LogLevel string `json:"log_level,omitempty"`

	// JSONLogEnable if true log in json format
	JSONLogEnable bool `json:"json_log_enable,omitempty"`

	// Records is a mapping of topics (records type) to a reference dispatch implementation (i,e: kafka)
	Records map[string][]telemetry.Dispatcher `json:"records,omitempty"`

	// TransmitDecodedRecords if true decodes proto message before dispatching it to supported datastores
	TransmitDecodedRecords bool `json:"transmit_decoded_records,omitempty"`

	// MetricCollector collects metrics for the application
	MetricCollector metrics.MetricCollector

	// AckChan is a channel used to push acknowledgment from the datastore to connected clients
	AckChan chan (*telemetry.Record)

	// Airbrake config
	Airbrake *Airbrake
}

// Airbrake config
type Airbrake struct {
	Host        string `json:"host"`
	ProjectKey  string `json:"project_key"`
	Environment string `json:"environment"`
	ProjectID   int64  `json:"project_id"`

	TLS *TLS `json:"tls" yaml:"tls"`
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

// TLS config
type TLS struct {
	CAFile     string `json:"ca_file"`
	ServerCert string `json:"server_cert"`
	ServerKey  string `json:"server_key"`
}

// AirbrakeTLSConfig return the TLS config needed for connecting with airbrake server
func (c *Config) AirbrakeTLSConfig() (*tls.Config, error) {
	if c.Airbrake.TLS == nil {
		return nil, nil
	}
	caPath := c.Airbrake.TLS.CAFile
	certPath := c.Airbrake.TLS.ServerCert
	keyPath := c.Airbrake.TLS.ServerKey
	tlsConfig := &tls.Config{}
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("can't properly load cert pair (%s, %s): %s", certPath, keyPath, err.Error())
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		// TODO remove the lint bypass
		// nolint:staticcheck
		tlsConfig.BuildNameToCertificate()
	}

	if caPath != "" {
		clientCACert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("can't properly load ca cert (%s): %s", caPath, err.Error())
		}
		clientCertPool := x509.NewCertPool()
		clientCertPool.AppendCertsFromPEM(clientCACert)
		tlsConfig.RootCAs = clientCertPool
	}

	return tlsConfig, nil
}

// ExtractServiceTLSConfig return the TLS config needed for stating the mTLS Server
func (c *Config) ExtractServiceTLSConfig(logger *logrus.Logger) (*tls.Config, error) {
	if c.TLS == nil {
		return nil, errors.New("tls config is empty - telemetry server is mTLS only, make sure to provide certificates in the config")
	}

	var caFileBytes []byte
	var caEnv string
	if c.UseDefaultEngCA {
		caEnv = "eng"
		caFileBytes = make([]byte, len(defaultEngCA))
		copy(caFileBytes, defaultEngCA)
	} else {
		caEnv = "prod"
		caFileBytes = make([]byte, len(defaultProdCA))
		copy(caFileBytes, defaultProdCA)
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caFileBytes)
	if !ok {
		return nil, fmt.Errorf("tls ca not properly loaded for %s environment", caEnv)
	}
	if c.TLS.CAFile != "" {
		customCaFileBytes, err := os.ReadFile(c.TLS.CAFile)
		if err != nil {
			return nil, err
		}
		ok := caCertPool.AppendCertsFromPEM(customCaFileBytes)
		if !ok {
			return nil, fmt.Errorf("custom ca not properly loaded: %s", c.TLS.CAFile)
		}
		logger.ActivityLog("custom_ca_file_appened", logrus.LogInfo{"ca_file_path": c.TLS.CAFile})
	}

	return &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

func (c *Config) configureLogger(logger *logrus.Logger) {
	level, err := githublogrus.ParseLevel(c.LogLevel)
	if err != nil {
		logger.ErrorLog("invalid_level", err, nil)
	} else {
		githublogrus.SetLevel(level)
	}
	logger.SetJSONFormatter(c.JSONLogEnable)
}

func (c *Config) configureMetricsCollector(logger *logrus.Logger) {
	c.MetricCollector = metrics.NewCollector(c.Monitoring, logger)
}

func (c *Config) prometheusEnabled() bool {
	if c.Monitoring != nil && c.Monitoring.PrometheusMetricsPort > 0 {
		return true
	}
	return false
}

// ConfigureProducers validates and establishes connections to the producers (kafka/pubsub/logger)
func (c *Config) ConfigureProducers(airbrakeHandler *airbrake.Handler, logger *logrus.Logger) (map[telemetry.Dispatcher]telemetry.Producer, map[string][]telemetry.Producer, error) {
	reliableAckSources, err := c.configureReliableAckSources()
	if err != nil {
		return nil, nil, err
	}

	producers := make(map[telemetry.Dispatcher]telemetry.Producer)
	producers[telemetry.Logger] = simple.NewProtoLogger(c.LoggerConfig, logger)

	requiredDispatchers := make(map[telemetry.Dispatcher][]string)
	for recordName, dispatchRules := range c.Records {
		for _, dispatchRule := range dispatchRules {
			requiredDispatchers[dispatchRule] = append(requiredDispatchers[dispatchRule], recordName)
		}
	}

	if _, ok := requiredDispatchers[telemetry.Kafka]; ok {
		if c.Kafka == nil {
			return nil, nil, errors.New("expected Kafka to be configured")
		}
		convertKafkaConfig(c.Kafka)
		kafkaProducer, err := kafka.NewProducer(c.Kafka, c.Namespace, c.prometheusEnabled(), c.MetricCollector, airbrakeHandler, c.AckChan, reliableAckSources[telemetry.Kafka], logger)
		if err != nil {
			return nil, nil, err
		}
		producers[telemetry.Kafka] = kafkaProducer
	}

	if _, ok := requiredDispatchers[telemetry.Pubsub]; ok {
		if c.Pubsub == nil {
			return nil, nil, errors.New("expected Pubsub to be configured")
		}
		googleProducer, err := googlepubsub.NewProducer(c.prometheusEnabled(), c.Pubsub.ProjectID, c.Namespace, c.MetricCollector, airbrakeHandler, c.AckChan, reliableAckSources[telemetry.Pubsub], logger)
		if err != nil {
			return nil, nil, err
		}
		producers[telemetry.Pubsub] = googleProducer
	}

	if recordNames, ok := requiredDispatchers[telemetry.Kinesis]; ok {
		if c.Kinesis == nil {
			return nil, nil, errors.New("expected Kinesis to be configured")
		}
		maxRetries := 1
		if c.Kinesis.MaxRetries != nil {
			maxRetries = *c.Kinesis.MaxRetries
		}
		streamMapping := c.CreateKinesisStreamMapping(recordNames)
		kinesis, err := kinesis.NewProducer(maxRetries, streamMapping, c.Kinesis.OverrideHost, c.prometheusEnabled(), c.MetricCollector, airbrakeHandler, c.AckChan, reliableAckSources[telemetry.Kinesis], logger)
		if err != nil {
			return nil, nil, err
		}
		producers[telemetry.Kinesis] = kinesis
	}

	if _, ok := requiredDispatchers[telemetry.ZMQ]; ok {
		if c.ZMQ == nil {
			return nil, nil, errors.New("expected ZMQ to be configured")
		}
		zmqProducer, err := zmq.NewProducer(context.Background(), c.ZMQ, c.MetricCollector, c.Namespace, airbrakeHandler, c.AckChan, reliableAckSources[telemetry.ZMQ], logger)
		if err != nil {
			return nil, nil, err
		}
		producers[telemetry.ZMQ] = zmqProducer
	}

	dispatchProducerRules := make(map[string][]telemetry.Producer)
	for recordName, dispatchRules := range c.Records {
		var dispatchFuncs []telemetry.Producer
		for _, dispatchRule := range dispatchRules {
			dispatchFuncs = append(dispatchFuncs, producers[dispatchRule])
		}
		dispatchProducerRules[recordName] = dispatchFuncs

		if len(dispatchProducerRules[recordName]) == 0 {
			return nil, nil, fmt.Errorf("unknown_dispatch_rule record: %v, dispatchRule:%v", recordName, dispatchRules)
		}
	}

	return producers, dispatchProducerRules, nil
}

func (c *Config) configureReliableAckSources() (map[telemetry.Dispatcher]map[string]interface{}, error) {
	reliableAckSources := make(map[telemetry.Dispatcher]map[string]interface{}, 0)
	for txType, dispatchRule := range c.ReliableAckSources {
		if txType == "connectivity" {
			return nil, fmt.Errorf("reliable ack not needed for txType: %s", txType)
		}
		if dispatchRule == telemetry.Logger {
			return nil, fmt.Errorf("logger cannot be configured as reliable ack for record: %s", txType)
		}
		dispatchers, ok := c.Records[txType]
		if !ok {
			return nil, fmt.Errorf("%s cannot be configured as reliable ack for record: %s since no record mapping exists", dispatchRule, txType)
		}
		dispatchRuleFound := false
		validDispatchers := parseValidDispatchers(dispatchers)
		for _, dispatcher := range validDispatchers {
			if dispatcher == dispatchRule {
				dispatchRuleFound = true
				reliableAckSources[dispatchRule] = map[string]interface{}{txType: true}
				break
			}
		}
		if !dispatchRuleFound {
			return nil, fmt.Errorf("%s cannot be configured as reliable ack for record: %s. Valid datastores configured %v", dispatchRule, txType, validDispatchers)
		}
	}
	return reliableAckSources, nil
}

// parseValidDispatchers removes no-op dispatcher from the input i.e. Logger
func parseValidDispatchers(input []telemetry.Dispatcher) []telemetry.Dispatcher {
	var result []telemetry.Dispatcher
	for _, v := range input {
		if v != telemetry.Logger {
			result = append(result, v)
		}
	}
	return result
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

// CreateAirbrakeNotifier intializes an airbrake notifier with standard configs
func (c *Config) CreateAirbrakeNotifier(logger *logrus.Logger) (*githubairbrake.Notifier, *githubairbrake.NotifierOptions, error) {
	if c.Airbrake == nil {
		return nil, nil, nil
	}
	tlsConfig, err := c.AirbrakeTLSConfig()
	if err != nil {
		return nil, nil, err
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	errbitHost := c.Airbrake.Host
	projectKey, ok := os.LookupEnv(airbrakeProjectKeyEnv)
	logInfo := logrus.LogInfo{}
	if ok {
		logInfo["source"] = "environment_variable"
		logInfo["env_key"] = airbrakeProjectKeyEnv

	} else {
		projectKey = c.Airbrake.ProjectKey
		logInfo["source"] = "config_file"
	}
	logger.ActivityLog("airbrake_configured", logInfo)
	options := &githubairbrake.NotifierOptions{
		Host:                errbitHost,
		RemoteConfigHost:    errbitHost,
		DisableRemoteConfig: true,
		APMHost:             errbitHost,
		DisableAPM:          true,
		ProjectId:           c.Airbrake.ProjectID,
		ProjectKey:          projectKey,
		Environment:         c.Airbrake.Environment,
		HTTPClient:          httpClient,
	}
	return githubairbrake.NewNotifierWithOptions(options), options, nil
}
