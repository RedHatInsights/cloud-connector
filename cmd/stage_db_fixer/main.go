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

	database, err := db.InitializeGormDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

	statement := database.Table("schema_migrations").Updates(map[string]interface{}{"version": 4, "dirty": "f"})

	if statement.Error != nil {
		log.Print("Insert/update failed: ", statement.Error)
		return
	}
}
