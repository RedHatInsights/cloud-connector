package connection_repository

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type connectionRepositoryMetrics struct {
	sqlLookupConnectionByAccountOrPermittedTenantAndClientIDDuration prometheus.Histogram

	sqlLookupConnectionByAccountAndClientIDDuration prometheus.Histogram
	sqlLookupConnectionsByAccountDuration           prometheus.Histogram
	sqlLookupAllConnectionsDuration                 prometheus.Histogram

	sqlConnectionRegistrationDuration     prometheus.Histogram
	sqlConnectionUnregistrationDuration   prometheus.Histogram
	sqlConnectionLookupByClientIDDuration prometheus.Histogram
}

var metrics *connectionRepositoryMetrics

func init() {
	metrics = new(connectionRepositoryMetrics)

	metrics.sqlLookupConnectionByAccountOrPermittedTenantAndClientIDDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connection_by_account_or_permitted_account_and_client_id_duration",
		Help: "The amount of time the it took to lookup a connection using (account or permitted accont) and client id ",
	})

	metrics.sqlLookupConnectionByAccountAndClientIDDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connection_by_account_and_client_id_duration",
		Help: "The amount of time the it took to lookup a connection using account and client id ",
	})

	metrics.sqlLookupConnectionsByAccountDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connections_by_account",
		Help: "The amount of time the it took to lookup a connection using account",
	})

	metrics.sqlLookupAllConnectionsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_all_connections",
		Help: "The amount of time the it took to lookup all connections",
	})

	metrics.sqlConnectionRegistrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_register_connection_duration",
		Help: "The amount of time the it took to register a connection in the db",
	})

	metrics.sqlConnectionUnregistrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_unregister_connection_duration",
		Help: "The amount of time the it took to unregister a connection in the db",
	})

	metrics.sqlConnectionLookupByClientIDDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connection_by_client_id_duration",
		Help: "The amount of time the it took to register a connection in the db",
	})
}
