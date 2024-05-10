package cloud_connector

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

func TestSanitizeValidInsightsId(t *testing.T) {

	canonicalFacts := make(map[string]interface{})
	canonicalFacts["insights_id"] = "e20ea474-7287-4b68-80a6-533aa440e7bd"
	canonicalFacts["bios_uuid"] = "e20ea474-7287-4b68-80a6-533aa440e7bd"
	canonicalFacts["fqdn"] = "fred.flintstone.com"

	sanitizedCanonicalFacts := sanitizeCanonicalFacts(canonicalFacts)

	sanitizedCF := sanitizedCanonicalFacts.(map[string]interface{})

	if canonicalFacts["insights_id"] != sanitizedCF["insights_id"] {
		t.Fatal("insights_id was removed")
	}

	if canonicalFacts["bios_uuid"] != sanitizedCF["bios_uuid"] {
		t.Fatal("bios_uuid was removed")
	}

	if canonicalFacts["fqdn"] != sanitizedCF["fqdn"] {
		t.Fatal("fqdn was removed")
	}

}

func TestSanitizeInvalidInsightsId(t *testing.T) {

	canonicalFacts := make(map[string]interface{})
	canonicalFacts["insights_id"] = "\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000\\u0000"
	canonicalFacts["bios_uuid"] = "e20ea474-7287-4b68-80a6-533aa440e7bd"
	canonicalFacts["fqdn"] = "fred.flintstone.com"

	sanitizedCanonicalFacts := sanitizeCanonicalFacts(canonicalFacts)

	sanitizedCF := sanitizedCanonicalFacts.(map[string]interface{})

	if _, ok := sanitizedCF["insights_id"]; ok {
		t.Fatal("insights_id should have been removed")
	}

	if canonicalFacts["bios_uuid"] != sanitizedCF["bios_uuid"] {
		t.Fatal("bios_uuid was removed")
	}

	if canonicalFacts["fqdn"] != sanitizedCF["fqdn"] {
		t.Fatal("fqdn was removed")
	}

}
