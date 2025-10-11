package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
)

type ingestRequest struct {
	Source  ingestSourceRequest `json:"source"`
	Records []map[string]any    `json:"records"`
}

type ingestSourceRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name,omitempty"`
}

type ingestResponse struct {
	Created     int `json:"created"`
	Linked      int `json:"linked"`
	NeedsReview int `json:"needs_review"`
}

func ingestHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		var req ingestRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		result, err := cfg.IngestService.Ingest(r.Context(), tenantID, services.IngestCommand{
			Source: services.IngestSource{
				Slug: req.Source.Slug,
				Name: req.Source.Name,
			},
			Records: req.Records,
		})
		if err != nil {
			switch {
			case errors.Is(err, services.ErrSourceSlugRequired),
				errors.Is(err, services.ErrNoRecords):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			case errors.Is(err, services.ErrTenantRequired):
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant missing"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ingest failed"})
			}
			return
		}

		writeJSON(w, http.StatusOK, ingestResponse{
			Created:     result.Created,
			Linked:      result.Linked,
			NeedsReview: result.NeedsReview,
		})
	})
}
