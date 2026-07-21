package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TenantAllowedOrigins returns the CORS origins configured for one tenant.
// A missing tenant yields an error; a tenant with no origins yields an empty
// slice (CORS disabled for it).
func (s *MongoStore) TenantAllowedOrigins(ctx context.Context, tenantSlug string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantSlug = normalizeSlug(tenantSlug)
	if tenantSlug == "" {
		return nil, errors.New("store: tenant slug required")
	}

	var tenant Tenant
	err := s.db.Collection("tenants").FindOne(ctx, bson.M{"slug": tenantSlug}).Decode(&tenant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: read tenant origins: %w", err)
	}
	return tenant.Config.AllowedOrigins, nil
}

// OriginRegistered reports whether any tenant allows the given origin. CORS
// preflight (OPTIONS) carries no X-KD-Tenant header, so the tenant is unknown
// at that point — the preflight is answered from this tenant-agnostic check,
// and the actual request is then validated against its specific tenant.
func (s *MongoStore) OriginRegistered(ctx context.Context, origin string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("store: mongo not configured")
	}
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false, nil
	}
	count, err := s.db.Collection("tenants").CountDocuments(
		ctx,
		bson.M{"config.allowedOrigins": origin},
		options.Count().SetLimit(1),
	)
	if err != nil {
		return false, fmt.Errorf("store: check origin: %w", err)
	}
	return count > 0, nil
}

// SetTenantAllowedOrigins replaces a tenant's CORS origin allowlist. Origins are
// trimmed, de-duplicated, and stored verbatim (scheme+host+optional port, no
// trailing slash) so they match the browser's Origin header exactly.
func (s *MongoStore) SetTenantAllowedOrigins(ctx context.Context, tenantSlug string, origins []string) (*Tenant, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantSlug = normalizeSlug(tenantSlug)
	if tenantSlug == "" {
		return nil, errors.New("store: tenant slug required")
	}

	cleaned := normalizeOrigins(origins)
	now := time.Now().UTC()
	var tenant Tenant
	err := s.db.Collection("tenants").FindOneAndUpdate(
		ctx,
		bson.M{"slug": tenantSlug},
		bson.M{"$set": bson.M{"config.allowedOrigins": cleaned, "updatedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&tenant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: set tenant origins: %w", err)
	}
	return &tenant, nil
}

// TenantOIDCProviders returns the external-IdP providers for a tenant (empty if
// none). Used by the auth layer to verify browser end-user tokens.
func (s *MongoStore) TenantOIDCProviders(ctx context.Context, tenantSlug string) ([]OIDCProvider, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantSlug = normalizeSlug(tenantSlug)
	if tenantSlug == "" {
		return nil, errors.New("store: tenant slug required")
	}

	var tenant Tenant
	err := s.db.Collection("tenants").FindOne(ctx, bson.M{"slug": tenantSlug}).Decode(&tenant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: read tenant oidc: %w", err)
	}
	return tenant.Config.OIDCProviders, nil
}

// SetTenantOIDCProviders replaces a tenant's external-IdP provider list. Each
// provider needs name, issuer, audience, and jwks_url; passing an empty list
// clears them.
func (s *MongoStore) SetTenantOIDCProviders(ctx context.Context, tenantSlug string, providers []OIDCProvider) (*Tenant, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantSlug = normalizeSlug(tenantSlug)
	if tenantSlug == "" {
		return nil, errors.New("store: tenant slug required")
	}

	cleaned := make([]OIDCProvider, 0, len(providers))
	for _, p := range providers {
		p.Name = strings.TrimSpace(p.Name)
		p.Issuer = strings.TrimSpace(p.Issuer)
		p.Audience = strings.TrimSpace(p.Audience)
		p.JWKSURL = strings.TrimSpace(p.JWKSURL)
		if p.Name == "" || p.Issuer == "" || p.Audience == "" || p.JWKSURL == "" {
			return nil, errors.New("store: oidc provider needs name, issuer, audience, and jwks_url")
		}
		cleaned = append(cleaned, p)
	}

	var tenant Tenant
	err := s.db.Collection("tenants").FindOneAndUpdate(
		ctx,
		bson.M{"slug": tenantSlug},
		bson.M{"$set": bson.M{"config.oidcProviders": cleaned, "updatedAt": time.Now().UTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&tenant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: set tenant oidc: %w", err)
	}
	return &tenant, nil
}

// normalizeOrigins trims, drops trailing slashes, and de-duplicates origins
// while preserving order. Empty entries are removed.
func normalizeOrigins(origins []string) []string {
	seen := make(map[string]struct{}, len(origins))
	cleaned := make([]string, 0, len(origins))
	for _, o := range origins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		if _, dup := seen[o]; dup {
			continue
		}
		seen[o] = struct{}{}
		cleaned = append(cleaned, o)
	}
	return cleaned
}
