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
