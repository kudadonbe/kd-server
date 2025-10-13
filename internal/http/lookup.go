package http

import (
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
)

func lookupPhoneHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		e164 := strings.TrimPrefix(r.URL.Path, "/v1/lookup/phone/")
		e164 = strings.TrimSpace(e164)
		if e164 == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone required"})
			return
		}

		view, err := cfg.LookupService.ByPhone(r.Context(), tenantID, e164)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lookup failed"})
			return
		}

		writeJSON(w, http.StatusOK, view)
	})
}

func lookupEmailHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		email := strings.TrimPrefix(r.URL.Path, "/v1/lookup/email/")
		email = strings.TrimSpace(email)
		if email == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email required"})
			return
		}

		view, err := cfg.LookupService.ByEmail(r.Context(), tenantID, email)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lookup failed"})
			return
		}

		writeJSON(w, http.StatusOK, view)
	})
}
