package main

import (
	"fmt"
	"log"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
)

func startDbCleaner(dryRun bool, removeEntriesBefore string) {

	cfg := config.GetConfig()
	log.Println("Starting Cloud-Connector DB cleaner")
	log.Println("Cloud-Connector configuration:\n", cfg)

	fmt.Println("dryRun: ", dryRun)
	fmt.Println("removeEntriesBefore: ", removeEntriesBefore)

    /*
	removeEntriesCreatedBeforeDate, err := time.Parse("01/02/2006", removeEntriesBefore)
	if err != nil {
		log.Print("Ugh: ", err)
		return
	}
    */

	removeEntriesCreatedBeforeDate, err := time.Parse(time.RFC3339, removeEntriesBefore)
	fmt.Println("removeEntriesCreatedBeforeDate: ", removeEntriesCreatedBeforeDate)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

	// FIXME: verify date

	sqlCommand := "select *"

	if !dryRun {
		sqlCommand = "delete"
	}

	query := sqlCommand + " from connections where created_at::date < $1"
	fmt.Println("query: ", query)

	statement, err := database.Prepare(query)
	if err != nil {
		log.Println("SQL Prepare failed", err)
		return
	}
	defer statement.Close()

	//results, err := statement.Exec(removeEntriesCreatedBeforeDate)
	results, err := statement.Query(removeEntriesCreatedBeforeDate)
	if err != nil {
		log.Print("Query failed: ", err)
		return
	}

	defer results.Close()
	// Loop through rows, using Scan to assign column data to struct fields.
	for results.Next() {
		fmt.Println("results: ", results)
	}

	//    fmt.Println("results: ", results)
}
