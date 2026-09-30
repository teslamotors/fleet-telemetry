package integration_test

import (
	"encoding/base64"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// TestSQSConsumer creates the queues the server dispatches to and reads them back.
type TestSQSConsumer struct {
	sqs       *sqs.SQS
	queueURLs map[string]string
}

// NewTestSQSConsumer creates each queue if needed and caches its URL.
func NewTestSQSConsumer(host string, queueNames []string) (*TestSQSConsumer, error) {
	creds := credentials.NewStaticCredentials(fakeAWSID, fakeAWSSecret, fakeAWSToken)
	awsConfig := aws.NewConfig().WithEndpoint(host).WithCredentialsChainVerboseErrors(true).WithRegion(fakeAWSRegion).WithCredentials(creds)
	sess, err := session.NewSessionWithOptions(session.Options{Config: *awsConfig})
	if err != nil {
		return nil, err
	}

	t := &TestSQSConsumer{sqs: sqs.New(sess, awsConfig), queueURLs: map[string]string{}}
	for _, queueName := range queueNames {
		url, err := t.createQueueIfNotExists(queueName)
		if err != nil {
			return nil, err
		}
		t.queueURLs[queueName] = url
	}
	return t, nil
}

func (t *TestSQSConsumer) createQueueIfNotExists(queueName string) (string, error) {
	out, err := t.sqs.CreateQueue(&sqs.CreateQueueInput{QueueName: aws.String(queueName)})
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.QueueUrl), nil
}

// QueueARN returns the ARN of a queue created by this consumer, for SNS subscriptions.
func (t *TestSQSConsumer) QueueARN(queueName string) (string, error) {
	url, ok := t.queueURLs[queueName]
	if !ok {
		return "", errors.New("unknown queue: " + queueName)
	}
	out, err := t.sqs.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(url),
		AttributeNames: []*string{aws.String("QueueArn")},
	})
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.Attributes["QueueArn"]), nil
}

// FetchFirstQueueMessage returns the decoded payload of the next message on the queue.
// The dispatcher base64 encodes payloads because SQS bodies must be valid UTF-8.
func (t *TestSQSConsumer) FetchFirstQueueMessage(queueName string) ([]byte, error) {
	url, ok := t.queueURLs[queueName]
	if !ok {
		return nil, errors.New("unknown queue: " + queueName)
	}

	out, err := t.sqs.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(url),
		MaxNumberOfMessages:   aws.Int64(1),
		WaitTimeSeconds:       aws.Int64(1),
		MessageAttributeNames: []*string{aws.String("All")},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Messages) == 0 {
		return nil, errors.New("empty messages")
	}

	return base64.StdEncoding.DecodeString(aws.StringValue(out.Messages[0].Body))
}
