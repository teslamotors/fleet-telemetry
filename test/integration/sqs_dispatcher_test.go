package integration_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("SQS dispatcher integration", func() {
	BeforeEach(configureLocalAWSEnvironment)

	It("delivers vehicle payloads and only acknowledges accepted messages", func() {
		queue := createDispatcherQueue(sqs.New(localAWSSession()))
		conn, hook := startAWSDispatcherSocket(&config.Config{
			Records:            map[string][]telemetry.Dispatcher{"V": {telemetry.SQS}},
			ReliableAckSources: map[string]telemetry.Dispatcher{"V": telemetry.SQS},
			SQS: &config.SQS{
				OverrideHost: kinesisHost, Queues: map[string]string{"V": queue.name},
				MaxRetries: aws.Int(0), PublishTimeoutMS: aws.Int(2000),
			},
		})
		payload := GenerateVehicleMessage(deviceID, vehicleName, location, timestamppb.Now())

		// Successful delivery also primes the producer's queue URL cache.
		Expect(conn.WriteMessage(websocket.BinaryMessage, payload)).To(Succeed())
		expectVehicleAck(conn)
		expectVehicleQueueMessage(queue.receive())

		// Delete the cached destination, then verify the real send failure does not ACK.
		queue.delete()
		hook.Reset()
		Expect(conn.WriteMessage(websocket.BinaryMessage, payload)).To(Succeed())
		expectPublishFailureWithoutAck(conn, hook, "sqs_err")
	})
})
