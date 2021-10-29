package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

func init() {
	logger.InitLogger()
}

func TestTurnpikeAuthenticatorAssociate(t *testing.T) {
	var req http.Request
	rr := httptest.NewRecorder()

	var xrhID identity.XRHID
	xrhID.Identity.Type = "Associate"

	applicationHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
	})

	handler := RequireTurnpikeAuthentication(applicationHandler)

	ctx := context.WithValue(req.Context(), identity.Key, xrhID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != 200 {
		t.Fatal("Code != 200")
	}
}

func TestTurnpikeAuthenticatorUser(t *testing.T) {
	var req http.Request
	rr := httptest.NewRecorder()

	var xrhID identity.XRHID
	xrhID.Identity.Type = "User"

	applicationHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
	})

	handler := RequireTurnpikeAuthentication(applicationHandler)

	ctx := context.WithValue(req.Context(), identity.Key, xrhID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != 401 {
		t.Fatal("Code != 401")
	}
}
