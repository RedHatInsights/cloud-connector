package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
)

type errorResponse struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func writeJSONResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Unable to encode payload!", http.StatusUnprocessableEntity)
		log.Println("Unable to encode payload!")
	}
}

func decodeJSON(body io.ReadCloser, data interface{}) error {
	dec := json.NewDecoder(body)
	if err := dec.Decode(&data); err != nil {
		// FIXME: More specific error handling needed.. case statement for different scenarios?
		return errors.New("Request body includes malformed json")
	}

	v := validator.New()
	if err := v.Struct(data); err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			log.Println(e)
		}
		return errors.New("Request body is missing required fields")
	} else if dec.More() {
		return errors.New("Request body must only contain one json object")
	}

	return nil
}

func writeInvalidInputResponse(logger *logrus.Entry, w http.ResponseWriter, err error) {
	errMsg := "Unable to process input parameters"
	logger.WithFields(logrus.Fields{"error": err}).Error(errMsg)
	errorResponse := errorResponse{Title: errMsg,
		Status: http.StatusBadRequest,
		Detail: err.Error()}
	writeJSONResponse(w, errorResponse.Status, errorResponse)
}

func getIntFromQueryParams(req *http.Request, paramName string, defaultValue int) (int, error) {
	value := req.URL.Query().Get(paramName)
	if value == "" {
		return defaultValue, nil
	}

	return strconv.Atoi(value)
}

func getLimitFromQueryParams(req *http.Request) (int, error) {
	limit, err := getIntFromQueryParams(req, "limit", 1000)
	if err != nil {
		return 0, errors.New("limit: " + err.Error())
	}

	if limit < 0 {
		return 0, errors.New("limit: must be > 0")
	}

	return limit, err
}

func getOffsetFromQueryParams(req *http.Request) (int, error) {
	offset, err := getIntFromQueryParams(req, "offset", 0)
	if err != nil {
		return 0, errors.New("offset: " + err.Error())
	}

	if offset < 0 {
		return 0, errors.New("offset: must be >= 0")
	}

	return offset, nil
}

func getOffsetAndLimitFromQueryParams(req *http.Request) (offset int, limit int, err error) {
	limit, err = getLimitFromQueryParams(req)
	if err != nil {
		return 0, 0, err
	}

	offset, err = getOffsetFromQueryParams(req)
	if err != nil {
		return 0, 0, err
	}

	return offset, limit, nil
}

func createConnectorClientProxy(ctx context.Context, log *logrus.Entry, tenantTranslator tenantid.Translator, getConnectionByClientID connection_repository.GetConnectionByClientID, proxyFactory controller.ConnectorClientProxyFactory, account domain.AccountID, clientId domain.ClientID) (controller.ConnectorClient, error) {

	resolvedOrgId, err := tenantTranslator.EANToOrgID(ctx, string(account))
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to translate account (%s) to org_id", account)
		return nil, err
	}

	log.Infof("Translated account %s to org_id %s", account, resolvedOrgId)

	clientState, err := getConnectionByClientID(ctx, log, domain.OrgID(resolvedOrgId), clientId)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to locate connection (%s:%s)", resolvedOrgId, clientId)
		return nil, err
	}

	proxy, err := proxyFactory.CreateProxy(ctx, clientState)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Errorf("Unable to create proxy for connection (%s:%s)", resolvedOrgId, clientId)
		return nil, err
	}

	return proxy, nil
}
