package connection_repository

import (
	"context"

	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

type RegistrationResults int

const (
	NewConnection RegistrationResults = iota
	ExistingConnection
)

type ConnectionRegistrar interface {
	Register(context.Context, domain.RhcClient) (RegistrationResults, error)
	Unregister(context.Context, domain.ClientID)
}

type ConnectionLocator interface {
	GetConnection(context.Context, domain.AccountID, domain.ClientID) controller.Receptor
	GetConnectionsByAccount(context.Context, domain.AccountID) map[domain.ClientID]controller.Receptor
	GetAllConnections(context.Context) map[domain.AccountID]map[domain.ClientID]controller.Receptor
}
