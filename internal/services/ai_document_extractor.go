package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kudadonbe/kd-server/internal/ai"
)

// maxAIDocumentBytes caps the document size sent to the vision model. Larger
// uploads fall back to OCR. Matches the practical per-image ceiling.
const maxAIDocumentBytes = 5 << 20

const identityInstruction = `This is a Maldivian national identity card (front or back). Read every printed field, including both the English/Latin text and the Dhivehi (Thaana) script. Record each field exactly as printed — do not translate or transliterate. Leave a field empty if it is not present on this side or is not legible. Dates must be ISO format YYYY-MM-DD. Do not guess; set confidence to "low" and add a warning when unsure.`

// identitySchema is the JSON-Schema "properties" map the model fills in.
var identitySchema = map[string]any{
	"national_id":         map[string]any{"type": "string", "description": "National ID number, e.g. A123456"},
	"name_english":        map[string]any{"type": "string", "description": "Full name in English/Latin script"},
	"name_dhivehi":        map[string]any{"type": "string", "description": "Full name in Dhivehi (Thaana script)"},
	"sex":                 map[string]any{"type": "string", "description": "M or F"},
	"date_of_birth":       map[string]any{"type": "string", "description": "Date of birth, ISO YYYY-MM-DD"},
	"house_english":       map[string]any{"type": "string", "description": "House/address name in English"},
	"house_dhivehi":       map[string]any{"type": "string", "description": "House/address name in Dhivehi"},
	"island_english":      map[string]any{"type": "string", "description": "Island in English"},
	"island_dhivehi":      map[string]any{"type": "string", "description": "Island in Dhivehi"},
	"common_name_english": map[string]any{"type": "string", "description": "Common name in English (back of card)"},
	"blood_group":         map[string]any{"type": "string", "description": "Blood group, e.g. O+"},
	"expiry_date":         map[string]any{"type": "string", "description": "Expiry date, ISO YYYY-MM-DD"},
	"serial_number":       map[string]any{"type": "string", "description": "Card serial number (back of card)"},
	"raw_text":            map[string]any{"type": "string", "description": "All visible text on the card, English and Dhivehi"},
	"confidence":          map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}, "description": "Overall extraction confidence"},
	"warnings":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Fields that were unclear or assumptions made"},
}

// AIExtractor is the subset of *ai.Service the document extractor uses.
type AIExtractor interface {
	Extract(ctx context.Context, tenantID string, req ai.ExtractionRequest) (ai.ExtractionResult, error)
	ProviderName() string
}

// AIDocumentExtractor extracts identity-document fields with a vision model and
// falls back to a secondary extractor (Tesseract OCR) when AI is not configured,
// the document is too large, or the AI call fails. It implements DocumentExtractor
// and returns the same DocumentExtraction the staff review form reads, so no
// handler changes are required. Output is always suggestion-only for human review.
type AIDocumentExtractor struct {
	ai       AIExtractor
	fallback DocumentExtractor
}

// NewAIDocumentExtractor builds the composite. fallback may be nil.
func NewAIDocumentExtractor(aiExtractor AIExtractor, fallback DocumentExtractor) *AIDocumentExtractor {
	return &AIDocumentExtractor{ai: aiExtractor, fallback: fallback}
}

// Extract runs the vision model first, then OCR as a fallback.
func (e *AIDocumentExtractor) Extract(ctx context.Context, filename, contentType string, source io.Reader) (*DocumentExtraction, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("services: read document: %w", err)
	}

	var aiErr error
	if e.ai != nil && len(data) <= maxAIDocumentBytes {
		result, err := e.ai.Extract(ctx, "", ai.ExtractionRequest{
			MediaType:   normalizeDocumentMediaType(contentType, filename),
			Data:        data,
			Instruction: identityInstruction,
			Schema:      identitySchema,
		})
		if err == nil {
			return identityExtractionFromFields(e.ai.ProviderName(), result), nil
		}
		aiErr = err
	}

	if e.fallback != nil {
		return e.fallback.Extract(ctx, filename, contentType, bytes.NewReader(data))
	}
	if aiErr != nil && !errors.Is(aiErr, ai.ErrNotConfigured) {
		return nil, aiErr
	}
	return nil, errors.New("services: document extraction not configured")
}

func identityExtractionFromFields(provider string, result ai.ExtractionResult) *DocumentExtraction {
	f := result.Fields
	confidence := confidenceScore(fieldString(f, "confidence"))

	doc := &DocumentExtraction{
		Engine:          provider + ":" + result.Model,
		PagesProcessed:  1,
		RawText:         fieldString(f, "raw_text"),
		NationalID:      strings.ToUpper(strings.ReplaceAll(fieldString(f, "national_id"), " ", "")),
		NameEnglish:     fieldString(f, "name_english"),
		NameDhivehi:     fieldString(f, "name_dhivehi"),
		Sex:             strings.ToUpper(fieldString(f, "sex")),
		DateOfBirth:     fieldString(f, "date_of_birth"),
		HouseEnglish:    fieldString(f, "house_english"),
		HouseDhivehi:    fieldString(f, "house_dhivehi"),
		IslandEnglish:   fieldString(f, "island_english"),
		IslandDhivehi:   fieldString(f, "island_dhivehi"),
		CommonName:      fieldString(f, "common_name_english"),
		BloodGroup:      fieldString(f, "blood_group"),
		ExpiryDate:      fieldString(f, "expiry_date"),
		SerialNumber:    fieldString(f, "serial_number"),
		FieldConfidence: map[string]float64{},
	}

	for key, value := range map[string]string{
		"national_id":         doc.NationalID,
		"name_english":        doc.NameEnglish,
		"name_dhivehi":        doc.NameDhivehi,
		"sex":                 doc.Sex,
		"date_of_birth":       doc.DateOfBirth,
		"house_english":       doc.HouseEnglish,
		"house_dhivehi":       doc.HouseDhivehi,
		"island_english":      doc.IslandEnglish,
		"island_dhivehi":      doc.IslandDhivehi,
		"common_name_english": doc.CommonName,
		"blood_group":         doc.BloodGroup,
		"expiry_date":         doc.ExpiryDate,
		"serial_number":       doc.SerialNumber,
	} {
		if value != "" {
			doc.FieldConfidence[key] = confidence
		}
	}

	doc.Warnings = append(doc.Warnings, fieldStrings(f, "warnings")...)
	doc.Warnings = append(doc.Warnings, "Review every field against the original document before saving.")
	return doc
}

func fieldString(fields map[string]any, key string) string {
	if v, ok := fields[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func fieldStrings(fields map[string]any, key string) []string {
	raw, ok := fields[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func confidenceScore(level string) float64 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return 0.95
	case "low":
		return 0.6
	default:
		return 0.8
	}
}

func normalizeDocumentMediaType(contentType, filename string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "application/pdf", "image/png", "image/jpeg":
		return ct
	}
	switch strings.ToLower(strings.TrimSpace(filename[strings.LastIndex(filename, ".")+1:])) {
	case "pdf":
		return "application/pdf"
	case "png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}
