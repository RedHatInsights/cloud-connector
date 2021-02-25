package mqtt

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

type mockSourcesRecorder struct {
	account         domain.AccountID
	clientId        domain.ClientID
	sourceRef       string
	sourceName      string
	sourceType      string
	applicationType string
}

func (msr *mockSourcesRecorder) RegisterWithSources(account domain.AccountID, clientId domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error {
	msr.account = account
	msr.clientId = clientId
	msr.sourceRef = sourceRef
	msr.sourceName = sourceName
	msr.sourceType = sourceType
	msr.applicationType = applicationType
	return nil
}

func TestProcessDispatchers(t *testing.T) {

	var expectedAccount domain.AccountID = "12345"
	var expectedClientId domain.ClientID = "98765"
	var expectedApplicationType string = "ima_app_type"
	var expectedSourceType string = "ima_source_type"
	var expectedSourceRef string = "ima_source_ref"
	var expectedSourceName string = "ima_source_name"

	// Used to verify that the SourcesRecorder is called
	sourcesRecorder := &mockSourcesRecorder{}

	catalogMap := make(map[string]interface{})
	catalogMap[CATALOG_APPLICATION_TYPE] = expectedApplicationType
	catalogMap[CATALOG_SOURCE_TYPE] = expectedSourceType
	catalogMap[CATALOG_SOURCE_REF] = expectedSourceRef
	catalogMap[CATALOG_SOURCE_NAME] = expectedSourceName

	dispatchersMap := make(map[string]interface{})
	dispatchersMap[CATALOG_DISPATCHER_KEY] = catalogMap

	contentMap := make(map[string]interface{})
	contentMap[DISPATCHERS_KEY] = dispatchersMap

	processDispatchers(sourcesRecorder, expectedAccount, expectedClientId, contentMap)

	// Verify that the SourcesRecorder is called and the parameters were as expected

	if sourcesRecorder.account != expectedAccount {
		t.Fatal("account does not match!")
	}

	if sourcesRecorder.clientId != expectedClientId {
		t.Fatal("client id does not match!")
	}

	if sourcesRecorder.sourceRef != expectedSourceRef {
		t.Fatal("source_ref does not match!")
	}

	if sourcesRecorder.sourceType != expectedSourceType {
		t.Fatal("source_type does not match!")
	}

	if sourcesRecorder.sourceName != expectedSourceName {
		t.Fatal("source_name does not match!")
	}

	if sourcesRecorder.applicationType != expectedApplicationType {
		t.Fatal("app_type does not match!")
	}
}
