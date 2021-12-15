package main

import (
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

	statement, err := database.Prepare(`update schema_migrations set version = 4, dirty = 'f'`)
	if err != nil {
		log.Println("SQL Prepare failed", err)
		return
	}
	defer statement.Close()

	_, err = statement.Exec()
	if err != nil {
		log.Print("Insert/update failed: ", err)
		return
	}
}
