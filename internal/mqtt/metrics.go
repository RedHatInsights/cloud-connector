package mqtt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	controlMessageReceivedCounter    prometheus.Counter
	dataMessageReceivedCounter       prometheus.Counter
	sentMessageDirectiveCounter      *prometheus.CounterVec
	mqttMessagesWaitingToBeProcessed prometheus.Gauge
}

func NewMetrics() *Metrics {
	metrics := new(Metrics)

	metrics.mqttMessagesWaitingToBeProcessed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_connector_mqtt_messages_waiting_to_be_processed_count",
		Help: "Number of inflight mqtt message (and go routines) waiting to be processed",
	})

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

	return metrics
}

var (
	metrics = NewMetrics()
)
