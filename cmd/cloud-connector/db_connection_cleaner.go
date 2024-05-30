package main

import (
    "fmt"
	"log"
    "time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
)

func startDbCleaner(dryRun bool, removeEntriesBefore time.Time) {

	cfg := config.GetConfig()
	log.Println("Starting Cloud-Connector DB cleaner")
	log.Println("Cloud-Connector configuration:\n", cfg)

    removeEntriesCreatedBeforeDate, err := time.Parse("01/01/1970", removeEntriesBefore)
    if err != nil {
        log.Print("Ugh: ", err)
        return
    }
   
    fmt.Println("removeEntriesCreatedBeforeDate: ", removeEntriesCreatedBeforeDate)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

    // FIXME: verify date

	statement, err := database.Prepare(`delete from connections where created_at = ` )
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
