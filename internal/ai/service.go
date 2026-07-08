package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
)

// CredentialStore persists encrypted credentials. Implemented by *store.MongoStore.
type CredentialStore interface {
	GetAICredential(ctx context.Context, tenantID, provider string) (*store.AICredential, error)
	SaveAICredential(ctx context.Context, cred store.AICredential) (*store.AICredential, error)
	ListAICredentials(ctx context.Context, tenantID string) ([]store.AICredential, error)
	DeleteAICredential(ctx context.Context, tenantID, provider string) error
}

// CredentialMeta is the non-secret view of a credential returned to clients.
// It never carries the API key.
type CredentialMeta struct {
	Provider    string     `json:"provider"`
	Model       string     `json:"model,omitempty"`
	KeyHint     string     `json:"key_hint"`
	ValidatedAt *time.Time `json:"validated_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Config configures a Service.
type Config struct {
	MasterKey    []byte   // 32 bytes for AES-256
	DefaultKey   string   // optional server-wide fallback API key
	DefaultModel string   // optional default model
	Provider     Provider // the LLM backend
}

// Service is the shared AI orchestrator: it resolves per-tenant credentials
// (with a server-wide fallback), validates and stores keys encrypted, and hides
// the vendor SDK from callers.
type Service struct {
	store        CredentialStore
	masterKey    []byte
	defaultKey   string
	defaultModel string
	provider     Provider
}

// NewService builds a Service from explicit config (used in tests and by
// NewServiceFromEnv).
func NewService(credStore CredentialStore, cfg Config) (*Service, error) {
	if credStore == nil {
		return nil, errors.New("ai: credential store is required")
	}
	if cfg.Provider == nil {
		return nil, errors.New("ai: provider is required")
	}
	// The master key is optional: without it, the env default key still works but
	// storing per-tenant/admin keys is disabled. A wrong-length key is an error.
	if len(cfg.MasterKey) != 0 && len(cfg.MasterKey) != masterKeySize {
		return nil, fmt.Errorf("ai: master key must be %d bytes", masterKeySize)
	}

	model := strings.TrimSpace(cfg.DefaultModel)
	if model == "" {
		model = DefaultModel
	}

	return &Service{
		store:        credStore,
		masterKey:    cfg.MasterKey,
		defaultKey:   strings.TrimSpace(cfg.DefaultKey),
		defaultModel: model,
		provider:     cfg.Provider,
	}, nil
}

// NewServiceFromEnv builds a Service from environment configuration. AI is
// enabled when either a server default key (ANTHROPIC_API_KEY) or an encryption
// key (AI_ENCRYPTION_KEY, which unlocks storing keys) is present. It returns
// (nil, error) when neither is set — or when AI_ENCRYPTION_KEY is set but
// invalid — so the caller can log a warning and disable the feature gracefully.
func NewServiceFromEnv(credStore CredentialStore) (*Service, error) {
	defaultKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))

	var masterKey []byte
	if raw := strings.TrimSpace(os.Getenv("AI_ENCRYPTION_KEY")); raw != "" {
		mk, err := parseMasterKey(raw)
		if err != nil {
			return nil, err
		}
		masterKey = mk
	}

	if len(masterKey) == 0 && defaultKey == "" {
		return nil, errors.New("ai: not configured (set ANTHROPIC_API_KEY and/or AI_ENCRYPTION_KEY)")
	}

	return NewService(credStore, Config{
		MasterKey:    masterKey,
		DefaultKey:   defaultKey,
		DefaultModel: os.Getenv("AI_DEFAULT_MODEL"),
		Provider:     newAnthropicProvider(),
	})
}

// encryptionEnabled reports whether stored credentials are available (a valid
// master key is present).
func (s *Service) encryptionEnabled() bool { return len(s.masterKey) == masterKeySize }

// ProviderName returns the configured provider identifier.
func (s *Service) ProviderName() string { return s.provider.Name() }

// ResolveCredential returns a usable credential for the tenant, in order of
// precedence: the tenant's own stored key, the admin-managed stored default, the
// server-wide env default, else ErrNotConfigured. An empty tenantID skips the
// tenant lookup (used by tenant-less admin flows).
func (s *Service) ResolveCredential(ctx context.Context, tenantID string) (Credential, error) {
	if tenantID != "" {
		if cred, ok, err := s.storedCredential(ctx, tenantID); err != nil {
			return Credential{}, err
		} else if ok {
			return cred, nil
		}
	}
	if cred, ok, err := s.storedCredential(ctx, globalScope); err != nil {
		return Credential{}, err
	} else if ok {
		return cred, nil
	}
	if s.defaultKey != "" {
		return Credential{Provider: s.provider.Name(), APIKey: s.defaultKey, Model: s.defaultModel}, nil
	}
	return Credential{}, ErrNotConfigured
}

// storedCredential loads and decrypts a stored credential for a scope (a tenant
// ID or the global sentinel). ok is false when none exists.
func (s *Service) storedCredential(ctx context.Context, scope string) (Credential, bool, error) {
	if !s.encryptionEnabled() {
		return Credential{}, false, nil
	}
	stored, err := s.store.GetAICredential(ctx, scope, s.provider.Name())
	if err != nil {
		return Credential{}, false, err
	}
	if stored == nil {
		return Credential{}, false, nil
	}
	key, err := decrypt(s.masterKey, stored.Ciphertext)
	if err != nil {
		return Credential{}, false, err
	}
	model := stored.Model
	if model == "" {
		model = s.defaultModel
	}
	return Credential{Provider: s.provider.Name(), APIKey: string(key), Model: model}, true, nil
}

// ValidateAndStore validates a plaintext key against the provider, then persists
// it encrypted. It returns only non-secret metadata.
func (s *Service) ValidateAndStore(ctx context.Context, tenantID, apiKey, model string) (CredentialMeta, error) {
	if !s.encryptionEnabled() {
		return CredentialMeta{}, ErrStorageDisabled
	}
	tenantID = strings.TrimSpace(tenantID)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if tenantID == "" {
		return CredentialMeta{}, errors.New("ai: tenant required")
	}
	if apiKey == "" {
		return CredentialMeta{}, errors.New("ai: api key required")
	}

	provider := s.provider.Name()
	if err := s.provider.Validate(ctx, Credential{Provider: provider, APIKey: apiKey, Model: model}); err != nil {
		return CredentialMeta{}, err
	}

	ciphertext, err := encrypt(s.masterKey, []byte(apiKey))
	if err != nil {
		return CredentialMeta{}, err
	}

	now := time.Now().UTC()
	saved, err := s.store.SaveAICredential(ctx, store.AICredential{
		TenantID:    tenantID,
		Provider:    provider,
		KeyHint:     maskKey(apiKey),
		Model:       model,
		Ciphertext:  ciphertext,
		ValidatedAt: &now,
	})
	if err != nil {
		return CredentialMeta{}, err
	}
	return metaFromStore(*saved), nil
}

// ListCredentials returns non-secret metadata for a tenant.
func (s *Service) ListCredentials(ctx context.Context, tenantID string) ([]CredentialMeta, error) {
	creds, err := s.store.ListAICredentials(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}
	metas := make([]CredentialMeta, 0, len(creds))
	for _, c := range creds {
		metas = append(metas, metaFromStore(c))
	}
	return metas, nil
}

// DeleteCredential removes a tenant credential. An empty provider defaults to the
// configured provider.
func (s *Service) DeleteCredential(ctx context.Context, tenantID, provider string) error {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = s.provider.Name()
	}
	return s.store.DeleteAICredential(ctx, strings.TrimSpace(tenantID), provider)
}

func metaFromStore(c store.AICredential) CredentialMeta {
	return CredentialMeta{
		Provider:    c.Provider,
		Model:       c.Model,
		KeyHint:     c.KeyHint,
		ValidatedAt: c.ValidatedAt,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// maskKey returns a non-secret hint from the tail of the key.
func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 4 {
		return "****"
	}
	return "..." + key[len(key)-4:]
}
