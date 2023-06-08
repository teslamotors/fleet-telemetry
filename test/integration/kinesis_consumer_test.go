package integration_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

var (
	txTypes = []string{"alerts", "errors", "V"}
)

type TestKinesisConsumer struct {
	kineses   *kinesis.Kinesis
	namespace string
}

func NewTestKinesisConsumer(namespace string) (*TestKinesisConsumer, error) {

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

	awsConfig := aws.NewConfig().WithRegion("us-west-2").WithEndpoint(kinesisHost).WithCredentialsChainVerboseErrors(true)

	t := &TestKinesisConsumer{
		kineses:   kinesis.New(sess, awsConfig),
		namespace: namespace,
	}
	for _, txType := range txTypes {
		err := t.createStreamIfNotExists(txType)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *TestKinesisConsumer) createStreamIfNotExists(txType string) error {
	stream := aws.String(fmt.Sprintf("%s_%s", t.namespace, txType))
	response, err := t.kineses.ListStreams(&kinesis.ListStreamsInput{
		Limit: aws.Int64(100),
	})
	if err != nil {
		return err
	}

	if len(response.StreamNames) == 0 {
		_, err := t.kineses.CreateStream(&kinesis.CreateStreamInput{
			StreamName: stream,
			ShardCount: aws.Int64(1),
		})
		if err != nil {
			return err
		}
		return nil
	}

	for _, streamName := range response.StreamNames {
		if strings.Compare(*streamName, *stream) == 0 {
			return nil
		}
	}
	return fmt.Errorf("unable to create stream %v", *stream)
}

func (t *TestKinesisConsumer) FetchStreamMessage(topic string) (*kinesis.Record, error) {
	stream := aws.String(topic)
	describeInput := &kinesis.DescribeStreamInput{
		StreamName: stream,
	}

	describeOutput, err := t.kineses.DescribeStream(describeInput)
	if err != nil {
		return nil, err
	}

	shardID := describeOutput.StreamDescription.Shards[0].ShardId

	getIteratorInput := &kinesis.GetShardIteratorInput{
		StreamName:        stream,
		ShardId:           aws.String(*shardID),
		ShardIteratorType: aws.String("LATEST"),
	}

	getIteratorOutput, err := t.kineses.GetShardIterator(getIteratorInput)
	if err != nil {
		return nil, err
	}

	shardIterator := getIteratorOutput.ShardIterator

	for {
		getRecordsInput := &kinesis.GetRecordsInput{
			ShardIterator: shardIterator,
		}

		getRecordsOutput, err := t.kineses.GetRecords(getRecordsInput)
		if err != nil {
			return nil, err
		}
		records := getRecordsOutput.Records
		if len(records) == 0 {
			return nil, errors.New("empty records")
		}
		return records[0], nil

	}
}
