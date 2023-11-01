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
	ClientID  domain.ClientID
	Identity  domain.Identity
	AccountID domain.AccountID
	OrgID     domain.OrgID
	Err       error
	WasCalled bool
}

func (this *testAccountResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, domain.OrgID, error) {
	this.WasCalled = true
	this.ClientID = clientID
	return this.Identity, this.AccountID, this.OrgID, this.Err
}

func TestCachingAccountResolverCacheValidResponse(t *testing.T) {

	var clientId domain.ClientID = "client1"

	wrappedAccountResolver := &testAccountResolver{
		Identity:  "ImaIdentity",
		AccountID: "0001",
		OrgID:     "111100",
		Err:       nil,
		WasCalled: false,
	}

	resolver, _ := NewExpirableCachedAccountIdResolver(wrappedAccountResolver, 10, 10*time.Millisecond, 2*time.Millisecond)

	id, acc, org, err := resolver.MapClientIdToAccountId(context.TODO(), clientId)

	verifyAccountResolverResponse(t, wrappedAccountResolver, id, acc, org, err)

	verifyAccountResolverWasCalled(t, wrappedAccountResolver, clientId)

	// Reset the WasCalled so that we can verify that it doesn't get called
	wrappedAccountResolver.WasCalled = false
	wrappedAccountResolver.ClientID = ""

	id, acc, org, err = resolver.MapClientIdToAccountId(context.TODO(), clientId)

	verifyAccountResolverResponse(t, wrappedAccountResolver, id, acc, org, err)

	verifyAccountResolverWasNotCalled(t, wrappedAccountResolver)
}

func TestCachingAccountResolverCacheValidResponseWaitForExpiration(t *testing.T) {

	wrappedAccountResolver := &testAccountResolver{
		Identity:  "ImaIdentity",
		AccountID: "0001",
		OrgID:     "111100",
		Err:       nil,
		WasCalled: false,
	}

	resolver, _ := NewExpirableCachedAccountIdResolver(wrappedAccountResolver, 10, 10*time.Millisecond, 2*time.Millisecond)

	id, acc, org, err := resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	fmt.Println("id:", id)
	fmt.Println("acc:", acc)
	fmt.Println("org:", org)
	fmt.Println("err:", err)

	if wrappedAccountResolver.WasCalled == false {
		t.Fatalf("Expected wrapper account resolver to be called")
	}

	time.Sleep(12 * time.Millisecond)

	wrappedAccountResolver.WasCalled = false

	id, acc, org, err = resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	if wrappedAccountResolver.WasCalled == false {
		t.Fatalf("Expected wrapper account resolver to be called")
	}
}

func TestCachingAccountResolverCacheErrorResponse(t *testing.T) {

	wrappedAccountResolver := &testAccountResolver{
		Err:       fmt.Errorf("Could not find account"),
		WasCalled: false,
	}

	resolver, _ := NewExpirableCachedAccountIdResolver(wrappedAccountResolver, 10, 10*time.Millisecond, 2*time.Millisecond)

	id, acc, org, err := resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	fmt.Println("id:", id)
	fmt.Println("acc:", acc)
	fmt.Println("org:", org)
	fmt.Println("err:", err)

	if wrappedAccountResolver.WasCalled == false {
		t.Fatalf("Expected wrapper account resolver to be called")
	}

	if wrappedAccountResolver.Err != err {
		t.Fatalf("Expected error returned to match wrapper account resolver error")
	}

	wrappedAccountResolver.WasCalled = false

	id, acc, org, err = resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	fmt.Println("err:", err)

	if wrappedAccountResolver.WasCalled == true {
		t.Fatalf("Expected wrapper account resolver to NOT be called")
	}

	if wrappedAccountResolver.Err != err {
		t.Fatalf("Expected error returned to match wrapper account resolver error")
	}
}

func TestCachingAccountResolverCacheErrorResponseWaitForExpiration(t *testing.T) {

	wrappedAccountResolver := &testAccountResolver{
		Err:       fmt.Errorf("Could not find account"),
		WasCalled: false,
	}

	resolver, _ := NewExpirableCachedAccountIdResolver(wrappedAccountResolver, 10, 10*time.Millisecond, 2*time.Millisecond)

	id, acc, org, err := resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	fmt.Println("id:", id)
	fmt.Println("acc:", acc)
	fmt.Println("org:", org)
	fmt.Println("err:", err)

	if wrappedAccountResolver.WasCalled == false {
		t.Fatalf("Expected wrapper account resolver to be called")
	}

	if wrappedAccountResolver.Err != err {
		t.Fatalf("Expected error returned to match wrapper account resolver error")
	}

	time.Sleep(4 * time.Millisecond)

	wrappedAccountResolver.WasCalled = false

	id, acc, org, err = resolver.MapClientIdToAccountId(nil, domain.ClientID("client1"))

	fmt.Println("err:", err)

	if wrappedAccountResolver.WasCalled == false {
		t.Fatalf("Expected wrapper account resolver to be called")
	}

	if wrappedAccountResolver.Err != err {
		t.Fatalf("Expected error returned to match wrapper account resolver error")
	}
}

func verifyAccountResolverResponse(t *testing.T, resolver *testAccountResolver, actualIdentity domain.Identity, actualAccountID domain.AccountID, actualOrgID domain.OrgID, actualErr error) {
	if resolver.Identity != actualIdentity {
		t.Fatalf("Expected identity (%s) did not match returned identity (%s)", resolver.Identity, actualIdentity)
	}

	if resolver.AccountID != actualAccountID {
		t.Fatalf("Expected account (%s) did not match returned account (%s)", resolver.AccountID, actualAccountID)
	}

	if resolver.OrgID != actualOrgID {
		t.Fatalf("Expected org id (%s) did not match returned org id (%s)", resolver.OrgID, actualOrgID)
	}

	if resolver.Err != actualErr {
		t.Fatalf("Expected error (%s) did not match returned error (%s)", resolver.Err, actualErr)
	}
}

func verifyAccountResolverWasCalled(t *testing.T, resolver *testAccountResolver, expectedClientId domain.ClientID) {
	if resolver.WasCalled != true {
		t.Fatalf("Expected wrapped account resolver to be called")
	}

	if resolver.ClientID != expectedClientId {
		t.Fatalf("Expected wrapped account resolver to be called with client-id %s, but was called with %s", expectedClientId, resolver.ClientID)
	}
}

func verifyAccountResolverWasNotCalled(t *testing.T, resolver *testAccountResolver) {
	if resolver.WasCalled == true {
		t.Fatalf("Expected wrapper account resolver to NOT be called")
	}
}
