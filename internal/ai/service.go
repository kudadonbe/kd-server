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
	if len(cfg.MasterKey) != masterKeySize {
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

// NewServiceFromEnv builds a Service from environment configuration. It returns
// (nil, error) when AI_ENCRYPTION_KEY is unset or invalid, so the caller can log
// a warning and disable the feature gracefully (like NewLocalDocumentExtractor).
func NewServiceFromEnv(credStore CredentialStore) (*Service, error) {
	masterKey, err := parseMasterKey(os.Getenv("AI_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	return NewService(credStore, Config{
		MasterKey:    masterKey,
		DefaultKey:   os.Getenv("ANTHROPIC_API_KEY"),
		DefaultModel: os.Getenv("AI_DEFAULT_MODEL"),
		Provider:     newAnthropicProvider(),
	})
}

// ProviderName returns the configured provider identifier.
func (s *Service) ProviderName() string { return s.provider.Name() }

// ResolveCredential returns a usable credential for the tenant: the tenant's own
// stored key, else the server-wide default, else ErrNotConfigured.
func (s *Service) ResolveCredential(ctx context.Context, tenantID string) (Credential, error) {
	provider := s.provider.Name()

	stored, err := s.store.GetAICredential(ctx, tenantID, provider)
	if err != nil {
		return Credential{}, err
	}
	if stored != nil {
		key, err := decrypt(s.masterKey, stored.Ciphertext)
		if err != nil {
			return Credential{}, err
		}
		model := stored.Model
		if model == "" {
			model = s.defaultModel
		}
		return Credential{Provider: provider, APIKey: string(key), Model: model}, nil
	}

	if s.defaultKey != "" {
		return Credential{Provider: provider, APIKey: s.defaultKey, Model: s.defaultModel}, nil
	}
	return Credential{}, ErrNotConfigured
}

// ValidateAndStore validates a plaintext key against the provider, then persists
// it encrypted. It returns only non-secret metadata.
func (s *Service) ValidateAndStore(ctx context.Context, tenantID, apiKey, model string) (CredentialMeta, error) {
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
