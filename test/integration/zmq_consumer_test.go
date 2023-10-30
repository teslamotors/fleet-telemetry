package integration_test

import (
	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

type TestZMQConsumer struct {
	sock *zmq4.Socket
}

func NewTestZMQConsumer(addr string, subscription string, logger *logrus.Logger) (*TestZMQConsumer, error) {
	sock, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		return nil, err
	}

	if err := sock.SetSubscribe(subscription); err != nil {
		return nil, err
	}

	if err := sock.Connect(addr); err != nil {
		return nil, err
	}

	return &TestZMQConsumer{sock}, nil
}

func (t *TestZMQConsumer) Close() {
	t.sock.Close()
}

type malformedZMQMessage struct{}

func (malformedZMQMessage) Error() string {
	return "Malformed message"
}

var ErrMalformedZMQMessage malformedZMQMessage

func (t *TestZMQConsumer) NextMessage() (topic string, data []byte, err error) {
	messages, err := t.sock.RecvMessageBytes(zmq4.Flag(0))
	if err != nil {
		return "", nil, err
	}

	if len(messages) != 2 {
		return "", nil, ErrMalformedZMQMessage
	}

	topic = string(messages[0])
	data = messages[1]
	return
}
