package controller

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

type AccountIdResolver interface {
	MapClientIdToAccountId(context.Context, domain.ClientID) (domain.AccountID, error)
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
}

func (bar *ConfigurableAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.AccountID, error) {
	// FIXME:  Make this configurable...this should be helpful in testing until we can get BOP / 3scale wired up correctly
	switch clientID {
	case "client-0":
		return domain.AccountID("010101"), nil
	case "client-1":
		return domain.AccountID("010102"), nil
	default:
		return domain.AccountID("0000001"), nil
	}
}
