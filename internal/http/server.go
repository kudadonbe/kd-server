package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/ai"
	"github.com/kudadonbe/kd-server/internal/auth"
	"github.com/kudadonbe/kd-server/internal/services"
)

const tenantHeader = "X-KD-Tenant"

// Config configures the HTTP handler stack.
type Config struct {
	Logger                *log.Logger
	VersionService        services.VersionProvider
	AuthVerifier          auth.Verifier
	APIKeyVerifier        auth.APIKeyVerifier
	TenantHeader          string
	IngestService         services.Ingestor
	ResolveService        services.Resolver
	LookupService         services.Lookup
	ReviewService         services.Review
	AssetService          *services.AssetService
	ClassificationService *services.ClassificationService
	AdminService          *services.AdminService
	IdentityDocuments     services.IdentityDocuments
	DocumentExtractor     services.DocumentExtractor
	AIService             *ai.Service
	AdminUsername         string
	AdminPassword         string
	ServerName            string
	Environment           string
}

// NewHandler wires the HTTP routes with basic middleware.
func NewHandler(cfg Config) http.Handler {
	if cfg.VersionService == nil {
		panic("http: VersionService is required")
	}

	if cfg.AuthVerifier == nil {
		panic("http: AuthVerifier is required")
	}

	if cfg.APIKeyVerifier == nil {
		panic("http: APIKeyVerifier is required")
	}

	if cfg.IngestService == nil {
		panic("http: IngestService is required")
	}

	if cfg.ResolveService == nil {
		panic("http: ResolveService is required")
	}

	if cfg.LookupService == nil {
		panic("http: LookupService is required")
	}

	if cfg.ReviewService == nil {
		panic("http: ReviewService is required")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", landingHandler)
	mux.Handle("/admin/static/", staticHandler())

	adminAuth := newAdminAuthenticator(cfg.AdminUsername, cfg.AdminPassword)
	branding := newBrandingState(cfg.ServerName, cfg.Environment)
	console := adminConsoleHandler(cfg, adminAuth, branding)
	mux.Handle("/admin", console)
	mux.Handle("/admin/", console)
	if cfg.AdminService != nil {
		mux.HandleFunc("/admin/identity", identityDocumentPageHandler)
		mux.Handle("/admin/hx/", adminHXHandler(cfg, adminAuth, branding))
		mux.Handle("/admin/api/", adminAPIHandler(cfg, adminAuth))
	}
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
	mux.Handle("/v1/ingest", authMiddleware(cfg)(ingestHandler(cfg)))
	mux.Handle("/v1/resolve", authMiddleware(cfg)(resolveHandler(cfg)))
	mux.Handle("/v1/lookup/phone/", authMiddleware(cfg)(lookupPhoneHandler(cfg)))
	mux.Handle("/v1/lookup/email/", authMiddleware(cfg)(lookupEmailHandler(cfg)))
	mux.Handle("/v1/review", authMiddleware(cfg)(reviewListHandler(cfg)))
	mux.Handle("/v1/review/", authMiddleware(cfg)(reviewDecisionHandler(cfg)))

	// Shared AI credential management (tenant brings its own key). Registered
	// only when the AI service is configured (AI_ENCRYPTION_KEY set).
	if cfg.AIService != nil {
		mux.Handle("/v1/ai/credentials", authMiddleware(cfg)(aiCredentialsHandler(cfg)))
		mux.Handle("/v1/ai/credentials/", authMiddleware(cfg)(aiCredentialsHandler(cfg)))
	}

	// Asset management endpoints
	if cfg.AssetService != nil {
		mux.Handle("/v1/ingest/assets", authMiddleware(cfg)(assetIngestHandler(cfg)))
		mux.Handle("/v1/lookup/asset/", authMiddleware(cfg)(assetLookupHandler(cfg)))
		mux.Handle("/v1/assets", authMiddleware(cfg)(assetsQueryHandler(cfg)))

		// Note: Order matters - more specific routes first
		mux.HandleFunc("/v1/assets/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/history") {
				authMiddleware(cfg)(assetHistoryHandler(cfg)).ServeHTTP(w, r)
			} else if strings.HasSuffix(r.URL.Path, "/transfer") && r.Method == "POST" {
				authMiddleware(cfg)(assetTransferHandler(cfg)).ServeHTTP(w, r)
			} else if strings.HasSuffix(r.URL.Path, "/dispose") && r.Method == "POST" {
				authMiddleware(cfg)(assetDisposeHandler(cfg)).ServeHTTP(w, r)
			} else {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			}
		})
	}

	// Asset classification endpoints
	if cfg.ClassificationService != nil {
		mux.Handle("/v1/assets/categories/", authMiddleware(cfg)(assetCategoryTypesHandler(cfg)))
		mux.Handle("/v1/assets/categories", authMiddleware(cfg)(assetCategoriesHandler(cfg)))
	}

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
			if strings.HasPrefix(token, "key_") {
				if err := cfg.APIKeyVerifier.VerifyAPIKey(r.Context(), tenantID, token); err != nil {
					writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
					return
				}

				ctx := ContextWithTenant(r.Context(), tenantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

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
