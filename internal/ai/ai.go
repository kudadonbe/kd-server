// Package ai is the shared, provider-agnostic AI capability for kd-server. Any
// app (identity documents, PEM, assets) uses it through the Provider interface
// and the Service orchestrator rather than talking to a vendor SDK directly.
// Credentials are bring-your-own, stored encrypted at rest, and resolved per
// tenant with an optional server-wide default. See docs — the plan file for the
// AI foundation.
package ai

import (
	"context"
	"errors"
)

// ProviderAnthropic identifies the Anthropic (Claude) provider.
const ProviderAnthropic = "anthropic"

// DefaultModel is used when a credential does not pin its own model.
const DefaultModel = "claude-sonnet-5"

var (
	// ErrNotConfigured means no credential could be resolved for the tenant
	// (no tenant key and no server-wide default). Callers treat this as
	// "feature disabled" rather than a hard error.
	ErrNotConfigured = errors.New("ai: no credential configured")

	// ErrInvalidKey means the provider rejected the API key.
	ErrInvalidKey = errors.New("ai: api key rejected by provider")

	// ErrStorageDisabled means a key cannot be stored because encryption is off
	// (AI_ENCRYPTION_KEY is not set). The env default key still works.
	ErrStorageDisabled = errors.New("ai: credential storage disabled (set AI_ENCRYPTION_KEY)")
)

// Credential is a resolved, decrypted credential ready to call a provider.
type Credential struct {
	Provider string
	APIKey   string
	Model    string
}

// Provider is an LLM backend.
type Provider interface {
	// Name returns the provider identifier (e.g. "anthropic").
	Name() string
	// Validate confirms the credential works via a minimal, cheap call.
	Validate(ctx context.Context, cred Credential) error
	// Extract reads a document (image or PDF) and returns structured fields
	// matching the request's schema.
	Extract(ctx context.Context, cred Credential, req ExtractionRequest) (ExtractionResult, error)
}
