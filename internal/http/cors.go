package http

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
)

const (
	corsAllowMethods = "GET, POST, OPTIONS"
	corsAllowHeaders = "Authorization, X-KD-Tenant, Content-Type"
	corsMaxAge       = "600"
)

// TenantConfigReader exposes the per-tenant integration settings the HTTP layer
// reads: CORS origins (this middleware) and external-IdP providers (auth). It
// depends on this narrow contract, not on the concrete store.
type TenantConfigReader interface {
	// TenantAllowedOrigins returns the CORS origins configured for one tenant.
	TenantAllowedOrigins(ctx context.Context, tenant string) ([]string, error)
	// OriginRegistered reports whether any tenant allows the origin. Used for
	// preflight, which carries no tenant header.
	OriginRegistered(ctx context.Context, origin string) (bool, error)
	// TenantOIDCProviders returns the external-IdP providers for a tenant.
	TenantOIDCProviders(ctx context.Context, tenant string) ([]store.OIDCProvider, error)
}

// corsMiddleware answers cross-origin requests for /v1 using per-tenant origin
// allowlists. Because a browser preflight (OPTIONS) carries no X-KD-Tenant
// header, it is validated against the union of all tenants' origins; the actual
// request is then validated against its own tenant. No wildcard and no
// credentials flag — auth travels in headers, not cookies. Runs BEFORE
// authMiddleware so an unauthenticated preflight is still answered.
func corsMiddleware(reader TenantConfigReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))

			// Only /v1 is cross-origin (admin is same-origin). A request with no
			// Origin is not a CORS request — pass it straight through.
			if reader == nil || origin == "" || !strings.HasPrefix(r.URL.Path, "/v1/") {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Origin")

			// Preflight: no auth, no tenant header — answer from the global set.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if ok, err := reader.OriginRegistered(r.Context(), origin); err == nil && ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
					w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
					w.Header().Set("Access-Control-Max-Age", corsMaxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Actual request: echo the origin only if this tenant allows it.
			if tenant := strings.TrimSpace(r.Header.Get(tenantHeader)); tenant != "" {
				if origins, err := reader.TenantAllowedOrigins(r.Context(), tenant); err == nil && slices.Contains(origins, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
