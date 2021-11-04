package middlewares

import (
	"fmt"
	"net/http"

	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

// EnforceTurnpikeAuthentication requires that the request be authenticated against Turnpike
// This middlware must be installed after the RedHatInsights Identity middleware
func EnforceTurnpikeAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		xrhID := identity.Get(r.Context())
		fmt.Println("xrhID: ", xrhID)

		if xrhID.Identity.Type != "Associate" {
			logger.Log.Debug(authErrorLogHeader + "Invalid identity type")
			http.Error(w, authErrorMessage, 401)
			return
		}

		next.ServeHTTP(w, r)
	})
}
