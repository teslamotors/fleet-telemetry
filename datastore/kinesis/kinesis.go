package kinesis

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle kinesis interactions
type Producer struct {
	kinesis           *kinesis.Kinesis
	namespace         string
	statsCollector    metrics.MetricCollector
	prometheusEnabled bool
	logger            *logrus.Logger
}

// NewProducer configures and tests the kinesis connection
func NewProducer(maxRetries int, overrideHost string, prometheusEnabled bool, namespace string, metrics metrics.MetricCollector, logger *logrus.Logger) (telemetry.Producer, error) {
	config := &aws.Config{
		MaxRetries:                    aws.Int(maxRetries),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}
	if overrideHost != "" {
		config = config.WithEndpoint(overrideHost)
	}
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}

	service := kinesis.New(sess, config)
	if _, err := service.ListStreams(&kinesis.ListStreamsInput{Limit: aws.Int64(1)}); err != nil {
		return nil, fmt.Errorf("failed to list streams (test connection): %v", err)
	}

	return &Producer{
		kinesis:           service,
		namespace:         namespace,
		prometheusEnabled: prometheusEnabled,
		statsCollector:    metrics,
		logger:            logger,
	}, nil
}

// Produce asyncronously sends the record payload to kineses
func (p *Producer) Produce(entry *telemetry.Record) {
	stream := aws.String(telemetry.BuildTopic(p.namespace, entry))
	entry.ProduceTime = time.Now()
	kinesisRecord := &kinesis.PutRecordInput{
		Data:         entry.Payload(),
		StreamName:   stream,
		PartitionKey: aws.String(entry.Vin),
	}
	kinesisRecordOutput, err := p.kinesis.PutRecord(kinesisRecord)
	if err != nil {
		p.logger.Errorf("kinesis_err: %v", err)
		metrics.StatsIncrement(p.statsCollector, "kinesis_err", 1, map[string]string{"record_type": entry.TxType})
		return
	}

	p.logger.Debugf("kinesis_publish vin=%s,type=%s,txid=%s,shard_id=%s,sequence_number=%s", entry.Vin, entry.TxType, entry.Txid, *kinesisRecordOutput.ShardId, *kinesisRecordOutput.SequenceNumber)

	if p.prometheusEnabled {
		metrics.StatsIncrement(p.statsCollector, "kinesis_publish_total", 1, map[string]string{"record_type": entry.TxType})
		metrics.StatsIncrement(p.statsCollector, "kinesis_publish_total_bytes", int64(entry.Length()), map[string]string{"record_type": entry.TxType})
	} else {
		metrics.StatsIncrement(p.statsCollector, entry.TxType+"_produce", 1, map[string]string{})
	}
}
