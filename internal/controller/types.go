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
	SendMessage(context.Context, string, string, string, interface{}, interface{}) (*uuid.UUID, error)
	Ping(context.Context, string, string) error
	Close(context.Context) error
}

type ReceptorProxyFactory interface {
	CreateProxy(ctx context.Context, account domain.AccountID, client_id domain.ClientID) (Receptor, error)
}
