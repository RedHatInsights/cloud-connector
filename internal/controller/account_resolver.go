package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type AccountIdResolver interface {
	MapClientIdToAccountId(context.Context, domain.ClientID) (domain.AccountID, error)
}

func NewAccountIdResolver(accountIdResolverImpl string, cfg *config.Config) (AccountIdResolver, error) {
	switch accountIdResolverImpl {
	case "config_file_based":
		resolver := ConfigurableAccountIdResolver{Config: cfg}
		err := resolver.init()
		return &resolver, err
	case "bop":
		return &BOPAccountIdResolver{}, nil
	default:
		return nil, errors.New("Invalid AccountIdResolver impl requested")
	}
}

type BOPAccountIdResolver struct {
}

func (bar *BOPAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.AccountID, error) {
	// FIXME: need to lookup the account number for the connected client
	fmt.Println("FIXME: looking up the connection's account number in BOP")

	/*
		Required
		x-rh-apitoken *
		x-rh-clientid

		Optional
		x-rh-insights-env

		Cert auth
		x-rh-certauth-cn
		x-rh-certauth-issuer
		x-rh-insights-certauth-secret

		make http GET



	*/

	return "010101", nil
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
