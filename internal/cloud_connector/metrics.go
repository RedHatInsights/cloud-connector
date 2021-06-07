package cloud_connector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type kafkaMetrics struct {
	controlMessageReceivedCounter prometheus.Counter
}

func newKafkaMetrics() *kafkaMetrics {
	metrics := new(kafkaMetrics)

	metrics.controlMessageReceivedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_control_message_received_from_kafka_count",
		Help: "The number of control messages received from the kafka topic",
	})

	return metrics
}

var (
	metrics = newKafkaMetrics()
)
