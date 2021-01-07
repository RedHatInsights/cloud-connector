package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	responseKafkaWriterGoRoutineGauge prometheus.Gauge
	responseKafkaWriterSuccessCounter prometheus.Counter
	responseKafkaWriterFailureCounter prometheus.Counter
	messageDirectiveCounter           *prometheus.CounterVec
	redisConnectionError              prometheus.Counter
}

func NewMetrics() *Metrics {
	metrics := new(Metrics)

	metrics.responseKafkaWriterGoRoutineGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_connector_kafka_response_writer_go_routine_count",
		Help: "The total number of active kakfa response writer go routines",
	})

	metrics.responseKafkaWriterSuccessCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_kafka_response_writer_success_count",
		Help: "The number of responses were sent to the kafka topic",
	})

	metrics.responseKafkaWriterFailureCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_kafka_response_writer_failure_count",
		Help: "The number of responses that failed to get produced to kafka topic",
	})

	metrics.redisConnectionError = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_redis_connection_error_count",
		Help: "The number of times a redis connection error has occurred",
	})

	metrics.messageDirectiveCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_connector_message_directive_count",
		Help: "The number of messages recieved by the receptor controller per directive",
	}, []string{"directive"})

	return metrics
}

var (
	metrics = NewMetrics()
)
