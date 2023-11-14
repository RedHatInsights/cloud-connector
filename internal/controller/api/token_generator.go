package api

import (
	//"context"
	"fmt"
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type TokenGeneratorServer struct {
	router    *mux.Router
	config    *config.Config
	urlPrefix string
}

func NewTokenGeneratorServer(r *mux.Router, urlPrefix string, cfg *config.Config) *TokenGeneratorServer {
	return &TokenGeneratorServer{
		router:    r,
		config:    cfg,
		urlPrefix: urlPrefix,
	}
}

func (s *TokenGeneratorServer) Routes() {
	mmw := &middlewares.MetricsMiddleware{}

	pathPrefix := fmt.Sprintf("%s/v1/token", s.urlPrefix)

	securedSubRouter := s.router.PathPrefix(pathPrefix).Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
		identity.EnforceIdentity,
		middlewares.EnforceCertAuthentication,
	)

	securedSubRouter.HandleFunc("/token", s.handleGenerateToken()).Methods(http.MethodPost)
}

type tokenRequest struct {
	NotSure string `json:"placeholder"` // FIXME
}

type tokenResponse struct {
	// FIXME
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   string `json:"expires_in"`
}

func (s *TokenGeneratorServer) handleGenerateToken() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

		logger.Debug("MADE IT HERE!!")

		body := http.MaxBytesReader(w, req.Body, 1048576)

		var tokenReq tokenRequest
		if err := decodeJSON(body, &tokenReq); err != nil {
			errorResponse := errorResponse{Title: DECODE_ERROR,
				Status: http.StatusBadRequest,
				Detail: err.Error()}
			writeJSONResponse(w, errorResponse.Status, errorResponse)
			return
		}

		var tokenResp tokenResponse
		writeJSONResponse(w, http.StatusOK, tokenResp)
	}
}
