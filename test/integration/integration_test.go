package integration_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/protos"
)

const (
	vehicleName   = "My Test Vehicle"
	projectID     = "test-project-id"
	subcriptionID = "sub-id-1"
	kafkaGroup    = "test-kafka-consumer"
	kafkaBroker   = "kafka:9092"
	pubsubHost    = "pubsub:8085"
)

var _ = Describe("Test vital payload messages", Ordered, func() {

	var (
		payload        []byte
		connection     *websocket.Conn
		vehicleTopic   string
		pubsubConsumer *TestConsumer
		kafkaConsumer  *kafka.Consumer
		tlsConfig      *tls.Config
		timestamp      *timestamppb.Timestamp
		logger         *logrus.Logger
	)

	BeforeAll(func() {
		logger = logrus.New()
		var err error
		tlsConfig, err = GetTLSConfig()
		Expect(err).To(BeNil())
		timestamp = timestamppb.Now()

		payload = GenerateVehicleMessage(vehicleName, timestamp)
		connection = CreateWebSocket(tlsConfig)
		vehicleTopic = "tesla_telemetry_V"
		err = os.Setenv("PUBSUB_EMULATOR_HOST", pubsubHost)
		Expect(err).To(BeNil())

		pubsubConsumer, err = NewTestConsumer(projectID, vehicleTopic, subcriptionID, logger)
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
		VerifyMessageBody(msg.Value, vehicleName, timestamp)
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
		time.Sleep(2 * time.Second)
		msg := pubsubConsumer.FetchPubsubMessage()
		Expect(msg).NotTo(BeNil())
		VerifyMessageHeaders(msg.Attributes)
		VerifyMessageBody(msg.Data, vehicleName, timestamp)
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
func VerifyMessageBody(body []byte, vehicleName string, timestamp *timestamppb.Timestamp) {
	payload := &protos.Payload{}
	err := proto.Unmarshal(body, payload)
	Expect(err).To(BeNil())
	Expect(payload.GetVin()).To(Equal(deviceID))
	Expect(len(payload.GetData())).To(Equal(1))
	datum := payload.GetData()[0]
	Expect(datum.GetKey()).To(Equal(protos.Field_VehicleName))
	Expect(datum.GetValue().GetStringValue()).To(Equal(vehicleName))
}
