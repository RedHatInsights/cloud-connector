package connection_repository

import (
	"context"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

type FatalError struct {
	Err error
}

func (fe FatalError) Error() string { return "FATAL: " + fe.Err.Error() }

var NotFoundError = errors.New("Not found")

type ConnectionRegistrar interface {
	Register(context.Context, domain.ConnectorClientState) error
	Unregister(context.Context, domain.ClientID) error
	FindConnectionByClientID(context.Context, domain.ClientID) (domain.ConnectorClientState, error)
}

type ConnectionLocator interface {
	GetConnection(context.Context, domain.AccountID, domain.ClientID) controller.ConnectorClient
	GetConnectionsByAccount(context.Context, domain.AccountID, int, int) (map[domain.ClientID]controller.ConnectorClient, int, error)
	GetAllConnections(context.Context, int, int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error)
}
