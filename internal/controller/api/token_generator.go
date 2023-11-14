package api

import (
	//"context"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type TokenGeneratorServer struct {
	router      *mux.Router
	config      *config.Config
	urlPrefix   string
	tokenExpiry int
	signingKey  *rsa.PrivateKey
}

func NewTokenGeneratorServer(r *mux.Router, urlPrefix string, cfg *config.Config) *TokenGeneratorServer {

	privateKeyFile := "newkey.pem"
	privateKeyFile = filepath.Clean(privateKeyFile)
	signBytes, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		panic(err)
	}
	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		panic(err)
	}

	tokenExpiry := 10

	return &TokenGeneratorServer{
		router:      r,
		config:      cfg,
		urlPrefix:   urlPrefix,
		tokenExpiry: tokenExpiry,
		signingKey:  signKey,
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

		expiryDate := time.Now().Add(time.Minute * time.Duration(s.tokenExpiry))
		token, err := jwt_utils.CreateRsaToken("ima-client", "ima-group", expiryDate, s.signingKey)

		fmt.Println("token: ", token)
		fmt.Println("err: ", err)

		tokenResp := tokenResponse{
			AccessToken: token,
			TokenType:   "its_a_secret",
			ExpiresIn:   "time.Minute * time.Duration(s.tokenExpiry).String()",
		}
		writeJSONResponse(w, http.StatusOK, tokenResp)
	}
}
