package integration_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"cloud.google.com/go/pubsub"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gorilla/websocket"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/protos"
)

const (
	deviceID    = "device-1"
	vehicleName = "My Test Vehicle"
	location    = "(37.412374 S, 122.145867 E)"
	projectID   = "test-project-id"
	kafkaGroup  = "test-kafka-consumer"
	kafkaBroker = "kafka:9092"
	pubsubHost  = "pubsub:8085"
	zmqAddr     = "tcp://app:5284"
	mqttBroker  = "mqtt:1883"
	mqttTopic   = "telemetry/device-1/v/VehicleName"
	kinesisHost = "http://kinesis:4566"

	kinesisStreamName             = "test_V"
	kinesisConnectivityStreamName = "test_connectivity"
)

var expectedLocation = &protos.LocationValue{Latitude: -37.412374, Longitude: 122.145867}

func setEnv(key string, value string) {
	err := os.Setenv(key, value)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("Test messages", Ordered, func() {
	var (
		vehicleTopic             = "tesla_telemetry_V"
		vehicleConnectivityTopic = "tesla_telemetry_connectivity"

		payload         []byte
		connection      *websocket.Conn
		pubsubConsumer  *TestConsumer
		kinesisConsumer *TestKinesisConsumer
		kafkaConsumer   *kafka.Consumer
		zmqConsumer     *TestZMQConsumer
		mqttConsumer    *TestMQTTConsumer
		tlsConfig       *tls.Config
		timestamp       *timestamppb.Timestamp
		logger          *logrus.Logger
	)

	BeforeAll(func() {
		var err error
		logger, err = logrus.NewBasicLogrusLogger("fleet-telemetry-integration-test")
		Expect(err).NotTo(HaveOccurred())
		tlsConfig, err = GetTLSConfig()
		Expect(err).NotTo(HaveOccurred())
		timestamp = timestamppb.Now()

		payload = GenerateVehicleMessage(deviceID, vehicleName, location, timestamp)

		kinesisConsumer, err = NewTestKinesisConsumer(kinesisHost, []string{kinesisStreamName, kinesisConnectivityStreamName})
		Expect(err).NotTo(HaveOccurred())

		setEnv("PUBSUB_EMULATOR_HOST", pubsubHost)
		pubsubConsumer, err = NewTestPubsubConsumer(projectID, []string{vehicleTopic, vehicleConnectivityTopic}, logger)
		Expect(err).NotTo(HaveOccurred())

		kafkaConsumer, err = kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers": kafkaBroker,
			"group.id":          kafkaGroup,
			"auto.offset.reset": "earliest",
		})
		Expect(err).NotTo(HaveOccurred())

		zmqConsumer, err = NewTestZMQConsumer(zmqAddr, []string{vehicleTopic, vehicleConnectivityTopic})
		Expect(err).NotTo(HaveOccurred())

		mqttConsumer, err = NewTestMQTTConsumer(mqttBroker, mqttTopic, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		connection = CreateWebSocket(tlsConfig)
	})

	AfterEach(func() {
		Expect(connection.Close()).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		_ = kafkaConsumer.Close()
		pubsubConsumer.ClearSubscriptions()
		_ = connection.Close()
		zmqConsumer.Close()
		os.Clearenv()
	})

	Describe("v records", Ordered, func() {

		It("reads vehicle data from kafka consumer", func() {
			defer GinkgoRecover()
			err := kafkaConsumer.Subscribe(vehicleTopic, nil)
			Expect(err).NotTo(HaveOccurred())
			err = connection.WriteMessage(websocket.BinaryMessage, GenerateVehicleMessage(deviceID, vehicleName, location, timestamp))
			verifyAckMessage(connection, "V")
			Expect(err).NotTo(HaveOccurred())
			msg, err := kafkaConsumer.ReadMessage(10 * time.Second)
			Expect(err).NotTo(HaveOccurred())

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

		It("does not allow spoofing", func() {
			defer GinkgoRecover()
			err := kafkaConsumer.Subscribe(vehicleTopic, nil)
			Expect(err).NotTo(HaveOccurred())

			spoofedID := "BAD123"
			vehicleMessage := GenerateVehicleMessage(spoofedID, vehicleName, location, timestamp)
			err = connection.WriteMessage(websocket.BinaryMessage, vehicleMessage)
			verifyAckMessage(connection, "V")
			Expect(err).NotTo(HaveOccurred())
			msg, err := kafkaConsumer.ReadMessage(10 * time.Second)
			Expect(err).NotTo(HaveOccurred())

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

		It("reads vehicle data from google subscriber", func() {
			err := connection.WriteMessage(websocket.BinaryMessage, payload)
			Expect(err).NotTo(HaveOccurred())

			var msg *pubsub.Message
			Eventually(func() error {
				msg, err = pubsubConsumer.FetchPubsubMessage(vehicleTopic)
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
				Expect(err).NotTo(HaveOccurred())
			}

			var record *kinesis.Record
			Eventually(func() error {
				record, err = kinesisConsumer.FetchFirstStreamMessage(kinesisStreamName)
				return err
			}, time.Second*5, time.Millisecond*100).Should(BeNil())
			VerifyMessageBody(record.Data, vehicleName)
		})

		It("reads vehicle data from MQTT broker", func() {
			err := connection.WriteMessage(websocket.BinaryMessage, payload)
			Expect(err).NotTo(HaveOccurred())

			var msg []byte
			Eventually(func() error {
				msg, err = mqttConsumer.FetchMQTTMessage()
				return err
			}, time.Second*5, time.Millisecond*100).Should(BeNil())
			Expect(msg).NotTo(BeNil())

			// Parse the JSON message
			var jsonMsg interface{}
			err = json.Unmarshal(msg, &jsonMsg)
			Expect(err).NotTo(HaveOccurred())

			// The json message should directly contain the vehicle name
			Expect(jsonMsg).To(Equal(vehicleName), "Vehicle name %s not found in MQTT message", vehicleName)
		})

		It("reads data from zmq subscriber", func() {
			err := connection.WriteMessage(websocket.BinaryMessage, payload)
			Expect(err).NotTo(HaveOccurred())

			topic, data, err := zmqConsumer.NextMessage(vehicleTopic)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).NotTo(BeNil())
			Expect(topic).To(Equal(vehicleTopic))
			VerifyMessageBody(data, vehicleName)
		})
	})

	Describe("connectivity records", Ordered, func() {

		It("reads vehicle data from kafka consumer", func() {
			defer GinkgoRecover()
			err := kafkaConsumer.Subscribe(vehicleConnectivityTopic, nil)
			Expect(err).NotTo(HaveOccurred())
			msg, err := kafkaConsumer.ReadMessage(10 * time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(msg).NotTo(BeNil())
			Expect(*msg.TopicPartition.Topic).To(Equal(vehicleConnectivityTopic))
			Expect(string(msg.Key)).To(Equal(deviceID))

			headers := make(map[string]string)
			for _, h := range msg.Headers {
				headers[string(h.Key)] = string(h.Value)
			}
			VerifyConnectivityMessageHeaders(headers)
			VerifyConnectivityMessageBody(msg.Value)
		})

		It("reads vehicle data from google subscriber", func() {
			var err error
			var msg *pubsub.Message
			Eventually(func() error {
				msg, err = pubsubConsumer.FetchPubsubMessage(vehicleConnectivityTopic)
				return err
			}, time.Second*2, time.Millisecond*100).Should(BeNil())
			Expect(msg).NotTo(BeNil())
			VerifyConnectivityMessageHeaders(msg.Attributes)
			VerifyConnectivityMessageBody(msg.Data)
		})

		It("reads vehicle data from aws kinesis", func() {
			var err error
			var record *kinesis.Record
			Eventually(func() error {
				record, err = kinesisConsumer.FetchFirstStreamMessage(kinesisConnectivityStreamName)
				return err
			}, time.Second*5, time.Millisecond*100).Should(BeNil())
			VerifyConnectivityMessageBody(record.Data)
		})

		It("reads data from zmq subscriber", func() {
			err := connection.WriteMessage(websocket.BinaryMessage, payload)
			Expect(err).NotTo(HaveOccurred())

			topic, data, err := zmqConsumer.NextMessage(vehicleConnectivityTopic)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).NotTo(BeNil())
			Expect(topic).To(Equal(vehicleConnectivityTopic))
			VerifyConnectivityMessageBody(data)
		})

	})

	Describe("health checks", Ordered, func() {

		It("returns 200 for mtls status", func() {
			body, err := VerifyHTTPSRequest(serviceURL, "status", tlsConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("mtls ok"))
		})

		It("returns 200 for status", func() {
			body, err := VerifyHTTPRequest(statusURL, "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("ok"))
		})

		It("returns 200 for prom metrics", func() {
			_, err := VerifyHTTPRequest(prometheusURL, "metrics")
			Expect(err).NotTo(HaveOccurred())
		})
	})

})

func verifyAckMessage(connection *websocket.Conn, expectedTxType string) {
	mType, msg, err := connection.ReadMessage()
	Expect(err).NotTo(HaveOccurred())
	Expect(mType).To(Equal(websocket.BinaryMessage))
	streamMessage, err := messages.StreamAckMessageFromBytes(msg)
	Expect(err).NotTo(HaveOccurred())
	Expect(streamMessage.MessageTopic).To(Equal([]byte(expectedTxType)))
	Expect(streamMessage.TXID).To(Equal([]byte("integration-test-txid")))
}

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

// VerifyConnectivityMessageHeaders validates headers returned from dispatchers
func VerifyConnectivityMessageHeaders(headers map[string]string) {
	Expect(headers["txid"]).NotTo(BeEmpty())
	Expect(headers["txtype"]).To(Equal("connectivity"))
	Expect(headers["vin"]).To(Equal(deviceID))
	Expect(headers["device_client_version"]).To(Equal(deviceClientVersion))
}

// VerifyMessageHeaders validates headers returned from dispatchers
func VerifyMessageHeaders(headers map[string]string) {
	Expect(headers["txid"]).To(Equal("integration-test-txid"))
	Expect(headers["txtype"]).To(Equal("V"))
	Expect(headers["vin"]).To(Equal(deviceID))
	Expect(headers["device_client_version"]).To(Equal(deviceClientVersion))
}

// VerifyConnectivityMessageBody validates record message returned from dispatchers
func VerifyConnectivityMessageBody(body []byte) {
	payload := &protos.VehicleConnectivity{}
	err := proto.Unmarshal(body, payload)
	Expect(err).NotTo(HaveOccurred())
	Expect(payload.GetVin()).To(Equal(deviceID))
	Expect(payload.GetNetworkInterface()).To(Equal("wifi"))
	Expect(payload.GetConnectionId()).NotTo(BeEmpty())
	Expect(payload.GetCreatedAt().AsTime().Unix()).NotTo(BeEquivalentTo(0))
	Expect(payload.GetStatus()).To(Or(Equal(protos.ConnectivityEvent_CONNECTED), Equal(protos.ConnectivityEvent_DISCONNECTED)))
}

// VerifyMessageBody validates record message returned from dispatchers
func VerifyMessageBody(body []byte, vehicleName string) {
	payload := &protos.Payload{}
	err := proto.Unmarshal(body, payload)
	Expect(err).NotTo(HaveOccurred())
	Expect(payload.GetVin()).To(Equal(deviceID))
	data := payload.GetData()
	Expect(data).To(HaveLen(2))

	// Make the test more predictable
	sort.Slice(data, func(i, j int) bool {
		return data[i].Key < data[j].Key
	})

	datum := data[0]
	Expect(datum.GetKey()).To(Equal(protos.Field_Location))
	Expect(datum.GetValue().GetLocationValue()).To(Equal(expectedLocation))

	datum = data[1]
	Expect(datum.GetKey()).To(Equal(protos.Field_VehicleName))
	Expect(datum.GetValue().GetStringValue()).To(Equal(vehicleName))
}
