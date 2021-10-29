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

type ConnectorClient interface {
	SendMessage(context.Context, string, interface{}, interface{}) (*uuid.UUID, error)
	Ping(context.Context) error
	Reconnect(context.Context, string, int) error
	GetDispatchers(context.Context) (domain.Dispatchers, error)
	Disconnect(context.Context, string) error
}

type ConnectorClientProxyFactory interface {
	CreateProxy(context.Context, domain.AccountID, domain.ClientID, domain.Dispatchers) (ConnectorClient, error)
}
