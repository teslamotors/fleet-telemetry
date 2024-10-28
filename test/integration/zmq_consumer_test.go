package integration_test

import (
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/pebbe/zmq4"
)

type TestZMQConsumer struct {
	socks map[string]*zmq4.Socket
}

func NewTestZMQConsumer(addr string, topics []string) (*TestZMQConsumer, error) {
	socks := make(map[string]*zmq4.Socket)
	for _, topicID := range topics {
		sock, err := zmq4.NewSocket(zmq4.SUB)
		if err != nil {
			return nil, err
		}

		if err := sock.SetSubscribe(topicID); err != nil {
			return nil, err
		}

		if err := sock.Connect(addr); err != nil {
			return nil, err
		}
		socks[topicID] = sock
	}

	return &TestZMQConsumer{socks: socks}, nil
}

func (t *TestZMQConsumer) Close() {
	for _, sock := range t.socks {
		Expect(sock.Close()).NotTo(HaveOccurred())
	}
}

type malformedZMQMessage struct{}

func (malformedZMQMessage) Error() string {
	return "Malformed message"
}

var ErrMalformedZMQMessage malformedZMQMessage

func (t *TestZMQConsumer) NextMessage(topicId string) (topic string, data []byte, err error) {
	sock, ok := t.socks[topicId]
	if !ok {
		return "", nil, fmt.Errorf("no consumer for %s", topicId)
	}
	messages, err := sock.RecvMessageBytes(0)
	if err != nil {
		return "", nil, err
	}

	if len(messages) != 2 {
		return "", nil, ErrMalformedZMQMessage
	}

	return string(messages[0]), messages[1], nil
}
