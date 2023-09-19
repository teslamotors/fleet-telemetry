package zmq

import (
	"context"
	"encoding/json"
	"os"
	"strings"
  "sync"

	zmq "github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// DefaultTopicSep is the default join character used to build a topic.
const DefaultTopicSep = "."

// Config contains the data necessary to configure a zmq producer.
type Config struct {
  // Addr is the address to which to producer will attempt to bind.
  Addr string

  // ServerKeyJSONPath is the path to a file which contains the server's secret
  // key as a json. This key can be generated using zmq4.NewCurveKeypair
  ServerKeyJSONPath string

  // TopicPrefix is an optional topic prefix which will be added to the
  // published data.
  TopicPrefix string

  // TopicSuffix is an optional topic suffix which will be added to the
  // published data.
  TopicSuffix string

  // TopicSep is the seperator character used in creating topics. By default
  // this is ".".
  TopicSep string

  // Verbose controls if verbose logging is enabled for the socket.
  Verbose bool
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
	errorCount   adapter.Counter
	publishCount adapter.Counter
	byteTotal    adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// ZMQProducer implements the telemetry.Producer interface by publishing to a
// bound zmq socket.
type ZMQProducer struct {
  topicPrefix string
  topicSuffix string
  topicSep string
  ctx context.Context
  sock *zmq.Socket
  logger *logrus.Logger
}

// Publish the record to the socket.
func (p *ZMQProducer) Produce(rec *telemetry.Record) {
  if p.ctx.Err() != nil {
    return
  }

  topicParts := make([]string, 0, 4)
  if len(p.topicPrefix) > 0 {
    topicParts = append(topicParts, p.topicPrefix)
  }
  topicParts = append(topicParts, rec.Vin)
  topicParts = append(topicParts, rec.TxType)
  if len(p.topicSuffix) > 0 {
    topicParts = append(topicParts, p.topicSuffix)
  }
  joinChar := DefaultTopicSep
  if len(p.topicSep) > 0 {
    joinChar = p.topicSep
  }
  topic := strings.Join(topicParts, joinChar)

  if nBytes, err := p.sock.SendMessage(topic, rec.Payload()); err != nil {
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": rec.TxType})
    p.logger.Errorf("Failed sending log on zmq socket: %s", err.Error())
  } else {
    metricsRegistry.byteTotal.Add(int64(nBytes), map[string]string{"record_type": rec.TxType})
    metricsRegistry.publishCount.Inc(map[string]string{"record_type": rec.TxType})
  }
}

// NewProducer creates a ZMQProducer with the given config.
func NewProducer(ctx context.Context, config *Config, metrics metrics.MetricCollector, logger *logrus.Logger) (producer telemetry.Producer, err error) {
  sock, err := zmq.NewSocket(zmq.PUB)
  if err != nil {
    return
  }

  if config.Verbose {
    ready := make(chan struct{})
    if err = logSocketInBackground(logger, "inproc://zmq_socket_monitor.rep", ready, ctx); err != nil {
      return
    }
    <-ready
  }

  fi, err := os.Open(config.ServerKeyJSONPath)
  if err != nil {
    return
  }
  defer fi.Close()
  key := KeyJSON{}
  if err = json.NewDecoder(fi).Decode(&key); err != nil {
    return
  }

  if err = sock.SetCurveServer(1); err != nil {
    return 
  }

  if err = sock.SetCurveSecretkey(key.Secret); err != nil {
    return
  }

  if err = sock.Bind(config.Addr); err != nil {
    return
  }

  return &ZMQProducer{
    config.TopicPrefix, config.TopicSuffix, config.TopicSep, ctx, sock, logger,
  }, nil
}

// logSocketInBackground logs the socket activity in the background.
func logSocketInBackground(logger *logrus.Logger, addr string, ready chan<- struct{}, ctx context.Context) error {
  monitor, err := zmq.NewSocket(zmq.PAIR)
  if err != nil {
    return err
  }

  if err := monitor.Connect(addr); err != nil {
    return err
  }

  ready <- struct{}{}
  go func() {
    defer monitor.Close()
    for {
      if ctx.Err() != nil {
        return
      }

      eventType, addr, value, err := monitor.RecvEvent(0)
      if err != nil {
        logger.Errorf("Failed receiving event on zmq socket: %s", err)
        continue
      }

      logger.Infof("ZMQ socket event: %v %v %v", eventType, addr, value)
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
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}
