package middlewares

import (
	"context"

	"github.com/redhatinsights/platform-go-middlewares/identity"
)

// Principal interface can be implemented and expanded by various principal objects (type depends on middleware being used)
type Principal interface {
	GetAccount() string
	GetOrgID() string
}

type key int

var principalKey key

type serviceToServicePrincipal struct {
	account, clientID, orgID string
}

func (sp serviceToServicePrincipal) GetAccount() string {
	return sp.account
}

func (sp serviceToServicePrincipal) GetOrgID() string {
	return sp.orgID
}

type identityPrincipal struct {
	account, orgID string
}

func (ip identityPrincipal) GetAccount() string {
	return ip.account
}

func (ip identityPrincipal) GetOrgID() string {
	return ip.orgID
}

// GetPrincipal takes the request context and determines which middleware (identity header vs service to service) was used
// before returning a principal object.
func GetPrincipal(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(serviceToServicePrincipal)
	if !ok {
		id, ok := ctx.Value(identity.Key).(identity.XRHID)
		p := identityPrincipal{account: id.Identity.AccountNumber, orgID: id.Identity.OrgID}
		return p, ok
	}
	return p, ok
}
