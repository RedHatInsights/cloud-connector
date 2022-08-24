package middlewares

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"net/http"
	"strconv"
)

var (
	statusCodeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloud_connector_http_status_code_counter",
		Help: "The number of http status codes per interface",
	}, []string{"status_code"})

	httpResponseDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "cloud_connector_http_response_duration",
		Help: "The amount of time the http request took to process",
	}, []string{"status_code"})
)

// MetricsMiddleware allows the passage of parameters into the metrics middleware
type MetricsMiddleware struct {
}

func (mw *MetricsMiddleware) RecordHTTPMetrics(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		resp := &wrappedResponseWriter{w, 200}

		httpResponseTimer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			httpResponseDuration.WithLabelValues(strconv.Itoa(resp.statusCode)).Observe(v)
		}))
		defer httpResponseTimer.ObserveDuration()

		next.ServeHTTP(resp, req)

		statusCodeCounter.With(prometheus.Labels{
			"status_code": strconv.Itoa(resp.statusCode)}).Inc()
	})
}

type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (ww *wrappedResponseWriter) WriteHeader(status int) {
	ww.statusCode = status
	ww.ResponseWriter.WriteHeader(status)
}
