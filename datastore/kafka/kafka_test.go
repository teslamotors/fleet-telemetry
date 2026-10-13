package kafka

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("buildMessage", func() {
	var record *telemetry.Record

	BeforeEach(func() {
		record = &telemetry.Record{
			Vin:          "TESTVIN000000042",
			TxType:       "V",
			Txid:         "1234",
			PayloadBytes: []byte("payload"),
		}
	})

	It("keys the message by vin so a vehicle's records stay on one partition", func() {
		msg := buildMessage(record, "tesla_V")

		Expect(msg.Key).To(Equal([]byte("TESTVIN000000042")))
		Expect(*msg.TopicPartition.Topic).To(Equal("tesla_V"))
		Expect(msg.TopicPartition.Partition).To(Equal(kafka.PartitionAny))
	})

	It("carries the payload, metadata headers, and the record as opaque", func() {
		msg := buildMessage(record, "tesla_V")

		Expect(msg.Value).To(Equal([]byte("payload")))
		Expect(msg.Opaque).To(BeIdenticalTo(record))

		headers := map[string]string{}
		for _, header := range msg.Headers {
			headers[header.Key] = string(header.Value)
		}
		Expect(headers).To(HaveKeyWithValue("vin", "TESTVIN000000042"))
		Expect(headers).To(HaveKeyWithValue("txid", "1234"))
		Expect(headers).To(HaveKeyWithValue("txtype", "V"))
	})
})
