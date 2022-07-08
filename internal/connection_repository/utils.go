package connection_repository

import (
	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

func verifyOrgId(orgId domain.OrgID) error {
	if orgId == "" {
		return InvalidOrgIDError
	}

	return nil
}

func verifyClientId(clientId domain.ClientID) error {
	if clientId == "" {
		return InvalidClientIDError
	}

	return nil
}
