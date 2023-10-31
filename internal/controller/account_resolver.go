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
	"strings"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru"

	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type AccountIdResolver interface {
	MapClientIdToAccountId(context.Context, domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error)
}

type AuthGwResp struct {
	Identity string `json:"x-rh-identity"`
}

type authGwErrorResponse struct {
	Errors []struct {
		Meta struct {
			ResponseBy string `json:"response_by"`
		} `json:"meta"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func (this authGwErrorResponse) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Gateway returned an error: ")
	for _, err := range this.Errors {
		fmt.Fprintf(&b, " (response_by: %s, status: %d, detail: %s)", err.Meta.ResponseBy, err.Status, err.Detail)
	}
	return b.String()
}

type ExpirableCachedAccountIdResolver struct {
	AccountIdResolver
	cache 		*lru.Cache
	cacheTTL 	time.Duration
	errorTTL 	time.Duration
}


func NewExpirableCachedAccountIdResolver(baseResolver AccountIdResolver, cacheSize int, cacheTTL, errorTTL time.Duration)(AccountIdResolver, error) {
	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	return &ExpirableCachedAccountIdResolver{
		AccountIdResolver: 	baseResolver,
		cache:				cache,
		cacheTTL: 			cacheTTL,
		errorTTL: 			errorTTL,
	},nil
}

func (ecar *ExpirableCachedAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error){
	//Check cache 
	cached, ok := ecar.cache.Get(clientID)
	if ok {
		cachedResult := cached.(cachedResult)

		now := time.Now()
		//Check if cached result is still valid 
		if cachedResult.err == nil && now.Sub(cachedResult.timestamp) < ecar.cacheTTL {
			return cachedResult.identity, cachedResult.accountID, cachedResult.orgID, nil
		}

		if now.Sub(cachedResult.timestamp) < ecar.errorTTL && cachedResult.err != nil {
			//if cach err is whin the erro ttl return it
			return "","","", cachedResult.err
		}
	}
	//if not in cache or cache expired, call base resolver
	identity, accountID, orgID, err := ecar.AccountIdResolver.MapClientIdToAccountId(ctx, clientID)

	cachedResult := cachedResult{
		identity: 	identity,
		accountID: 	accountID,
		orgID: 		orgID,
		timestamp: 	time.Now(),
		err: 		err,
	}
	 
	ecar.cache.Add(clientID, cachedResult)

	return identity, accountID, orgID, err
}

type cachedResult struct {
	identity 	domain.Identity
	accountID  	domain.AccountID
	orgID 		domain.OrgID
	timestamp   time.Time
	err			error
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

func (bar *BOPAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error) {

	callDurationTimer := prometheus.NewTimer(metrics.authGatewayAccountLookupDuration)
	defer callDurationTimer.ObserveDuration()
	requestID := uuid.NewString()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "request_id": requestID})

	logger.Debugf("Looking up the client %s account number in via Gateway", clientID)

	client := &http.Client{
		Timeout: bar.Config.AuthGatewayHttpClientTimeout,
	}

	req, err := http.NewRequest("GET", bar.Config.AuthGatewayUrl, nil)
	if err != nil {
		return "", "", "", err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-certauth-cn", fmt.Sprintf("/CN=%s", clientID))
	req.Header.Add("x-rh-insights-request-id", requestID)
	logger.Debug("About to call Auth Gateway")
	r, err := client.Do(req)
	logger.Debug("Returned from call to Auth Gateway")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to Auth Gateway failed")
		return "", "", "", err
	}
	defer r.Body.Close()

	metrics.authGatewayAccountLookupStatusCodeCounter.With(prometheus.Labels{
		"status_code": strconv.Itoa(r.StatusCode)}).Inc()

	if r.StatusCode != 200 {
		logger.Debugf("Call to Auth Gateway returned http status code %d", r.StatusCode)
		var errResponse authGwErrorResponse
		if err := json.NewDecoder(r.Body).Decode(&errResponse); err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse error reponse")
			return "", "", "", fmt.Errorf("Unable to find account: %w", err)
		}
		return "", "", "", fmt.Errorf("Unable to find account: %s", errResponse)
	}

	var resp AuthGwResp
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse Auth Gateway response")
		return "", "", "", err
	}
	idRaw, err := base64.StdEncoding.DecodeString(resp.Identity)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to decode identity from Auth Gateway")
		return "", "", "", err
	}

	var jsonData identity.XRHID
	err = json.Unmarshal(idRaw, &jsonData)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse identity from Auth Gateway")
		return "", "", "", err
	}

	logger.WithFields(logrus.Fields{"account": jsonData.Identity.AccountNumber, "org_id": jsonData.Identity.Internal.OrgID}).Debug("Located account number and org ID for client")

	return domain.Identity(resp.Identity), domain.AccountID(jsonData.Identity.AccountNumber), domain.OrgID(jsonData.Identity.Internal.OrgID), nil
}

type ConfigurableAccountIdResolver struct {
	Config                 *config.Config
	clientIdToAccountIdMap map[domain.ClientID]struct {
		AccountId domain.AccountID `json:"accountId"`
		OrgId     domain.OrgID     `json:"orgId"`
	}
	defaultAccountId domain.AccountID
	defaultOrgId     domain.OrgID
}

func (bar *ConfigurableAccountIdResolver) init() error {

	err := bar.loadAccountIdMapFromFile()
	if err != nil {
		return err
	}

	bar.defaultAccountId = domain.AccountID(bar.Config.ClientIdToAccountIdDefaultAccountId)
	bar.defaultOrgId = domain.OrgID(bar.Config.ClientIdToAccountIdDefaultOrgId)

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

func (bar *ConfigurableAccountIdResolver) createIdentityHeader(account domain.AccountID, org_id domain.OrgID) domain.Identity {
	identityJson := fmt.Sprintf(`
        {"identity":
            {
            "type": "User",
            "auth_type": "cert-auth",
            "account_number": "%s",
            "org_id": "%s",
            "internal":
                {"org_id": "%s"},
            "user":
                {"email": "fred@flintstone.com", "is_org_admin": true}
            }
        }`,
		string(account),
		string(org_id),
		string(org_id))
	identityJsonBase64 := base64.StdEncoding.EncodeToString([]byte(identityJson))
	return domain.Identity(identityJsonBase64)
}

func (bar *ConfigurableAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error) {

	if account, ok := bar.clientIdToAccountIdMap[clientID]; ok == true {
		return bar.createIdentityHeader(account.AccountId, account.OrgId), account.AccountId, account.OrgId, nil
	}

	return bar.createIdentityHeader(bar.defaultAccountId, bar.defaultOrgId), bar.defaultAccountId, bar.defaultOrgId, nil
}
