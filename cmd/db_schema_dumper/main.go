package main

import (
	"database/sql"
	"log"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
)

func main() {

	cfg := config.GetConfig()
	log.Println("Starting Cloud-Connector DB migration")
	log.Println("Cloud-Connector configuration:\n", cfg)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

	printConnectionsColumns(database)

	dumpSchemaMigrationsTable(database)
}

func printConnectionsColumns(database *sql.DB) {

	statement, err := database.Prepare(
		`SELECT column_name, data_type, column_default, is_nullable FROM information_schema.columns
        WHERE table_name = 'connections'`)
	if err != nil {
		log.Println("SQL Prepare failed", err)
		return
	}
	defer statement.Close()

	rows, err := statement.Query()
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

func dumpSchemaMigrationsTable(database *sql.DB) {

	statement, err := database.Prepare(
		`SELECT version, dirty FROM schema_migrations`)
	if err != nil {
		log.Println("SQL Prepare failed", err)
		return
	}
	defer statement.Close()

	rows, err := statement.Query()
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
