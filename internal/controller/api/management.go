package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	CONNECTED_STATUS     = "connected"
	DISCONNECTED_STATUS  = "disconnected"
	DECODE_ERROR         = "Unable to process json input"
	NEGATIVE_DELAY_ERROR = "Delay field cannot be negative"
	PING_ERROR           = "Ping failed"
)

type ManagementServer struct {
	connectionMgr           connection_repository.ConnectionLocator
	getConnectionByClientID connection_repository.GetConnectionByClientID
	tenantTranslator        tenantid.Translator
	router                  *mux.Router
	config                  *config.Config
	urlPrefix               string
	proxyFactory            controller.ConnectorClientProxyFactory
}

func NewManagementServer(cm connection_repository.ConnectionLocator, byClientID connection_repository.GetConnectionByClientID, tenantTranslator tenantid.Translator, proxyFactory controller.ConnectorClientProxyFactory, r *mux.Router, urlPrefix string, cfg *config.Config) *ManagementServer {

	return &ManagementServer{
		connectionMgr:           cm,
		getConnectionByClientID: byClientID,
		tenantTranslator:        tenantTranslator,
		router:                  r,
		config:                  cfg,
		urlPrefix:               urlPrefix,
		proxyFactory:            proxyFactory,
	}
}

func (s *ManagementServer) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{
		Secrets: s.config.ServiceToServiceCredentials,
		IdentityAuth: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				identity.EnforceIdentity(middlewares.EnforceTurnpikeAuthentication(next)).ServeHTTP(w, r)
				return
			})
		},
		RequiredTenantIdentifier: middlewares.Account, // Account is the required tenant identifier for v1 rest interface
	}

	pathPrefix := fmt.Sprintf("%s/v1/connection", s.urlPrefix)

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
	Status         string      `json:"status"`
	Dispatchers    interface{} `json:"dispatchers,omitempty"`
	CanonicalFacts interface{} `json:"canonical_facts,omitempty"`
	Tags           interface{} `json:"tags,omitempty"`
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
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var disconnectReq disconnectRequest

		if err := decodeJSON(body, &disconnectReq); err != nil {
			errorResponse := errorResponse{Title: DECODE_ERROR,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		client, err := s.createConnectorClient(req.Context(), logger, disconnectReq.Account, disconnectReq.NodeID)
		if err != nil {
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
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var reconnectReq reconnectRequest

		if err := decodeJSON(body, &reconnectReq); err != nil {
			errorResponse := errorResponse{Title: DECODE_ERROR,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		if reconnectReq.Delay < 0 {
			errMsg := NEGATIVE_DELAY_ERROR
			logger.Info(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: errMsg}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		client, err := s.createConnectorClient(req.Context(), logger, reconnectReq.Account, reconnectReq.NodeID)
		if err != nil {
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

		client.Reconnect(req.Context(), reconnectReq.Message, reconnectReq.Delay)

		writeJSONResponse(w, http.StatusOK, nil)
	}
}

func (s *ManagementServer) handleConnectionStatus() http.HandlerFunc {

	inputVerifier := func(req *http.Request, connID connectionID) error {
		// This is the management interface...so do not verify that the
		// account from the header matches the account in the request
		return nil
	}

	return func(w http.ResponseWriter, req *http.Request) {
		getConnectionStatus(w, req, s.tenantTranslator, s.getConnectionByClientID, inputVerifier)
	}
}

type connectionListingParams struct {
	offset int
	limit  int
}

func getConnectionListingParams(req *http.Request) (*connectionListingParams, error) {

	limit, err := getLimitFromQueryParams(req)
	if err != nil {
		return nil, err
	}

	offset, err := getOffsetFromQueryParams(req)
	if err != nil {
		return nil, err
	}

	return &connectionListingParams{offset, limit}, nil
}

func (s *ManagementServer) handleConnectionListing() http.HandlerFunc {

	type ConnectionsPerAccount struct {
		AccountNumber string   `json:"account"`
		Connections   []string `json:"connections"`
	}

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())

		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		logger.Debugf("Getting connection list")

		requestParams, err := getConnectionListingParams(req)
		if err != nil {
			writeInvalidInputResponse(logger, w, err)
			return
		}

		allReceptorConnections, totalConnections, _ := s.connectionMgr.GetAllConnections(req.Context(), requestParams.offset, requestParams.limit)

		logger.Debugf("*** totalConnections: %d", totalConnections)

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

		response := buildPaginatedResponse(req.URL, requestParams.offset, requestParams.limit, totalConnections, connections)

		writeJSONResponse(w, http.StatusOK, response)
	}
}

type connectionListingByAccountParams struct {
	accountId string
	offset    int
	limit     int
}

func getConnectionListingByAccountParams(req *http.Request) (*connectionListingByAccountParams, error) {
	accountId := mux.Vars(req)["id"]

	limit, err := getLimitFromQueryParams(req)
	if err != nil {
		return nil, err
	}

	offset, err := getOffsetFromQueryParams(req)
	if err != nil {
		return nil, err
	}

	return &connectionListingByAccountParams{accountId, offset, limit}, nil
}

func (s *ManagementServer) handleConnectionListingByAccount() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())

		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		requestParams, err := getConnectionListingByAccountParams(req)
		if err != nil {
			writeInvalidInputResponse(logger, w, err)
			return
		}

		logger.Debug("Getting connections for ", requestParams.accountId)

		accountConnections, totalConnections, _ := s.connectionMgr.GetConnectionsByAccount(
			req.Context(),
			domain.AccountID(requestParams.accountId),
			requestParams.offset,
			requestParams.limit)

		logger.Debugf("*** totalConnections: %d", totalConnections)

		connections := make([]string, len(accountConnections))

		connCount := 0
		for conn := range accountConnections {
			connections[connCount] = string(conn)
			connCount++
		}

		response := buildPaginatedResponse(req.URL, requestParams.offset, requestParams.limit, totalConnections, connections)

		writeJSONResponse(w, http.StatusOK, response)
	}
}

func (s *ManagementServer) handleConnectionPing() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var connID connectionID

		if err := decodeJSON(body, &connID); err != nil {
			errorResponse := errorResponse{Title: DECODE_ERROR,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Submitting ping for account:%s - node id:%s",
			connID.Account, connID.NodeID)

		pingResponse := connectionPingResponse{Status: DISCONNECTED_STATUS}

		client, err := s.createConnectorClient(req.Context(), logger, domain.AccountID(connID.Account), domain.ClientID(connID.NodeID))
		if err != nil {
			errMsg := fmt.Sprintf("No connection found for node (%s:%s)", connID.Account, connID.NodeID)
			logger.Info(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: errMsg}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		pingResponse.Status = CONNECTED_STATUS

		pingErr := client.Ping(req.Context())

		if pingErr != nil {
			errorResponse := errorResponse{Title: PING_ERROR,
				Status: http.StatusBadRequest,
				Detail: pingErr.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		writeJSONResponse(w, http.StatusOK, pingResponse)
	}
}

func (s *ManagementServer) createConnectorClient(ctx context.Context, log *logrus.Entry, account domain.AccountID, clientId domain.ClientID) (controller.ConnectorClient, error) {

	resolvedOrgId, err := s.tenantTranslator.EANToOrgID(ctx, string(account))
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to translate account (%s) to org_id", account)
		return nil, err
	}

	log.Infof("Translated account %s to org_id %s", account, resolvedOrgId)

	clientState, err := s.getConnectionByClientID(ctx, log, domain.OrgID(resolvedOrgId), clientId)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to locate connection (%s:%s)", resolvedOrgId, clientId)
		return nil, err
	}

	proxy, err := s.proxyFactory.CreateProxy(ctx, clientState.OrgID, clientState.Account, clientState.ClientID, clientState.Dispatchers)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to create proxy for connection (%s:%s)", resolvedOrgId, clientId)
		return nil, err
	}

	return proxy, nil
}
