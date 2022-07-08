package middlewares

import (
	"context"
	"errors"
	"net/http"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/sirupsen/logrus"
)

const (
	authErrorMessage   = "Authentication failed"
	authErrorLogHeader = "Authentication error: "
	identityHeader     = "x-rh-identity"
	PSKClientIdHeader  = "x-rh-cloud-connector-client-id"
	PSKOrgIdHeader     = "x-rh-cloud-connector-org-id"
	PSKAccountHeader   = "x-rh-cloud-connector-account"
	PSKHeader          = "x-rh-cloud-connector-psk"
)

type RequiredTenantIdentifier int

const (
	Account RequiredTenantIdentifier = iota
	OrgID
)

type AuthMiddleware struct {
	Secrets                  map[string]interface{}
	IdentityAuth             func(http.Handler) http.Handler
	RequiredTenantIdentifier RequiredTenantIdentifier
}

// Authenticate determines which authentication method should be used, and delegates identity header
// auth to the identity middleware
func (amw *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(identityHeader) != "" {
			// identity header auth
			amw.IdentityAuth(next).ServeHTTP(w, r)
		} else {
			handlePSKAuthentication(next, w, r, amw.Secrets, amw.RequiredTenantIdentifier)
		}
	})
}

func handlePSKAuthentication(next http.Handler, w http.ResponseWriter, r *http.Request, secrets map[string]interface{}, requiredTenant RequiredTenantIdentifier) {

	clientID, orgID, account, psk, err := retrieveHeaderValues(r, requiredTenant)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Debug("Authentication failure")
		http.Error(w, authErrorMessage, 401)
		return
	}

	sr := newServiceCredentials(
		clientID,
		orgID,
		account,
		psk,
	)

	logger.Log.Debugf("Received service to service request from %s using account:%s and org_id:%s", sr.clientID, sr.account, sr.orgID)

	validator := serviceCredentialsValidator{knownServiceCredentials: secrets}
	if err := validator.validate(sr); err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Debug("Authentication failure")
		http.Error(w, authErrorMessage, 401)
		return
	}

	principal := serviceToServicePrincipal{account: sr.account, clientID: sr.clientID, orgID: sr.orgID}

	ctx := context.WithValue(r.Context(), principalKey, principal)

	next.ServeHTTP(w, r.WithContext(ctx))
}

func retrieveHeaderValues(r *http.Request, requiredTenant RequiredTenantIdentifier) (clientID string, orgID string, account string, psk string, err error) {
	clientID, err = verifyHeader(r, PSKClientIdHeader, true)
	if err != nil {
		return clientID, orgID, account, psk, err
	}

	orgID, err = verifyHeader(r, PSKOrgIdHeader, requiredTenant == OrgID)
	if err != nil {
		return clientID, orgID, account, psk, err
	}

	account, err = verifyHeader(r, PSKAccountHeader, requiredTenant == Account)
	if err != nil {
		return clientID, orgID, account, psk, err
	}

	psk, err = verifyHeader(r, PSKHeader, true)
	if err != nil {
		return clientID, orgID, account, psk, err
	}

	return clientID, orgID, account, psk, err
}

func verifyHeader(r *http.Request, header string, required bool) (string, error) {
	value := r.Header.Get(header)

	if required == true && value == "" {
		return "", errors.New(authErrorLogHeader + "Missing " + header + " header")
	}

	return value, nil
}
