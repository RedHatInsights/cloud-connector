package connection_repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"gorm.io/gorm/clause"

	//"gorm.io/gorm"
	"github.com/jinzhu/gorm"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

//func (db *gorm.DB) Prepare(query string) (*gorm.Stmt, error) {
//	return db.PrepareContext(context.Background(), query)
//}

type Stmt struct {
	// Immutable:
	db        *gorm.DB // where we came from
	query     string   // that created the Stmt
	stickyErr error    // if non-nil, this error is returned for all operations

	closemu sync.RWMutex // held exclusively during close, for read otherwise.

	// If Stmt is prepared on a Tx or Conn then cg is present and will
	// only ever grab a connection from cg.
	// If cg is nil then the Stmt must grab an arbitrary connection
	// from db and determine if it must prepare the stmt again by
	// inspecting css.
	cg   stmtConnGrabber
	cgds *driverStmt

	// parentStmt is set when a transaction-specific statement
	// is requested from an identical statement prepared on the same
	// conn. parentStmt is used to track the dependency of this statement
	// on its originating ("parent") statement so that parentStmt may
	// be closed by the user without them having to know whether or not
	// any transactions are still using it.
	parentStmt *Stmt

	mu     sync.Mutex // protects the rest of the fields
	closed bool

	// css is a list of underlying driver statement interfaces
	// that are valid on particular connections. This is only
	// used if cg == nil and one is found that has idle
	// connections. If cg != nil, cgds is always used.
	css []connStmt

	// lastNumClosed is copied from db.numClosed when Stmt is created
	// without tx and closed connections in css are removed.
	lastNumClosed uint64
}

type connectionsNew struct {
	gorm.Model
	accountID uint
	client_ID uint

	//createdAt time.Time
	//updatedAt time.Time
	//deletedAt time.Time
}

type newDatabase struct {
	gorm.Model
	database     *gorm.DB
	queryTimeout time.Duration
}

type NullString struct {
	String string
	Valid  bool // Valid is true if String is not NULL
}

func PrepareContext(db gorm.DB) (ctx context.Context, query string, hi *gorm.Stmt, hii error) {
	var stmt *gorm.Stmt
	var err error
	var isBadConn bool
	for i := 0; i < maxBadConnRetries; i++ {
		stmt, err = db.prepare(ctx, query, cachedOrNewConn)
		isBadConn = errors.Is(err, driver.ErrBadConn)
		if !isBadConn {
			break
		}
	}
	if isBadConn {
		return db.prepare(ctx, query, alwaysNewConn)
	}
	return stmt, err
}

func NSCR(cfg *config.Config, database *gorm.DB) (*newDatabase, error) {
	return &newDatabase{
		database:     database,
		queryTimeout: cfg.ConnectionDatabaseQueryTimeout,
	}, nil
}

func (scm *newDatabase) Registerr(ctx context.Context, rhcClient domain.ConnectorClientState) error {

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	org_id := rhcClient.OrgID
	client_id := rhcClient.ClientID

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": org_id, "client_id": client_id})

	permittedTenants, err := retrievePermittedTenants(logger, rhcClient)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to determine permitted tenants")
	}

	db.Model(User{}).Create(map[string]interface{}{
		"Name":     "jinzhu",
		"Location": clause.Expr{SQL: "ST_PointFromText(?)", Vars: []interface{}{"POINT(100 100)"}},
	})
	// INSERT INTO `users` (`name`,`location`) VALUES ("jinzhu",ST_PointFromText("POINT(100 100)"));

	update := scm.database.Model(scm).Where("account = ?", "$6").Updates(connections{dispatchers: "$1", tags: "$2", updated_at: "NOW()", message_id: "$3", message_sent: "$4", permitted_tenants: "$5"})
	insert := scm.database.Model(---{}).Create(map[string]interface{}{
		"account":   "$8",
		"org_id":    "$9",
		"clinet_id": "$10",
		//"org_id": "$9",
		"dispatchers":       "$11",
		"canonical_facts":   "$12",
		"tags":              "$13",
		"permitted_tenants": "$14",
		"message_id":        "$15",
		"message_sent":      "$16",
	})

	//update := "UPDATE connections SET dispatchers=$1, tags = $2, updated_at = NOW(), message_id = $3, message_sent = $4, permitted_tenants = $5 WHERE account=$6 AND client_id=$7"
	//insert := "INSERT INTO connections (account, org_id, client_id, dispatchers, canonical_facts, tags, permitted_tenants, message_id, message_sent) SELECT $8, $9, $10, $11, $12, $13, $14, $15, $16"

	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "dispatchers": rhcClient.Dispatchers}).Error("Unable to marshal dispatchers")
		return err
	}

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "canonical_facts": rhcClient.CanonicalFacts}).Error("Unable to marshal canonicalfacts")
		return err
	}

	tagsString, err := json.Marshal(rhcClient.Tags)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "tags": rhcClient.CanonicalFacts}).Error("Unable to marshal tags")
		return err
	}

	_, err = statement.ExecContext(ctx, dispatchersString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, permittedTenants, account, client_id, account, org_id, client_id, dispatchersString, canonicalFactsString, tagsString, permittedTenants, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Insert/update failed")
		return FatalError{err}
	}

	logger.Debug("Registered a connection")
	return nil
}

type SqlConnectionRegistrar struct {
	database     *sql.DB
	queryTimeout time.Duration
}

func NewSqlConnectionRegistrar(cfg *config.Config, database *sql.DB) (*SqlConnectionRegistrar, error) {
	return &SqlConnectionRegistrar{
		database:     database,
		queryTimeout: cfg.ConnectionDatabaseQueryTimeout,
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.ConnectorClientState) error {

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	org_id := rhcClient.OrgID
	client_id := rhcClient.ClientID

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": org_id, "client_id": client_id})

	permittedTenants, err := retrievePermittedTenants(logger, rhcClient)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to determine permitted tenants")
	}

	//update := database.Model().Where("account = ?", "$6").Updates(Connection{dispatchers: "$1", tags: "$2", updated_at: "NOW()", message_id : "$3", message_sent: "$4", permitted_tenants: "$5"})

	update := "UPDATE connections SET dispatchers=$1, tags = $2, updated_at = NOW(), message_id = $3, message_sent = $4, permitted_tenants = $5 WHERE account=$6 AND client_id=$7"
	insert := "INSERT INTO connections (account, org_id, client_id, dispatchers, canonical_facts, tags, permitted_tenants, message_id, message_sent) SELECT $8, $9, $10, $11, $12, $13, $14, $15, $16"

	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "dispatchers": rhcClient.Dispatchers}).Error("Unable to marshal dispatchers")
		return err
	}

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "canonical_facts": rhcClient.CanonicalFacts}).Error("Unable to marshal canonicalfacts")
		return err
	}

	tagsString, err := json.Marshal(rhcClient.Tags)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "tags": rhcClient.CanonicalFacts}).Error("Unable to marshal tags")
		return err
	}

	_, err = statement.ExecContext(ctx, dispatchersString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, permittedTenants, account, client_id, account, org_id, client_id, dispatchersString, canonicalFactsString, tagsString, permittedTenants, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Insert/update failed")
		return FatalError{err}
	}

	logger.Debug("Registered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, client_id domain.ClientID) error {

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionUnregistrationDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	statement, err := scm.database.Prepare("DELETE FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	_, err = statement.ExecContext(ctx, client_id)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Delete failed")
		return FatalError{err}
	}

	logger.Debug("Unregistered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) FindConnectionByClientID(ctx context.Context, client_id domain.ClientID) (domain.ConnectorClientState, error) {
	var connectorClient domain.ConnectorClientState
	var err error

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionLookupByClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	statement, err := scm.database.Prepare("SELECT account, org_id, client_id, dispatchers, canonical_facts, tags, message_id, message_sent FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("SQL prepare failed")
		return connectorClient, FatalError{err}
	}
	defer statement.Close()

	var account domain.AccountID
	var orgID sql.NullString
	var dispatchersString sql.NullString
	var canonicalFactsString sql.NullString
	var tagsString sql.NullString
	var latestMessageID sql.NullString

	err = statement.QueryRowContext(ctx, client_id).Scan(&account,
		&orgID,
		&connectorClient.ClientID,
		&dispatchersString,
		&canonicalFactsString,
		&tagsString,
		&latestMessageID,
		&connectorClient.MessageMetadata.LatestTimestamp)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Debug("No connection found!")
			return connectorClient, NotFoundError
		} else if errors.Is(err, sql.ErrNoRows) == false {
			logger.WithFields(logrus.Fields{"error": err}).Error("SQL query failed")
			err = FatalError{err}
		}

		return connectorClient, err
	}

	connectorClient.Account = account

	if orgID.Valid {
		connectorClient.OrgID = domain.OrgID(orgID.String)
	}

	logger = logger.WithFields(logrus.Fields{"account": account, "org_id": connectorClient.OrgID})

	if dispatchersString.Valid {
		err = json.Unmarshal([]byte(dispatchersString.String), &connectorClient.Dispatchers)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal dispatchers from database")
		}
	}

	if canonicalFactsString.Valid {
		err = json.Unmarshal([]byte(canonicalFactsString.String), &connectorClient.CanonicalFacts)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal canonical facts from database")
		}
	}

	if tagsString.Valid {
		err = json.Unmarshal([]byte(tagsString.String), &connectorClient.Tags)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal tags from database")
		}
	}

	if latestMessageID.Valid {
		connectorClient.MessageMetadata.LatestMessageID = latestMessageID.String
	}

	return connectorClient, nil
}

func retrievePermittedTenants(logger *logrus.Entry, clientState domain.ConnectorClientState) (string, error) {

	emptyPermittedTenants := "[]"

	if clientState.Dispatchers == nil {
		logger.Debug("No permitted tenants found")
		return emptyPermittedTenants, nil
	}

	dispatchersMap := clientState.Dispatchers.(map[string]interface{})

	satelliteMapInterface, gotSatellite := dispatchersMap["satellite"]

	if gotSatellite == false {
		logger.Debug("No satellite dispatcher found")
		return emptyPermittedTenants, nil
	}

	logger.Debug("***** Found satellite map: ", satelliteMapInterface)

	satelliteMap := satelliteMapInterface.(map[string]interface{})

	permittedTenantsString, gotPermittedTenants := satelliteMap["tenant_list"]

	if gotPermittedTenants == false {
		logger.Debug("No permitted tenants found")
		return emptyPermittedTenants, nil
	}

	permittedTenantsList := strings.Split(permittedTenantsString.(string), ",")

	j, err := json.Marshal(permittedTenantsList)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Debug("Unable to parse permitted tenants list")
		return emptyPermittedTenants, nil
	}

	return string(j), nil
}
