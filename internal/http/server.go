package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/auth"
	"github.com/kudadonbe/kd-server/internal/services"
)

const tenantHeader = "X-KD-Tenant"

// Config configures the HTTP handler stack.
type Config struct {
	Logger         *log.Logger
	VersionService services.VersionProvider
	AuthVerifier   auth.Verifier
	TenantHeader   string
}

// NewHandler wires the HTTP routes with basic middleware.
func NewHandler(cfg Config) http.Handler {
	if cfg.VersionService == nil {
		panic("http: VersionService is required")
	}

	if cfg.AuthVerifier == nil {
		panic("http: AuthVerifier is required")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/healthz", healthHandler)

	versionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		version, err := cfg.VersionService.Version(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "version unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, versionResponse{Version: version})
	})

	mux.Handle("/v1/version", authMiddleware(cfg)(versionHandler))

	return loggingMiddleware(cfg.Logger)(mux)
}

type versionResponse struct {
	Version string `json:"version"`
}

type healthResponse struct {
	OK bool `json:"ok"`
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{OK: true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func loggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(lrw, r)

			if logger == nil {
				return
			}

			tenantID, ok := TenantFromContext(r.Context())
			if !ok {
				tenantID = strings.TrimSpace(r.Header.Get(tenantHeader))
			}

			logger.Printf("tenantId=%q path=%q method=%q status=%d latency=%s",
				tenantID, r.URL.Path, r.Method, lrw.status, time.Since(start))
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(status int) {
	lrw.status = status
	lrw.ResponseWriter.WriteHeader(status)
}

func authMiddleware(cfg Config) func(http.Handler) http.Handler {
	headerName := strings.TrimSpace(cfg.TenantHeader)
	if headerName == "" {
		headerName = tenantHeader
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := strings.TrimSpace(r.Header.Get(headerName))
			if tenantID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing tenant header"})
				return
			}

			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if len(authHeader) < 7 || !strings.EqualFold(authHeader[:7], "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
				return
			}

			token := strings.TrimSpace(authHeader[7:])
			claims, err := cfg.AuthVerifier.Verify(r.Context(), token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}

			if claims.Tenant != tenantID {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "tenant mismatch"})
				return
			}

			ctx := ContextWithTenant(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
