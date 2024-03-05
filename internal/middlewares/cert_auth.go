package middlewares

import (
	"net/http"

	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

// This middlware must be installed after the RedHatInsights Identity middleware
func EnforceCertAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		xrhID := identity.Get(r.Context())

		if xrhID.Identity.AuthType != "cert-auth" {
			logger.Log.Debug(authErrorLogHeader + "Invalid auth type: " + xrhID.Identity.AuthType)
			http.Error(w, authErrorMessage, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
