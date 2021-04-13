package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type SourcesRecorder interface {
	RegisterWithSources(identity domain.Identity, account domain.AccountID, client domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error
}

func NewSourcesRecorder(impl string, cfg *config.Config) (SourcesRecorder, error) {
	switch impl {
	case "sources":
		return &SourcesRecorderImpl{config: cfg}, nil
	case "fake":
		return &FakeSourcesRecorder{}, nil
	default:
		return nil, errors.New("Invalid SourcesRecorder impl requested")
	}
}

type SourcesRecorderImpl struct {
	config *config.Config
}

type sourceEntry struct {
	SourceRef  string `json:"source_ref"`
	SourceName string `json:"name"`
	SourceType string `json:"source_type_name"`
}

type applicationEntry struct {
	SourceName      string `json:"source_name"`
	ApplicationType string `json:"application_type_name"`
}

type endpointEntry struct {
	SourceName string `json:"source_name"`
	ClientID   string `json:"receptor_node"`
}

type sourcesBulkOperation struct {
	Sources      []sourceEntry      `json:"sources"`
	Applications []applicationEntry `json:"applications"`
	Endpoints    []endpointEntry    `json:"endpoints"`
}

func (sri *SourcesRecorderImpl) RegisterWithSources(identity domain.Identity, account domain.AccountID, clientID domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "account": account})

	sourceEntryExists, err := sri.checkForExistingSourcesEntry(logger, identity, sourceRef)

	if err != nil {
		// Just log the error and try to create the sources entry
		logger.WithFields(logrus.Fields{
			"error":       err,
			"source_ref":  sourceRef,
			"source_name": sourceName,
		}).Error("Unable to find catalog entry in sources")
	}

	if sourceEntryExists == true {
		logger.WithFields(logrus.Fields{
			"source_ref":  sourceRef,
			"source_name": sourceName,
		}).Debug("Sources entry already exists")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"source_ref":  sourceRef,
		"source_name": sourceName,
	}).Debug("Sources entry does not exist...proceeding with creation of sources entry")

	return sri.createSourcesEntry(logger, identity, clientID, sourceRef, sourceName, sourceType, applicationType)
}

func (sri *SourcesRecorderImpl) createSourcesEntry(logger *logrus.Entry, identity domain.Identity, clientID domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error {

	requestID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	logger = logger.WithFields(logrus.Fields{"request_id": requestID})

	source := sourceEntry{SourceRef: sourceRef, SourceName: sourceName, SourceType: sourceType}
	application := applicationEntry{SourceName: sourceName, ApplicationType: applicationType}
	endpoint := endpointEntry{SourceName: sourceName, ClientID: string(clientID)}

	bulkOp := sourcesBulkOperation{Sources: []sourceEntry{source},
		Applications: []applicationEntry{application},
		Endpoints:    []endpointEntry{endpoint},
	}

	jsonBytes, err := json.Marshal(bulkOp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to marshal source bulk op into json")
		return err
	}

	url := fmt.Sprintf("%s/api/sources/v3.1/bulk_create", sri.config.SourcesBaseUrl)

	logger.Debug("Sources url:", url)

	resp, err := makeHttpRequest(
		context.TODO(),
		identity,
		requestID.String(),
		http.MethodPost,
		url,
		bytes.NewBuffer(jsonBytes),
		sri.config.SourcesHttpClientTimeout,
	)

	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to create sources entry")
		return err
	}

	defer resp.Body.Close()

	logger.Debug("Sources bulk create - HTTP status code:", resp.StatusCode)

	if resp.StatusCode == http.StatusBadRequest {
		logger.WithFields(logrus.Fields{"error": resp.Body}).Error("Unable to create sources")
		// source was already created
		return nil
	}

	if resp.StatusCode != http.StatusCreated {
		logger.WithFields(logrus.Fields{"error": resp.Body, "http_status": resp.StatusCode}).Error("Unable to create sources")
		return errors.New("Unable to create sources")
	}

	return nil
}

type getSourcesResponse struct {
	Metadata interface{}   `json:"meta"`
	Data     []interface{} `json:"data"`
}

func (sri *SourcesRecorderImpl) checkForExistingSourcesEntry(logger *logrus.Entry, identity domain.Identity, sourceRef string) (bool, error) {
	requestID, err := uuid.NewRandom()
	if err != nil {
		return false, err
	}

	logger = logger.WithFields(logrus.Fields{"request_id": requestID})

	url := fmt.Sprintf("%s/api/sources/v3.0/sources?source_ref=%s", sri.config.SourcesBaseUrl, sourceRef)

	logger.Debug("Sources url:", url)

	resp, err := makeHttpRequest(
		context.TODO(),
		identity,
		requestID.String(),
		http.MethodGet,
		url,
		nil,
		sri.config.SourcesHttpClientTimeout,
	)

	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to lookup sources entry")
		return false, err
	}

	defer resp.Body.Close()

	logger.Debug("Sources existence check - HTTP status code:", resp.StatusCode)

	getSourcesResponse := getSourcesResponse{}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&getSourcesResponse); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse sources GET response")
		return false, errors.New("Unable to parse sources GET response")
	}
	logger.Debugf("sources response:%+v\n", getSourcesResponse)

	return len(getSourcesResponse.Data) > 0, nil
}

func makeHttpRequest(ctx context.Context, identity domain.Identity, requestID, method, url string, body io.Reader, timeout time.Duration) (*http.Response, error) {

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-rh-identity", string(identity))

	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("x-rh-insights-request-id", requestID)

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))

	return resp, err
}

type FakeSourcesRecorder struct {
}

func (f *FakeSourcesRecorder) RegisterWithSources(identity domain.Identity, account domain.AccountID, clientID domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error {
	logger.Log.Debug("FAKE ... registering with sources:", account, clientID, sourceRef, sourceName)
	return nil
}
