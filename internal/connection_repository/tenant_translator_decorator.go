package connection_repository

import (
	"context"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"

	"github.com/sirupsen/logrus"
)

// The TenantTranslatorDecorator is only responsible for converting an account
// number to an org_id and calling the wrapped connection locator.
type TenantTranslatorDecorator struct {
	tenantTranslator         tenantid.Translator
	wrappedConnectionLocator ConnectionLocator
}

func NewTenantTranslatorDecorator(cfg *config.Config, tenantTranslator tenantid.Translator, connectionLocator ConnectionLocator) (ConnectionLocator, error) {
	return &TenantTranslatorDecorator{
		wrappedConnectionLocator: connectionLocator,
		tenantTranslator:         tenantTranslator,
	}, nil
}

func (this *TenantTranslatorDecorator) GetConnection(ctx context.Context, account domain.AccountID, orgId domain.OrgID, clientId domain.ClientID) controller.ConnectorClient {

	log := logger.Log.WithFields(logrus.Fields{"client_id": clientId,
		"account": account,
		"org_id":  orgId})

	if orgId == "" {
		resolvedOrgId, err := this.tenantTranslator.EANToOrgID(ctx, string(account))
		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Unable to translate account to org_id")
			return nil
		}

		log.Infof("Translated account %s to org_id %s", account, resolvedOrgId)

		orgId = domain.OrgID(resolvedOrgId)
	}

	return this.wrappedConnectionLocator.GetConnection(ctx, account, orgId, clientId)
}

func (this *TenantTranslatorDecorator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {
	return nil, 0, errors.New("Not implemented!")
}
