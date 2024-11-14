package telemetry

import (
	"fmt"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
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
	// ZMQ registers a zmq logger
	ZMQ Dispatcher = "zmq"
)

// BuildTopicName creates a topic from a namespace and a recordName
func BuildTopicName(namespace, recordName string) string {
	return fmt.Sprintf("%s_%s", namespace, recordName)
}

// Producer handles dispatching data received from the vehicle
type Producer interface {
	Close() error
	Produce(entry *Record)
	ProcessReliableAck(entry *Record)
	ReportError(message string, err error, logInfo logrus.LogInfo)
}
