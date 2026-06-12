package integration_test

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type TestRedisConsumer struct {
	client *goredis.Client
	pubsub *goredis.PubSub
}

// NewTestRedisConsumer connects to Redis, registers channel in the per-VIN
// subscriber sorted set (setKey) with a future lease expiry so the producer
// routes records to it, and subscribes to channel.
func NewTestRedisConsumer(addr, setKey, channel string) (*TestRedisConsumer, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	leaseExpiry := float64(time.Now().Add(time.Hour).Unix())
	if err := client.ZAdd(context.Background(), setKey, goredis.Z{Score: leaseExpiry, Member: channel}).Err(); err != nil {
		return nil, err
	}
	pubsub := client.Subscribe(context.Background(), channel)
	if _, err := pubsub.Receive(context.Background()); err != nil {
		return nil, err
	}
	return &TestRedisConsumer{client: client, pubsub: pubsub}, nil
}

func (t *TestRedisConsumer) FetchRedisMessage() ([]byte, error) {
	select {
	case msg := <-t.pubsub.Channel():
		return []byte(msg.Payload), nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("timeout waiting for Redis message")
	}
}

func (t *TestRedisConsumer) Close() {
	_ = t.pubsub.Close()
	_ = t.client.Close()
}
