package http

import (
	"encoding/json"
	"net/http"
	"strings"
)

type reviewDecisionRequest struct {
	Decision string `json:"decision"`
}

func reviewListHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		status := r.URL.Query().Get("status")
		items, err := cfg.ReviewService.List(r.Context(), tenantID, status, 100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "review fetch failed"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
}

func reviewDecisionHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		rawPath := strings.TrimPrefix(r.URL.Path, "/v1/review/")
		parts := strings.Split(rawPath, "/")
		if len(parts) < 2 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return
		}

		rawID := parts[0]

		var req reviewDecisionRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		if err := cfg.ReviewService.Decide(r.Context(), tenantID, rawID, req.Decision); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	})
}
