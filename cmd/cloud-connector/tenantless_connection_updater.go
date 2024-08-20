package main

import (
	"context"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

func startTenantlessConnectionUpdater() {

	logger.Log.Info("Starting Cloud-Connector Tenantless Connection Updater")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	databaseConn, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Failed to connect to the database", err)
	}

	accountResolver, err := controller.NewAccountIdResolver(cfg.ClientIdToAccountIdImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Account ID Resolver", err)
	}

	sqlTimeout := cfg.ConnectionDatabaseQueryTimeout
	// FIXME
	tooOldIfBeforeThisTime := time.Now().Add(-1 * (30 * time.Minute))
	chunkSize := cfg.InventoryStaleTimestampUpdaterChunkSize

	logger.Log.Debug("Host's should be updated if their tenant_lookup_timestamp is before ", tooOldIfBeforeThisTime.UTC())

	connection_repository.ProcessTenantlessConnections(context.TODO(), databaseConn, sqlTimeout, tooOldIfBeforeThisTime, chunkSize,
		func(ctx context.Context, rhcClient domain.ConnectorClientState) error {

			log := logger.Log.WithFields(logrus.Fields{"client_id": rhcClient.ClientID, "account": rhcClient.Account, "org_id": rhcClient.OrgID})

			log.Debug("Processing tenantless connection")

			_, account, orgId, err := accountResolver.MapClientIdToAccountId(ctx, rhcClient.ClientID)
			if err != nil {

				if rhcClient.TenantLookupFailureCount > 2 { // FIXME: Make count configurable

					dberr := connection_repository.RecordMaximumTenantLookupFailures(ctx, databaseConn, sqlTimeout, rhcClient)
					if dberr != nil {
						logger.LogErrorWithAccountAndClientId("Unable to record failed tenant lookup for connection", dberr, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
					}
					return nil
				}

				logger.LogErrorWithAccountAndClientId("Unable to retrieve identity for connection", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
				dberr := connection_repository.RecordFailedTenantLookup(ctx, databaseConn, sqlTimeout, rhcClient)
				if dberr != nil {
					logger.LogErrorWithAccountAndClientId("Unable to record failed tenant lookup for connection", dberr, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
				}
				return err
			}

			rhcClient.Account = account
			rhcClient.OrgID = orgId

			logger.LogErrorWithAccountAndClientId("Successfully able to retrieve identity for connection", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
			dberr := connection_repository.RecordSuccessfulTenantLookup(ctx, databaseConn, sqlTimeout, rhcClient)
			if dberr != nil {
				logger.LogErrorWithAccountAndClientId("Unable to record successful tenant lookup for connection", dberr, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
			}

			return nil
		})
}
