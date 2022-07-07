package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	accountMismatchErrorMsg   = "Account mismatch"
	emptyDirectictiveErrorMsg = "Directive field is empty"
)

type MessageReceiver struct {
	connectionMgr connection_repository.ConnectionLocator
	router        *mux.Router
	config        *config.Config
	urlPrefix     string
}

func NewMessageReceiver(cm connection_repository.ConnectionLocator, r *mux.Router, urlPrefix string, cfg *config.Config) *MessageReceiver {
	return &MessageReceiver{
		connectionMgr: cm,
		router:        r,
		config:        cfg,
		urlPrefix:     urlPrefix,
	}
}

func (jr *MessageReceiver) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{
		Secrets:                  jr.config.ServiceToServiceCredentials,
		IdentityAuth:             identity.EnforceIdentity,
		RequiredTenantIdentifier: middlewares.Account, // Account is the required tenant identifier for v1 rest interface
	}

	securedSubRouter := jr.router.PathPrefix(jr.urlPrefix).Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		amw.Authenticate)

	securedSubRouter.HandleFunc("/message", jr.handleJob()).Methods(http.MethodPost)
	securedSubRouter.HandleFunc("/connection_status", jr.handleConnectionStatus()).Methods(http.MethodPost)
}

type messageRequest struct {
	Account   string      `json:"account" validate:"required"`
	Recipient string      `json:"recipient" validate:"required"`
	Payload   interface{} `json:"payload" validate:"required"`
	Metadata  interface{} `json:"metadata"`
	Directive string      `json:"directive" validate:"required"`
}

type messageResponse struct {
	JobID string `json:"id"`
}

func (jr *MessageReceiver) handleJob() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		var msgRequest messageRequest

		body := http.MaxBytesReader(w, req.Body, 1048576)

		if err := decodeJSON(body, &msgRequest); err != nil {
			errMsg := "Unable to process json input"
			logger.WithFields(logrus.Fields{"error": err}).Debug(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		if principal.GetAccount() != msgRequest.Account {
			logger.Debug(accountMismatchErrorMsg)
			errorResponse := errorResponse{Title: accountMismatchErrorMsg,
				Status: http.StatusForbidden,
				Detail: accountMismatchErrorMsg}
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

		orgID := principal.GetOrgID()

		var client controller.ConnectorClient
		client = jr.connectionMgr.GetConnection(req.Context(), domain.AccountID(msgRequest.Account), domain.OrgID(orgID), domain.ClientID(msgRequest.Recipient))
		if client == nil {
			writeConnectionFailureResponse(logger, w)
			return
		}

		logger = logger.WithFields(logrus.Fields{"recipient": msgRequest.Recipient,
			"directive": msgRequest.Directive})
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
			logger.WithFields(logrus.Fields{"error": err}).Info("Error passing message to rhc client")
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

type verifyConnectionIDMessage func(*http.Request, connectionID) error

func (jr *MessageReceiver) handleConnectionStatus() http.HandlerFunc {

	inputVerifier := func(req *http.Request, connID connectionID) error {
		principal, _ := middlewares.GetPrincipal(req.Context())
		if principal.GetAccount() != connID.Account {
			return fmt.Errorf(accountMismatchErrorMsg)
		}
		return nil
	}

	return func(w http.ResponseWriter, req *http.Request) {
		getConnectionStatus(w, req, jr.connectionMgr, inputVerifier)
	}
}

func getConnectionStatus(w http.ResponseWriter, req *http.Request, connectionLocator connection_repository.ConnectionLocator, verifyInput verifyConnectionIDMessage) {
	principal, _ := middlewares.GetPrincipal(req.Context())
	requestId := request_id.GetReqID(req.Context())
	logger := logger.Log.WithFields(logrus.Fields{
		"account":    principal.GetAccount(),
		"org_id":     principal.GetOrgID(),
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

	if err := verifyInput(req, connID); err != nil {
		errorResponse := errorResponse{Title: err.Error(),
			Status: http.StatusForbidden,
			Detail: err.Error()}
		writeJSONResponse(w, errorResponse.Status, errorResponse)
		return
	}

	logger.Infof("Checking connection status for account:%s - node id:%s",
		connID.Account, connID.NodeID)

	connectionStatus := connectionStatusResponse{Status: DISCONNECTED_STATUS}

	orgID := principal.GetOrgID()

	client := connectionLocator.GetConnection(req.Context(), domain.AccountID(connID.Account), domain.OrgID(orgID), domain.ClientID(connID.NodeID))
	if client != nil {
		connectionStatus.Status = CONNECTED_STATUS
		connectionStatus.Dispatchers, _ = client.GetDispatchers(req.Context())
	}

	logger.Infof("Connection status for account:%s - node id:%s => %s\n",
		connID.Account, connID.NodeID, connectionStatus.Status)

	writeJSONResponse(w, http.StatusOK, connectionStatus)
}

func writeConnectionFailureResponse(logger *logrus.Entry, w http.ResponseWriter) {
	// The connection to the customer's rhc client was not available
	errMsg := "No connection to the rhc client"
	logger.Info(errMsg)
	errorResponse := errorResponse{Title: errMsg,
		Status: http.StatusNotFound,
		Detail: errMsg}
	writeJSONResponse(w, errorResponse.Status, errorResponse)
}
