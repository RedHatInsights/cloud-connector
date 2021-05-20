package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type AccountIdResolver interface {
	MapClientIdToAccountId(context.Context, domain.ClientID) (domain.Identity, domain.AccountID, error)
}

type AuthGwResp struct {
	Identity string `json:"x-rh-identity"`
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

func (bar *BOPAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, error) {

	callDurationTimer := prometheus.NewTimer(metrics.authGatewayAccountLookupDuration)
	defer callDurationTimer.ObserveDuration()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debugf("Looking up the client %s account number in via Gateway", clientID)

	client := &http.Client{
		Timeout: bar.Config.AuthGatewayHttpClientTimeout,
	}

	req, err := http.NewRequest("GET", bar.Config.AuthGatewayUrl, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-certauth-cn", fmt.Sprintf("/CN=%s", clientID))
	logger.Debug("About to call Auth Gateway")
	r, err := client.Do(req)
	logger.Debug("Returned from call to Auth Gateway")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to Auth Gateway failed")
		return "", "", err
	}
	defer r.Body.Close()

	metrics.authGatewayAccountLookupStatusCodeCounter.With(prometheus.Labels{
		"status_code": strconv.Itoa(r.StatusCode)}).Inc()

	if r.StatusCode != 200 {
		logger.Debugf("Call to Auth Gateway returned http status code %d", r.StatusCode)
		b, _ := ioutil.ReadAll(r.Body)
		return "", "", fmt.Errorf("Unable to find account %s", string(b))
	}
	var resp AuthGwResp
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse Auth Gateway response")
		return "", "", err
	}
	idRaw, err := base64.StdEncoding.DecodeString(resp.Identity)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to decode identity from Auth Gateway")
		return "", "", err
	}

	var jsonData identity.XRHID
	err = json.Unmarshal(idRaw, &jsonData)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse identity from Auth Gateway")
		return "", "", err
	}

	logger.WithFields(logrus.Fields{"account": jsonData.Identity.AccountNumber}).Debug("Located account number for client")

	return domain.Identity(resp.Identity), domain.AccountID(jsonData.Identity.AccountNumber), nil
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

func (bar *ConfigurableAccountIdResolver) createIdentityHeader(account domain.AccountID) domain.Identity {
	identityJson := fmt.Sprintf(`
        {"identity":
            {
            "type": "User",
            "auth_type": "cert-auth",
            "account_number": "%s",
            "internal":
                {"org_id": "%s"},
            "user":
                {"email": "fred@flintstone.com", "is_org_admin": true}
            }
        }`,
		string(account),
		string(account))
	identityJsonBase64 := base64.StdEncoding.EncodeToString([]byte(identityJson))
	return domain.Identity(identityJsonBase64)
}

func (bar *ConfigurableAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, error) {

	if accountId, ok := bar.clientIdToAccountIdMap[clientID]; ok == true {
		return bar.createIdentityHeader(accountId), accountId, nil
	}

	return bar.createIdentityHeader(bar.defaultAccountId), bar.defaultAccountId, nil
}
