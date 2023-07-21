package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

type resolverResponse struct {
	Identity  domain.Identity
	AccountID domain.AccountID
	OrgID     domain.OrgID
}

func TestBopResolver(t *testing.T) {
	cases := []struct {
		mockResponse      string
		mockResponseCode  int
		inputClientID     string
		expectedOutHeader string
		expectedOutput    resolverResponse
		expectError       bool
	}{
		{
			mockResponse:      "{\"x-rh-identity\":\"eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sImNvc3RfbWFuYWdlbWVudCI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwiaW50ZXJuYWwiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInVzZXJfcHJlZmVyZW5jZXMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sIm9wZW5zaGlmdCI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwic2V0dGluZ3MiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInN1YnNjcmlwdGlvbnMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInNtYXJ0X21hbmFnZW1lbnQiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX19LCJpZGVudGl0eSI6eyJpbnRlcm5hbCI6eyJjcm9zc19hY2Nlc3MiOmZhbHNlLCJhdXRoX3RpbWUiOjEwMDAsIm9yZ19pZCI6IjExNzg5NzcyIn0sImFjY291bnRfbnVtYmVyIjoiNjA4OTcxOSIsImF1dGhfdHlwZSI6ImNlcnQtYXV0aCIsInN5c3RlbSI6eyJjbiI6IjE2ZDUwNDFmLWYyZTUtNDVkNy05MmE4LTU2NTBiMTUwMzk1OCIsImNlcnRfdHlwZSI6InN5c3RlbSJ9LCJ0eXBlIjoiU3lzdGVtIn19\"}",
			mockResponseCode:  200,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sImNvc3RfbWFuYWdlbWVudCI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwiaW50ZXJuYWwiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInVzZXJfcHJlZmVyZW5jZXMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sIm9wZW5zaGlmdCI6eyJpc19lbnRpdGxlZCI6dHJ1ZSwiaXNfdHJpYWwiOmZhbHNlfSwic2V0dGluZ3MiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInN1YnNjcmlwdGlvbnMiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX0sInNtYXJ0X21hbmFnZW1lbnQiOnsiaXNfZW50aXRsZWQiOnRydWUsImlzX3RyaWFsIjpmYWxzZX19LCJpZGVudGl0eSI6eyJpbnRlcm5hbCI6eyJjcm9zc19hY2Nlc3MiOmZhbHNlLCJhdXRoX3RpbWUiOjEwMDAsIm9yZ19pZCI6IjExNzg5NzcyIn0sImFjY291bnRfbnVtYmVyIjoiNjA4OTcxOSIsImF1dGhfdHlwZSI6ImNlcnQtYXV0aCIsInN5c3RlbSI6eyJjbiI6IjE2ZDUwNDFmLWYyZTUtNDVkNy05MmE4LTU2NTBiMTUwMzk1OCIsImNlcnRfdHlwZSI6InN5c3RlbSJ9LCJ0eXBlIjoiU3lzdGVtIn19", "6089719", "11789772"},
			expectError:       false,
		},
		{
			mockResponse:      "{}",
			mockResponseCode:  500,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"", "", ""},
			expectError:       true,
		},
		{
			mockResponse:      "{}",
			mockResponseCode:  401,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"", "", ""},
			expectError:       true,
		},
		{
			mockResponse:      "{\"errors\":[{\"meta\":{\"response_by\":\"service\"},\"status\":500,\"detail\":\"\"}]}",
			mockResponseCode:  500,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"", "", ""},
			expectError:       true,
		},
		{
			mockResponse:      "{\"errors\":[{\"meta\":{}]}",
			mockResponseCode:  500,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"", "", ""},
			expectError:       true,
		},
		{
			mockResponse:      "{\"errors\":[{\"meta\":{\"response_by\":\"service\"},\"status\":200,\"detail\":\"Got the detail\"}]}",
			mockResponseCode:  200,
			inputClientID:     "testID",
			expectedOutHeader: "/CN=testID",
			expectedOutput:    resolverResponse{"", "", ""},
			expectError:       true,
		},
	}
	for _, c := range cases {
		conf := config.GetConfig()
		outHeader := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			outHeader = r.Header.Get("x-rh-certauth-cn")
			w.WriteHeader(c.mockResponseCode)
			fmt.Fprintln(w, c.mockResponse)
		}))
		defer ts.Close()
		conf.AuthGatewayUrl = ts.URL
		resolver, _ := NewAccountIdResolver("bop", conf)
		id, acc, org, err := resolver.MapClientIdToAccountId(nil, domain.ClientID(c.inputClientID))
		if c.expectError && err == nil {
			t.Fatalf("Expected an error response but got nil")
		}
		if !c.expectError && err != nil {
			t.Fatalf("Did not expect an error response but got an error")
		}
		if outHeader != c.expectedOutHeader {
			t.Fatalf("Expected header sent to server  %s, got  %s", c.expectedOutHeader, outHeader)
		}
		resp := resolverResponse{id, acc, org}
		if resp != c.expectedOutput {
			t.Fatalf("Expected  %v, got  %v", c.expectedOutput, resp)
		}
	}

}
