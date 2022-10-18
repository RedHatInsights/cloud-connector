package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type ConnectionProcessor func(context.Context, domain.ConnectorClientState) error

func ProcessStaleConnections(ctx context.Context, databaseConn *gorm.DB, sqlTimeout time.Duration, staleTimeCutoff time.Time, chunkSize int, processConnection ConnectionProcessor) error {

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	rows, err := databaseConn.WithContext(queryCtx).
		Table("connections").
		Select("account", "org_id", "client_id", "canonical_facts", "tags", "dispatchers").
		Where("canonical_facts != '{}'").
		Where(databaseConn.Where(datatypes.JSONQuery("dispatchers").HasKey("rhc-worker-playbook")).Or(datatypes.JSONQuery("dispatchers").HasKey("package-manager"))).
		Where("stale_timestamp < ?", staleTimeCutoff).
		Order("stale_timestamp asc").
		Limit(chunkSize).
		Rows()

	if err != nil {
		logger.LogFatalError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var orgID sql.NullString
		var canonicalFactsString sql.NullString
		var tagsString sql.NullString
		var dispatchersString sql.NullString

		var connectorClient domain.ConnectorClientState

		if err := rows.Scan(&connectorClient.Account, &orgID, &connectorClient.ClientID, &canonicalFactsString, &tagsString, &dispatchersString); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		if orgID.Valid {
			connectorClient.OrgID = domain.OrgID(orgID.String)
		}

		if dispatchersString.Valid {
			err = json.Unmarshal([]byte(dispatchersString.String), &connectorClient.Dispatchers)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse dispatchers from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		if canonicalFactsString.Valid {
			err = json.Unmarshal([]byte(canonicalFactsString.String), &connectorClient.CanonicalFacts)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		if tagsString.Valid {
			err = json.Unmarshal([]byte(tagsString.String), &connectorClient.Tags)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse tags from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		processConnection(ctx, connectorClient)
	}

	return nil
}

func UpdateStaleTimestampInDB(log *logrus.Entry, ctx context.Context, databaseConn *gorm.DB, sqlTimeout time.Duration, rhcClient domain.ConnectorClientState) {

	log.Debug("Updating stale timestamp")

	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	var now = time.Now()

	result := databaseConn.WithContext(ctx).
		Table("connections").
		Where("account = ?", rhcClient.Account).
		Where("client_id = ?", rhcClient.ClientID).
		Updates(&Connection{
			StaleTimestamp: &now,
		})

	if result.Error != nil {
		log.Fatal(result.Error)
	}

	log.Debug("rowsAffected:", result.RowsAffected)
}
