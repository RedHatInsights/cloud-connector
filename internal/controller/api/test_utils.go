package api

import (
	"encoding/base64"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

func buildIdentityHeader(account domain.AccountID, identityType string) string {
	identityJson := fmt.Sprintf(
		"{ \"identity\": {\"account_number\": \"%s\", \"type\": \"%s\", \"internal\": { \"org_id\": \"1979710\" } } }",
		account,
		identityType)
	return base64.StdEncoding.EncodeToString([]byte(identityJson))
}
