package integration_test

import (
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
)

type TestMQTTConsumer struct {
	client  pahomqtt.Client
	topic   string
	logger  *logrus.Logger
	msgChan chan []byte
}

func NewTestMQTTConsumer(broker, topic string, logger *logrus.Logger) (*TestMQTTConsumer, error) {
	opts := pahomqtt.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s", broker))
	opts.SetClientID("test-mqtt-consumer")

	client := pahomqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	msgChan := make(chan []byte, 10)

	consumer := &TestMQTTConsumer{
		client:  client,
		topic:   topic,
		logger:  logger,
		msgChan: msgChan,
	}

	if token := client.Subscribe(topic, 0, consumer.messageHandler); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return consumer, nil
}

func (t *TestMQTTConsumer) messageHandler(_ pahomqtt.Client, msg pahomqtt.Message) {
	t.msgChan <- msg.Payload()
}

func (t *TestMQTTConsumer) FetchMQTTMessage() ([]byte, error) {
	select {
	case msg := <-t.msgChan:
		return msg, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for MQTT message")
	}
}

func (t *TestMQTTConsumer) Close() {
	t.client.Unsubscribe(t.topic)
	t.client.Disconnect(250)
}
