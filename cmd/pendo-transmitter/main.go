package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	cr "github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

var accInfo []accountInfo
var cfg *config.Config

type accountInfo struct {
	AccountID domain.AccountID `json:"accountId"`
	Values    connectionValue  `json:"values"`
}

type connectionValue struct {
	ConnCount int `json:"connectionCount"`
}

func connectionCountProcessor(ctx context.Context, account domain.AccountID, count int) error {
	fmt.Printf("%s - %d\n", account, count)
	accInfo = append(accInfo, accountInfo{AccountID: account, Values: connectionValue{ConnCount: count}})

	if len(accInfo) >= cfg.PendoRequestSize {
		requestHandler(cfg.PendoApiEndpoint, cfg.PendoRequestTimeout, cfg.PendoIntegrationKey)
		accInfo = nil
	}
	return nil
}

func requestHandler(endpoint string, timeout time.Duration, apiKey string) {
	info, err := makeRequest(endpoint, timeout, apiKey)

	if err != nil {
		logger.Log.Error(err)
	} else {
		logger.Log.Info(info)
	}
}

func makeRequest(endpoint string, timeout time.Duration, apiKey string) (string, error) {
	reqBody, err := json.Marshal(accInfo)

	if err != nil {
		return "", err
	}

	url := endpoint + "/metadata/account/custom/value"

	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-pendo-integration-key", apiKey)

	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)

	switch {
	case err != nil:
		return "", err
	case resp.StatusCode != 200:
		return "", fmt.Errorf("Pendo Request Unsuccessful. Status: %v", resp.StatusCode)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func main() {
	logger.InitLogger()

	logger.Log.Info("Starting Cloud-Connector Pendo Transmitter")

	cfg = config.GetConfig()

	if cfg.PendoIntegrationKey == "" {
		logger.Log.Fatal("No Pendo Integration key.")
	}

	cr.StartConnectedAccountReport("477931,6089719,540155", connectionCountProcessor)

	if len(accInfo) > 0 {
		requestHandler(cfg.PendoApiEndpoint, cfg.PendoRequestTimeout, cfg.PendoIntegrationKey)
	}
}
