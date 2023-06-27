package googlepubsub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle google pubsub interactions
type Producer struct {
	pubsubClient      *pubsub.Client
	projectID         string
	namespace         string
	statsCollector    metrics.MetricCollector
	prometheusEnabled bool
	logger            *logrus.Logger
}

func configurePubsub(projectID string) (*pubsub.Client, error) {
	if projectID == "" {
		return nil, errors.New("GCP Project ID cannot be empty")
	}
	_, useEmulator := os.LookupEnv("PUBSUB_EMULATOR_HOST")
	_, useGcpPubsub := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if !useEmulator && !useGcpPubsub {
		return nil, errors.New("must set environment variable GOOGLE_APPLICATION_CREDENTIALS or PUBSUB_EMULATOR_HOST")
	}
	if useEmulator && useGcpPubsub {
		return nil, errors.New("pubsub cannot initialize with both emulator and GCP resource")
	}
	return pubsub.NewClient(context.Background(), projectID)
}

// NewProducer establishes the pubsub connection and define the dispatch method
func NewProducer(ctx context.Context, prometheusEnabled bool, projectID string, namespace string, metrics metrics.MetricCollector, logger *logrus.Logger) (telemetry.Producer, error) {
	pubsubClient, err := configurePubsub(projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub_connect_error %s", err)
	}
	logger.Infof("registered pubsub for project: %s, namespace: %s", projectID, namespace)
	return &Producer{
		projectID:         projectID,
		namespace:         namespace,
		pubsubClient:      pubsubClient,
		prometheusEnabled: prometheusEnabled,
		statsCollector:    metrics,
		logger:            logger,
	}, nil
}

// Produce sends the record payload to pubsub
func (p *Producer) Produce(entry *telemetry.Record) {
	ctx := context.Background()

	pubsubTopic, err := p.createTopicIfNotExists(ctx, telemetry.BuildTopicName(p.namespace, entry.TxType))

	if err != nil {
		p.logger.Errorf("error creating topic %v", err)
		metrics.StatsIncrement(p.statsCollector, "pubsub_not_connected_total", 1, map[string]string{})
		return
	}

	if exists, err := pubsubTopic.Exists(ctx); !exists || err != nil {
		p.logger.Errorf("error checking existing topic %v", err)
		metrics.StatsIncrement(p.statsCollector, "pubsub_not_connected_total", 1, map[string]string{})
		return
	}

	entry.ProduceTime = time.Now()
	result := pubsubTopic.Publish(ctx, &pubsub.Message{
		Data:       entry.Payload(),
		Attributes: entry.Metadata(),
	})
	if _, err = result.Get(ctx); err != nil {
		p.logger.Errorf("pubsub_err err: %v", err)
		if p.prometheusEnabled {
			metrics.StatsIncrement(p.statsCollector, "pubsub_publish_total", 1, map[string]string{"record_type": entry.TxType})
			metrics.StatsIncrement(p.statsCollector, "pubsub_publish_total_bytes", int64(entry.Length()), map[string]string{"record_type": entry.TxType})
		} else {
			metrics.StatsIncrement(p.statsCollector, entry.TxType+"_produce", 1, map[string]string{})
		}
	} else {
		metrics.StatsIncrement(p.statsCollector, "pubsub_err", 1, map[string]string{})
	}
}

func (p *Producer) createTopicIfNotExists(ctx context.Context, topic string) (*pubsub.Topic, error) {
	pubsubTopic := p.pubsubClient.Topic(topic)
	exists, err := pubsubTopic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return pubsubTopic, nil
	}

	return p.pubsubClient.CreateTopic(ctx, topic)
}
