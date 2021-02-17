package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type AccountIdResolver interface {
	MapClientIdToAccountId(context.Context, domain.ClientID) (domain.AccountID, error) //this should really be returning Identity header in 3scale format
}

//Fix me temporary
type BopResp struct {
	Mechanism string `json:"mechanism"`
	User      struct {
		OrgID         string `json:"org_id"`
		AccountNumber string `json:"account_number"`
		Type          string `json:"type"`
	} `json:"user"`
}

func NewAccountIdResolver(accountIdResolverImpl string, cfg *config.Config) (AccountIdResolver, error) {
	switch accountIdResolverImpl {
	case "config_file_based":
		resolver := ConfigurableAccountIdResolver{Config: cfg}
		err := resolver.init()
		return &resolver, err
	case "bop":
		return &BOPAccountIdResolver{cfg}, nil
	default:
		return nil, errors.New("Invalid AccountIdResolver impl requested")
	}
}

type BOPAccountIdResolver struct {
	Config *config.Config
}

func (bar *BOPAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.AccountID, error) {

	fmt.Println("Looking up the connection's account number in BOP")
	caCert, err := ioutil.ReadFile(bar.Config.BopCaFile)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	req, err := http.NewRequest("GET", bar.Config.BopUrl+"v1/auth", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("x-rh-insights-certauth-secret", bar.Config.BopCertAuthSecret)
	req.Header.Add("x-rh-certauth-issuer", bar.Config.BopCertIssuer)
	req.Header.Add("x-rh-apitoken", bar.Config.BopToken)
	req.Header.Add("x-rh-clientid", bar.Config.BopClientID)
	req.Header.Add("x-rh-insights-env", bar.Config.BopEnv)
	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-certauth-cn", fmt.Sprintf("/CN=%s", clientID))
	r, err := client.Do(req)
	defer r.Body.Close()
	if err != nil {
		return "", err
	}
	if r.StatusCode != 200 {
		b, _ := ioutil.ReadAll(r.Body)
		return "", fmt.Errorf("Unable to find account %s", string(b))
	}
	var resp BopResp
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return "", err
	}
	return domain.AccountID(resp.User.AccountNumber), nil
}

type ConfigurableAccountIdResolver struct {
	Config                 *config.Config
	clientIdToAccountIdMap map[domain.ClientID]domain.AccountID
	defaultAccountId       domain.AccountID
}

func (bar *ConfigurableAccountIdResolver) init() error {

	err := bar.loadAccountIdMapFromFile()
	if err != nil {
		return err
	}

	bar.defaultAccountId = domain.AccountID(bar.Config.ClientIdToAccountIdDefaultAccountId)

	return nil
}

func (bar *ConfigurableAccountIdResolver) loadAccountIdMapFromFile() error {

	logger.Log.Debug("Loading Client Id to Account Id config file: ", bar.Config.ClientIdToAccountIdConfigFile)

	configFile, err := os.Open(bar.Config.ClientIdToAccountIdConfigFile)
	if err != nil {
		logger.Log.Error("Could not load account resolver config file: ", err)
		return err
	}
	defer configFile.Close()

	jsonBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		logger.Log.Error("Could not load account resolver config file: ", err)
		return err
	}

	err = json.Unmarshal(jsonBytes, &bar.clientIdToAccountIdMap)
	if err != nil {
		logger.Log.Error("Could not parse account resolver config file: ", err)
		return err
	}

	return nil
}

func (bar *ConfigurableAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.AccountID, error) {

	if accountId, ok := bar.clientIdToAccountIdMap[clientID]; ok == true {
		return accountId, nil
	}

	return bar.defaultAccountId, nil
}
