package api

import (
	"fmt"
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	CONNECTED_STATUS    = "connected"
	DISCONNECTED_STATUS = "disconnected"
)

type ManagementServer struct {
	connectionMgr connection_repository.ConnectionLocator
	router        *mux.Router
	config        *config.Config
	urlPrefix     string
}

func NewManagementServer(cm connection_repository.ConnectionLocator, r *mux.Router, urlPrefix string, cfg *config.Config) *ManagementServer {
	return &ManagementServer{
		connectionMgr: cm,
		router:        r,
		config:        cfg,
		urlPrefix:     urlPrefix,
	}
}

func (s *ManagementServer) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{Secrets: s.config.ServiceToServiceCredentials}

	pathPrefix := fmt.Sprintf("%s/connection", s.urlPrefix)

	securedSubRouter := s.router.PathPrefix(pathPrefix).Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		amw.Authenticate)

	securedSubRouter.HandleFunc("", s.handleConnectionListing()).Methods(http.MethodGet)
	securedSubRouter.HandleFunc("/{id:[0-9]+}", s.handleConnectionListingByAccount()).Methods(http.MethodGet)
	securedSubRouter.HandleFunc("/disconnect", s.handleDisconnect()).Methods(http.MethodPost)
	securedSubRouter.HandleFunc("/reconnect", s.handleReconnect()).Methods(http.MethodPost)
	securedSubRouter.HandleFunc("/status", s.handleConnectionStatus()).Methods(http.MethodPost)
	securedSubRouter.HandleFunc("/ping", s.handleConnectionPing()).Methods(http.MethodPost)
}

type connectionID struct {
	Account string `json:"account" validate:"required"`
	NodeID  string `json:"node_id" validate:"required"`
}

type connectionStatusResponse struct {
	Status      string      `json:"status"`
	Dispatchers interface{} `json:"dispatchers,omitempty"`
}

type connectionPingResponse struct {
	Status  string      `json:"status"`
	Payload interface{} `json:"payload"`
}

func (s *ManagementServer) handleDisconnect() http.HandlerFunc {

	type disconnectRequest struct {
		Account domain.AccountID `json:"account" validate:"required"`
		NodeID  domain.ClientID  `json:"node_id" validate:"required"`
		Message string           `json:"message"`
	}

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var disconnectReq disconnectRequest

		if err := decodeJSON(body, &disconnectReq); err != nil {
			errorResponse := errorResponse{Title: "Unable to process json input",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		client := s.connectionMgr.GetConnection(req.Context(), domain.AccountID(disconnectReq.Account), domain.ClientID(disconnectReq.NodeID))
		if client == nil {
			errMsg := fmt.Sprintf("No connection found for node (%s:%s)", disconnectReq.Account, disconnectReq.NodeID)
			logger.Info(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: errMsg}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Attempting to disconnect account:%s - node id:%s",
			disconnectReq.Account, disconnectReq.NodeID)

		client.Disconnect(req.Context(), disconnectReq.Message)

		writeJSONResponse(w, http.StatusOK, struct{}{})
	}
}

func (s *ManagementServer) handleReconnect() http.HandlerFunc {

	type reconnectRequest struct {
		Account domain.AccountID `json:"account" validate:"required"`
		NodeID  domain.ClientID  `json:"node_id" validate:"required"`
		Delay   int              `json:"delay" validate:"required"`
		Message string           `json:"message"`
	}

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var reconnectReq reconnectRequest

		if err := decodeJSON(body, &reconnectReq); err != nil {
			errorResponse := errorResponse{Title: "Unable to process json input",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		client := s.connectionMgr.GetConnection(req.Context(), reconnectReq.Account, reconnectReq.NodeID)
		if client == nil {
			errMsg := fmt.Sprintf("No connection found for node (%s:%s)", reconnectReq.Account, reconnectReq.NodeID)
			logger.Info(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: errMsg}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Attempting to disconnect account:%s - node id:%s",
			reconnectReq.Account, reconnectReq.NodeID)

		client.Reconnect(req.Context(), domain.AccountID(reconnectReq.Account), domain.ClientID(reconnectReq.NodeID), reconnectReq.Message, reconnectReq.Delay)

		writeJSONResponse(w, http.StatusOK, nil)
	}
}

func (s *ManagementServer) handleConnectionStatus() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var connID connectionID

		if err := decodeJSON(body, &connID); err != nil {
			errorResponse := errorResponse{Title: "Unable to process json input",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Checking connection status for account:%s - node id:%s",
			connID.Account, connID.NodeID)

		connectionStatus := connectionStatusResponse{Status: DISCONNECTED_STATUS}

		client := s.connectionMgr.GetConnection(req.Context(), domain.AccountID(connID.Account), domain.ClientID(connID.NodeID))
		if client != nil {
			connectionStatus.Status = CONNECTED_STATUS
			connectionStatus.Dispatchers, _ = client.GetDispatchers(req.Context())
		}

		logger.Infof("Connection status for account:%s - node id:%s => %s\n",
			connID.Account, connID.NodeID, connectionStatus.Status)

		writeJSONResponse(w, http.StatusOK, connectionStatus)
	}
}

func (s *ManagementServer) handleConnectionListing() http.HandlerFunc {

	type ConnectionsPerAccount struct {
		AccountNumber string   `json:"account"`
		Connections   []string `json:"connections"`
	}

	type Response struct {
		Connections []ConnectionsPerAccount `json:"connections"`
	}

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		logger.Debugf("Getting connection list")

		allReceptorConnections := s.connectionMgr.GetAllConnections(req.Context())

		connections := make([]ConnectionsPerAccount, len(allReceptorConnections))

		accountCount := 0
		for key, value := range allReceptorConnections {
			connections[accountCount].AccountNumber = string(key)
			connections[accountCount].Connections = make([]string, len(value))
			nodeCount := 0
			for k, _ := range value {
				connections[accountCount].Connections[nodeCount] = string(k)
				nodeCount++
			}

			accountCount++
		}

		response := Response{Connections: connections}

		writeJSONResponse(w, http.StatusOK, response)
	}
}

func (s *ManagementServer) handleConnectionListingByAccount() http.HandlerFunc {

	type Response struct {
		Connections []string `json:"connections"`
	}

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		accountId := mux.Vars(req)["id"]
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		logger.Debug("Getting connections for ", accountId)

		accountConnections := s.connectionMgr.GetConnectionsByAccount(req.Context(), domain.AccountID(accountId))
		connections := make([]string, len(accountConnections))

		connCount := 0
		for conn := range accountConnections {
			connections[connCount] = string(conn)
			connCount++
		}

		response := Response{Connections: connections}

		writeJSONResponse(w, http.StatusOK, response)
	}
}

func (s *ManagementServer) handleConnectionPing() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var connID connectionID

		if err := decodeJSON(body, &connID); err != nil {
			errorResponse := errorResponse{Title: "Unable to process json input",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Submitting ping for account:%s - node id:%s",
			connID.Account, connID.NodeID)

		pingResponse := connectionPingResponse{Status: DISCONNECTED_STATUS}
		client := s.connectionMgr.GetConnection(req.Context(), domain.AccountID(connID.Account), domain.ClientID(connID.NodeID))
		if client == nil {
			writeJSONResponse(w, http.StatusOK, pingResponse)
			return
		}

		pingResponse.Status = CONNECTED_STATUS

		err := client.Ping(req.Context(), domain.AccountID(connID.Account), domain.ClientID(connID.NodeID))

		if err != nil {
			errorResponse := errorResponse{Title: "Ping failed",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		writeJSONResponse(w, http.StatusOK, pingResponse)
	}
}
