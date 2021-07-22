package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ConnectionProcessor func(context.Context, domain.RhcClient) error

func ProcessStaleConnections(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, staleTimeCutoff time.Time, chunkSize int, processConnection ConnectionProcessor) error {

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: databaseConn}),
		&gorm.Config{})

	if err != nil {
		logger.LogFatalError("Gorm open failed", err)
		return nil
	}

	query := db.Table("connections")

	query.Where("canonical_facts != '{}'")
	query.Where("dispatchers ? 'rhc-worker-playbook'")
	query.Where("stale_timestamp < ?", staleTimeCutoff)
	query.Order("stale_timestamp asc")
	query.Limit(chunkSize)

	var totalConnectionsFound int64
	var connections []Connection

	results := query.WithContext(queryCtx).Find(&connections).Count(&totalConnectionsFound)

	if results.Error != nil {
		logger.LogFatalError("SQL query failed", results.Error)
		return nil
	}

	for _, connection := range connections {
		var account domain.AccountID = domain.AccountID(connection.Account)
		var clientId domain.ClientID = domain.ClientID(connection.ClientID)

		var canonicalFacts interface{}
		err = json.Unmarshal([]byte(connection.CanonicalFacts), &canonicalFacts)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts.  Skipping connection.", err, account, clientId)
			continue
		}

		var tags domain.Tags
		err = json.Unmarshal([]byte(connection.Tags), &tags)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to parse tags.  Skipping connection.", err, account, clientId)
			continue
		}

		rhcClient := domain.RhcClient{
			Account:        account,
			ClientID:       clientId,
			CanonicalFacts: canonicalFacts,
			Tags:           tags,
		}

		processConnection(ctx, rhcClient)
	}

	return nil
}

func UpdateStaleTimestampInDB(log *logrus.Entry, ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, rhcClient domain.RhcClient) {

	log.Debug("Updating stale timestamp")

	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: databaseConn}),
		&gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	query := db.Table("connections")

	results := query.Model(&Connection{}).Where("account = ?", string(rhcClient.Account)).Where("client_id = ?", string(rhcClient.ClientID)).Update("stale_timestamp", "NOW()")

	if results.Error != nil {
		log.Fatal(err)
	}

	log.Debug("rowsAffected:", results.RowsAffected)
}
