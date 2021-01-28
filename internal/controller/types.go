package controller

import (
	"context"
	"errors"

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
