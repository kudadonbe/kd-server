package ai_test

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kudadonbe/kd-server/internal/ai"
	"github.com/kudadonbe/kd-server/internal/store"
)

type stubProvider struct {
	validateErr error
	lastKey     string
}

func (s *stubProvider) Name() string { return ai.ProviderAnthropic }

func (s *stubProvider) Validate(_ context.Context, cred ai.Credential) error {
	s.lastKey = cred.APIKey
	return s.validateErr
}

// memStore is an in-memory CredentialStore keyed by tenant|provider.
type memStore struct {
	creds map[string]store.AICredential
}

func newMemStore() *memStore { return &memStore{creds: map[string]store.AICredential{}} }

func memKey(tenant, provider string) string { return tenant + "|" + provider }

func (m *memStore) GetAICredential(_ context.Context, tenantID, provider string) (*store.AICredential, error) {
	if c, ok := m.creds[memKey(tenantID, provider)]; ok {
		return &c, nil
	}
	return nil, nil
}

func (m *memStore) SaveAICredential(_ context.Context, cred store.AICredential) (*store.AICredential, error) {
	m.creds[memKey(cred.TenantID, cred.Provider)] = cred
	return &cred, nil
}

func (m *memStore) ListAICredentials(_ context.Context, tenantID string) ([]store.AICredential, error) {
	var out []store.AICredential
	for _, c := range m.creds {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memStore) DeleteAICredential(_ context.Context, tenantID, provider string) error {
	delete(m.creds, memKey(tenantID, provider))
	return nil
}

func newTestService(t *testing.T, prov ai.Provider, defaultKey string) (*ai.Service, *memStore) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("gen master key: %v", err)
	}
	st := newMemStore()
	svc, err := ai.NewService(st, ai.Config{
		MasterKey:  key,
		DefaultKey: defaultKey,
		Provider:   prov,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc, st
}

func TestValidateAndStoreThenResolve(t *testing.T) {
	t.Parallel()

	svc, st := newTestService(t, &stubProvider{}, "")
	ctx := context.Background()

	meta, err := svc.ValidateAndStore(ctx, "tenant-a", "sk-ant-secret-1234", "claude-opus-4-8")
	if err != nil {
		t.Fatalf("validate and store: %v", err)
	}
	if meta.KeyHint != "...1234" {
		t.Fatalf("unexpected key hint %q", meta.KeyHint)
	}
	if meta.ValidatedAt == nil {
		t.Fatal("validated_at not set")
	}

	// Stored ciphertext must not contain the plaintext key.
	stored := st.creds[memKey("tenant-a", ai.ProviderAnthropic)]
	if strings.Contains(string(stored.Ciphertext), "sk-ant-secret-1234") {
		t.Fatal("stored ciphertext leaks plaintext key")
	}

	cred, err := svc.ResolveCredential(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cred.APIKey != "sk-ant-secret-1234" {
		t.Fatalf("resolved key mismatch: %q", cred.APIKey)
	}
	if cred.Model != "claude-opus-4-8" {
		t.Fatalf("resolved model mismatch: %q", cred.Model)
	}
}

func TestResolveFallsBackToDefault(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, &stubProvider{}, "sk-ant-default")
	cred, err := svc.ResolveCredential(context.Background(), "tenant-x")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cred.APIKey != "sk-ant-default" {
		t.Fatalf("expected default key, got %q", cred.APIKey)
	}
}

func TestResolveNotConfigured(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, &stubProvider{}, "")
	_, err := svc.ResolveCredential(context.Background(), "tenant-x")
	if !errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestValidateAndStoreRejectsBadKey(t *testing.T) {
	t.Parallel()

	svc, st := newTestService(t, &stubProvider{validateErr: ai.ErrInvalidKey}, "")
	_, err := svc.ValidateAndStore(context.Background(), "tenant-a", "bad-key", "")
	if !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
	if len(st.creds) != 0 {
		t.Fatal("invalid key should not be stored")
	}
}

func TestListAndDelete(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, &stubProvider{}, "")
	ctx := context.Background()

	if _, err := svc.ValidateAndStore(ctx, "tenant-a", "sk-ant-abcd", ""); err != nil {
		t.Fatalf("store: %v", err)
	}

	metas, err := svc.ListCredentials(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 || metas[0].KeyHint != "...abcd" {
		t.Fatalf("unexpected list result: %#v", metas)
	}

	if err := svc.DeleteCredential(ctx, "tenant-a", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	metas, _ = svc.ListCredentials(ctx, "tenant-a")
	if len(metas) != 0 {
		t.Fatal("credential not deleted")
	}
}
