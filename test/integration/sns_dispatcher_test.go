package integration_test

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("SNS dispatcher integration", func() {
	BeforeEach(configureLocalAWSEnvironment)

	It("delivers to a subscribed queue and only acknowledges accepted publications", func() {
		sess := localAWSSession()
		topic := createDispatcherTopic(sns.New(sess))
		queue := createDispatcherQueue(sqs.New(sess))
		topic.subscribe(queue)
		conn, hook := startAWSDispatcherSocket(&config.Config{
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.SNS}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SNS},
			SNS: &config.SNS{
				OverrideHost: kinesisHost, Topics: map[string]string{"V": aws.StringValue(topic.arn)},
				MaxRetries: aws.Int(0), PublishTimeoutMS: aws.Int(2000),
			},
		})
		payload := GenerateVehicleMessage(deviceID, vehicleName, location, timestamppb.Now())

		// SNS accepts the publication; the subscription receives its raw payload and attributes.
		Expect(conn.WriteMessage(websocket.BinaryMessage, payload)).To(Succeed())
		expectVehicleAck(conn)
		expectVehicleQueueMessage(queue.receive())

		// A later publication to the deleted topic must fail without a successful ACK.
		topic.delete()
		hook.Reset()
		Expect(conn.WriteMessage(websocket.BinaryMessage, payload)).To(Succeed())
		expectPublishFailureWithoutAck(conn, hook, "sns_err")
	})
})

type dispatcherTestTopic struct {
	client  *sns.SNS
	arn     *string
	deleted bool
}

func createDispatcherTopic(client *sns.SNS) *dispatcherTestTopic {
	GinkgoHelper()
	out, err := client.CreateTopic(&sns.CreateTopicInput{Name: aws.String("fleet-telemetry-" + uuid.NewString())})
	Expect(err).NotTo(HaveOccurred())
	topic := &dispatcherTestTopic{client: client, arn: out.TopicArn}
	DeferCleanup(topic.delete)
	return topic
}

func (topic *dispatcherTestTopic) delete() {
	GinkgoHelper()
	if topic.deleted {
		return
	}
	_, err := topic.client.DeleteTopic(&sns.DeleteTopicInput{TopicArn: topic.arn})
	Expect(err).NotTo(HaveOccurred())
	topic.deleted = true
}

func (topic *dispatcherTestTopic) subscribe(queue *dispatcherTestQueue) {
	GinkgoHelper()
	attrs, err := queue.client.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl: queue.url, AttributeNames: aws.StringSlice([]string{"QueueArn"}),
	})
	Expect(err).NotTo(HaveOccurred())
	queueARN := aws.StringValue(attrs.Attributes["QueueArn"])
	Expect(queueARN).NotTo(BeEmpty())

	// Only this SNS topic may send to the test queue.
	policy, err := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []interface{}{map[string]interface{}{
			"Effect": "Allow", "Principal": map[string]string{"Service": "sns.amazonaws.com"},
			"Action": "sqs:SendMessage", "Resource": queueARN,
			"Condition": map[string]interface{}{"ArnEquals": map[string]string{"aws:SourceArn": aws.StringValue(topic.arn)}},
		}},
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = queue.client.SetQueueAttributes(&sqs.SetQueueAttributesInput{
		QueueUrl: queue.url, Attributes: map[string]*string{"Policy": aws.String(string(policy))},
	})
	Expect(err).NotTo(HaveOccurred())
	_, err = topic.client.Subscribe(&sns.SubscribeInput{
		TopicArn: topic.arn, Protocol: aws.String("sqs"), Endpoint: aws.String(queueARN),
		Attributes: map[string]*string{"RawMessageDelivery": aws.String("true")},
	})
	Expect(err).NotTo(HaveOccurred())
}
