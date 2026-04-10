package integration_test

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
)

// TestNATSConsumer is a test consumer for NATS messages
type TestNATSConsumer struct {
	conn          *nats.Conn
	subscriptions map[string]*nats.Subscription
	msgChans      map[string]chan []byte
	logger        *logrus.Logger
}

// NewTestNATSConsumer creates a new NATS test consumer
func NewTestNATSConsumer(url string, subjects []string, logger *logrus.Logger) (*TestNATSConsumer, error) {
	conn, err := nats.Connect(url,
		nats.Name("test-nats-consumer"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(5),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	consumer := &TestNATSConsumer{
		conn:          conn,
		subscriptions: make(map[string]*nats.Subscription),
		msgChans:      make(map[string]chan []byte),
		logger:        logger,
	}

	// Subscribe to each subject pattern
	for _, subject := range subjects {
		msgChan := make(chan []byte, 100)
		consumer.msgChans[subject] = msgChan

		// Use wildcard subscription to catch all messages for this subject prefix
		sub, err := conn.Subscribe(subject+".>", func(msg *nats.Msg) {
			select {
			case msgChan <- msg.Data:
			default:
				logger.ActivityLog("nats_test_consumer_buffer_full", logrus.LogInfo{"subject": msg.Subject})
			}
		})
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		consumer.subscriptions[subject] = sub
	}

	return consumer, nil
}

// FetchNATSMessage retrieves a message from the specified subject with timeout
func (c *TestNATSConsumer) FetchNATSMessage(subject string) ([]byte, error) {
	msgChan, ok := c.msgChans[subject]
	if !ok {
		return nil, fmt.Errorf("no subscription for subject: %s", subject)
	}

	select {
	case msg := <-msgChan:
		return msg, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for NATS message on subject: %s", subject)
	}
}

// Close closes all subscriptions and the connection
func (c *TestNATSConsumer) Close() {
	for _, sub := range c.subscriptions {
		_ = sub.Unsubscribe()
	}
	c.conn.Close()
}
