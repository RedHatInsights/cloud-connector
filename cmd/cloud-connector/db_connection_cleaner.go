package main

import (
	"fmt"
	"log"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
)

func startDbCleaner(doIt bool, removeEntriesBefore string) {

	cfg := config.GetConfig()
	log.Println("Starting Cloud-Connector DB cleaner")
	log.Println("Cloud-Connector configuration:\n", cfg)

	fmt.Println("doIt: ", doIt)
	fmt.Println("removeEntriesBefore: ", removeEntriesBefore)

	removeEntriesCreatedBeforeDate, err := time.Parse(time.RFC3339, removeEntriesBefore)
	fmt.Println("removeEntriesCreatedBeforeDate: ", removeEntriesCreatedBeforeDate)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		log.Println("Unable to initialize database connection", err)
		return
	}

	var query string
	selectionClause := " from connections where created_at::date < $1"

	if doIt {
		query = "delete" + selectionClause + " returning org_id, client_id, created_at"
	} else {
		query = "select org_id, client_id, created_at " + selectionClause
	}

	fmt.Println("query: ", query)

	statement, err := database.Prepare(query)
	if err != nil {
		log.Println("SQL Prepare failed", err)
		return
	}
	defer statement.Close()

	results, err := statement.Query(removeEntriesCreatedBeforeDate)
	if err != nil {
		log.Print("Query failed: ", err)
		return
	}

	defer results.Close()

	for results.Next() {
		var orgId string
		var clientId string
		var createdAt string

		results.Scan(&orgId, &clientId, &createdAt)
		fmt.Printf("Removing entry: %s org-id, %s client-id, %s created_at\n", orgId, clientId, createdAt)
	}
}
