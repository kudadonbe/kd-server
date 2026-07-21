package services

import (
	"context"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
)

// AdminStore contains tenant and credential administration persistence.
type AdminStore interface {
	ListTenants(ctx context.Context) ([]store.Tenant, error)
	CreateTenant(ctx context.Context, input store.CreateTenantInput) (*store.Tenant, error)
	UpdateTenantName(ctx context.Context, tenantSlug, name string) (*store.Tenant, error)
	SetTenantAllowedOrigins(ctx context.Context, tenantSlug string, origins []string) (*store.Tenant, error)
	SetTenantOIDCProviders(ctx context.Context, tenantSlug string, providers []store.OIDCProvider) (*store.Tenant, error)
	IssueAPIKey(ctx context.Context, tenantSlug, label string) (*store.IssuedAPIKey, error)
	RevokeAPIKey(ctx context.Context, tenantID, keyID string) error
}

// AdminService manages tenant onboarding.
type AdminService struct {
	store AdminStore
}

// TenantSummary is the public tenant administration view.
type TenantSummary struct {
	Slug           string               `json:"slug"`
	Name           string               `json:"name"`
	AllowedOrigins []string             `json:"allowed_origins,omitempty"`
	OIDCProviders  []store.OIDCProvider `json:"oidc_providers,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

func tenantSummary(t *store.Tenant) TenantSummary {
	return TenantSummary{
		Slug:           t.Slug,
		Name:           t.Name,
		AllowedOrigins: t.Config.AllowedOrigins,
		OIDCProviders:  t.Config.OIDCProviders,
		CreatedAt:      t.CreatedAt,
	}
}

// IssuedCredential contains a newly issued secret, returned only once.
type IssuedCredential struct {
	Tenant string `json:"tenant"`
	KeyID  string `json:"key_id"`
	Secret string `json:"secret"`
	Label  string `json:"label"`
}

// NewAdminService creates a tenant administration service.
func NewAdminService(adminStore AdminStore) *AdminService {
	return &AdminService{store: adminStore}
}

// ListTenants returns tenant summaries.
func (s *AdminService) ListTenants(ctx context.Context) ([]TenantSummary, error) {
	tenants, err := s.store.ListTenants(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]TenantSummary, 0, len(tenants))
	for i := range tenants {
		result = append(result, tenantSummary(&tenants[i]))
	}
	return result, nil
}

// CreateTenant creates a tenant.
func (s *AdminService) CreateTenant(ctx context.Context, slug, name string) (*TenantSummary, error) {
	tenant, err := s.store.CreateTenant(ctx, store.CreateTenantInput{Slug: slug, Name: name})
	if err != nil {
		return nil, err
	}
	summary := tenantSummary(tenant)
	return &summary, nil
}

// UpdateTenantName changes a tenant's display name.
func (s *AdminService) UpdateTenantName(ctx context.Context, slug, name string) (*TenantSummary, error) {
	tenant, err := s.store.UpdateTenantName(ctx, slug, name)
	if err != nil {
		return nil, err
	}
	summary := tenantSummary(tenant)
	return &summary, nil
}

// SetTenantAllowedOrigins replaces a tenant's CORS origin allowlist (aqd Part B).
func (s *AdminService) SetTenantAllowedOrigins(ctx context.Context, slug string, origins []string) (*TenantSummary, error) {
	tenant, err := s.store.SetTenantAllowedOrigins(ctx, slug, origins)
	if err != nil {
		return nil, err
	}
	summary := tenantSummary(tenant)
	return &summary, nil
}

// SetTenantOIDCProviders replaces a tenant's external-IdP provider list (aqd
// Part C). Pass an empty list to clear them.
func (s *AdminService) SetTenantOIDCProviders(ctx context.Context, slug string, providers []store.OIDCProvider) (*TenantSummary, error) {
	tenant, err := s.store.SetTenantOIDCProviders(ctx, slug, providers)
	if err != nil {
		return nil, err
	}
	summary := tenantSummary(tenant)
	return &summary, nil
}

// IssueAPIKey issues a tenant credential.
func (s *AdminService) IssueAPIKey(ctx context.Context, tenant, label string) (*IssuedCredential, error) {
	issued, err := s.store.IssueAPIKey(ctx, tenant, label)
	if err != nil {
		return nil, err
	}
	return &IssuedCredential{
		Tenant: issued.Tenant.Slug,
		KeyID:  issued.KeyID,
		Secret: issued.Secret,
		Label:  issued.Label,
	}, nil
}

// RevokeAPIKey revokes a tenant credential.
func (s *AdminService) RevokeAPIKey(ctx context.Context, tenant, keyID string) error {
	return s.store.RevokeAPIKey(ctx, tenant, keyID)
}
