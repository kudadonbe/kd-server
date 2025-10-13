package http_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kudadonbe/kd-server/internal/auth"
	apphttp "github.com/kudadonbe/kd-server/internal/http"
	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

const testSecret = "test-secret"

func TestHealthRoute(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", res.StatusCode, http.StatusOK)
	}

	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: got %s", ct)
	}

	if body := strings.TrimSpace(readBody(t, res)); body != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestVersionRouteAuth(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)
	validToken := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))
	invalidToken := signToken(t, "other-secret", "tenant-a", time.Now().Add(time.Hour))

	tests := []struct {
		name       string
		headers    map[string]string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "missing tenant header",
			headers:    map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"missing tenant header"}`,
		},
		{
			name: "missing authorization",
			headers: map[string]string{
				"X-KD-Tenant": "tenant-a",
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":"missing authorization"}`,
		},
		{
			name: "invalid token",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-a",
				"Authorization": "Bearer " + invalidToken,
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":"invalid token"}`,
		},
		{
			name: "tenant mismatch",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-b",
				"Authorization": "Bearer " + validToken,
			},
			wantStatus: http.StatusForbidden,
			wantBody:   `{"error":"tenant mismatch"}`,
		},
		{
			name: "success",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-a",
				"Authorization": "Bearer " + validToken,
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"version":"1.0.0"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("unexpected status: got %d want %d", res.StatusCode, tt.wantStatus)
			}

			body := strings.TrimSpace(readBody(t, res))
			if body != tt.wantBody {
				t.Fatalf("unexpected body: got %s want %s", body, tt.wantBody)
			}

			if tt.wantStatus == http.StatusOK {
				if ct := res.Header.Get("Content-Type"); ct != "application/json" {
					t.Fatalf("unexpected content-type: got %s", ct)
				}
			}
		})
	}
}

func TestIngestRoute(t *testing.T) {
	t.Parallel()

	stub := &stubIngestService{
		result: services.IngestResult{
			Created:     3,
			Linked:      1,
			NeedsReview: 2,
		},
	}

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.IngestService = stub
	})
	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))

	body := `{
		"source": {"slug": "system", "name": "System Import"},
		"records": [
			{"id": "1"},
			{"id": "2"},
			{"id": "3"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-KD-Tenant", "tenant-a")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	expectedBody := `{"created":3,"linked":1,"needs_review":2}`
	if got := strings.TrimSpace(readBody(t, res)); got != expectedBody {
		t.Fatalf("unexpected body: got %s want %s", got, expectedBody)
	}

	if stub.lastTenant != "tenant-a" {
		t.Fatalf("unexpected tenant passed to service: %s", stub.lastTenant)
	}

	if stub.lastCommand.Source.Slug != "system" {
		t.Fatalf("unexpected source slug: %s", stub.lastCommand.Source.Slug)
	}

	if len(stub.lastCommand.Records) != 3 {
		t.Fatalf("unexpected record count: %d", len(stub.lastCommand.Records))
	}
}

func TestIngestRouteValidation(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.IngestService = &stubIngestService{err: services.ErrNoRecords}
	})

	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))
	body := `{"source": {"slug": "system"}, "records": []}`

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-KD-Tenant", "tenant-a")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusBadRequest)
	}
}

func TestResolveRoute(t *testing.T) {
	t.Parallel()

	stub := &stubResolver{result: services.ResolveResult{Resolved: 5, NeedsReview: 1}}
	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.ResolveService = stub
	})

	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(`{"batch_size":10}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-KD-Tenant", "tenant-a")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}

	expected := `{"resolved":5,"needs_review":1,"skipped":0}`
	if got := strings.TrimSpace(readBody(t, res)); got != expected {
		t.Fatalf("unexpected body: %s", got)
	}

	if stub.lastTenant != "tenant-a" {
		t.Fatalf("tenant not captured")
	}
	if stub.lastOptions.BatchSize != 10 {
		t.Fatalf("batch size not captured")
	}
}

func TestLookupPhoneRoute(t *testing.T) {
	t.Parallel()

	stub := &stubLookup{
		view: &services.PersonView{
			Person: services.PersonPayload{PersonID: "p1", TenantID: "tenant-a"},
			Links:  []services.LinkPayload{{Source: "system"}},
		},
	}

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.LookupService = stub
	})

	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/v1/lookup/phone/+123", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-KD-Tenant", "tenant-a")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}

	if stub.lastPhone != "+123" {
		t.Fatalf("phone not captured")
	}
}

func TestReviewRoutes(t *testing.T) {
	t.Parallel()

	stub := &stubReview{
		items: []services.ReviewItem{{ID: "raw1", Status: store.RawStatusNeedsReview}},
	}

	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.ReviewService = stub
	})

	token := signToken(t, testSecret, "tenant-a", time.Now().Add(time.Hour))

	listReq := httptest.NewRequest(http.MethodGet, "/v1/review?status=needs_review", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listReq.Header.Set("X-KD-Tenant", "tenant-a")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, listReq)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for review list: %d", rec.Result().StatusCode)
	}

	decisionReq := httptest.NewRequest(http.MethodPost, "/v1/review/raw1/decision", strings.NewReader(`{"decision":"accept"}`))
	decisionReq.Header.Set("Authorization", "Bearer "+token)
	decisionReq.Header.Set("X-KD-Tenant", "tenant-a")

	decisionRec := httptest.NewRecorder()
	handler.ServeHTTP(decisionRec, decisionReq)

	if decisionRec.Result().StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for review decision: %d", decisionRec.Result().StatusCode)
	}

	if stub.lastDecision != "accept" || stub.lastID != "raw1" {
		t.Fatalf("decision not captured")
	}
}

func newTestHandler(t *testing.T, overrides ...func(*apphttp.Config)) http.Handler {
	t.Helper()

	verifier, err := auth.NewHMACVerifier(testSecret)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	cfg := apphttp.Config{
		Logger:         log.New(io.Discard, "", 0),
		VersionService: services.NewStaticVersionService("1.0.0"),
		AuthVerifier:   verifier,
		IngestService:  &stubIngestService{},
		ResolveService: &stubResolver{},
		LookupService:  &stubLookup{},
		ReviewService:  &stubReview{},
	}

	for _, override := range overrides {
		override(&cfg)
	}

	return apphttp.NewHandler(cfg)
}

func signToken(tb testing.TB, secret, tenant string, expiresAt time.Time) string {
	tb.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		Tenant: tenant,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		tb.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func readBody(tb testing.TB, res *http.Response) string {
	tb.Helper()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		tb.Fatalf("failed to read response body: %v", err)
	}
	return string(bodyBytes)
}

type stubIngestService struct {
	result      services.IngestResult
	err         error
	lastTenant  string
	lastCommand services.IngestCommand
}

func (s *stubIngestService) Ingest(ctx context.Context, tenantID string, command services.IngestCommand) (services.IngestResult, error) {
	s.lastTenant = tenantID
	s.lastCommand = command
	if s.err != nil {
		return services.IngestResult{}, s.err
	}
	return s.result, nil
}

type stubResolver struct {
	result      services.ResolveResult
	err         error
	lastTenant  string
	lastOptions services.ResolveOptions
}

func (s *stubResolver) Resolve(ctx context.Context, tenantID string, opts services.ResolveOptions) (services.ResolveResult, error) {
	s.lastTenant = tenantID
	s.lastOptions = opts
	if s.err != nil {
		return services.ResolveResult{}, s.err
	}
	return s.result, nil
}

type stubLookup struct {
	view       *services.PersonView
	err        error
	lastTenant string
	lastEmail  string
	lastPhone  string
}

func (s *stubLookup) ByEmail(ctx context.Context, tenantID, email string) (*services.PersonView, error) {
	s.lastTenant = tenantID
	s.lastEmail = email
	if s.err != nil {
		return nil, s.err
	}
	return s.view, nil
}

func (s *stubLookup) ByPhone(ctx context.Context, tenantID, phone string) (*services.PersonView, error) {
	s.lastTenant = tenantID
	s.lastPhone = phone
	if s.err != nil {
		return nil, s.err
	}
	return s.view, nil
}

type stubReview struct {
	items        []services.ReviewItem
	err          error
	decisionErr  error
	lastTenant   string
	lastStatus   string
	lastLimit    int
	lastID       string
	lastDecision string
}

func (s *stubReview) List(ctx context.Context, tenantID, status string, limit int) ([]services.ReviewItem, error) {
	s.lastTenant = tenantID
	s.lastStatus = status
	s.lastLimit = limit
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func (s *stubReview) Decide(ctx context.Context, tenantID, rawID, decision string) error {
	s.lastTenant = tenantID
	s.lastID = rawID
	s.lastDecision = decision
	if s.decisionErr != nil {
		return s.decisionErr
	}
	return nil
}
