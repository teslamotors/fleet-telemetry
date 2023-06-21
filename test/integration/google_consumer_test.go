package integration_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"cloud.google.com/go/pubsub"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

type TestConsumer struct {
	pubsubClient *pubsub.Client
	topic        *pubsub.Topic
	sub          *pubsub.Subscription
}

func NewTestPubsubConsumer(projectID, topicID, subID string, logger *logrus.Logger) (*TestConsumer, error) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	topic, err := createTopicIfNotExists(context.Background(), topicID, client)
	if err != nil {
		return nil, err
	}

	sub, err := createSubcriptionfNotExists(ctx, subID, topicID, client, logger)
	if err != nil {
		return nil, err
	}
	sub.ReceiveSettings.Synchronous = false
	sub.ReceiveSettings.MaxOutstandingMessages = -1
	return &TestConsumer{
		pubsubClient: client,
		topic:        topic,
		sub:          sub,
	}, nil

}

func createSubcriptionfNotExists(ctx context.Context, subID string, topicID string, pubsubClient *pubsub.Client, logger *logrus.Logger) (*pubsub.Subscription, error) {
	topic, err := createTopicIfNotExists(ctx, topicID, pubsubClient)
	if err != nil {
		return nil, err
	}
	sub := pubsubClient.Subscription(subID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		logger.Infof("subscription %v already present", sub)
		return sub, nil
	}
	return pubsubClient.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
		Topic:       topic,
		AckDeadline: 20 * time.Second,
	})
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
		Expect(err).To(BeNil())
	}
}

func (c *TestConsumer) FetchPubsubMessage() (*pubsub.Message, error) {
	ctx := context.Background()
	var mu sync.Mutex
	var m *pubsub.Message
	var unsafepL = (*unsafe.Pointer)(unsafe.Pointer(&m))
	received := 0
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	err := c.sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		msg.Ack()
		atomic.StorePointer(unsafepL, unsafe.Pointer(msg))
		mu.Lock()
		defer mu.Unlock()
		received++
		cancel()
	})
	return m, err
}
