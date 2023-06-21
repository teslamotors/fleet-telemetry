package telemetry

import (
	"fmt"
)

// Dispatcher type of telemetry record dispatcher
type Dispatcher string

const (
	// Pubsub registers a google pubsub dispatcher
	Pubsub Dispatcher = "pubsub"
	// Kafka registers a kafka dispatcher
	Kafka Dispatcher = "kafka"
	// Kinesis registers a kinesis publisher
	Kinesis Dispatcher = "kinesis"
	// Logger registers a simple logger
	Logger Dispatcher = "logger"
)

// BuildTopic produces records into the custom topic
func BuildTopic(namespace string, record *Record) string {
	return fmt.Sprintf("%s_%s", namespace, record.TxType)
}

// Producer handles dispatching data received from the vehicle
type Producer interface {
	Produce(entry *Record)
}
