package http

import (
	"context"
	"net/http"

	"github.com/kudadonbe/kd-server/internal/auth"
	"github.com/kudadonbe/kd-server/internal/store"
)

// Scopes gate what an authenticated caller may do. Routes check scopes, never
// the auth method — so a new auth mode (Part C's external IdP) only needs to
// produce a Principal with the right scopes, not new route logic (OCP).
const (
	ScopeExtract      = "extract"       // read an ID document, suggestion-only
	ScopeSearchRead   = "search:read"   // query the entity index
	ScopeRecordsWrite = "records:write" // ingest/resolve, finalize identity docs
	ScopeVerify       = "verify"        // promote a record to verified
)

// AuthMode records how a caller authenticated (for audit and scope decisions).
type AuthMode string

const (
	AuthModeAPIKey AuthMode = "apikey" // tenant server-side secret
	AuthModeJWT    AuthMode = "jwt"    // kd-server issued HS256 token
	AuthModeIDP    AuthMode = "idp"    // external IdP end-user token (Part C)
)

// Principal is the authenticated caller for a request.
type Principal struct {
	TenantID string
	Subject  string // key id, or end-user sub/email for IdP callers
	Mode     AuthMode
	Scopes   map[string]struct{}
	// Verified marks a caller acting for a government-verified identity (eFaas).
	// Such a caller may see their own record; NationalID names whose (Part D).
	Verified   bool
	NationalID string
}

// HasScope reports whether the principal holds the given scope.
func (p *Principal) HasScope(scope string) bool {
	if p == nil {
		return false
	}
	_, ok := p.Scopes[scope]
	return ok
}

func scopeSet(scopes ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		set[s] = struct{}{}
	}
	return set
}

// trustedScopes are granted to server-side callers (API key, kd-server JWT):
// the credential is the tenant's own secret, so it is trusted to act broadly.
// Browser end-users (Part C) get a restricted set instead.
func trustedScopes() map[string]struct{} {
	return scopeSet(ScopeExtract, ScopeSearchRead, ScopeRecordsWrite, ScopeVerify)
}

type principalContextKey struct{}

// ContextWithPrincipal stores the authenticated principal for downstream handlers.
func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

// PrincipalFromContext retrieves the authenticated principal for the request.
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(*Principal)
	return p, ok
}

// findOIDCProvider returns the provider whose issuer matches the token's, so the
// right JWKS/audience is used for verification.
func findOIDCProvider(providers []store.OIDCProvider, issuer string) (store.OIDCProvider, bool) {
	for _, p := range providers {
		if p.Issuer == issuer {
			return p, true
		}
	}
	return store.OIDCProvider{}, false
}

// idpPrincipal builds the principal for a verified external-IdP end-user. The
// default is public — extract only. A signed role claim (Firebase) can elevate
// to staff/admin; an identity-verified provider (eFaas) carries a gov-verified
// national ID that later gates self-service reveal. kd-server owns this mapping,
// so a browser can never grant itself scopes it wasn't issued.
func idpPrincipal(tenantID string, provider store.OIDCProvider, claims *auth.OIDCClaims) *Principal {
	p := &Principal{
		TenantID: tenantID,
		Subject:  claims.Subject,
		Mode:     AuthModeIDP,
		Scopes:   scopeSet(ScopeExtract),
	}

	if provider.RoleClaim != "" {
		if role, _ := claims.Raw[provider.RoleClaim].(string); role != "" {
			switch role {
			case "staff":
				p.Scopes = scopeSet(ScopeExtract, ScopeSearchRead)
			case "admin":
				p.Scopes = scopeSet(ScopeExtract, ScopeSearchRead, ScopeRecordsWrite, ScopeVerify)
			}
		}
	}

	if provider.IdentityVerified {
		p.Verified = true
		if provider.NationalIDClaim != "" {
			p.NationalID, _ = claims.Raw[provider.NationalIDClaim].(string)
		}
	}
	return p
}

// requireScope rejects requests whose principal lacks the given scope. It runs
// inside authMiddleware, so a principal is always present on success.
func requireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p, ok := PrincipalFromContext(r.Context()); !ok || !p.HasScope(scope) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient scope"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
