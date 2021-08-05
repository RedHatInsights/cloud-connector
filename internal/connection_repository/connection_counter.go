package connection_repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type ConnectionCountProcessor func(context.Context, domain.AccountID, int) error

func ProcessConnectionCounts(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, accountsToExclude []string, processConnectionCount ConnectionCountProcessor) error {

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	queryStmt := `SELECT account, COUNT(1) AS connection_count FROM connections`

	if len(accountsToExclude) > 0 {
		inClause := strings.Join(accountsToExclude, "','")
		queryStmt = fmt.Sprintf(" %s WHERE account NOT IN ('%s')", queryStmt, inClause)
	}

	queryStmt = fmt.Sprintf(" %s GROUP BY account ORDER BY connection_count DESC", queryStmt)

	logger.Log.Debug("queryStmt:", queryStmt)

	statement, err := databaseConn.Prepare(queryStmt)
	if err != nil {
		logger.LogFatalError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := databaseConn.QueryContext(queryCtx, queryStmt)

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
