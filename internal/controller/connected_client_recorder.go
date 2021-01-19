package controller

import (
	"context"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

type ConnectedClientRecorder interface {
	RecordConnectedClient(context.Context, domain.AccountID, domain.ClientID, interface{}) error
}

type InventoryBasedConnectedClientRecorder struct {
}

func (ibccr *InventoryBasedConnectedClientRecorder) RecordConnectedClient(ctx context.Context, account domain.AccountID, clientID domain.ClientID, canonicalFacts interface{}) error {
	fmt.Println("FIXME: send inventory kafka message - ", account, clientID, canonicalFacts)
	return nil
}
