package controller

import (
	"context"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/google/uuid"
)

var (
	ErrUnableToSendMessage = errors.New("unable to send message")
	ErrDisconnectedNode    = errors.New("disconnected node")
)

type Receptor interface {
	SendMessage(context.Context, domain.AccountID, domain.ClientID, string, interface{}, interface{}) (*uuid.UUID, error)
	Ping(context.Context, domain.AccountID, domain.ClientID) error
	Reconnect(context.Context, domain.AccountID, domain.ClientID, string, int) error
	GetDispatchers(context.Context) (domain.Dispatchers, error)
	Disconnect(context.Context, string) error
}

type ReceptorProxyFactory interface {
	CreateProxy(context.Context, domain.AccountID, domain.ClientID, domain.Dispatchers) (Receptor, error)
}

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
	GetConnection(context.Context, domain.AccountID, domain.ClientID) Receptor
	GetConnectionsByAccount(context.Context, domain.AccountID) map[domain.ClientID]Receptor
	GetAllConnections(context.Context) map[domain.AccountID]map[domain.ClientID]Receptor
}
