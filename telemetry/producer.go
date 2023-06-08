package telemetry

import (
	"fmt"
)

// Dispatcher type of telemetry record dispatcher
type Dispatcher string

const (
	// Pubsub registers google pubsub dispatcher
	Pubsub Dispatcher = "pubsub"
	// Kafka registers kafka dispatcher
	Kafka Dispatcher = "kafka"
	// Logger registers file logger
	Logger Dispatcher = "logger"
	// Logger registers file logger
	Kinesis Dispatcher = "kinesis"
)

// BuildTopic produces records into the custom topic
func BuildTopic(namespace string, record *Record) string {
	return fmt.Sprintf("%s_%s", namespace, record.TxType)
}

// Producer handles dispatching vitals received from the vehicle
type Producer interface {
	Produce(entry *Record)
}
