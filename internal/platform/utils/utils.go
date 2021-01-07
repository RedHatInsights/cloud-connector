package utils

import (
	"context"
	"net"
	"net/http"
	"os"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func StartHTTPServer(addr, name string, handler *mux.Router) *http.Server {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		logger.Log.Infof("Starting %s server:  %s", name, addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Log.WithFields(logrus.Fields{"error": err}).Fatalf("%s server error", name)
		}
	}()

	return srv
}

func ShutdownHTTPServer(ctx context.Context, name string, srv *http.Server) {
	logger.Log.Infof("Shutting down %s server", name)
	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Infof("Error shutting down %s server: %e", name, err)
	}
}

func GetHostname() string {
	name, err := os.Hostname()
	if err != nil {
		logger.Log.Info("Error getting hostname")
	}

	return name
}

func GetIPAddress() net.IP {
	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			return ipv4
		}
	}
	return nil
}
