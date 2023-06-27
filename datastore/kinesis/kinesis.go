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
	logger            *logrus.Logger
	prometheusEnabled bool
	statsCollector    metrics.MetricCollector
	streams           map[string]string
}

// NewProducer configures and tests the kinesis connection
func NewProducer(maxRetries int, streams map[string]string, overrideHost string, prometheusEnabled bool, metrics metrics.MetricCollector, logger *logrus.Logger) (telemetry.Producer, error) {
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
		logger:            logger,
		prometheusEnabled: prometheusEnabled,
		statsCollector:    metrics,
		streams:           streams,
	}, nil
}

// Produce asyncronously sends the record payload to kineses
func (p *Producer) Produce(entry *telemetry.Record) {
	entry.ProduceTime = time.Now()
	stream, ok := p.streams[entry.TxType]
	if !ok {
		p.logger.Errorf("kinesis_produce_stream_not_configured: %s", entry.TxType)
		return
	}
	kinesisRecord := &kinesis.PutRecordInput{
		Data:         entry.Payload(),
		StreamName:   aws.String(stream),
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
