package integration_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/protos"
)

const (
	vehicleName       = "My Test Vehicle"
	projectID         = "test-project-id"
	subcriptionID     = "sub-id-1"
	kafkaGroup        = "test-kafka-consumer"
	kafkaBroker       = "kafka:9092"
	pubsubHost        = "pubsub:8085"
	kinesisHost       = "http://kinesis:4567"
	kinesisStreamName = "test_V"
)

func setEnv(key string, value string) {
	err := os.Setenv(key, value)
	Expect(err).To(BeNil())
}

var _ = Describe("Test messages", Ordered, func() {
	var (
		vehicleTopic    = "tesla_telemetry_V"
		payload         []byte
		connection      *websocket.Conn
		pubsubConsumer  *TestConsumer
		kinesisConsumer *TestKinesisConsumer
		kafkaConsumer   *kafka.Consumer
		tlsConfig       *tls.Config
		timestamp       *timestamppb.Timestamp
		logger          *logrus.Logger
	)

	BeforeAll(func() {
		logger = logrus.New()
		var err error
		tlsConfig, err = GetTLSConfig()
		Expect(err).To(BeNil())
		timestamp = timestamppb.Now()

		payload = GenerateVehicleMessage(vehicleName, timestamp)
		connection = CreateWebSocket(tlsConfig)

		kinesisConsumer, err = NewTestKinesisConsumer(kinesisHost, kinesisStreamName)
		Expect(err).To(BeNil())

		setEnv("PUBSUB_EMULATOR_HOST", pubsubHost)
		pubsubConsumer, err = NewTestPubsubConsumer(projectID, vehicleTopic, subcriptionID, logger)
		Expect(err).To(BeNil())

		kafkaConsumer, err = kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers": kafkaBroker,
			"group.id":          kafkaGroup,
			"auto.offset.reset": "earliest",
		})
		Expect(err).To(BeNil())
	})

	AfterAll(func() {
		_ = kafkaConsumer.Close()
		pubsubConsumer.ClearSubscriptions()
		_ = connection.Close()
		os.Clearenv()
	})

	It("reads vehicle data from consumer", func() {
		err := kafkaConsumer.Subscribe(vehicleTopic, nil)
		Expect(err).To(BeNil())
		err = connection.WriteMessage(websocket.BinaryMessage, GenerateVehicleMessage(vehicleName, timestamp))
		Expect(err).To(BeNil())
		msg, err := kafkaConsumer.ReadMessage(10 * time.Second)
		Expect(err).To(BeNil())

		Expect(msg).NotTo(BeNil())
		Expect(*msg.TopicPartition.Topic).To(Equal(vehicleTopic))
		Expect(string(msg.Key)).To(Equal(deviceID))

		headers := make(map[string]string)
		for _, h := range msg.Headers {
			headers[string(h.Key)] = string(h.Value)
		}
		VerifyMessageHeaders(headers)
		VerifyMessageBody(msg.Value, vehicleName)
	})

	It("returns 200 for mtls status", func() {
		body, err := VerifyHTTPSRequest(serviceURL, "status", tlsConfig)
		Expect(err).To(BeNil())
		Expect(string(body)).To(Equal("mtls ok"))
	})

	It("returns 200 for status", func() {
		body, err := VerifyHTTPRequest(statusURL, "status")
		Expect(err).To(BeNil())
		Expect(string(body)).To(Equal("ok"))
	})

	It("returns 200 for gc stats", func() {
		_, err := VerifyHTTPSRequest(serviceURL, "gc_stats", tlsConfig)
		Expect(err).To(BeNil())
	})

	It("returns 200 for prom metrics", func() {
		_, err := VerifyHTTPRequest(prometheusURL, "metrics")
		Expect(err).To(BeNil())
	})

	It("reads vehicle data from google subscriber", func() {
		err := connection.WriteMessage(websocket.BinaryMessage, payload)
		Expect(err).To(BeNil())

		var msg *pubsub.Message
		Eventually(func() error {
			msg, err = pubsubConsumer.FetchPubsubMessage()
			return err
		}, time.Second*2, time.Millisecond*100).Should(BeNil())
		Expect(msg).NotTo(BeNil())
		VerifyMessageHeaders(msg.Attributes)
		VerifyMessageBody(msg.Data, vehicleName)
	})

	It("reads vehicle data from aws kinesis", func() {
		var err error
		// We found publishing a few records makes this test consistent, and
		// no obvious way to enforce delivery with a single message
		for i := 1; i <= 4; i++ {
			err = connection.WriteMessage(websocket.BinaryMessage, payload)
			Expect(err).To(BeNil())
		}

		var record *kinesis.Record
		Eventually(func() error {
			record, err = kinesisConsumer.FetchFirstStreamMessage(kinesisStreamName)
			return err
		}, time.Second*5, time.Millisecond*100).Should(BeNil())
		VerifyMessageBody(record.Data, vehicleName)
	})
})

// VerifyHTTPSRequest validates API returns 200 status code
func VerifyHTTPSRequest(url string, path string, tlsConfig *tls.Config) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	res, err := client.Get(fmt.Sprintf("https://%s/%s", url, path))
	if err != nil {
		return nil, err
	}
	Expect(res.StatusCode).To(Equal(200))
	return io.ReadAll(res.Body)
}

// VerifyHTTPRequest validates API returns 200 status code
func VerifyHTTPRequest(url string, path string) ([]byte, error) {
	res, err := http.Get(fmt.Sprintf("http://%s/%s", url, path))
	if err != nil {
		return nil, err
	}

	Expect(res.StatusCode).To(Equal(200))

	// nolint:errcheck
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

// VerifyMessageHeaders validates headers returned from kafka/pubsub
func VerifyMessageHeaders(headers map[string]string) {
	Expect(headers["txid"]).To(Equal("integration-test-txid"))
	Expect(headers["txtype"]).To(Equal("V"))
	Expect(headers["vin"]).To(Equal(deviceID))
}

// VerifyMessageHeaders validates record message returned from kafka/pubsub
func VerifyMessageBody(body []byte, vehicleName string) {
	payload := &protos.Payload{}
	err := proto.Unmarshal(body, payload)
	Expect(err).To(BeNil())
	Expect(payload.GetVin()).To(Equal(deviceID))
	Expect(len(payload.GetData())).To(Equal(1))
	datum := payload.GetData()[0]
	Expect(datum.GetKey()).To(Equal(protos.Field_VehicleName))
	Expect(datum.GetValue().GetStringValue()).To(Equal(vehicleName))
}
