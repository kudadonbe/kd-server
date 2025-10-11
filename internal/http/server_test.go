package http_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apphttp "github.com/kudadonbe/kd-server/internal/http"
	"github.com/kudadonbe/kd-server/internal/services"
)

func TestRoutes(t *testing.T) {
	t.Parallel()

	handler := apphttp.NewHandler(apphttp.Config{
		Logger:         log.New(io.Discard, "", 0),
		VersionService: services.NewStaticVersionService("1.0.0"),
	})

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "health",
			method:     http.MethodGet,
			target:     "/v1/healthz",
			wantStatus: http.StatusOK,
			wantBody:   `{"ok":true}`,
		},
		{
			name:       "version",
			method:     http.MethodGet,
			target:     "/v1/version",
			wantStatus: http.StatusOK,
			wantBody:   `{"version":"1.0.0"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("unexpected status: got %d, want %d", res.StatusCode, tt.wantStatus)
			}

			gotBody := strings.TrimSpace(readBody(t, res))
			if gotBody != tt.wantBody {
				t.Fatalf("unexpected body: got %s, want %s", gotBody, tt.wantBody)
			}

			if ct := res.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("unexpected content-type: got %s", ct)
			}
		})
	}
}

func readBody(tb testing.TB, res *http.Response) string {
	tb.Helper()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		tb.Fatalf("failed to read response body: %v", err)
	}
	return string(bodyBytes)
}
