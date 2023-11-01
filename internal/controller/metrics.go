package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	inventoryKafkaWriterGoRoutineGauge prometheus.Gauge
	inventoryKafkaWriterSuccessCounter prometheus.Counter
	inventoryKafkaWriterFailureCounter prometheus.Counter

	authGatewayAccountLookupStatusCodeCounter *prometheus.CounterVec
	authGatewayAccountLookupDuration          prometheus.Histogram
	accountLookupCacheHit                     prometheus.Counter
	accountLookupCacheMiss                    prometheus.Counter
}

func NewMetrics() *Metrics {
	metrics := new(Metrics)

	metrics.inventoryKafkaWriterGoRoutineGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_connector_inventory_kafka_writer_go_routine_count",
		Help: "The total number of active kafka response writer go routines",
	})

	metrics.inventoryKafkaWriterSuccessCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_inventory_kafka_writer_success_count",
		Help: "The number of responses were sent to the kafka topic",
	})

	metrics.inventoryKafkaWriterFailureCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_inventory_kafka_writer_failure_count",
		Help: "The number of responses that failed to get produced to kafka topic",
	})

	metrics.authGatewayAccountLookupStatusCodeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_connector_auth_gateway_account_lookup_status_code_counter",
		Help: "The number of http status codes received from the auth gateway account lookup",
	}, []string{"status_code"})

	metrics.authGatewayAccountLookupDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_auth_gateway_account_lookup_duration",
		Help: "The amount of time the auth gateway account lookup took",
	})

	metrics.accountLookupCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_account_lookup_cache_hit",
		Help: "The number of account lookup cache hits",
	})

	metrics.accountLookupCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_account_lookup_cache_miss",
		Help: "The number of account lookup cache misses",
	})

	return metrics
}

var (
	metrics = NewMetrics()
)
