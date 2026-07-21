package http

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
)

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

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, extraction)
	})
}
