package http_test

import (
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

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()

	verifier, err := auth.NewHMACVerifier(testSecret)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	return apphttp.NewHandler(apphttp.Config{
		Logger:         log.New(io.Discard, "", 0),
		VersionService: services.NewStaticVersionService("1.0.0"),
		AuthVerifier:   verifier,
	})
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
