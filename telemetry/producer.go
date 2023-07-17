package telemetry

import (
	"fmt"
)

// Dispatcher type of telemetry record dispatcher
type Dispatcher string

const (
	// Pubsub registers a Google pubsub dispatcher
	Pubsub Dispatcher = "pubsub"
	// Kafka registers a kafka dispatcher
	Kafka Dispatcher = "kafka"
	// Kinesis registers a kinesis publisher
	Kinesis Dispatcher = "kinesis"
	// Logger registers a simple logger
	Logger Dispatcher = "logger"
)

// BuildTopicName creates a topic from a namespace and a recordName
func BuildTopicName(namespace, recordName string) string {
	return fmt.Sprintf("%s_%s", namespace, recordName)
}

// Producer handles dispatching data received from the vehicle
type Producer interface {
	Produce(entry *Record)
}
