package api

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type ConnectorClientHTTPProxyFactory struct {
	config *config.Config

	// FIXME: Add the cache here
	cache map[domain.ClientID]string
}

func NewConnectorClientHTTPProxyFactory(cfg *config.Config, cache map[domain.ClientID]string) (controller.ConnectorClientProxyFactory, error) {
	proxyFactory := ConnectorClientHTTPProxyFactory{config: cfg, cache: cache}
	return &proxyFactory, nil
}

func (this *ConnectorClientHTTPProxyFactory) CreateProxy(ctx context.Context, orgID domain.OrgID, account domain.AccountID, client_id domain.ClientID, canonicalFacts domain.CanonicalFacts, dispatchers domain.Dispatchers, tags domain.Tags) (controller.ConnectorClient, error) {

	// Look up connection in cache
	childCloudConnectorUrl, ok := this.cache[client_id]
	if !ok {
		return nil, fmt.Errorf("FIXME: child cloud-connector url not found in cache!")
	}

	logger := logger.Log.WithFields(logrus.Fields{"org_id": orgID, "account": account, "client_id": client_id})

	proxy := ConnectorClientHTTPProxy{
		Url:            childCloudConnectorUrl,
		Logger:         logger,
		Config:         this.config,
		OrgID:          orgID,
		AccountID:      account,
		ClientID:       client_id,
		CanonicalFacts: canonicalFacts,
		Dispatchers:    dispatchers,
		Tags:           tags,
	}

	return &proxy, nil
}
