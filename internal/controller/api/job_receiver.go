package api

import (
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type JobReceiver struct {
	connectionMgr controller.ConnectionLocator
	router        *mux.Router
	config        *config.Config
}

func NewJobReceiver(cm controller.ConnectionLocator, r *mux.Router, cfg *config.Config) *JobReceiver {
	return &JobReceiver{
		connectionMgr: cm,
		router:        r,
		config:        cfg,
	}
}

func (jr *JobReceiver) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
	amw := &middlewares.AuthMiddleware{Secrets: jr.config.ServiceToServiceCredentials}

	securedSubRouter := jr.router.PathPrefix("/").Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		amw.Authenticate)

	securedSubRouter.HandleFunc("/job", jr.handleJob()).Methods(http.MethodPost)
}

type jobRequest struct {
	Account   string      `json:"account" validate:"required"`
	Recipient string      `json:"recipient" validate:"required"`
	Payload   interface{} `json:"payload" validate:"required"`
	Directive string      `json:"directive" validate:"required"`
}

type jobResponse struct {
	JobID string `json:"id"`
}

func (jr *JobReceiver) handleJob() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"request_id": requestId})

		var jobRequest jobRequest

		body := http.MaxBytesReader(w, req.Body, 1048576)

		if err := decodeJSON(body, &jobRequest); err != nil {
			errMsg := "Unable to process json input"
			logger.WithFields(logrus.Fields{"error": err}).Debug(errMsg)
			errorResponse := errorResponse{Title: errMsg,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		var client controller.Receptor
		client = jr.connectionMgr.GetConnection(req.Context(), jobRequest.Account, jobRequest.Recipient)
		if client == nil {
			writeConnectionFailureResponse(logger, w)
			return
		}

		logger = logger.WithFields(logrus.Fields{"recipient": jobRequest.Recipient,
			"directive": jobRequest.Directive})
		logger.Info("Sending a message")

		jobID, err := client.SendMessage(req.Context(), jobRequest.Account, jobRequest.Recipient,
			[]string{jobRequest.Recipient},
			jobRequest.Payload,
			jobRequest.Directive)

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

		jobResponse := jobResponse{jobID.String()}

		writeJSONResponse(w, http.StatusCreated, jobResponse)
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
