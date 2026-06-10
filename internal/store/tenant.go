package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
	defer func() {
		_ = cur.Close(ctx)
	}()

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

// UpdateTenantName changes a tenant's display name without changing its slug.
func (s *MongoStore) UpdateTenantName(ctx context.Context, tenantSlug, name string) (*Tenant, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	tenantSlug = normalizeSlug(tenantSlug)
	name = strings.TrimSpace(name)
	if tenantSlug == "" || name == "" {
		return nil, errors.New("store: tenant slug and name are required")
	}

	now := time.Now().UTC()
	var tenant Tenant
	err := s.db.Collection("tenants").FindOneAndUpdate(
		ctx,
		bson.M{"slug": tenantSlug},
		bson.M{"$set": bson.M{"name": name, "updatedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&tenant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("store: tenant %q not found", tenantSlug)
		}
		return nil, fmt.Errorf("store: update tenant: %w", err)
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

// VerifyAPIKey validates an issued API key for the requested tenant.
func (s *MongoStore) VerifyAPIKey(ctx context.Context, tenantID, apiKey string) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}

	tenantID = normalizeSlug(tenantID)
	keyID, secret, ok := strings.Cut(strings.TrimSpace(apiKey), ".")
	if tenantID == "" || !ok || !strings.HasPrefix(keyID, "key_") || secret == "" {
		return errors.New("store: invalid API key")
	}

	var record APIKey
	err := s.db.Collection("keys").FindOne(ctx, bson.M{
		"tenantId": tenantID,
		"keyId":    keyID,
	}).Decode(&record)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("store: invalid API key")
		}
		return fmt.Errorf("store: find API key: %w", err)
	}

	if record.RevokedAt != nil {
		return errors.New("store: invalid API key")
	}

	expectedHash, err := hex.DecodeString(record.KeyHash)
	if err != nil || len(expectedHash) != sha256.Size {
		return errors.New("store: invalid API key")
	}

	actualHash := sha256.Sum256([]byte(apiKey))
	if subtle.ConstantTimeCompare(actualHash[:], expectedHash) != 1 {
		return errors.New("store: invalid API key")
	}

	return nil
}

// RevokeAPIKey prevents an issued API key from authenticating again.
func (s *MongoStore) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}

	tenantID = normalizeSlug(tenantID)
	keyID = strings.TrimSpace(keyID)
	if tenantID == "" || !strings.HasPrefix(keyID, "key_") {
		return errors.New("store: tenant and key ID are required")
	}

	now := time.Now().UTC()
	result, err := s.db.Collection("keys").UpdateOne(
		ctx,
		bson.M{"tenantId": tenantID, "keyId": keyID, "revokedAt": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revokedAt": now, "updatedAt": now}},
	)
	if err != nil {
		return fmt.Errorf("store: revoke API key: %w", err)
	}
	if result.MatchedCount == 0 {
		return errors.New("store: active API key not found")
	}

	return nil
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
