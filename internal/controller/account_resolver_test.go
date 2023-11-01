package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

const validResponseCacheTTL time.Duration = 10 * time.Millisecond
const errorResponseCacheTTL time.Duration = 2 * time.Millisecond

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
			mockResponse:      "{\"errors\":[{\"meta\":{\"response_by\":\"service\"},\"status\":500,\"detail\":\"ima detail\"}]}",
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

type testAccountResolver struct {
	clientID  domain.ClientID
	identity  domain.Identity
	accountID domain.AccountID
	orgID     domain.OrgID
	err       error
	wasCalled bool
}

func (this *testAccountResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error) {
	this.wasCalled = true
	this.clientID = clientID
	return this.identity, this.accountID, this.orgID, this.err
}

func TestAccountResolverCacheVerifyWrappedResolverNotCalled(t *testing.T) {
	cases := []struct {
		testName        string
		wrappedResolver *testAccountResolver
		clientId        domain.ClientID
	}{
		{
			testName: "Cache a good response",
			wrappedResolver: &testAccountResolver{
				identity:  "ImaIdentity",
				accountID: "0001",
				orgID:     "111100",
				err:       nil,
				wasCalled: false,
			},
			clientId: "client1",
		},
		{
			testName: "Cache an error response",
			wrappedResolver: &testAccountResolver{
				err:       fmt.Errorf("Could not find account"),
				wasCalled: false,
			},
			clientId: "client2",
		},
	}
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			resolver, _ := NewExpirableCachedAccountIdResolver(c.wrappedResolver, 10, validResponseCacheTTL, errorResponseCacheTTL)

			id, acc, org, err := resolver.MapClientIdToAccountId(context.TODO(), c.clientId)

			verifyAccountResolverResponse(t, c.wrappedResolver, id, acc, org, err)

			verifyAccountResolverWasCalled(t, c.wrappedResolver, c.clientId)

			// Reset the WasCalled so that we can verify that it doesn't get called
			c.wrappedResolver.wasCalled = false
			c.wrappedResolver.clientID = ""

			id, acc, org, err = resolver.MapClientIdToAccountId(context.TODO(), c.clientId)

			verifyAccountResolverResponse(t, c.wrappedResolver, id, acc, org, err)

			verifyAccountResolverWasNotCalled(t, c.wrappedResolver)
		})
	}
}

func TestAccountResolverCacheVerifyWrappedResolverCalledAfterExpiration(t *testing.T) {

	cases := []struct {
		testName        string
		wrappedResolver *testAccountResolver
		clientId        domain.ClientID
		sleepTime       time.Duration
	}{
		{
			testName: "Expire a good response",
			wrappedResolver: &testAccountResolver{
				identity:  "ImaIdentity",
				accountID: "0001",
				orgID:     "111100",
				err:       nil,
				wasCalled: false,
			},
			clientId:  "client3",
			sleepTime: validResponseCacheTTL + (2 * time.Millisecond),
		},
		{
			testName: "Expire an error response",
			wrappedResolver: &testAccountResolver{
				err:       fmt.Errorf("Could not find account"),
				wasCalled: false,
			},
			clientId:  "client4",
			sleepTime: errorResponseCacheTTL + (2 * time.Millisecond),
		},
	}
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			resolver, _ := NewExpirableCachedAccountIdResolver(c.wrappedResolver, 10, validResponseCacheTTL, errorResponseCacheTTL)

			id, acc, org, err := resolver.MapClientIdToAccountId(context.TODO(), c.clientId)

			verifyAccountResolverResponse(t, c.wrappedResolver, id, acc, org, err)

			verifyAccountResolverWasCalled(t, c.wrappedResolver, c.clientId)

			time.Sleep(c.sleepTime)

			// Reset the WasCalled so that we can verify that it was called
			c.wrappedResolver.wasCalled = false
			c.wrappedResolver.clientID = ""

			id, acc, org, err = resolver.MapClientIdToAccountId(context.TODO(), c.clientId)

			verifyAccountResolverResponse(t, c.wrappedResolver, id, acc, org, err)

			verifyAccountResolverWasCalled(t, c.wrappedResolver, c.clientId)
		})
	}
}

func verifyAccountResolverResponse(t *testing.T, resolver *testAccountResolver, actualIdentity domain.Identity, actualAccountID domain.AccountID, actualOrgID domain.OrgID, actualErr error) {
	if resolver.identity != actualIdentity {
		t.Fatalf("Expected identity (%s) did not match returned identity (%s)", resolver.identity, actualIdentity)
	}

	if resolver.accountID != actualAccountID {
		t.Fatalf("Expected account (%s) did not match returned account (%s)", resolver.accountID, actualAccountID)
	}

	if resolver.orgID != actualOrgID {
		t.Fatalf("Expected org id (%s) did not match returned org id (%s)", resolver.orgID, actualOrgID)
	}

	if resolver.err != actualErr {
		t.Fatalf("Expected error (%s) did not match returned error (%s)", resolver.err, actualErr)
	}
}

func verifyAccountResolverWasCalled(t *testing.T, resolver *testAccountResolver, expectedClientId domain.ClientID) {
	if resolver.wasCalled != true {
		t.Fatalf("Expected wrapped account resolver to be called")
	}

	if resolver.clientID != expectedClientId {
		t.Fatalf("Expected wrapped account resolver to be called with client-id %s, but was called with %s", expectedClientId, resolver.clientID)
	}
}

func verifyAccountResolverWasNotCalled(t *testing.T, resolver *testAccountResolver) {
	if resolver.wasCalled == true {
		t.Fatalf("Expected wrapper account resolver to NOT be called")
	}
}
