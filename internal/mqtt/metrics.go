package mqtt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type mqttMetrics struct {
	controlMessageReceivedCounter    prometheus.Counter
	dataMessageReceivedCounter       prometheus.Counter
	sentMessageDirectiveCounter      *prometheus.CounterVec
	messagePublishedSuccessCounter   prometheus.Counter
	messagePublishedFailureCounter   prometheus.Counter
	kafkaWriterGoRoutineGauge        prometheus.Gauge
	kafkaWriterPublishDuration       prometheus.Histogram
	mqttMessagesWaitingToBeProcessed prometheus.Gauge
}

func newMqttMetrics() *mqttMetrics {
	metrics := new(mqttMetrics)

	metrics.controlMessageReceivedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_mqtt_control_message_received_count",
		Help: "The number of control messages received",
	})

	metrics.dataMessageReceivedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_mqtt_data_message_received_count",
		Help: "The number of data messages received",
	})

	metrics.sentMessageDirectiveCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_connector_mqtt_sent_message_directive_count",
		Help: "The number of messages recieved by the receptor controller per directive",
	}, []string{"directive"})

	metrics.messagePublishedSuccessCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_mqtt_message_published_success_count",
		Help: "The number of messages published successfully",
	})

	metrics.messagePublishedFailureCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_mqtt_message_published_failure_count",
		Help: "The number of messages published failures",
	})

	metrics.kafkaWriterGoRoutineGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_connector_mqtt_message_consumer_kafka_writer_go_routine_count",
		Help: "The total number of active kafka writer go routines for the mqtt message consumer",
	})

	metrics.kafkaWriterPublishDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_mqtt_message_consumer_kafka_writer_publish_duration",
		Help: "The amount of time the mqtt consumer spends waiting on a kafka write",
	})

	metrics.mqttMessagesWaitingToBeProcessed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_connector_mqtt_messages_waiting_to_be_processed_count",
		Help: "Number of inflight mqtt message (and go routines) waiting to be processed",
	})

	return metrics
}

var (
	metrics = newMqttMetrics()
)
