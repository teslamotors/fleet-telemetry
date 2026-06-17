package redis_test

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/redis"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("RedisProducer", func() {
	const (
		setPrefix = "consumer"
		vinTopic  = "test_namespace_V_{TEST123}"
		vinSetKey = "consumer_test_namespace_V_{TEST123}"
	)

	var (
		mockServer     *miniredis.Miniredis
		mockLogger     *logrus.Logger
		mockCollector  metrics.MetricCollector
		mockAirbrake   *airbrake.Handler
		client         *goredis.Client
		namespace      string
		publishTimeout time.Duration
	)

	BeforeEach(func() {
		var err error
		mockServer, err = miniredis.Run()
		Expect(err).NotTo(HaveOccurred())

		mockLogger, _ = logrus.NoOpLogger()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockAirbrake = airbrake.NewAirbrakeHandler(nil)
		namespace = "test_namespace"
		publishTimeout = 2 * time.Second

		client = goredis.NewClient(&goredis.Options{Addr: mockServer.Addr()})
	})

	AfterEach(func() {
		_ = client.Close()
		mockServer.Close()
	})

	newProducer := func(publishVINTopics bool, subscriberSetPrefix string, ackChan chan *telemetry.Record, reliableAckTxTypes map[string]interface{}) telemetry.Producer {
		producer, err := redis.NewProducer(&goredis.UniversalOptions{Addrs: []string{mockServer.Addr()}}, publishTimeout, publishVINTopics, subscriberSetPrefix, namespace, false, mockCollector, mockAirbrake, ackChan, reliableAckTxTypes, mockLogger)
		Expect(err).NotTo(HaveOccurred())
		return producer
	}

	record := func() *telemetry.Record {
		return &telemetry.Record{
			TxType:       "V",
			Vin:          "TEST123",
			Txid:         "txid-1",
			PayloadBytes: []byte("payload-bytes"),
		}
	}

	subscribe := func(channels ...string) <-chan *goredis.Message {
		pubsub := client.Subscribe(context.Background(), channels...)
		_, err := pubsub.Receive(context.Background())
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(pubsub.Close)
		return pubsub.Channel()
	}

	register := func(channel string, score int64) {
		Expect(client.ZAdd(context.Background(), vinSetKey, goredis.Z{Score: float64(score), Member: channel}).Err()).To(Succeed())
	}
	future := func() int64 { return time.Now().Add(time.Hour).Unix() }
	past := func() int64 { return time.Now().Add(-time.Hour).Unix() }

	Describe("NewProducer", func() {
		It("errors when no addrs are configured", func() {
			_, err := redis.NewProducer(&goredis.UniversalOptions{}, publishTimeout, false, setPrefix, namespace, false, mockCollector, mockAirbrake, nil, nil, mockLogger)
			Expect(err).To(HaveOccurred())
		})

		It("errors when the server is unreachable", func() {
			addr := mockServer.Addr()
			mockServer.Close()
			_, err := redis.NewProducer(&goredis.UniversalOptions{Addrs: []string{addr}}, publishTimeout, false, setPrefix, namespace, false, mockCollector, mockAirbrake, nil, nil, mockLogger)
			Expect(err).To(HaveOccurred())
		})

		It("errors when neither PublishVINTopics nor a subscriber set prefix is configured", func() {
			_, err := redis.NewProducer(&goredis.UniversalOptions{Addrs: []string{mockServer.Addr()}}, publishTimeout, false, "", namespace, false, mockCollector, mockAirbrake, nil, nil, mockLogger)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Produce", func() {
		It("publishes to the unexpired channels registered in the VIN subscriber set", func() {
			register("channel_a", future())
			register("channel_b", future())
			messages := subscribe("channel_a", "channel_b")

			newProducer(false, setPrefix, nil, nil).Produce(record())

			var first, second *goredis.Message
			Eventually(messages, 2*time.Second).Should(Receive(&first))
			Eventually(messages, 2*time.Second).Should(Receive(&second))
			Expect([]string{first.Channel, second.Channel}).To(ConsistOf("channel_a", "channel_b"))
			Expect(first.Payload).To(Equal("payload-bytes"))
			Expect(second.Payload).To(Equal("payload-bytes"))
		})

		It("purges subscribers whose score is below the current time", func() {
			register("stale_channel", past())
			register("live_channel", future())
			staleMessages := subscribe("stale_channel")
			liveMessages := subscribe("live_channel")

			newProducer(false, setPrefix, nil, nil).Produce(record())

			var msg *goredis.Message
			Eventually(liveMessages, 2*time.Second).Should(Receive(&msg))
			Expect(msg.Channel).To(Equal("live_channel"))
			Consistently(staleMessages, 500*time.Millisecond).ShouldNot(Receive())
			Expect(client.ZScore(context.Background(), vinSetKey, "stale_channel").Err()).To(MatchError(goredis.Nil))
		})

		It("publishes to the VIN channel when the set is empty and PublishVINTopics is enabled", func() {
			messages := subscribe(vinTopic)

			newProducer(true, setPrefix, nil, nil).Produce(record())

			var msg *goredis.Message
			Eventually(messages, 2*time.Second).Should(Receive(&msg))
			Expect(msg.Channel).To(Equal(vinTopic))
			Expect(msg.Payload).To(Equal("payload-bytes"))
		})

		It("publishes to both the subscriber set members and the VIN channel when PublishVINTopics is enabled", func() {
			register("channel_a", future())
			setMessages := subscribe("channel_a")
			vinMessages := subscribe(vinTopic)

			newProducer(true, setPrefix, nil, nil).Produce(record())

			var memberMsg, vinMsg *goredis.Message
			Eventually(setMessages, 2*time.Second).Should(Receive(&memberMsg))
			Eventually(vinMessages, 2*time.Second).Should(Receive(&vinMsg))
			Expect(memberMsg.Channel).To(Equal("channel_a"))
			Expect(vinMsg.Channel).To(Equal(vinTopic))
		})

		It("publishes nothing when the set is empty and PublishVINTopics is disabled", func() {
			messages := subscribe(vinSetKey)

			newProducer(false, setPrefix, nil, nil).Produce(record())

			Consistently(messages, 500*time.Millisecond).ShouldNot(Receive())
		})

		It("ignores the sorted set and publishes to the VIN channel when no prefix is set", func() {
			register("channel_a", future())
			setMessages := subscribe("channel_a")
			vinMessages := subscribe(vinTopic)

			newProducer(true, "", nil, nil).Produce(record())

			var msg *goredis.Message
			Eventually(vinMessages, 2*time.Second).Should(Receive(&msg))
			Expect(msg.Channel).To(Equal(vinTopic))
			Consistently(setMessages, 500*time.Millisecond).ShouldNot(Receive())
		})
	})

	Describe("ProcessReliableAck", func() {
		It("sends to the ack channel for configured tx types", func() {
			ackChan := make(chan *telemetry.Record, 1)
			entry := record()

			newProducer(false, setPrefix, ackChan, map[string]interface{}{"V": true}).Produce(entry)

			Eventually(ackChan).Should(Receive(Equal(entry)))
		})

		It("does not ack tx types that are not configured", func() {
			ackChan := make(chan *telemetry.Record, 1)

			newProducer(false, setPrefix, ackChan, map[string]interface{}{"other": true}).Produce(record())

			Consistently(ackChan, 300*time.Millisecond).ShouldNot(Receive())
		})
	})

	Describe("Close", func() {
		It("closes the client without error", func() {
			Expect(newProducer(false, setPrefix, nil, nil).Close()).To(Succeed())
		})
	})
})
