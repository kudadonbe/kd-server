package store

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RawRecord represents a document in the raw ingest collection.
type RawRecord struct {
	ID          primitive.ObjectID `bson:"_id"`
	TenantID    string             `bson:"tenantId"`
	SourceSlug  string             `bson:"sourceSlug"`
	Payload     map[string]any     `bson:"payload"`
	Status      string             `bson:"status"`
	PersonID    string             `bson:"personId,omitempty"`
	IngestBatch string             `bson:"ingestBatch"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty"`
}

// Person represents a resolved entity stored in the people collection.
type Person struct {
	ID           primitive.ObjectID `bson:"_id"`
	TenantID     string             `bson:"tenantId"`
	PersonID     string             `bson:"personId"`
	NationalID   string             `bson:"nationalId,omitempty"`
	PrimaryEmail string             `bson:"primaryEmail,omitempty"`
	PrimaryPhone string             `bson:"primaryPhone,omitempty"`
	Attributes   map[string]any     `bson:"attributes,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"`
}

// Link represents the connection between a person and a source record.
type Link struct {
	ID         primitive.ObjectID `bson:"_id"`
	TenantID   string             `bson:"tenantId"`
	PersonID   string             `bson:"personId"`
	Source     string             `bson:"source"`
	ExternalID string             `bson:"externalId"`
	Payload    map[string]any     `bson:"payload,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
}

// Tenant represents a kd-server tenant.
type Tenant struct {
	ID        primitive.ObjectID `bson:"_id"`
	Slug      string             `bson:"slug"`
	Name      string             `bson:"name"`
	Config    TenantConfig       `bson:"config"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

// TenantConfig holds per-tenant integration settings shared across consuming
// apps. Extended as the aqd enablement track lands: CORS origins (Part B) and
// external-IdP providers for browser-safe auth (Part C).
type TenantConfig struct {
	AllowedOrigins []string       `bson:"allowedOrigins,omitempty" json:"allowed_origins,omitempty"`
	OIDCProviders  []OIDCProvider `bson:"oidcProviders,omitempty" json:"oidc_providers,omitempty"`
}

// OIDCProvider configures verification of one external-IdP browser token. A
// tenant may have several (e.g. Firebase for app login now, eFaas for
// government-verified identity later). The token's issuer selects the provider.
type OIDCProvider struct {
	Name     string `bson:"name" json:"name"`         // "firebase" | "efaas" | ...
	Issuer   string `bson:"issuer" json:"issuer"`     // must equal the token's iss
	Audience string `bson:"audience" json:"audience"` // must equal the token's aud
	JWKSURL  string `bson:"jwksUrl" json:"jwks_url"`  // RS256 public keys
	// IdentityVerified marks a provider whose token carries a government-verified
	// national ID (eFaas). Such a caller may see their own record + linked data.
	IdentityVerified bool `bson:"identityVerified,omitempty" json:"identity_verified,omitempty"`
	// NationalIDClaim names the claim holding the verified national ID (eFaas).
	NationalIDClaim string `bson:"nationalIdClaim,omitempty" json:"national_id_claim,omitempty"`
	// RoleClaim names the claim holding a staff/admin role (Firebase). Absent or
	// unknown role → public (extract only).
	RoleClaim string `bson:"roleClaim,omitempty" json:"role_claim,omitempty"`
}

// APIKey represents an API key record stored for a tenant.
type APIKey struct {
	ID        primitive.ObjectID `bson:"_id"`
	TenantID  string             `bson:"tenantId"`
	KeyID     string             `bson:"keyId"`
	KeyHash   string             `bson:"keyHash"`
	Label     string             `bson:"label,omitempty"`
	RevokedAt *time.Time         `bson:"revokedAt,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

// LocalizedText stores English and Dhivehi representations of a value.
type LocalizedText struct {
	English string `bson:"english" json:"english"`
	Dhivehi string `bson:"dhivehi" json:"dhivehi"`
}

// IdentityAddress stores the address fields printed on a Maldivian identity card.
type IdentityAddress struct {
	House  LocalizedText `bson:"house" json:"house"`
	Island LocalizedText `bson:"island" json:"island"`
}

// IdentityDocument represents an identity document linked to a resolved person.
type IdentityDocument struct {
	ID                 primitive.ObjectID `bson:"_id" json:"-"`
	TenantID           string             `bson:"tenantId" json:"tenant_id"`
	DocumentID         string             `bson:"documentId" json:"document_id"`
	PersonID           string             `bson:"personId" json:"person_id"`
	DocumentType       string             `bson:"documentType" json:"document_type"`
	CountryCode        string             `bson:"countryCode" json:"country_code"`
	NationalID         string             `bson:"nationalId" json:"national_id"`
	SerialNumber       string             `bson:"serialNumber,omitempty" json:"serial_number,omitempty"`
	Name               LocalizedText      `bson:"name" json:"name"`
	CommonName         LocalizedText      `bson:"commonName,omitempty" json:"common_name,omitempty"`
	Sex                string             `bson:"sex,omitempty" json:"sex,omitempty"`
	DateOfBirth        string             `bson:"dateOfBirth,omitempty" json:"date_of_birth,omitempty"`
	Address            IdentityAddress    `bson:"address,omitempty" json:"address,omitempty"`
	BloodGroup         string             `bson:"bloodGroup,omitempty" json:"blood_group,omitempty"`
	ExpiryDate         string             `bson:"expiryDate,omitempty" json:"expiry_date,omitempty"`
	SignaturePresent   bool               `bson:"signaturePresent" json:"signature_present"`
	FingerprintPresent bool               `bson:"fingerprintPresent" json:"fingerprint_present"`
	Source             string             `bson:"source" json:"source"`
	ExtractionMethod   string             `bson:"extractionMethod" json:"extraction_method"`
	FieldConfidence    map[string]float64 `bson:"fieldConfidence,omitempty" json:"field_confidence,omitempty"`
	VerificationStatus string             `bson:"verificationStatus" json:"verification_status"`
	VerifiedBy         string             `bson:"verifiedBy,omitempty" json:"verified_by,omitempty"`
	FrontImageRef      string             `bson:"frontImageRef,omitempty" json:"front_image_ref,omitempty"`
	BackImageRef       string             `bson:"backImageRef,omitempty" json:"back_image_ref,omitempty"`
	Version            int                `bson:"version" json:"version"`
	CreatedAt          time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt          time.Time          `bson:"updatedAt" json:"updated_at"`
}

// IdentityDocumentHistory stores an immutable identity-document snapshot.
type IdentityDocumentHistory struct {
	ID         primitive.ObjectID `bson:"_id"`
	TenantID   string             `bson:"tenantId"`
	DocumentID string             `bson:"documentId"`
	PersonID   string             `bson:"personId"`
	EventType  string             `bson:"eventType"`
	Actor      string             `bson:"actor,omitempty"`
	Version    int                `bson:"version"`
	Snapshot   IdentityDocument   `bson:"snapshot"`
	CreatedAt  time.Time          `bson:"createdAt"`
}
