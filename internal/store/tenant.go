package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9\-]+$`)

// CreateTenantInput contains the data required for a new tenant.
type CreateTenantInput struct {
	Slug string
	Name string
}

// IssuedAPIKey contains the details returned after issuing a new key.
type IssuedAPIKey struct {
	Tenant   Tenant
	KeyID    string
	Secret   string
	Label    string
	IssuedAt time.Time
}

// ListTenants returns all tenants ordered by slug.
func (s *MongoStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	cur, err := s.db.Collection("tenants").Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "slug", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("store: list tenants: %w", err)
	}
	defer cur.Close(ctx)

	var tenants []Tenant
	for cur.Next(ctx) {
		var tenant Tenant
		if err := cur.Decode(&tenant); err != nil {
			return nil, fmt.Errorf("store: decode tenant: %w", err)
		}
		tenants = append(tenants, tenant)
	}

	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("store: cursor error: %w", err)
	}

	return tenants, nil
}

// CreateTenant inserts a new tenant document.
func (s *MongoStore) CreateTenant(ctx context.Context, input CreateTenantInput) (*Tenant, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	slug := normalizeSlug(input.Slug)
	if slug == "" {
		return nil, errors.New("store: tenant slug is required and must be lowercase alphanumeric or hyphen")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = strings.ToUpper(slug)
	}

	now := time.Now().UTC()
	tenant := Tenant{
		ID:        primitive.NewObjectID(),
		Slug:      slug,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := s.db.Collection("tenants").InsertOne(ctx, tenant); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("store: tenant slug %q already exists", slug)
		}
		return nil, fmt.Errorf("store: create tenant: %w", err)
	}

	return &tenant, nil
}

// IssueAPIKey creates a new API key for the given tenant slug and returns the cleartext secret.
func (s *MongoStore) IssueAPIKey(ctx context.Context, tenantSlug, label string) (*IssuedAPIKey, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	tenantSlug = normalizeSlug(tenantSlug)
	if tenantSlug == "" {
		return nil, errors.New("store: tenant slug required")
	}

	var tenant Tenant
	if err := s.db.Collection("tenants").FindOne(ctx, bson.M{"slug": tenantSlug}).Decode(&tenant); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: find tenant: %w", err)
	}

	keyID := fmt.Sprintf("key_%s", primitive.NewObjectID().Hex())
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("store: generate secret: %w", err)
	}
	fullSecret := fmt.Sprintf("%s.%s", keyID, secret)
	hash := sha256.Sum256([]byte(fullSecret))

	now := time.Now().UTC()
	record := APIKey{
		ID:        primitive.NewObjectID(),
		TenantID:  tenant.Slug,
		KeyID:     keyID,
		KeyHash:   hex.EncodeToString(hash[:]),
		Label:     strings.TrimSpace(label),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := s.db.Collection("keys").InsertOne(ctx, record); err != nil {
		return nil, fmt.Errorf("store: insert key: %w", err)
	}

	return &IssuedAPIKey{
		Tenant:   tenant,
		KeyID:    keyID,
		Secret:   fullSecret,
		Label:    record.Label,
		IssuedAt: now,
	}, nil
}

func normalizeSlug(slug string) string {
	slug = strings.TrimSpace(strings.ToLower(slug))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		return ""
	}
	if !slugPattern.MatchString(slug) {
		return ""
	}
	return slug
}

func generateSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
