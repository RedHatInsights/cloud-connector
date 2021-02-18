package api

import (
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type MessageReceiver struct {
	connectionMgr controller.ConnectionLocator
	router        *mux.Router
	config        *config.Config
	urlPrefix     string
}

func NewMessageReceiver(cm controller.ConnectionLocator, r *mux.Router, urlPrefix string, cfg *config.Config) *MessageReceiver {
	return &MessageReceiver{
		connectionMgr: cm,
		router:        r,
		config:        cfg,
		urlPrefix:     urlPrefix,
	}
}

func (jr *MessageReceiver) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{Secrets: jr.config.ServiceToServiceCredentials}

	securedSubRouter := jr.router.PathPrefix(jr.urlPrefix).Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		amw.Authenticate)

	securedSubRouter.HandleFunc("/message", jr.handleJob()).Methods(http.MethodPost)
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

		var client controller.Receptor
		client = jr.connectionMgr.GetConnection(req.Context(), domain.AccountID(msgRequest.Account), domain.ClientID(msgRequest.Recipient))
		if client == nil {
			writeConnectionFailureResponse(logger, w)
			return
		}

		logger = logger.WithFields(logrus.Fields{"recipient": msgRequest.Recipient,
			"directive": msgRequest.Directive})
		logger.Info("Sending a message")

		jobID, err := client.SendMessage(req.Context(), domain.AccountID(msgRequest.Account), domain.ClientID(msgRequest.Recipient),
			msgRequest.Directive,
			msgRequest.Metadata,
			msgRequest.Payload)

		if err == controller.ErrDisconnectedNode {
			writeConnectionFailureResponse(logger, w)
			return
		}

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Info("Error passing message to receptor")
			errorResponse := errorResponse{Title: "Error passing message to receptor",
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

func writeConnectionFailureResponse(logger *logrus.Entry, w http.ResponseWriter) {
	// The connection to the customer's receptor node was not available
	errMsg := "No connection to the receptor node"
	logger.Info(errMsg)
	errorResponse := errorResponse{Title: errMsg,
		Status: http.StatusNotFound,
		Detail: errMsg}
	writeJSONResponse(w, errorResponse.Status, errorResponse)
}
