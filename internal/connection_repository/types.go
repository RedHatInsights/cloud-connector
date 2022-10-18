package connection_repository

import (
	"context"
	"errors"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/sirupsen/logrus"
)

// Connection DAO for db operations on connections table
type Connection struct {
	Id             *int
	Account        domain.AccountID
	ClientId       domain.ClientID
	Dispatchers    string
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
	CanonicalFacts string
	StaleTimestamp *time.Time
	Tags           string
	MessageId      string
	MessageSent    *time.Time
	OrgId          domain.OrgID
}

type FatalError struct {
	Err error
}

func (fe FatalError) Error() string { return "FATAL: " + fe.Err.Error() }

var NotFoundError = errors.New("Not found")
var InvalidOrgIDError = errors.New("Invalid OrgID")
var InvalidClientIDError = errors.New("Invalid ClientID")

type ConnectionRegistrar interface {
	Register(context.Context, domain.ConnectorClientState) error
	Unregister(context.Context, domain.ClientID) error
	FindConnectionByClientID(context.Context, domain.ClientID) (domain.ConnectorClientState, error)
}

type ConnectionLocator interface {
	GetConnection(context.Context, domain.AccountID, domain.OrgID, domain.ClientID) controller.ConnectorClient
	GetConnectionsByAccount(context.Context, domain.AccountID, int, int) (map[domain.ClientID]controller.ConnectorClient, int, error)
	GetAllConnections(context.Context, int, int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error)
}

type GetConnectionByClientID func(context.Context, *logrus.Entry, domain.OrgID, domain.ClientID) (domain.ConnectorClientState, error)
type GetConnectionsByOrgID func(context.Context, *logrus.Entry, domain.OrgID, int, int) (map[domain.ClientID]domain.ConnectorClientState, int, error)
