package cloud_connector

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

type mockSourcesRecorder struct {
	identity        domain.Identity
	account         domain.AccountID
	orgId           domain.OrgID
	clientId        domain.ClientID
	sourceRef       string
	sourceName      string
	sourceType      string
	applicationType string
}

func (msr *mockSourcesRecorder) RegisterWithSources(identity domain.Identity, account domain.AccountID, orgId domain.OrgID, clientId domain.ClientID, sourceRef, sourceName, sourceType, applicationType string) error {
	msr.identity = identity
	msr.account = account
	msr.orgId = orgId
	msr.clientId = clientId
	msr.sourceRef = sourceRef
	msr.sourceName = sourceName
	msr.sourceType = sourceType
	msr.applicationType = applicationType
	return nil
}

func TestProcessDispatchers(t *testing.T) {

	var expectedIdentity domain.Identity
	var expectedAccount domain.AccountID = "12345"
	var expectedOrgID domain.OrgID = "54321"
	var expectedClientId domain.ClientID = "98765"
	var expectedApplicationType string = "ima_app_type"
	var expectedSourceType string = "ima_source_type"
	var expectedSourceRef string = "ima_source_ref"
	var expectedSourceName string = "ima_source_name"

	// Used to verify that the SourcesRecorder is called
	sourcesRecorder := &mockSourcesRecorder{}

	catalogMap := make(map[string]interface{})
	catalogMap[catalogApplicationType] = expectedApplicationType
	catalogMap[catalogSourceType] = expectedSourceType
	catalogMap[catalogSourceRef] = expectedSourceRef
	catalogMap[catalogSourceName] = expectedSourceName

	dispatchersMap := make(map[string]interface{})
	dispatchersMap[catalogDispatcherKey] = catalogMap

	contentMap := make(map[string]interface{})
	contentMap[dispatchersKey] = dispatchersMap

	logger := logger.Log.WithFields(logrus.Fields{})

	processDispatchers(logger, sourcesRecorder, expectedIdentity, expectedAccount, expectedOrgID, expectedClientId, contentMap)

	// Verify that the SourcesRecorder is called and the parameters were as expected

	if sourcesRecorder.identity != expectedIdentity {
		t.Fatal("identity does not match!")
	}

	if sourcesRecorder.account != expectedAccount {
		t.Fatal("account does not match!")
	}

	if sourcesRecorder.orgId != expectedOrgID {
		t.Fatal("org id does not match!")
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
