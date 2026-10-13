package zmq

import (
	"context"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("ZMQ producer", func() {
	var producer *Producer

	BeforeEach(func() {
		logger, _ := logrus.NoOpLogger()
		config := &Config{Addr: "tcp://127.0.0.1:*"}
		zmqProducer, err := NewProducer(context.Background(), config, noop.NewCollector(), "tesla", airbrake.NewAirbrakeHandler(nil), nil, nil, logger)
		Expect(err).NotTo(HaveOccurred())
		producer = zmqProducer.(*Producer)
	})

	AfterEach(func() {
		Expect(producer.Close()).To(Succeed())
	})

	It("produces safely from concurrent connections", func() {
		record := &telemetry.Record{TxType: "V", PayloadBytes: []byte("payload"), Vin: "TESTVIN000000042"}

		var wg sync.WaitGroup
		for worker := 0; worker < 8; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				for i := 0; i < 500; i++ {
					producer.Produce(record)
				}
			}()
		}
		wg.Wait()
	})

	It("does not panic when producing after close", func() {
		record := &telemetry.Record{TxType: "V", PayloadBytes: []byte("payload")}

		Expect(producer.Close()).To(Succeed())
		Expect(func() { producer.Produce(record) }).NotTo(Panic())
	})

	It("survives produce racing with close", func() {
		record := &telemetry.Record{TxType: "V", PayloadBytes: []byte("payload")}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer GinkgoRecover()
			for i := 0; i < 200; i++ {
				producer.Produce(record)
			}
		}()
		Expect(producer.Close()).To(Succeed())
		wg.Wait()
	})
})
