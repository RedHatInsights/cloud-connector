package main

import (
	"database/sql"
	"gorm.io/gorm"
	"log"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
)

func main() {

	cfg := config.GetConfig()
	log.Println("Starting Cloud-Connector DB migration")
	log.Println("Cloud-Connector configuration:\n", cfg)

	database, err := db.InitializeGormDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

	printConnectionsColumns(database)

	dumpSchemaMigrationsTable(database)
}

func printConnectionsColumns(database *gorm.DB) {

	rows, err := database.Table("information_schema.columns").
		Select("column_name", "data_type", "column_default", "is_nullable").
		Where("table_name = ?", "connections").
		Rows()

	if err != nil {
		log.Println("SQL query failed", err)
		return
	}
	defer rows.Close()

	log.Print("-----------------")
	log.Print("connections table:")
	log.Print("-----------------")
	for rows.Next() {
		var columnName string
		var dataType string
		var defaultValueString sql.NullString
		var isNullable string

		if err := rows.Scan(&columnName, &dataType, &defaultValueString, &isNullable); err != nil {
			log.Println("SQL scan failed.  Skipping row.", err)
			return
		}

		defaultValue := "null"
		if defaultValueString.Valid {
			defaultValue = defaultValueString.String
		}

		log.Printf("%s|%s|%s|%s\n", columnName, dataType, defaultValue, isNullable)
	}
}

func dumpSchemaMigrationsTable(database *gorm.DB) {

	rows, err := database.Table("schema_migrations").Select("version", "dirty").Rows()

	if err != nil {
		log.Println("SQL query failed", err)
		return
	}
	defer rows.Close()

	log.Print("-----------------")
	log.Print("schema_migrations table:")
	log.Print("-----------------")
	for rows.Next() {
		var version int
		var dirty bool

		if err := rows.Scan(&version, &dirty); err != nil {
			log.Println("SQL scan failed.  Skipping row.", err)
			return
		}

		log.Printf("%d|%v\n", version, dirty)
	}
}
