package mqtt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	controlMessageReceivedCounter prometheus.Counter
	dataMessageReceivedCounter    prometheus.Counter
	sentMessageDirectiveCounter   *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	metrics := new(Metrics)

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
