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
	SendMessage(context.Context, string, string, interface{}, string) (*uuid.UUID, error)
	Close(context.Context) error
}
