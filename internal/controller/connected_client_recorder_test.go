package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

var validCanonicalFacts = map[string]interface{}{
	"ip_addresses":    []string{"192.168.1.120"},
	"rhel_machine_id": "6ca6a085-8d86-11eb-8bd1-f875a43f7183",
}

const validIdentityWithBasicAuth = domain.Identity("eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMSIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyJ9fQ==")
const validIdentityWithCertAuth = domain.Identity("eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMSIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyIsICJhdXRoX3R5cGUiOiAiY2VydC1hdXRoIn19")

func TestCopyAndCleanupCanonicalFacts(t *testing.T) {

	canonicalFacts := map[string]interface{}{"insights_id": "",
		"mac_addresses":   []string{},
		"ip_addresses":    []string{"192.168.1.120"},
		"rhel_machine_id": "6ca6a085-8d86-11eb-8bd1-f875a43f7183",
	}

	logger := logger.Log.WithFields(logrus.Fields{"testing": "just a test"})

	newData := cleanupCanonicalFacts(logger, canonicalFacts)

	if _, found := newData["insights_id"]; found {
		t.Fatalf("insights_id should have been removed from the map, but it was not!")
	}

	if _, found := newData["mac_addresses"]; found {
		t.Fatalf("mac_addresses should have been removed from the map, but it was not!")
	}

	if _, found := newData["ip_addresses"]; !found {
		t.Fatalf("ip_addresses should have been in the map, but it was not!")
	}

	if _, found := newData["rhel_machine_id"]; !found {
		t.Fatalf("rhel_machine_id should have been in the map, but it was not!")
	}
}

func TestConvertRHCTagsToInventoryTagsNilRhcTags(t *testing.T) {
	inventoryTags := convertRHCTagsToInventoryTags(nil)
	if inventoryTags != nil {
		t.Fatalf("inventory tags should have been nil, but it was not!")
	}
}

func TestConvertRHCTagsToInventoryTagsInvalidRhcTags(t *testing.T) {
	badRhcTags := make(map[string]string)
	inventoryTags := convertRHCTagsToInventoryTags(badRhcTags)
	if inventoryTags != nil {
		t.Fatalf("inventory tags should have been nil, but it was not!")
	}
}

func TestConvertRHCTagsToInventoryTagsEmptyRhcTags(t *testing.T) {
	emptyRhcTags := make(map[string]interface{})

	actualInventoryTags := convertRHCTagsToInventoryTags(emptyRhcTags)
	if actualInventoryTags == nil {
		t.Fatalf("inventory tags should have been non-nil, but it was not!")
	}
}

func TestConvertRHCTagsToInventoryTagsValidRhcTags(t *testing.T) {
	rhcTags := make(map[string]interface{})
	rhcTags["key1"] = "value1"
	rhcTags["key2"] = "value2"

	expectedInventoryTags := make(map[string]map[string][]string)
	expectedInventoryTags[inventoryTagNamespace] = make(map[string][]string)
	expectedInventoryTags[inventoryTagNamespace]["key1"] = []string{"value1"}
	expectedInventoryTags[inventoryTagNamespace]["key2"] = []string{"value2"}

	actualInventoryTags := convertRHCTagsToInventoryTags(rhcTags)
	if actualInventoryTags == nil {
		t.Fatalf("inventory tags should have been non-nil, but it was not!")
	}

	if diff := cmp.Diff(expectedInventoryTags, actualInventoryTags); diff != "" {
		t.Fatalf("actual inventory tags != expected inventory tags: \n%s ", diff)
	}
}

type mockKafkaWriter struct {
	message   []byte
	callCount int
}

func (m *mockKafkaWriter) WriteMessages(ctx context.Context, msg []byte) error {
	m.message = msg
	m.callCount++
	return nil
}

func buildMockInventoryMessageProducer(kafkaWriter *mockKafkaWriter) InventoryMessageProducer {
	return func(ctx context.Context, log *logrus.Entry, msg []byte) error {
		return kafkaWriter.WriteMessages(ctx, msg)
	}
}

func TestDoNotRegisterHostWithInventoryWithValidCanonicalFactsNoDispatchers(t *testing.T) {
	kafkaWriter := mockKafkaWriter{}
	messageProducer := buildMockInventoryMessageProducer(&kafkaWriter)
	connectedClientRecorder := &InventoryBasedConnectedClientRecorder{MessageProducer: messageProducer}
	id := validIdentityWithBasicAuth

	connectedClient := domain.ConnectorClientState{
		Account:        "1234567",
		OrgID:          "9876",
		ClientID:       "8974",
		CanonicalFacts: validCanonicalFacts,
	}

	connectedClientRecorder.RecordConnectedClient(context.TODO(), id, connectedClient)

	if kafkaWriter.callCount > 0 {
		t.Fatalf("kafka writer should not have been called")
	}
}

func TestRegisterHostWithInventory(t *testing.T) {

	testCases := []struct {
		dispatcher         string
		dispatcherFeatures map[string]string
	}{
		{playbookWorkerDispatcherKey, map[string]string{"version": "0.1"}},
		{packageManagerDispatcherKey, map[string]string{"version": "0.0.1"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("dispatcher = '%s'", tc.dispatcher), func(t *testing.T) {
			kafkaWriter := mockKafkaWriter{}
			messageProducer := buildMockInventoryMessageProducer(&kafkaWriter)
			connectedClientRecorder := &InventoryBasedConnectedClientRecorder{MessageProducer: messageProducer, ReporterName: "unit-test"}

			id := validIdentityWithBasicAuth

			connectedClient := domain.ConnectorClientState{
				Account:        "1234567",
				OrgID:          "9876",
				ClientID:       "8974",
				CanonicalFacts: validCanonicalFacts,
				Dispatchers: map[string]interface{}{
					tc.dispatcher:       tc.dispatcherFeatures,
					"spacely_sprockets": map[string]string{"sprocket_version": "10.01"},
				},
			}

			connectedClientRecorder.RecordConnectedClient(context.TODO(), id, connectedClient)

			if kafkaWriter.callCount != 1 {
				t.Fatalf("kafka writer should have been called once")
			}
		})
	}
}

func TestRegisterHostWithInventoryWithCertAuth(t *testing.T) {

	kafkaWriter := mockKafkaWriter{}
	messageProducer := buildMockInventoryMessageProducer(&kafkaWriter)
	connectedClientRecorder := &InventoryBasedConnectedClientRecorder{MessageProducer: messageProducer, ReporterName: "unit-test"}

	id := validIdentityWithCertAuth

	connectedClient := domain.ConnectorClientState{
		Account:        "1234567",
		OrgID:          "9876",
		ClientID:       "8974",
		CanonicalFacts: validCanonicalFacts,
		Dispatchers: map[string]interface{}{
			packageManagerDispatcherKey: map[string]string{"version": "0.0.1"},
			"spacely_sprockets":         map[string]string{"sprocket_version": "10.01"},
		},
	}

	connectedClientRecorder.RecordConnectedClient(context.TODO(), id, connectedClient)

	if kafkaWriter.callCount != 1 {
		t.Fatalf("kafka writer should have been called once")
	}

	err := validateInventoryMessage(kafkaWriter.message, connectedClient.OrgID, connectedClient.ClientID)
	if err != nil {
		t.Fatalf("validation of inventory message failed: %s", err)
	}
}

func validateInventoryMessage(b []byte, expectedOrgID domain.OrgID, expectedClientID domain.ClientID) error {
	var envelope inventoryMessageEnvelope

	err := json.Unmarshal(b, &envelope)
	if err != nil {
		return err
	}

	// This is really gross.  Using map[string]interface{} was pretty easy when
	// building the message to send to inventory, but now its painful to validate
	// the message within a test.  This needs to be converted to use a real struct
	// and "encoding/json" to do the marshalling.

	var ok bool
	var data map[string]interface{}
	var systemProfile map[string]interface{}

	if data, ok = envelope.Data.(map[string]interface{}); !ok {
		return fmt.Errorf("could not parse inventory data field")
	}

	var orgID string
	if orgID, ok = data["org_id"].(string); !ok {
		return fmt.Errorf("could not parse \"org_id\" field")
	}

	if orgID != string(expectedOrgID) {
		return fmt.Errorf("\"org_id\" (%s) field does not match expected data (%s)", orgID, expectedOrgID)
	}

	if systemProfile, ok = data["system_profile"].(map[string]interface{}); !ok {
		return fmt.Errorf("could not parse system_profile data")
	}

	var ownerID string
	if ownerID, ok = systemProfile["owner_id"].(string); !ok {
		return fmt.Errorf("could not parse \"owner_id\" field")
	}

	if ownerID != string(expectedClientID) {
		return fmt.Errorf("\"owner_id\" (%s) field does not match expected data (%s)", ownerID, expectedClientID)
	}

	return nil
}
