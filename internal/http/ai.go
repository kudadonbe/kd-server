package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kudadonbe/kd-server/internal/ai"
)

type aiCredentialRequest struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model,omitempty"`
}

// aiCredentialsHandler manages a tenant's bring-your-own AI credential. Responses
// never include the API key — only masked metadata. The key is validated against
// the provider before it is stored (encrypted).
func aiCredentialsHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		switch {
		case r.URL.Path == "/v1/ai/credentials" && r.Method == http.MethodGet:
			metas, err := cfg.AIService.ListCredentials(r.Context(), tenantID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"credentials": metas})

		case r.URL.Path == "/v1/ai/credentials" && r.Method == http.MethodPost:
			var req aiCredentialRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			if strings.TrimSpace(req.APIKey) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api_key is required"})
				return
			}
			meta, err := cfg.AIService.ValidateAndStore(r.Context(), tenantID, req.APIKey, req.Model)
			if err != nil {
				if errors.Is(err, ai.ErrInvalidKey) {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api key rejected by provider"})
					return
				}
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not validate api key"})
				return
			}
			writeJSON(w, http.StatusOK, meta)

		case strings.HasPrefix(r.URL.Path, "/v1/ai/credentials/") && r.Method == http.MethodDelete:
			provider := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/ai/credentials/"))
			if err := cfg.AIService.DeleteCredential(r.Context(), tenantID, provider); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})
}
