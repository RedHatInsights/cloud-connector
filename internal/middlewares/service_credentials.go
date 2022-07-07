package middlewares

import (
	"errors"
)

type serviceCredentials struct {
	clientID string
	orgID    string
	account  string
	psk      string
}

func newServiceCredentials(clientID, orgID, account, psk string) *serviceCredentials {
	return &serviceCredentials{
		clientID: clientID,
		orgID:    orgID,
		account:  account,
		psk:      psk,
	}
}

type serviceCredentialsValidator struct {
	knownServiceCredentials map[string]interface{}
}

func (scv *serviceCredentialsValidator) validate(sc *serviceCredentials) error {
	switch {
	case scv.knownServiceCredentials[sc.clientID] == nil:
		return errors.New(authErrorLogHeader + "Provided ClientID not attached to any known keys")
	case sc.psk != scv.knownServiceCredentials[sc.clientID]:
		return errors.New(authErrorLogHeader + "Provided PSK does not match known key for this client")
	}
	return nil
}
