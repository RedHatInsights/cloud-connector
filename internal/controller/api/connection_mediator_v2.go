package api

import (
	"net/http"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	logging "github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type ConnectionMediatorV2 struct {
	getConnectionByClientID connection_repository.GetConnectionByClientID
	getConnectionsByOrgID   connection_repository.GetConnectionsByOrgID
	router                  *mux.Router
	config                  *config.Config
	urlPrefix               string
	proxyFactory            controller.ConnectorClientProxyFactory
}

func NewConnectionMediatorV2(byClientID connection_repository.GetConnectionByClientID, byOrgID connection_repository.GetConnectionsByOrgID, proxyFactory controller.ConnectorClientProxyFactory, r *mux.Router, urlPrefix string, cfg *config.Config) *ConnectionMediatorV2 {
	return &ConnectionMediatorV2{
		getConnectionByClientID: byClientID,
		getConnectionsByOrgID:   byOrgID,
		router:                  r,
		config:                  cfg,
		urlPrefix:               urlPrefix,
		proxyFactory:            proxyFactory,
	}
}

func (this *ConnectionMediatorV2) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{
		Secrets:                  this.config.ServiceToServiceCredentials,
		IdentityAuth:             identity.EnforceIdentity,
		RequiredTenantIdentifier: middlewares.OrgID, // OrgID is the required tenant identifier for v2 rest interface
	}

	securedSubRouter := this.router.PathPrefix(this.urlPrefix).Subrouter()
	securedSubRouter.Use(logging.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		amw.Authenticate)

	securedSubRouter.HandleFunc("/v2/connections/{id}/message", this.handleSendMessage()).Methods(http.MethodPost)
	securedSubRouter.HandleFunc("/v2/connections/{id}/status", this.handleConnectionStatus()).Methods(http.MethodGet)
	securedSubRouter.HandleFunc("/v2/connections", this.handleConnectionListByOrgId()).Methods(http.MethodGet)
}

type messageRequestV2 struct {
	Payload   interface{} `json:"payload" validate:"required"`
	Metadata  interface{} `json:"metadata"`
	Directive string      `json:"directive" validate:"required"`
}

func (this *ConnectionMediatorV2) handleSendMessage() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())

		recipient := getClientIDFromRequestPath(req)

		logger := logging.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId,
			"recipient":  recipient,
		})

		var msgRequest messageRequestV2

		body := http.MaxBytesReader(w, req.Body, 1048576)

		if err := decodeJSON(body, &msgRequest); err != nil {
			errMsg := "Unable to process json input"
			logging.LogWithError(logger, errMsg, err)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		if len(strings.TrimSpace(msgRequest.Directive)) == 0 {
			logger.Debug(emptyDirectictiveErrorMsg)
			errorResponse := errorResponse{Title: emptyDirectictiveErrorMsg,
				Status: http.StatusBadRequest,
				Detail: emptyDirectictiveErrorMsg}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.Infof("Looking up connection for org_id:%s - client id:%s",
			principal.GetOrgID(), recipient)

		var clientState domain.ConnectorClientState
		var err error
		clientState, err = this.getConnectionByClientID(req.Context(), logger, domain.OrgID(principal.GetOrgID()), recipient)
		if err != nil {

			if err == connection_repository.NotFoundError {
				writeConnectionFailureResponse(logger, w)
				return
			}

			logging.LogWithError(logger, "Unable to locate connection", err)

			writeConnectionFailureResponse(logger, w)
			return
		}

		client, err := this.proxyFactory.CreateProxy(req.Context(), clientState.OrgID, clientState.Account, clientState.ClientID, clientState.Dispatchers)
		if err != nil {
			logging.LogWithError(logger, "Unable to create proxy for connection", err)
			writeConnectionFailureResponse(logger, w)
			return
		}

		logger = logger.WithFields(logrus.Fields{"directive": msgRequest.Directive})
		logger.Info("Sending a message")

		jobID, err := client.SendMessage(req.Context(),
			msgRequest.Directive,
			msgRequest.Metadata,
			msgRequest.Payload)

		if err == controller.ErrDisconnectedNode {
			writeConnectionFailureResponse(logger, w)
			return
		}

		if err != nil {
			logging.LogWithError(logger, "Error passing message to rhc client", err)
			errorResponse := errorResponse{Title: "Error passing message to rhc client",
				Status: http.StatusInternalServerError,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		logger.WithFields(logrus.Fields{"message_id": jobID}).Info("Message sent")

		msgResponse := messageResponse{jobID.String()}

		writeJSONResponse(w, http.StatusCreated, msgResponse)
	}
}

func getClientIDFromRequestPath(req *http.Request) domain.ClientID {
	params := mux.Vars(req)
	return domain.ClientID(params["id"])
}

type connectionResponseV2 struct {
	Account        domain.AccountID      `json:"account,omitempty"`
	OrgID          domain.OrgID          `json:"org_id,omitempty"`
	ClientID       domain.ClientID       `json:"client_id,omitempty"`
	CanonicalFacts domain.CanonicalFacts `json:"canonical_facts,omitempty"`
	Dispatchers    domain.Dispatchers    `json:"dispatchers,omitempty"`
	Tags           domain.Tags           `json:"tags,omitempty"`
}

type connectionStatusResponseV2 struct {
	Status string `json:"status"`
	connectionResponseV2
}

func (this *ConnectionMediatorV2) handleConnectionStatus() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())

		recipient := getClientIDFromRequestPath(req)

		logger := logging.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId,
			"recipient":  recipient,
		})

		var clientState domain.ConnectorClientState
		var err error

		logger.Infof("Checking connection status for org_id:%s - client id:%s",
			principal.GetOrgID(), recipient)

		clientState, err = this.getConnectionByClientID(req.Context(), logger, domain.OrgID(principal.GetOrgID()), recipient)
		if err != nil {
			if err == connection_repository.NotFoundError {
				logger.Debug("Connection not found")
				response := connectionStatusResponseV2{Status: DISCONNECTED_STATUS}
				writeJSONResponse(w, http.StatusOK, response)
				return
			}

			logging.LogWithError(logger, "Failed to lookup connection", err)

			writeConnectionFailureResponse(logger, w)
			return
		}

		logger.Debug("Connection found")

		response := convertConnectorClientStateToConnectionStatusResponseV2(clientState)
		writeJSONResponse(w, http.StatusOK, response)
		return
	}
}

func convertConnectorClientStateToConnectionStatusResponseV2(clientState domain.ConnectorClientState) connectionStatusResponseV2 {
	return connectionStatusResponseV2{"connected", convertConnectorClientStateToConnectionResponseV2(clientState)}
}

func convertConnectorClientStateToConnectionResponseV2(clientState domain.ConnectorClientState) connectionResponseV2 {
	return connectionResponseV2{
		Account:        clientState.Account,
		OrgID:          clientState.OrgID,
		ClientID:       clientState.ClientID,
		CanonicalFacts: clientState.CanonicalFacts,
		Dispatchers:    clientState.Dispatchers,
		Tags:           clientState.Tags,
	}
}

func (this *ConnectionMediatorV2) handleConnectionListByOrgId() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())

		logger := logging.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		logger.Debug("Getting connections for ", principal.GetOrgID())

		offset, limit, err := getOffsetAndLimitFromQueryParams(req)
		if err != nil {
			logging.LogWithError(logger, "Unable to retrieve offset/limit from request", err)
			errorResponse := errorResponse{Title: "Invalid request",
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		accountConnections, totalConnections, err := this.getConnectionsByOrgID(
			req.Context(),
			logger,
			domain.OrgID(principal.GetOrgID()),
			offset,
			limit)

		if err != nil {
			logging.LogWithError(logger, "Error looking up connections by org_id", err)
			errorResponse := errorResponse{Title: "Error looking up connections by org_id",
				Status: http.StatusInternalServerError,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
		}

		connections := make([]connectionResponseV2, len(accountConnections))

		connCount := 0
		for _, conn := range accountConnections {
			connections[connCount] = convertConnectorClientStateToConnectionResponseV2(conn)
			connCount++
		}

		response := buildPaginatedResponse(req.URL, offset, limit, totalConnections, connections)

		writeJSONResponse(w, http.StatusOK, response)
	}
}
