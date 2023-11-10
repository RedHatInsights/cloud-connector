package api

import (
	"context"
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


type TokenExchangerServer struct {
	router                  *mux.Router
	config                  *config.Config
	urlPrefix               string
}

func NewTokenExchangerServer(r *mux.Router, urlPrefix string, cfg *config.Config) *TokenExchangerServer {

	return &TokenExchangerServer{
		router:                  r,
		config:                  cfg,
		urlPrefix:               urlPrefix,
	}
}

func (s *TokenExchangerServer) Routes() {
	mmw := &middlewares.MetricsMiddleware{}
    /*
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
*/

	pathPrefix := fmt.Sprintf("%s/v1/token", s.urlPrefix)

	securedSubRouter := s.router.PathPrefix(pathPrefix).Subrouter()
	securedSubRouter.Use(logger.AccessLoggerMiddleware,
		mmw.RecordHTTPMetrics,
    )
//		amw.Authenticate)

	securedSubRouter.HandleFunc("/token", s.handleGenerateToken()).Methods(http.MethodPost)
}

type tokenRequest struct {
	Account string `json:"account" validate:"required"`
	NodeID  string `json:"node_id" validate:"required"`
}

type tokenResponse struct {
	Status         string      `json:"status"`
}

func (s *TokenExchangerServer) handleGenerateToken() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {

		principal, _ := middlewares.GetPrincipal(req.Context())
		requestId := request_id.GetReqID(req.Context())
		logger := logger.Log.WithFields(logrus.Fields{
			"account":    principal.GetAccount(),
			"org_id":     principal.GetOrgID(),
			"request_id": requestId})

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
