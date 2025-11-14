package zmq

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/pebbe/zmq4"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Config contains the data necessary to configure a zmq producer.
type Config struct {
	// Addr is the address to which to producer will attempt to bind.
	Addr string `json:"addr"`

	// ServerKeyJSONPath is the path to a file which contains the server's secret
	// key as a json. This key can be generated using zmq4.NewCurveKeypair
	ServerKeyJSONPath string `json:"server_key_json_path"`

	// AllowedPublicKeysJSONPath is the path to a file which contains a list of
	// allowed public keys for client connections. This field is optional.
	AllowedPublicKeysJSONPath string `json:"allowed_public_keys_json_path"`

	// Verbose controls if verbose logging is enabled for the socket.
	Verbose bool `json:"verbose"`
}

// KeyJSON contains z85 key data
type KeyJSON struct {
	// Secret is the secret key encoded as a 40 char z85 string.
	Secret string `json:"secret"`

	// Public is the public key encoded as a 40 char z85 string.
	Public string `json:"public"`
}

// Metrics stores metrics reported from this package
type Metrics struct {
	errorCount       adapter.Counter
	publishCount     adapter.Counter
	byteTotal        adapter.Counter
	reliableAckCount adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// MonitorSocketAddr is the address at which the socket monitor connects to the
// ZMQ publisher socket.
const MonitorSocketAddr = "inproc://zmq_socket_monitor.rep"

// Producer implements the telemetry.Producer interface by publishing to a
// bound zmq socket.
type Producer struct {
	namespace          string
	ctx                context.Context
	sock               *zmq4.Socket
	logger             *logrus.Logger
	airbrakeHandler    *airbrake.Handler
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
}

// Produce the record to the socket.
func (p *Producer) Produce(rec *telemetry.Record) {
	if p.ctx.Err() != nil {
		return
	}
	nBytes, err := p.sock.SendMessage(telemetry.BuildTopicName(p.namespace, rec.TxType), rec.Payload())
	if err != nil {
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
		p.ReportError("zmq_dispatch_error", err, nil)
		return
	}
	p.ProcessReliableAck(rec)
	metricsRegistry.byteTotal.Add(int64(nBytes), map[string]string{"record_type": rec.TxType})
	metricsRegistry.publishCount.Inc(map[string]string{"record_type": rec.TxType})
}

// ReportError to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
}

// Close the underlying socket.
func (p *Producer) Close() error {
	if p.sock != nil {
		if err := p.sock.Close(); err != nil {
			return err
		}
	}
	p.sock = nil
	return nil
}

// ProcessReliableAck sends to ackChan if reliable ack is configured
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	_, ok := p.reliableAckTxTypes[entry.TxType]
	if ok {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// NewProducer creates a ZMQProducer with the given config.
func NewProducer(ctx context.Context, config *Config, metrics metrics.MetricCollector, namespace string, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (producer telemetry.Producer, err error) {
	registerMetricsOnce(metrics)
	sock, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		return
	}

	if config.Verbose {
		ready := make(chan struct{})
		if err = logSocketInBackground(ctx, sock, logger, MonitorSocketAddr, ready); err != nil {
			return
		}
		<-ready
	}

	if config.ServerKeyJSONPath != "" {
		var fi *os.File
		fi, err = os.Open(config.ServerKeyJSONPath)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := fi.Close(); err != nil {
				logger.ErrorLog("zmq_server_key_file_close_error", err, nil)
			}
		}()
		key := KeyJSON{}
		if err = json.NewDecoder(fi).Decode(&key); err != nil {
			return nil, err
		}

		if len(config.AllowedPublicKeysJSONPath) > 0 {
			fi2, err := os.Open(config.AllowedPublicKeysJSONPath)
			if err != nil {
				return nil, err
			}
			defer func() {
				if err := fi2.Close(); err != nil {
					logger.ErrorLog("zmq_allowed_public_keys_file_close_error", err, nil)
				}
			}()

			var keys []string
			if err = json.NewDecoder(fi2).Decode(&keys); err != nil {
				return nil, err
			}

			zmq4.AuthCurveAdd("*", keys...)
		}

		if err = zmq4.AuthStart(); err != nil {
			return
		}

		if err = sock.ServerAuthCurve("*", key.Secret); err != nil {
			return
		}
	}

	if err = sock.Bind(config.Addr); err != nil {
		return
	}

	return &Producer{
		namespace:          namespace,
		ctx:                ctx,
		sock:               sock,
		logger:             logger,
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}, nil
}

// logSocketInBackground logs the socket activity in the background.
func logSocketInBackground(ctx context.Context, target *zmq4.Socket, logger *logrus.Logger, addr string, ready chan<- struct{}) error {
	if err := target.Monitor(addr, zmq4.EVENT_ALL); err != nil {
		return err
	}

	monitor, err := zmq4.NewSocket(zmq4.PAIR)
	if err != nil {
		return err
	}

	if err := monitor.Connect(addr); err != nil {
		return err
	}

	go func() {
		ready <- struct{}{}
		defer func() {
			if err := monitor.Close(); err != nil {
				logger.ErrorLog("zmq_monitor_close_error", err, nil)
			}
		}()
		for {
			if ctx.Err() != nil {
				return
			}

			eventType, addr, value, err := monitor.RecvEvent(0)
			if err != nil {
				logger.ErrorLog("zmq_event_receive_error", err, nil)
				continue
			}
			logger.Log(logrus.DEBUG, "zmq_socket_event", logrus.LogInfo{"event_type": eventType, "addr": addr, "value": value})
		}
	}()

	return nil
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "zmq_err",
		Help:   "The number of errors while producing to ZMQ.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.publishCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "zmq_publish_total",
		Help:   "The number of messages published to ZMQ.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.byteTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "zmq_publish_total_bytes",
		Help:   "The number of bytes published to ZMQ.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "zmq_reliable_ack_total",
		Help:   "The number of records produced to ZMQ for which we sent a reliable ACK.",
		Labels: []string{"record_type"},
	})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}
