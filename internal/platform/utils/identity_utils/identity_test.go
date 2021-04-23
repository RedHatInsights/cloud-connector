package identity_utils

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

func TestIsCertAuthWithValidInput(t *testing.T) {

	testCases := []struct {
		authType        string
		identity        string
		expectedResults bool
	}{
		{"cert-auth", buildIdentityString("cert-auth"), true},
		{"basic-auth", buildIdentityString("basic-auth"), false},
		{"no-auth-type", base64.StdEncoding.EncodeToString([]byte(`{"identity":{}}`)), false},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("auth-type = '%s'", tc.authType), func(t *testing.T) {

			actualResults, err := AuthenticatedWithCertificate(domain.Identity(tc.identity))

			if err != nil {
				t.Fatal("unexpected error ", err)
			}

			if actualResults != tc.expectedResults {
				t.Fatalf("expected results %t, but got %t!", tc.expectedResults, actualResults)
			}
		})
	}
}

func TestIsCertAuthWithInValidInput(t *testing.T) {

	testCases := []struct {
		testName      string
		identity      string
		expectedError bool
	}{
		{"no-identity", base64.StdEncoding.EncodeToString([]byte(`{"noidentity":{}}`)), true},
		{"bad-input", base64.StdEncoding.EncodeToString([]byte(`}`)), true},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			actualResults, err := AuthenticatedWithCertificate(domain.Identity(tc.identity))

			if err == nil {
				t.Fatal("expected error but did not get one")
			}

			if actualResults != false {
				t.Fatalf("expected results %t, but got %t!", false, actualResults)
			}
		})
	}
}

func buildIdentityString(authType string) string {
	identityJson := fmt.Sprintf(`
        {"identity":
            {
            "type": "User",
            "auth_type": "%s",
            "account_number": "000001",
            "internal":
                {"org_id": "000001"},
            "user":
                {"email": "fred@flintstone.com", "is_org_admin": true}
            }
        }`,
		authType)

	return base64.StdEncoding.EncodeToString([]byte(identityJson))
}
