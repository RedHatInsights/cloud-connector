package connection_repository

import (
	"context"
	"gorm.io/gorm"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type ConnectionCountProcessor func(context.Context, domain.AccountID, int) error

func ProcessConnectionCounts(ctx context.Context, databaseConn *gorm.DB, sqlTimeout time.Duration, accountsToExclude []string, processConnectionCount ConnectionCountProcessor) error {

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	query := databaseConn.WithContext(queryCtx).
		Table("connections").
		Select("account", "COUNT(1) AS connection_count").
		Group("account").
		Order("connection_count desc")

	if len(accountsToExclude) > 0 {
		query = query.Not(map[string]interface{}{"account": accountsToExclude})
	}

	logger.Log.Debug("queryStmt:", query.ToSQL(func(tx *gorm.DB) *gorm.DB { return query }))

	rows, err := databaseConn.Rows()

	if err != nil {
		logger.LogFatalError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var account domain.AccountID
		var connectionCount int

		if err := rows.Scan(&account, &connectionCount); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		processConnectionCount(ctx, account, connectionCount)
	}

	return nil
}
