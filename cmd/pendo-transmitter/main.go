package main

import (
	"bytes"
	"fmt"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	cr "github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

var accInfo []accountInfo

type accountInfo struct {
	AccountID	domain.AccountID	`json:"accountId"`
	Values		connectionValue	`json:"values"`
}

type connectionValue struct {
	ConnCount  int	`json:"connectionCount"`
}


func connectionCountProcessor(ctx context.Context, account domain.AccountID, count int) error {
	fmt.Printf("%s - %d\n", account, count)
	accInfo = append(accInfo, accountInfo{AccountID: account, Values: connectionValue{ConnCount: count}})
	return nil
}

func makeRequst(endpoint string, timeout time.Duration, apiKey string, requestSize int) {
	for i := 0; i < len(accInfo); i += requestSize {
		end := i + requestSize

		if end > len(accInfo) {
			end = len(accInfo)
		}

		reqBody, err := json.Marshal(accInfo[i:end])

		if err != nil {
			logger.Log.Error(err)
		}

		url := endpoint+"/metadata/account/custom/value"

		client := http.Client{
			Timeout: timeout,
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		req.Header.Set("content-type", "application/json")
		req.Header.Set("x-pendo-integration-key", apiKey)

		if err != nil {
			logger.Log.Error(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			logger.Log.Error(err)
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		switch {
		case err != nil:
			logger.Log.Error(err)
		case len(body) == 0:
			logger.Log.Error("Invalid Request. API key may be invalid.")
		}
		logger.Log.Info(string(body))
	}
}


func main() {
	logger.InitLogger()

	logger.Log.Info("Starting Cloud-Connector Pendo Transmitter")

	cfg := config.GetConfig()

	if cfg.PendoIntegrationKey == "" {
		logger.Log.Fatal("No Pendo Integration key.")
	}

	cr.StartConnectedAccountReport("477931,6089719,540155", connectionCountProcessor)

	makeRequst(cfg.PendoApiEndpoint, cfg.PendoRequestTimeout, cfg.PendoIntegrationKey, cfg.PendoRequestSize)
}

