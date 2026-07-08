package http_test

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kudadonbe/kd-server/internal/ai"
	apphttp "github.com/kudadonbe/kd-server/internal/http"
	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

type stubAIProvider struct {
	validateErr error
}

func (s stubAIProvider) Name() string { return ai.ProviderAnthropic }

func (s stubAIProvider) Validate(context.Context, ai.Credential) error { return s.validateErr }

func (s stubAIProvider) Extract(context.Context, ai.Credential, ai.ExtractionRequest) (ai.ExtractionResult, error) {
	return ai.ExtractionResult{}, nil
}

type memAICredentialStore struct {
	creds map[string]store.AICredential
}

func (m *memAICredentialStore) key(tenant, provider string) string { return tenant + "|" + provider }

func (m *memAICredentialStore) GetAICredential(_ context.Context, tenantID, provider string) (*store.AICredential, error) {
	if c, ok := m.creds[m.key(tenantID, provider)]; ok {
		return &c, nil
	}
	return nil, nil
}

func (m *memAICredentialStore) SaveAICredential(_ context.Context, cred store.AICredential) (*store.AICredential, error) {
	m.creds[m.key(cred.TenantID, cred.Provider)] = cred
	return &cred, nil
}

func (m *memAICredentialStore) ListAICredentials(_ context.Context, tenantID string) ([]store.AICredential, error) {
	var out []store.AICredential
	for _, c := range m.creds {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memAICredentialStore) DeleteAICredential(_ context.Context, tenantID, provider string) error {
	delete(m.creds, m.key(tenantID, provider))
	return nil
}

func testAIService(t *testing.T, validateErr error) *ai.Service {
	t.Helper()
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		t.Fatalf("gen master key: %v", err)
	}
	svc, err := ai.NewService(&memAICredentialStore{creds: map[string]store.AICredential{}}, ai.Config{
		MasterKey: masterKey,
		Provider:  stubAIProvider{validateErr: validateErr},
	})
	if err != nil {
		t.Fatalf("new ai service: %v", err)
	}
	return svc
}

func TestAICredentialsLifecycle(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AIService = testAIService(t, nil)
	})
	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))

	authed := func(method, path, body string) *httptest.ResponseRecorder {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-KD-Tenant", "tenant-a")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	// Store a key.
	post := authed(http.MethodPost, "/v1/ai/credentials", `{"api_key":"sk-ant-secret-9876","model":"claude-opus-4-8"}`)
	if post.Code != http.StatusOK {
		t.Fatalf("store status %d body=%s", post.Code, post.Body.String())
	}
	body := post.Body.String()
	if strings.Contains(body, "sk-ant-secret-9876") {
		t.Fatalf("response leaks api key: %s", body)
	}
	if !strings.Contains(body, `"key_hint":"...9876"`) {
		t.Fatalf("missing masked hint: %s", body)
	}
	if post.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache control")
	}

	// List returns the masked credential, still no plaintext.
	list := authed(http.MethodGet, "/v1/ai/credentials", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status %d", list.Code)
	}
	if strings.Contains(list.Body.String(), "sk-ant-secret-9876") {
		t.Fatalf("list leaks api key: %s", list.Body.String())
	}
	if !strings.Contains(list.Body.String(), `"...9876"`) {
		t.Fatalf("list missing hint: %s", list.Body.String())
	}

	// Delete, then list is empty.
	del := authed(http.MethodDelete, "/v1/ai/credentials/anthropic", "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete status %d", del.Code)
	}
	list = authed(http.MethodGet, "/v1/ai/credentials", "")
	if strings.Contains(list.Body.String(), "9876") {
		t.Fatalf("credential not deleted: %s", list.Body.String())
	}
}

func TestAICredentialsRejectsBadKey(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AIService = testAIService(t, ai.ErrInvalidKey)
	})
	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodPost, "/v1/ai/credentials", strings.NewReader(`{"api_key":"bad"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-KD-Tenant", "tenant-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for rejected key, got %d", rec.Code)
	}
}

func TestAdminAIDefaultCredential(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AdminService = services.NewAdminService(&stubAdminStore{})
		cfg.AIService = testAIService(t, nil)
		cfg.AdminUsername = "admin-user"
		cfg.AdminPassword = "admin-password"
	})

	login := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin-user","password":"admin-password"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status %d", loginRec.Code)
	}
	cookie := loginRec.Result().Cookies()[0]

	do := func(method, path, body string) *httptest.ResponseRecorder {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if st := do(http.MethodGet, "/admin/api/ai-status", ""); st.Code != http.StatusOK || !strings.Contains(st.Body.String(), `"paired":false`) {
		t.Fatalf("initial status: %d %s", st.Code, st.Body.String())
	}

	pair := do(http.MethodPost, "/admin/api/ai-credentials", `{"api_key":"sk-ant-adminkey-4321","model":"claude-sonnet-5"}`)
	if pair.Code != http.StatusOK {
		t.Fatalf("pair status %d body=%s", pair.Code, pair.Body.String())
	}
	if strings.Contains(pair.Body.String(), "sk-ant-adminkey-4321") {
		t.Fatalf("pair response leaks key: %s", pair.Body.String())
	}

	st := do(http.MethodGet, "/admin/api/ai-status", "")
	if !strings.Contains(st.Body.String(), `"paired":true`) || !strings.Contains(st.Body.String(), `"...4321"`) || !strings.Contains(st.Body.String(), `"source":"stored"`) {
		t.Fatalf("expected paired/stored: %s", st.Body.String())
	}

	if del := do(http.MethodDelete, "/admin/api/ai-credentials", ""); del.Code != http.StatusOK {
		t.Fatalf("unpair status %d", del.Code)
	}
	if st := do(http.MethodGet, "/admin/api/ai-status", ""); !strings.Contains(st.Body.String(), `"paired":false`) {
		t.Fatalf("expected unpaired after delete: %s", st.Body.String())
	}
}

func TestAICredentialsRequiresAuth(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AIService = testAIService(t, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/ai/credentials", nil)
	req.Header.Set("X-KD-Tenant", "tenant-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer token, got %d", rec.Code)
	}
}
