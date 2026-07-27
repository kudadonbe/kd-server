package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

// IdentityFinalizer saves reviewed extraction as a versioned identity document
// and promotes a document to verified.
type IdentityFinalizer interface {
	Finalize(ctx context.Context, tenantID, actor, phone string, doc store.IdentityDocument) (*services.FinalizeResult, error)
	Verify(ctx context.Context, tenantID, documentID, actor string) (*store.IdentityDocument, error)
}

// identityDocumentExtractHandler serves POST /v1/identity-documents/extract — a
// tenant-facing mirror of the admin extract. It reads an uploaded identity
// document and returns suggested fields for the client to review; nothing is
// looked up, saved, or persisted. The tenant is resolved from auth context so
// per-tenant AI credentials apply (falling back to the server default).
func identityDocumentExtractHandler(cfg Config) http.Handler {
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

		r.Body = http.MaxBytesReader(w, r.Body, maxAdminDocumentUpload)
		if err := r.ParseMultipartForm(maxAdminDocumentUpload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document must be a PDF, JPEG, or PNG no larger than 10 MB"})
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		file, header, err := r.FormFile("document")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document file is required"})
			return
		}
		defer func() {
			_ = file.Close()
		}()

		prefix := make([]byte, 512)
		read, readErr := file.Read(prefix)
		if readErr != nil && readErr != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read document failed"})
			return
		}
		prefix = prefix[:read]
		contentType := http.DetectContentType(prefix)
		if !allowedDocumentUpload(header.Filename, contentType) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only PDF, JPEG, and PNG documents are supported"})
			return
		}

		extraction, err := cfg.DocumentExtractor.Extract(
			r.Context(),
			tenantID,
			filepath.Base(header.Filename),
			contentType,
			io.MultiReader(bytes.NewReader(prefix), file),
		)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}

		// Untrusted (extract-only) callers cannot finalize, so any genuinely new
		// info a trace surfaces would be lost. Quarantine it to the review queue
		// as an untrusted lead. Best-effort and side-effect-only: the response is
		// unchanged and never reveals what (if anything) was captured.
		captureExtractionForReview(r.Context(), cfg, tenantID, extraction)

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, extraction)
	})
}

// finalizeRequest is the reviewed identity document to save. It is the document
// shape plus an optional phone used only to resolve-or-create the person.
type finalizeRequest struct {
	store.IdentityDocument
	Phone string `json:"phone,omitempty"`
}

// identityDocumentFinalizeHandler serves POST /v1/identity-documents — save a
// reviewed identity document (resolve-or-create person, stored unverified).
// Gated by records:write. The server owns verification_status.
func identityDocumentFinalizeHandler(cfg Config) http.Handler {
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

		var req finalizeRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		result, err := cfg.IdentityFinalize.Finalize(r.Context(), tenantID, actorFromContext(r.Context()), req.Phone, req.IdentityDocument)
		if err != nil {
			var verr *services.ValidationError
			if errors.As(err, &verr) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": verr.Errors})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "finalize failed"})
			return
		}
		// A change that would alter a verified record is queued, not applied.
		status := http.StatusOK
		if result.Status == services.FinalizeStatusQueuedForReview {
			status = http.StatusAccepted
		}
		writeJSON(w, status, result)
	})
}

// identityDocumentVerifyHandler serves POST /v1/identity-documents/{id}/verify —
// promote a document to verified, recording the authorized actor. Gated by the
// verify scope.
func identityDocumentVerifyHandler(cfg Config) http.Handler {
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
		documentID := r.PathValue("id")
		saved, err := cfg.IdentityFinalize.Verify(r.Context(), tenantID, documentID, actorFromContext(r.Context()))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, saved)
	})
}

// captureExtractionForReview quarantines newly-traced info from an untrusted
// (extract-only) caller into the review queue. It is a no-op for trusted
// callers (they finalize instead), when no capture service is wired, or when the
// trace carries no national ID to resolve against. Errors are swallowed — this
// must never affect the extraction response.
func captureExtractionForReview(ctx context.Context, cfg Config, tenantID string, ext *services.DocumentExtraction) {
	if cfg.IdentityCapture == nil || ext == nil || strings.TrimSpace(ext.NationalID) == "" {
		return
	}
	if p, ok := PrincipalFromContext(ctx); ok && p.HasScope(ScopeRecordsWrite) {
		return // trusted callers capture via finalize, not extract
	}
	doc := documentFromExtraction(ext)
	_, _, _ = cfg.IdentityCapture.Capture(ctx, tenantID, "public:"+tenantID, services.CaptureTrustUntrusted, "", doc)
}

// documentFromExtraction maps a suggestion-only extraction into the identity
// document shape used for diffing and storage.
func documentFromExtraction(ext *services.DocumentExtraction) store.IdentityDocument {
	return store.IdentityDocument{
		NationalID:       ext.NationalID,
		Name:             store.LocalizedText{English: ext.NameEnglish, Dhivehi: ext.NameDhivehi},
		CommonName:       store.LocalizedText{English: ext.CommonName},
		Sex:              ext.Sex,
		DateOfBirth:      ext.DateOfBirth,
		Address:          store.IdentityAddress{House: store.LocalizedText{English: ext.HouseEnglish, Dhivehi: ext.HouseDhivehi}, Island: store.LocalizedText{English: ext.IslandEnglish, Dhivehi: ext.IslandDhivehi}},
		BloodGroup:       ext.BloodGroup,
		ExpiryDate:       ext.ExpiryDate,
		SerialNumber:     ext.SerialNumber,
		ExtractionMethod: ext.Engine,
	}
}

// actorFromContext returns the audit actor for the request (the principal's
// subject — API key id or end-user sub), falling back to the auth mode.
func actorFromContext(ctx context.Context) string {
	if p, ok := PrincipalFromContext(ctx); ok {
		if p.Subject != "" {
			return p.Subject
		}
		return string(p.Mode)
	}
	return ""
}
