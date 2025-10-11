package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/kudadonbe/kd-server/internal/services"
)

type resolveRequest struct {
	BatchSize int `json:"batch_size,omitempty"`
}

type resolveResponse struct {
	Resolved int `json:"resolved"
`
	NeedsReview int `json:"needs_review"
`
	Skipped int `json:"skipped"
`
}

func resolveHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		var req resolveRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		result, err := cfg.ResolveService.Resolve(r.Context(), tenantID, services.ResolveOptions{BatchSize: req.BatchSize})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "resolve failed"})
			return
		}

		writeJSON(w, http.StatusOK, resolveResponse{
			Resolved:    result.Resolved,
			NeedsReview: result.NeedsReview,
			Skipped:     result.Skipped,
		})
	})
}
