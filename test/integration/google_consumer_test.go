package integration_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"cloud.google.com/go/pubsub"
	. "github.com/onsi/gomega"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"google.golang.org/api/iterator"
)

type TestConsumer struct {
	pubsubClient *pubsub.Client
	subs         map[string]*pubsub.Subscription
}

func NewTestPubsubConsumer(projectID string, topicIDs []string, logger *logrus.Logger) (*TestConsumer, error) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	subs := map[string]*pubsub.Subscription{}

	for _, topicID := range topicIDs {
		_, err := createTopicIfNotExists(context.Background(), topicID, client)
		if err != nil {
			return nil, err
		}

		sub, err := createSubscriptionIfNotExists(ctx, topicID, client, logger)
		if err != nil {
			return nil, err
		}

		sub.ReceiveSettings.Synchronous = false
		sub.ReceiveSettings.MaxOutstandingMessages = -1

		subs[topicID] = sub
	}
	return &TestConsumer{
		pubsubClient: client,
		subs:         subs,
	}, nil

}

func subID(topicID string) string {
	return fmt.Sprintf("sub-id-%s", topicID)
}

func createSubscriptionIfNotExists(ctx context.Context, topicID string, pubsubClient *pubsub.Client, logger *logrus.Logger) (*pubsub.Subscription, error) {
	topic, err := createTopicIfNotExists(ctx, topicID, pubsubClient)
	if err != nil {
		return nil, err
	}
	subID := subID(topicID)
	sub := pubsubClient.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		logger.ActivityLog("subscription_present", logrus.LogInfo{"sub": sub})
		return sub, nil
	}
	c, err := pubsubClient.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
		Topic:                 topic,
		AckDeadline:           20 * time.Second,
		EnableMessageOrdering: true,
	})
	if err != nil {
		return nil, err
	}
	c.ReceiveSettings = pubsub.ReceiveSettings{
		MaxExtension: 10 * time.Hour,
	}
	return c, nil
}

func createTopicIfNotExists(ctx context.Context, topic string, pubsubClient *pubsub.Client) (*pubsub.Topic, error) {

	pubsubTopic := pubsubClient.Topic(topic)
	exists, err := pubsubTopic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return pubsubTopic, nil
	}

	return pubsubClient.CreateTopic(ctx, topic)
}

func (c *TestConsumer) ClearSubscriptions() {
	ctx := context.Background()
	it := c.pubsubClient.Subscriptions(ctx)
	for {
		sub, err := it.Next()
		if err == iterator.Done {
			break
		}
		err = sub.Delete(ctx)
		Expect(err).NotTo(HaveOccurred())
	}
}

func (c *TestConsumer) FetchPubsubMessage(topicID string) (*pubsub.Message, error) {
	sub, ok := c.subs[topicID]
	if !ok {
		return nil, fmt.Errorf("unknown topic: %s", topicID)
	}
	ctx := context.Background()
	var mu sync.Mutex
	var m *pubsub.Message
	var unsafepL = (*unsafe.Pointer)(unsafe.Pointer(&m))
	received := 0
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	err := sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
		msg.Ack()
		atomic.StorePointer(unsafepL, unsafe.Pointer(msg))
		mu.Lock()
		defer mu.Unlock()
		received++
		cancel()
	})
	return m, err
}
