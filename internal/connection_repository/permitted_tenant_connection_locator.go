package connection_repository

import (
	"context"
	"errors"
	"gorm.io/gorm"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type PermittedTenantConnectionLocator struct {
	proxyFactory            controller.ConnectorClientProxyFactory
	getConnectionByClientID GetConnectionByClientID
}

func NewPermittedTenantConnectionLocator(cfg *config.Config, database *gorm.DB, proxyFactory controller.ConnectorClientProxyFactory) (*PermittedTenantConnectionLocator, error) {
	getConnectionByClientID, err := NewPermittedTenantSqlGetConnectionByClientID(cfg, database)

	if err != nil {
		return nil, err
	}

	return &PermittedTenantConnectionLocator{
		proxyFactory:            proxyFactory,
		getConnectionByClientID: getConnectionByClientID,
	}, nil
}

func (pacl *PermittedTenantConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, org_id domain.OrgID, client_id domain.ClientID) controller.ConnectorClient {

	log := logger.Log.WithFields(logrus.Fields{"client_id": client_id,
		"account": account,
		"org_id":  org_id})

	clientState, err := pacl.getConnectionByClientID(ctx, log, org_id, client_id)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Error("Unable to locate connection")
		return nil
	}

	conn, err := pacl.proxyFactory.CreateProxy(ctx, clientState.OrgID, clientState.Account, clientState.ClientID, clientState.Dispatchers)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Error("Unable to create the proxy")
		return nil
	}

	return conn
}

func (pacl *PermittedTenantConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {
	return nil, 0, errors.New("Not implemented!")
}

func (pacl *PermittedTenantConnectionLocator) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {
	return nil, 0, errors.New("Not implemented!")
}
