package ai

import "context"

// ExtractionRequest asks a provider to read a document and return structured
// fields. Schema is the JSON-Schema "properties" map for the fields to extract;
// keeping it generic lets any app (identity docs, PEM, assets) reuse Extract.
type ExtractionRequest struct {
	MediaType   string         // "image/jpeg", "image/png", or "application/pdf"
	Data        []byte         // raw document bytes
	Instruction string         // what to extract and how
	Schema      map[string]any // JSON-Schema properties for the returned fields
	Model       string         // optional model override
}

// ExtractionResult is the structured output of an extraction.
type ExtractionResult struct {
	Model  string
	Fields map[string]any
}

// Extract resolves the tenant's credential (or the server default) and runs a
// structured extraction. An empty tenantID resolves the server default only.
func (s *Service) Extract(ctx context.Context, tenantID string, req ExtractionRequest) (ExtractionResult, error) {
	cred, err := s.ResolveCredential(ctx, tenantID)
	if err != nil {
		return ExtractionResult{}, err
	}
	if req.Model == "" {
		req.Model = cred.Model
	}
	return s.provider.Extract(ctx, cred, req)
}
