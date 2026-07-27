package http_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"mime/multipart"
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
	defer func() {
		_ = res.Body.Close()
	}()

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

func TestLandingAndAdminPages(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t)
	for _, path := range []string{"/", "/admin"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: unexpected status %d", path, rec.Code)
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s: unexpected content type", path)
		}
	}
}

func TestAdminAPIAuthAndTenantCreation(t *testing.T) {
	t.Parallel()

	admin := &stubAdminStore{}
	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AdminService = services.NewAdminService(admin)
		cfg.AdminUsername = "admin-user"
		cfg.AdminPassword = "admin-password"
	})

	unauthorized := httptest.NewRequest(http.MethodGet, "/admin/api/tenants", nil)
	unauthorizedRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected unauthorized status: %d", unauthorizedRec.Code)
	}

	invalidLogin := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin-user","password":"wrong"}`))
	invalidLoginRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidLoginRec, invalidLogin)
	if invalidLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected invalid login status: %d", invalidLoginRec.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin-user","password":"admin-password"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("unexpected login status: %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	loginCookies := loginRec.Result().Cookies()
	if len(loginCookies) != 1 || !loginCookies[0].HttpOnly || loginCookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected admin session cookie: %#v", loginCookies)
	}

	create := httptest.NewRequest(http.MethodPost, "/admin/api/tenants", strings.NewReader(`{"slug":"fc","name":"Family Court"}`))
	create.AddCookie(loginCookies[0])
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createRec.Code, createRec.Body.String())
	}
	if admin.lastSlug != "fc" || admin.lastName != "Family Court" {
		t.Fatalf("unexpected tenant input: %s %s", admin.lastSlug, admin.lastName)
	}

	update := httptest.NewRequest(http.MethodPost, "/admin/api/tenants/update", strings.NewReader(`{"slug":"fc","name":"Family Court Maldives"}`))
	update.AddCookie(loginCookies[0])
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, update)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	if admin.lastName != "Family Court Maldives" {
		t.Fatalf("unexpected updated name: %s", admin.lastName)
	}
}

func TestAdminIdentityDocumentAPI(t *testing.T) {
	t.Parallel()

	documents := &stubIdentityDocuments{}
	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AdminService = services.NewAdminService(&stubAdminStore{})
		cfg.IdentityDocuments = documents
		cfg.AdminUsername = "admin-user"
		cfg.AdminPassword = "admin-password"
	})

	login := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin-user","password":"admin-password"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("unexpected login status: %d", loginRec.Code)
	}
	cookie := loginRec.Result().Cookies()[0]

	save := httptest.NewRequest(http.MethodPost, "/admin/api/identity-documents", strings.NewReader(`{
		"tenant_id":"fc",
		"person_id":"person-1",
		"national_id":"A000001",
		"name":{"english":"Sample Person","dhivehi":""},
		"common_name":{"english":"Sample","dhivehi":""},
		"address":{"house":{"english":"Example House","dhivehi":""},"island":{"english":"K. Male","dhivehi":""}},
		"source":"manual-admin",
		"extraction_method":"manual",
		"verification_status":"unverified",
		"signature_present":false,
		"fingerprint_present":false
	}`))
	save.AddCookie(cookie)
	saveRec := httptest.NewRecorder()
	handler.ServeHTTP(saveRec, save)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("unexpected save status: %d body=%s", saveRec.Code, saveRec.Body.String())
	}
	if documents.lastSaved.TenantID != "fc" || documents.lastSaved.PersonID != "person-1" {
		t.Fatalf("unexpected saved document scope: %#v", documents.lastSaved)
	}
	if documents.lastActor != "admin-user" {
		t.Fatalf("unexpected document actor: %s", documents.lastActor)
	}

	list := httptest.NewRequest(http.MethodGet, "/admin/api/identity-documents?tenant=fc&person_id=person-1", nil)
	list.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, list)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}
	if documents.lastTenant != "fc" || documents.lastPerson != "person-1" {
		t.Fatalf("unexpected list filters: tenant=%s person=%s", documents.lastTenant, documents.lastPerson)
	}
}

func TestAdminIdentityDocumentExtraction(t *testing.T) {
	t.Parallel()

	extractor := &stubDocumentExtractor{
		result: &services.DocumentExtraction{
			Engine:         "test-ocr",
			PagesProcessed: 1,
			NationalID:     "A123456",
			NameEnglish:    "Sample Person",
			RawText:        "Number A123456 Name Sample Person",
		},
	}
	handler := newTestHandler(t, func(cfg *apphttp.Config) {
		cfg.AdminService = services.NewAdminService(&stubAdminStore{})
		cfg.DocumentExtractor = extractor
		cfg.AdminUsername = "admin-user"
		cfg.AdminPassword = "admin-password"
	})

	login := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin-user","password":"admin-password"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, login)
	cookie := loginRec.Result().Cookies()[0]

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("document", "sample.jpg")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/admin/api/identity-documents/extract", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected extraction status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if extractor.filename != "sample.jpg" || extractor.contentType != "image/jpeg" {
		t.Fatalf("unexpected extraction input: %s %s", extractor.filename, extractor.contentType)
	}
	if !strings.Contains(recorder.Body.String(), `"national_id":"A123456"`) {
		t.Fatalf("unexpected extraction response: %s", recorder.Body.String())
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
			name: "invalid API key",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-a",
				"Authorization": "Bearer key_unknown.invalid",
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":"invalid API key"}`,
		},
		{
			name: "API key tenant mismatch",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-b",
				"Authorization": "Bearer key_tenant_a.valid",
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   `{"error":"invalid API key"}`,
		},
		{
			name: "API key success",
			headers: map[string]string{
				"X-KD-Tenant":   "tenant-a",
				"Authorization": "Bearer key_tenant_a.valid",
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"version":"1.0.0"}`,
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
			defer func() {
				_ = res.Body.Close()
			}()

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
	defer func() {
		_ = res.Body.Close()
	}()

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
	defer func() {
		_ = res.Body.Close()
	}()

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
	defer func() {
		_ = res.Body.Close()
	}()

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
	defer func() {
		_ = res.Body.Close()
	}()

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
		APIKeyVerifier: stubAPIKeyVerifier{
			keys: map[string]string{
				"key_tenant_a.valid": "tenant-a",
			},
		},
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

type stubAPIKeyVerifier struct {
	keys map[string]string
}

type stubIdentityDocuments struct {
	lastSaved  store.IdentityDocument
	lastActor  string
	lastTenant string
	lastPerson string
}

type stubDocumentExtractor struct {
	result      *services.DocumentExtraction
	err         error
	tenant      string
	filename    string
	contentType string
}

func (s *stubDocumentExtractor) Extract(_ context.Context, tenantID, filename, contentType string, source io.Reader) (*services.DocumentExtraction, error) {
	s.tenant = tenantID
	s.filename = filename
	s.contentType = contentType
	_, _ = io.ReadAll(source)
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *stubIdentityDocuments) List(_ context.Context, tenantID, personID string, _ int) ([]store.IdentityDocument, error) {
	s.lastTenant = tenantID
	s.lastPerson = personID
	return []store.IdentityDocument{s.lastSaved}, nil
}

func (s *stubIdentityDocuments) Save(_ context.Context, document store.IdentityDocument, actor string) (*store.IdentityDocument, error) {
	document.DocumentID = "doc_test"
	document.Version = 1
	s.lastSaved = document
	s.lastActor = actor
	return &document, nil
}

type stubAdminStore struct {
	lastSlug string
	lastName string
}

func (s *stubAdminStore) ListTenants(context.Context) ([]store.Tenant, error) {
	return []store.Tenant{}, nil
}

func (s *stubAdminStore) CreateTenant(_ context.Context, input store.CreateTenantInput) (*store.Tenant, error) {
	s.lastSlug = input.Slug
	s.lastName = input.Name
	return &store.Tenant{Slug: input.Slug, Name: input.Name, CreatedAt: time.Now()}, nil
}

func (s *stubAdminStore) UpdateTenantName(_ context.Context, slug, name string) (*store.Tenant, error) {
	s.lastSlug = slug
	s.lastName = name
	return &store.Tenant{Slug: slug, Name: name, CreatedAt: time.Now()}, nil
}

func (s *stubAdminStore) SetTenantAllowedOrigins(_ context.Context, slug string, origins []string) (*store.Tenant, error) {
	s.lastSlug = slug
	return &store.Tenant{Slug: slug, Config: store.TenantConfig{AllowedOrigins: origins}, CreatedAt: time.Now()}, nil
}

func (s *stubAdminStore) SetTenantOIDCProviders(_ context.Context, slug string, providers []store.OIDCProvider) (*store.Tenant, error) {
	s.lastSlug = slug
	return &store.Tenant{Slug: slug, Config: store.TenantConfig{OIDCProviders: providers}, CreatedAt: time.Now()}, nil
}

func (s *stubAdminStore) IssueAPIKey(context.Context, string, string) (*store.IssuedAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (s *stubAdminStore) RevokeAPIKey(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (s stubAPIKeyVerifier) VerifyAPIKey(_ context.Context, tenantID, apiKey string) error {
	if s.keys[apiKey] != tenantID {
		return errors.New("invalid API key")
	}
	return nil
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

func (s *stubReview) Decide(ctx context.Context, tenantID, rawID, decision, actor string) error {
	s.lastTenant = tenantID
	s.lastID = rawID
	s.lastDecision = decision
	if s.decisionErr != nil {
		return s.decisionErr
	}
	return nil
}
