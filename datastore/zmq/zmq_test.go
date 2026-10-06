package zmq_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pebbe/zmq4"
	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/fleet-telemetry/datastore/zmq"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("ZMQ Producer", func() {
	var (
		mockLogger    *logrus.Logger
		mockCollector metrics.MetricCollector
		mockAirbrake  *airbrake.Handler
		mockConfig    *zmq.Config
		serializer    *telemetry.BinarySerializer
	)

	BeforeEach(func() {
		mockLogger, _ = logrus.NoOpLogger()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockAirbrake = airbrake.NewAirbrakeHandler(nil)
		mockConfig = &zmq.Config{Addr: "tcp://127.0.0.1:*"}

		serializer = telemetry.NewBinarySerializer(
			&telemetry.RequestIdentity{
				DeviceID: "TEST123",
				SenderID: "vehicle_device.TEST123",
			},
			map[string][]telemetry.Producer{},
			mockLogger,
		)
	})

	newRecord := func() *telemetry.Record {
		payload := &protos.Payload{Vin: "TEST123"}
		payloadBytes, err := proto.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())

		message := messages.StreamMessage{
			TXID:         []byte("1234"),
			SenderID:     []byte("vehicle_device.TEST123"),
			MessageTopic: []byte("V"),
			Payload:      payloadBytes,
		}
		msgBytes, err := message.ToBytes()
		Expect(err).NotTo(HaveOccurred())
		record, err := telemetry.NewRecord(serializer, msgBytes, "1", true)
		Expect(err).NotTo(HaveOccurred())
		return record
	}

	It("handles concurrent Produce calls safely", func() {
		producer, err := zmq.NewProducer(
			context.Background(),
			mockConfig,
			mockCollector,
			"test_namespace",
			mockAirbrake,
			nil,
			nil,
			mockLogger,
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(producer.Close()).To(Succeed())
		}()

		rec := newRecord()

		const goroutines = 50
		const perGoroutines = 100
		var wg sync.WaitGroup
		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range perGoroutines {
					producer.Produce(rec)
				}
			}()
		}
		wg.Wait()
	})

	It("does not block or panic when Close races with Produce", func() {
		producer, err := zmq.NewProducer(
			context.Background(),
			mockConfig,
			mockCollector,
			"test_namespace",
			mockAirbrake,
			nil,
			nil,
			mockLogger,
		)
		Expect(err).NotTo(HaveOccurred())

		rec := newRecord()

		const goroutines = 10
		const perGoroutines = 1000
		var wg sync.WaitGroup

		for range goroutines {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				for range perGoroutines {
					producer.Produce(rec)
				}
			}()
		}

		Expect(producer.Close()).To(Succeed())

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		Eventually(done, "5s").Should(BeClosed())
	})

	It("can be closed more than once", func() {
		producer, err := zmq.NewProducer(
			context.Background(),
			mockConfig,
			mockCollector,
			"test_namespace",
			mockAirbrake,
			nil,
			nil,
			mockLogger,
		)
		Expect(err).NotTo(HaveOccurred())

		Expect(producer.Close()).To(Succeed())
		Expect(producer.Close()).To(Succeed())
	})

	It("delivers published records to a subscriber with intact framing", func() {
		addr := "inproc://zmq-delivery-test"
		cfg := &zmq.Config{Addr: addr}
		producer, err := zmq.NewProducer(
			context.Background(),
			cfg,
			mockCollector,
			"test_namespace",
			mockAirbrake,
			nil,
			nil,
			mockLogger,
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(producer.Close()).To(Succeed())
		}()

		sub, err := zmq4.NewSocket(zmq4.SUB)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = sub.Close() }()
		Expect(sub.SetSubscribe("")).To(Succeed())
		Expect(sub.Connect(addr)).To(Succeed())

		rec := newRecord()
		expectedTopic := telemetry.BuildTopicName("test_namespace", rec.TxType)
		expectedPayload := rec.Payload()

		Expect(sub.SetRcvtimeo(100 * time.Millisecond)).To(Succeed())

		// warm up: publish one at a time until the first message gets through
		// a successful receive proves the subscription has reached the publisher
		Eventually(func() bool {
			producer.Produce(rec)
			_, err := sub.RecvMessageBytes(0)
			return err == nil
		}, "5s", "10ms").Should(BeTrue())

		for range 5 {
			producer.Produce(rec)
			frames, err := sub.RecvMessageBytes(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(frames).To(HaveLen(2))
			Expect(string(frames[0])).To(Equal(expectedTopic))
			Expect(frames[1]).To(Equal(expectedPayload))
		}
	})
})
