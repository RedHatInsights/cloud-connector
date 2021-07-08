package controller

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

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
