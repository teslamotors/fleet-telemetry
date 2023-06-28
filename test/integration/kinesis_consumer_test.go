package integration_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

var (
	// The current integration test docker image only supports 1 stream saidsef/aws-kinesis-local
	fakeAWSID     = "id"
	fakeAWSSecret = "secret"
	fakeAWSToken  = "token"
	fakeAWSRegion = "us-west-2"
)

type TestKinesisConsumer struct {
	kineses *kinesis.Kinesis
}

func NewTestKinesisConsumer(host, streamName string) (*TestKinesisConsumer, error) {
	creds := credentials.NewStaticCredentials(fakeAWSID, fakeAWSSecret, fakeAWSToken)
	awsConfig := aws.NewConfig().WithEndpoint(host).WithCredentialsChainVerboseErrors(true).WithRegion(fakeAWSRegion).WithCredentials(creds)
	sess, err := session.NewSessionWithOptions(session.Options{Config: *awsConfig})
	if err != nil {
		return nil, err
	}

	t := &TestKinesisConsumer{
		kineses: kinesis.New(sess, awsConfig),
	}
	if err = t.createStreamIfNotExists(streamName); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TestKinesisConsumer) createStreamIfNotExists(streamName string) error {
	response, err := t.kineses.ListStreams(&kinesis.ListStreamsInput{
		Limit: aws.Int64(100),
	})
	if err != nil {
		return err
	}

	if len(response.StreamNames) == 0 {
		_, err := t.kineses.CreateStream(&kinesis.CreateStreamInput{
			StreamName: aws.String(streamName),
			ShardCount: aws.Int64(1),
		})
		if err != nil {
			return err
		}
		return nil
	}

	for _, streamName := range response.StreamNames {
		if strings.Compare(*streamName, *streamName) == 0 {
			return nil
		}
	}
	return fmt.Errorf("unable to create stream %s", streamName)
}

func (t *TestKinesisConsumer) FetchFirstStreamMessage(topic string) (*kinesis.Record, error) {
	stream := aws.String(topic)
	describeInput := &kinesis.DescribeStreamInput{
		StreamName: stream,
	}

	describeOutput, err := t.kineses.DescribeStream(describeInput)
	if err != nil {
		return nil, err
	}
	if len(describeOutput.StreamDescription.Shards) == 0 {
		return nil, errors.New("empty shards")
	}

	shardID := describeOutput.StreamDescription.Shards[0].ShardId
	getIteratorInput := &kinesis.GetShardIteratorInput{
		StreamName:        stream,
		ShardId:           shardID,
		ShardIteratorType: aws.String("TRIM_HORIZON"),
	}

	getIteratorOutput, err := t.kineses.GetShardIterator(getIteratorInput)
	if err != nil {
		return nil, err
	}

	shardIterator := getIteratorOutput.ShardIterator
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
