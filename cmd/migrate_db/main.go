package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "migrate_db",
}

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade to a later version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("up called")
		return performDbMigration("up")
	},
}

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "downgrade",
	Short: "Revert to a previous version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("down called")
		return performDbMigration("down")
	},
}

type loggerWrapper struct {
	*logrus.Logger
}

func (lw loggerWrapper) Verbose() bool {
	return true
}

func performDbMigration(direction string) error {

	cfg := config.GetConfig()
	logger.Log.Info("Starting Cloud-Connector DB migration")
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	db, err := db.InitializeGormDatabaseConnection(cfg)
	if err != nil {
		logger.LogError("Unable to initialize database connection", err)
		return err
	}

	postgresDb, err := db.DB()
	if err != nil {
		logger.LogError("Unable to retrieve DB from gorm connection", err)
		return err
	}

	driver, err := postgres.WithInstance(postgresDb, &postgres.Config{})
	if err != nil {
		logger.LogError("Unable to get postgres driver from database connection", err)
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://db/migrations", "postgres", driver)
	if err != nil {
		logger.LogError("Unable to initialize database migration util", err)
		return err
	}

	m.Log = loggerWrapper{logger.Log}

	if direction == "up" {
		err = m.Up()
	} else if direction == "down" {
		err = m.Steps(-1)
	} else {
		return errors.New("Invalid operation")
	}

	if errors.Is(err, migrate.ErrNoChange) {
		logger.Log.Info("DB migration resulted in no changes")
	} else if err != nil {
		logger.LogError("DB migration resulted in an error", err)
		return err
	}

	return nil
}

func main() {

	logger.InitLogger()
	defer logger.FlushLogger()

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)

	err := rootCmd.Execute()
	if err != nil {
		logger.FlushLogger()
		os.Exit(1)
	}
}
