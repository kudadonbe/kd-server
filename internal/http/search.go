package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

// searchRequest is the multi-signal entity search body. All fields are optional
// but at least one must be present. q is free text matched across names and all
// linked-record fields (vehicle number, property, business, ...).
type searchRequest struct {
	Q           string `json:"q,omitempty"`
	Name        string `json:"name,omitempty"`
	NationalID  string `json:"national_id,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
	Island      string `json:"island,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}

type searchResponse struct {
	Candidates []searchCandidate `json:"candidates"`
	Total      int               `json:"total"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
}

type searchCandidate struct {
	PersonID  string              `json:"person_id"`
	Score     int                 `json:"score"`
	MatchedOn []searchMatch       `json:"matched_on"`
	Summary   store.EntitySummary `json:"summary"`
}

type searchMatch struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Points int    `json:"points"`
}

type reindexResponse struct {
	Reindexed int `json:"reindexed"`
}

// searchHandler serves POST /v1/search — a read-only, tenant-scoped multi-signal
// people search. POST (not GET) because the query is a structured body that
// routinely carries Dhivehi/Thaana Unicode.
func searchHandler(cfg Config) http.Handler {
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

		var req searchRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		result, err := cfg.SearchService.Search(r.Context(), tenantID, services.SearchRequest{
			Q:           req.Q,
			Name:        req.Name,
			NationalID:  req.NationalID,
			Email:       req.Email,
			Phone:       req.Phone,
			DateOfBirth: req.DateOfBirth,
			Island:      req.Island,
			Limit:       req.Limit,
			Offset:      req.Offset,
		})
		if err != nil {
			if errors.Is(err, services.ErrNoSearchSignal) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one search signal is required"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
			return
		}

		writeJSON(w, http.StatusOK, toSearchResponse(result))
	})
}

// reindexHandler serves POST /v1/search/reindex — rebuilds the tenant's entity
// index (cold-start population and drift reconciliation).
func reindexHandler(cfg Config) http.Handler {
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

		count, err := cfg.SearchService.Reindex(r.Context(), tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reindex failed"})
			return
		}
		writeJSON(w, http.StatusOK, reindexResponse{Reindexed: count})
	})
}

func toSearchResponse(result *services.SearchResult) searchResponse {
	candidates := make([]searchCandidate, 0, len(result.Candidates))
	for _, c := range result.Candidates {
		matched := make([]searchMatch, 0, len(c.MatchedOn))
		for _, m := range c.MatchedOn {
			matched = append(matched, searchMatch{Field: m.Field, Value: m.Value, Points: m.Points})
		}
		candidates = append(candidates, searchCandidate{
			PersonID:  c.PersonID,
			Score:     c.Score,
			MatchedOn: matched,
			Summary:   c.Summary,
		})
	}
	return searchResponse{
		Candidates: candidates,
		Total:      result.Total,
		Limit:      result.Limit,
		Offset:     result.Offset,
	}
}
