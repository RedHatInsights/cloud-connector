package controller

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

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
