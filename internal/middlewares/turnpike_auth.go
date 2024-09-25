package middlewares

import (
	"net/http"

	"github.com/redhatinsights/platform-go-middlewares/v2/identity"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

// EnforceTurnpikeAuthentication requires that the request be authenticated against Turnpike
// This middlware must be installed after the RedHatInsights Identity middleware
func EnforceTurnpikeAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		xrhID := identity.Get(r.Context())

		if xrhID.Identity.Type != "Associate" {
			logger.Log.Debug(authErrorLogHeader + "Invalid identity type: " + xrhID.Identity.Type)
			http.Error(w, authErrorMessage, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
