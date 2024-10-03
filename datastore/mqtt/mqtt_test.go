package mqtt_test

import (
	"context"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/protos"

	"github.com/teslamotors/fleet-telemetry/datastore/mqtt"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MockMQTTClient struct {
	ConnectFunc           func() pahomqtt.Token
	PublishFunc           func(topic string, qos byte, retained bool, payload interface{}) pahomqtt.Token
	DisconnectFunc        func(quiesce uint)
	IsConnectedFunc       func() bool
	IsConnectionOpenFunc  func() bool
	SubscribeFunc         func(topic string, qos byte, callback pahomqtt.MessageHandler) pahomqtt.Token
	SubscribeMultipleFunc func(filters map[string]byte, callback pahomqtt.MessageHandler) pahomqtt.Token
	UnsubscribeFunc       func(topics ...string) pahomqtt.Token
	AddRouteFunc          func(topic string, callback pahomqtt.MessageHandler)
	OptionsReaderFunc     func() pahomqtt.ClientOptionsReader
}

func (m *MockMQTTClient) Connect() pahomqtt.Token {
	return m.ConnectFunc()
}

func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) pahomqtt.Token {
	return m.PublishFunc(topic, qos, retained, payload)
}

func (m *MockMQTTClient) Disconnect(quiesce uint) {
	m.DisconnectFunc(quiesce)
}

func (m *MockMQTTClient) IsConnected() bool {
	return m.IsConnectedFunc()
}

func (m *MockMQTTClient) IsConnectionOpen() bool {
	return m.IsConnectionOpenFunc()
}

func (m *MockMQTTClient) Subscribe(topic string, qos byte, callback pahomqtt.MessageHandler) pahomqtt.Token {
	return m.SubscribeFunc(topic, qos, callback)
}

func (m *MockMQTTClient) SubscribeMultiple(filters map[string]byte, callback pahomqtt.MessageHandler) pahomqtt.Token {
	return m.SubscribeMultipleFunc(filters, callback)
}

func (m *MockMQTTClient) Unsubscribe(topics ...string) pahomqtt.Token {
	return m.UnsubscribeFunc(topics...)
}

func (m *MockMQTTClient) AddRoute(topic string, callback pahomqtt.MessageHandler) {
	m.AddRouteFunc(topic, callback)
}

func (m *MockMQTTClient) OptionsReader() pahomqtt.ClientOptionsReader {
	return m.OptionsReaderFunc()
}

type MockToken struct {
	WaitFunc        func() bool
	WaitTimeoutFunc func(time.Duration) bool
	DoneFunc        func() <-chan struct{}
	ErrorFunc       func() error
}

func (m *MockToken) Wait() bool {
	return m.WaitFunc()
}

func (m *MockToken) WaitTimeout(d time.Duration) bool {
	return m.WaitTimeoutFunc(d)
}

func (m *MockToken) Done() <-chan struct{} {
	return m.DoneFunc()
}

func (m *MockToken) Error() error {
	return m.ErrorFunc()
}

var publishedTopics = make(map[string][]byte)

func resetPublishedTopics() {
	publishedTopics = make(map[string][]byte)
}

func mockPahoNewClient(o *pahomqtt.ClientOptions) pahomqtt.Client {
	return &MockMQTTClient{

		ConnectFunc: func() pahomqtt.Token {
			return &MockToken{
				WaitFunc:  func() bool { return true },
				ErrorFunc: func() error { return nil },
			}
		},
		IsConnectedFunc: func() bool {
			return true
		},
		PublishFunc: func(topic string, qos byte, retained bool, payload interface{}) pahomqtt.Token {
			publishedTopics[topic] = payload.([]byte)
			return &MockToken{
				WaitTimeoutFunc: func(d time.Duration) bool { return true },
				WaitFunc:        func() bool { return true },
				ErrorFunc:       func() error { return nil },
			}
		},
	}
}

var _ = Describe("MQTTProducer", func() {
	var (
		mockLogger    *logrus.Logger
		mockCollector metrics.MetricCollector
		mockConfig    *mqtt.Config
	)

	BeforeEach(func() {
		mockLogger, _ = logrus.NoOpLogger()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockConfig = &mqtt.Config{
			Broker:         "tcp://localhost:1883",
			ClientID:       "test-client",
			Username:       "testuser",
			Password:       "testpass",
			TopicBase:      "test/topic",
			QoS:            1,
			Retained:       false,
			ConnectTimeout: 30,
		}
	})

	Describe("Produce", func() {
		It("should publish MQTT messages for each field in the payload", func() {
			// Mock pahomqtt.NewClient
			originalNewClient := mqtt.PahoNewClient
			mqtt.PahoNewClient = mockPahoNewClient
			defer func() { mqtt.PahoNewClient = originalNewClient }()

			producer, err := mqtt.NewProducer(
				context.Background(),
				mockConfig,
				mockCollector,
				"test_namespace",
				nil, // airbrakeHandler
				nil, // ackChan
				nil, // reliableAckTxTypes
				mockLogger,
			)
			Expect(err).NotTo(HaveOccurred())

			payload := &protos.Payload{
				Vin: "TEST123",
				Data: []*protos.Datum{
					{
						Key: protos.Field_VehicleName,
						Value: &protos.Value{
							Value: &protos.Value_StringValue{StringValue: "My Tesla"},
						},
					},
					{
						Key: protos.Field_BatteryLevel,
						Value: &protos.Value{
							Value: &protos.Value_FloatValue{FloatValue: 75.5},
						},
					},
				},
				CreatedAt: timestamppb.Now(),
			}

			payloadBytes, err := proto.Marshal(payload)
			Expect(err).NotTo(HaveOccurred())

			record := &telemetry.Record{
				TxType:       "V",
				Vin:          "TEST123",
				PayloadBytes: payloadBytes,
			}

			producer.Produce(record)

			Expect(publishedTopics).To(HaveLen(2))

			vehicleNameTopic := "test/topic/vin/TEST123/VehicleName"
			batteryLevelTopic := "test/topic/vin/TEST123/BatteryLevel"

			vehicleNameValue := "{\"value\":\"My Tesla\"}"
			batterLevelValue := "{\"value\":75.5}"

			Expect(publishedTopics).To(HaveKey(vehicleNameTopic))
			Expect(publishedTopics).To(HaveKey(batteryLevelTopic))
			Expect(publishedTopics[vehicleNameTopic]).To(Equal([]byte(vehicleNameValue)))
			Expect(publishedTopics[batteryLevelTopic]).To(Equal([]byte(batterLevelValue)))
		})
	})

})
