package http

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

const (
	jsonContentType     = "application/json"
	protobufContentType = "application/x-protobuf"
)

// Producer client to handle http
type Producer struct {
	namespace             string
	metricsCollector      metrics.MetricCollector
	logger                *logrus.Logger
	workerChannels        []chan *telemetry.Record
	address               string
	httpClient            http.Client
	produceDecodedRecords bool
}

// Metrics stores metrics reported from this package
type Metrics struct {
	produceCount adapter.Counter
	byteTotal    adapter.Counter
	errorCount   adapter.Counter
	bufferSize   adapter.Gauge
}

// TLSCertificate contains the paths to the certificate and key files.
type TLSCertificate struct {
	CertFile string `json:"cert"`
	KeyFile  string `json:"key"`
}

// Config contains the data necessary to configure an http producer.
type Config struct {
	// WorkerCount is the number of http producer routines running.
	// This number should be increased if `http_produce_buffer_size` is growing.
	// To guarantee message delivery order, messages from a specific vehicle are
	// always assigned to the same worker. Load among workers may not be evenly distributed.
	WorkerCount int `json:"worker_count"`

	// Address is the address to produce requests to.
	Address string `json:"address"`

	// Timeout is the number of seconds to wait for a response. Defaults to 10.
	Timeout int `json:"timeout"`

	// TLS is the TLS configuration for the http producer.
	TLS *TLSCertificate `json:"tls,omitempty"`
}

const (
	statusCodePreSendErr = "NOT_SENT"
	bufferSize           = 128
)

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer sets up an HTTP producer
func NewProducer(config *Config, produceDecodedRecords bool, metricsCollector metrics.MetricCollector, namespace string, logger *logrus.Logger) (telemetry.Producer, error) {
	registerMetricsOnce(metricsCollector)

	err := validateConfig(config)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport
	if config.TLS != nil {
		transport, err = getTransport(config.TLS)
		if err != nil {
			return nil, err
		}
	}

	producer := &Producer{
		namespace:             namespace,
		metricsCollector:      metricsCollector,
		workerChannels:        make([]chan *telemetry.Record, config.WorkerCount),
		address:               config.Address,
		logger:                logger,
		produceDecodedRecords: produceDecodedRecords,
		httpClient: http.Client{
			Timeout:   time.Duration(config.Timeout) * time.Second,
			Transport: transport,
		},
	}

	for i := 0; i < config.WorkerCount; i++ {
		producer.workerChannels[i] = make(chan *telemetry.Record, bufferSize)
		go producer.worker(getWorkerId(i), producer.workerChannels[i])
	}

	logger.Infof("registered http producer for namespace: %s", namespace)
	return producer, nil
}

// validateConfig validates configuration values and sets defaults if value not set
func validateConfig(c *Config) error {
	if c.WorkerCount < 0 {
		return errors.New("invalid http worker count")
	}
	if c.WorkerCount == 0 {
		c.WorkerCount = 5
	}
	if c.Timeout < 0 {
		return errors.New("invalid http timeout")
	}
	if c.Timeout == 0 {
		c.Timeout = 10
	}
	return nil
}

func getTransport(tlsConfig *TLSCertificate) (*http.Transport, error) {
	clientTLSCert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, err
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{clientTLSCert},
		},
	}, nil
}

func (p *Producer) worker(id string, records <-chan *telemetry.Record) {
	for record := range records {
		metricsRegistry.bufferSize.Sub(1, map[string]string{"worker": id})
		p.sendHTTP(record)
	}
}

func getWorkerId(idx int) string {
	return fmt.Sprintf("worker-%d", idx)
}

func (p *Producer) sendHTTP(record *telemetry.Record) {
	url := fmt.Sprintf("%s?namespace=%s&type=%s", p.address, p.namespace, record.TxType)
	payload := record.Payload()

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		p.logError(fmt.Errorf("create_request_err %s", err.Error()))
		return
	}

	contentType := protobufContentType
	if p.produceDecodedRecords {
		contentType = jsonContentType
	}
	req.Header.Set("Content-Type", contentType)
	for key, value := range record.Metadata() {
		req.Header.Set(key, value)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logError(fmt.Errorf("send_request_err %s", err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		metricsRegistry.produceCount.Inc(map[string]string{"record_type": record.TxType})
		metricsRegistry.byteTotal.Add(int64(record.Length()), map[string]string{"record_type": record.TxType})
		return
	}
	p.logError(fmt.Errorf("response_status_code %d", resp.StatusCode))
}

func vinToHash(vin string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(vin))
	return h.Sum32()
}

func (p *Producer) getRecordChannelIndex(vin string) int {
	return int(vinToHash(vin) % uint32(len(p.workerChannels)))
}

// Produce asynchronously sends the record payload to http endpoint
func (p *Producer) Produce(record *telemetry.Record) {
	idx := p.getRecordChannelIndex(record.Vin)
	p.workerChannels[idx] <- record
	metricsRegistry.bufferSize.Inc(map[string]string{"worker": getWorkerId(idx)})
}

func (p *Producer) logError(err error) {
	p.logger.Errorf("http_producer_err err: %v", err)
	metricsRegistry.errorCount.Inc(map[string]string{})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.produceCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "http_produce_total",
		Help:   "The number of records produced to http.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "http_produce_total_bytes",
		Help:   "The number of bytes produced to http.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "http_produce_err",
		Help:   "The number of errors while producing to http.",
		Labels: []string{},
	})

	metricsRegistry.bufferSize = metricsCollector.RegisterGauge(adapter.CollectorOptions{
		Name:   "http_produce_buffer_size",
		Help:   "The number of records waiting to be produced per worker.",
		Labels: []string{"worker"},
	})
}
