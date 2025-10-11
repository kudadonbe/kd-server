package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/services"
)

// Config configures the HTTP handler stack.
type Config struct {
	Logger         *log.Logger
	VersionService services.VersionProvider
}

// NewHandler wires the HTTP routes with basic middleware.
func NewHandler(cfg Config) http.Handler {
	if cfg.VersionService == nil {
		panic("http: VersionService is required")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/healthz", healthHandler)
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		version, err := cfg.VersionService.Version(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "version unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, versionResponse{Version: version})
	})

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

			tenantID := strings.TrimSpace(r.Header.Get("X-KD-Tenant"))
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
