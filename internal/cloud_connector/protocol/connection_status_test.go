package protocol

import (
	"testing"
)

func TestGetClientNameFromConnectionStatusContent(t *testing.T) {
	testCases := []struct {
		testName      string
		expectedValue string
		expectedFound bool
		payload       map[string]interface{}
	}{
		{"empty payload", "", false, make(map[string]interface{})},
		{"valid client_name", "barney.rubble", true, map[string]interface{}{"client_name": "barney.rubble"}},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			actualValue, actualFound := GetClientNameFromConnectionStatusContent(tc.payload)

			if actualFound != tc.expectedFound {
				t.Fatalf("expected found to equal %v, got %v", tc.expectedFound, actualFound)
			}

			if actualValue != tc.expectedValue {
				t.Fatalf("expected value to equal %s, got %s", tc.expectedValue, actualValue)
			}
		})
	}
}

func TestGetClientVersionFromConnectionStatusContent(t *testing.T) {
	testCases := []struct {
		testName      string
		expectedValue string
		expectedFound bool
		payload       map[string]interface{}
	}{
		{"empty payload", "", false, make(map[string]interface{})},
		{"valid client_version", "12.1.101", true, map[string]interface{}{"client_version": "12.1.101"}},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			actualValue, actualFound := GetClientVersionFromConnectionStatusContent(tc.payload)

			if actualFound != tc.expectedFound {
				t.Fatalf("expected found to equal %v, got %v", tc.expectedFound, actualFound)
			}

			if actualValue != tc.expectedValue {
				t.Fatalf("expected value to equal %s, got %s", tc.expectedValue, actualValue)
			}
		})
	}
}
