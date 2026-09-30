package integration_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
)

// TestSNSConsumer creates the topics the server publishes to and wires each one to an SQS
// queue, which is the standard way to observe SNS deliveries in a test.
type TestSNSConsumer struct {
	sns       *sns.SNS
	topicARNs map[string]string
}

// NewTestSNSConsumer creates each topic if needed and caches its ARN.
func NewTestSNSConsumer(host string, topicNames []string) (*TestSNSConsumer, error) {
	creds := credentials.NewStaticCredentials(fakeAWSID, fakeAWSSecret, fakeAWSToken)
	awsConfig := aws.NewConfig().WithEndpoint(host).WithCredentialsChainVerboseErrors(true).WithRegion(fakeAWSRegion).WithCredentials(creds)
	sess, err := session.NewSessionWithOptions(session.Options{Config: *awsConfig})
	if err != nil {
		return nil, err
	}

	t := &TestSNSConsumer{sns: sns.New(sess, awsConfig), topicARNs: map[string]string{}}
	for _, topicName := range topicNames {
		out, err := t.sns.CreateTopic(&sns.CreateTopicInput{Name: aws.String(topicName)})
		if err != nil {
			return nil, err
		}
		t.topicARNs[topicName] = aws.StringValue(out.TopicArn)
	}
	return t, nil
}

// TopicARN returns the ARN of a topic created by this consumer.
func (t *TestSNSConsumer) TopicARN(topicName string) string {
	return t.topicARNs[topicName]
}

// SubscribeQueue points a topic at an SQS queue. RawMessageDelivery keeps the body as the
// published message rather than wrapping it in the SNS JSON envelope, so the consumer can
// base64 decode it the same way an SQS subscriber would.
func (t *TestSNSConsumer) SubscribeQueue(topicName, queueARN string) error {
	_, err := t.sns.Subscribe(&sns.SubscribeInput{
		TopicArn:   aws.String(t.topicARNs[topicName]),
		Protocol:   aws.String("sqs"),
		Endpoint:   aws.String(queueARN),
		Attributes: map[string]*string{"RawMessageDelivery": aws.String("true")},
	})
	return err
}
