package api

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MonitoringServer struct {
	router *mux.Router
	config *config.Config
}

func NewMonitoringServer(r *mux.Router, cfg *config.Config) *MonitoringServer {
	return &MonitoringServer{
		router: r,
		config: cfg,
	}
}

func (s *MonitoringServer) Routes() {
	s.router.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	s.router.HandleFunc("/liveness", s.handleLiveness()).Methods(http.MethodGet)
	s.router.HandleFunc("/readiness", s.handleReadiness()).Methods(http.MethodGet)

	if s.config.Profile {
		logger.Log.Warn("WARNING: Enabling the profiler endpoint!!")
		s.router.PathPrefix("/debug").Handler(http.DefaultServeMux)
	}
}

func (s *MonitoringServer) handleLiveness() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *MonitoringServer) handleReadiness() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
