package http

import "context"

type contextKey string

const tenantContextKey contextKey = "tenantId"

// ContextWithTenant stores the tenant identifier for downstream handlers.
func ContextWithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenantID)
}

// TenantFromContext retrieves the tenant identifier for the current request.
func TenantFromContext(ctx context.Context) (string, bool) {
	tenant, ok := ctx.Value(tenantContextKey).(string)
	return tenant, ok
}
