package telemetry

import (
	"errors"
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
	// ZMQ registers a zmq logger
	ZMQ Dispatcher = "zmq"
)

// BuildTopicName creates a topic from a namespace and a recordName
func BuildTopicName(namespace, recordName string) string {
	return fmt.Sprintf("%s_%s", namespace, recordName)
}

// Producer handles dispatching data received from the vehicle
type Producer interface {
	// Produce posts a Record to the producer.
	Produce(entry *Record)

	// Close releases resources tied to the producer.
	Close() error
}

// CloseDispatchRules closes the set of dispatch rules.
func CloseDispatchRules(dispatchRules map[string][]Producer) error {
	closeErrors := make([]error, 0)

	// Close each producer only once.
	closedProducers := make(map[Producer]struct{})
	for _, producerSet := range dispatchRules {
		for _, producer := range producerSet {
			if _, ok := closedProducers[producer]; !ok {
				closeErrors = append(closeErrors, producer.Close())
				closedProducers[producer] = struct{}{}
			}
		}
	}

	if len(closeErrors) == 0 {
		return nil
	}

	return errors.Join(closeErrors...)
}
