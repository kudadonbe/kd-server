package http

import "context"

type contextKey string

const tenantContextKey contextKey = "tenantId"

// ContextWithTenant stores the tenant identifier for downstream handlers.
func ContextWithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenantID)
}

// TenantFromContext retrieves the tenant identifier for the current request.
// It reads the authenticated principal first (the current source of truth),
// then falls back to a directly-stored tenant for older callers/tests.
func TenantFromContext(ctx context.Context) (string, bool) {
	if p, ok := PrincipalFromContext(ctx); ok && p.TenantID != "" {
		return p.TenantID, true
	}
	tenant, ok := ctx.Value(tenantContextKey).(string)
	return tenant, ok
}
