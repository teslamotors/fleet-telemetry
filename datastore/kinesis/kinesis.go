package kinesis

import (
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Producer client to handle kafka interactions
type Producer struct {
	kinesis           *kinesis.Kinesis
	namespace         string
	statsCollector    metrics.MetricCollector
	prometheusEnabled bool
	logger            *logrus.Logger
}

// NewProducer establishes the kin connection and define the dispatch method
func NewProducer(accessKey string, secretKey string, defaultRegion string, hosturl string, prometheusEnabled bool, namespace string, metrics metrics.MetricCollector, logger *logrus.Logger) (telemetry.Producer, error) {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			MaxRetries:                    aws.Int(1),
			CredentialsChainVerboseErrors: aws.Bool(true),
			// HTTP client is required to fetch EC2 metadata values
			// having zero timeout on the default HTTP client sometimes makes
			// it fail with Credential error
			// https://github.com/aws/aws-sdk-go/issues/2914
			HTTPClient: &http.Client{Timeout: 10 * time.Second},
		},
	}))

	awsConfig := aws.NewConfig().WithRegion(defaultRegion).WithCredentialsChainVerboseErrors(true)
	if hosturl != "" {
		awsConfig = aws.NewConfig().WithEndpoint(hosturl)

	} else {
		// TODO validate params
		os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
		os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
		os.Setenv("AWS_DEFAULT_REGION", defaultRegion)
	}

	return &Producer{
		kinesis:           kinesis.New(sess, awsConfig),
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
		// TODO Handle header
		Data:         entry.Payload(),
		StreamName:   stream,
		PartitionKey: aws.String(entry.Vin),
	}
	kinesisRecordOutput, err := p.kinesis.PutRecord(kinesisRecord)

	if err != nil {
		p.logError(err)
		return
	}

	// TODO REMOVE THIS
	p.logger.Printf("record is %v", *kinesisRecordOutput.ShardId)

	if p.prometheusEnabled {
		metrics.StatsIncrement(p.statsCollector, "kinesis_publish_total", 1, map[string]string{"record_type": entry.TxType})
		metrics.StatsIncrement(p.statsCollector, "kinesis_publish_total_bytes", int64(entry.Length()), map[string]string{"record_type": entry.TxType})
	} else {
		metrics.StatsIncrement(p.statsCollector, entry.TxType+"_produce", 1, map[string]string{})
	}

}

func (p *Producer) logError(err error) {
	p.logger.Errorf("kinesis_err err: %v", err)
	metrics.StatsIncrement(p.statsCollector, "kinesis_err", 1, map[string]string{})
}
