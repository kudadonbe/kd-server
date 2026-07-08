package services

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kudadonbe/kd-server/internal/ai"
)

type stubAIExtractor struct {
	result ai.ExtractionResult
	err    error
	called bool
}

func (s *stubAIExtractor) Extract(_ context.Context, _ string, _ ai.ExtractionRequest) (ai.ExtractionResult, error) {
	s.called = true
	if s.err != nil {
		return ai.ExtractionResult{}, s.err
	}
	return s.result, nil
}

func (s *stubAIExtractor) ProviderName() string { return "anthropic" }

type stubFallback struct {
	result *DocumentExtraction
	called bool
}

func (s *stubFallback) Extract(_ context.Context, _, _ string, source io.Reader) (*DocumentExtraction, error) {
	s.called = true
	_, _ = io.ReadAll(source)
	return s.result, nil
}

func TestAIDocumentExtractorMapsFields(t *testing.T) {
	t.Parallel()

	aiStub := &stubAIExtractor{result: ai.ExtractionResult{
		Model: "claude-sonnet-5",
		Fields: map[string]any{
			"national_id":   "a123456",
			"name_english":  "Sample Person",
			"name_dhivehi":  "ސާމްޕަލް",
			"sex":           "m",
			"date_of_birth": "1990-01-02",
			"confidence":    "high",
			"raw_text":      "NATIONAL ID CARD ...",
			"warnings":      []any{"Photo area glare"},
		},
	}}
	fallback := &stubFallback{}
	extractor := NewAIDocumentExtractor(aiStub, fallback)

	doc, err := extractor.Extract(context.Background(), "card.jpg", "image/jpeg", strings.NewReader("fake-bytes"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if fallback.called {
		t.Fatal("fallback should not be called on AI success")
	}
	if doc.NationalID != "A123456" {
		t.Fatalf("national id not normalized: %q", doc.NationalID)
	}
	if doc.NameEnglish != "Sample Person" || doc.NameDhivehi != "ސާމްޕަލް" {
		t.Fatalf("names not mapped: %#v", doc)
	}
	if doc.Sex != "M" {
		t.Fatalf("sex not upper-cased: %q", doc.Sex)
	}
	if doc.Engine != "anthropic:claude-sonnet-5" {
		t.Fatalf("unexpected engine: %q", doc.Engine)
	}
	if doc.FieldConfidence["national_id"] != 0.95 {
		t.Fatalf("high confidence not applied: %v", doc.FieldConfidence)
	}
	if len(doc.Warnings) == 0 || doc.Warnings[0] != "Photo area glare" {
		t.Fatalf("warnings not carried: %#v", doc.Warnings)
	}
}

func TestAIDocumentExtractorFallsBackWhenNotConfigured(t *testing.T) {
	t.Parallel()

	aiStub := &stubAIExtractor{err: ai.ErrNotConfigured}
	fallback := &stubFallback{result: &DocumentExtraction{Engine: "tesseract-5"}}
	extractor := NewAIDocumentExtractor(aiStub, fallback)

	doc, err := extractor.Extract(context.Background(), "card.jpg", "image/jpeg", strings.NewReader("bytes"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !aiStub.called || !fallback.called {
		t.Fatalf("expected AI attempt then fallback: ai=%v fallback=%v", aiStub.called, fallback.called)
	}
	if doc.Engine != "tesseract-5" {
		t.Fatalf("expected fallback result, got %q", doc.Engine)
	}
}

func TestAIDocumentExtractorFallsBackOnAIError(t *testing.T) {
	t.Parallel()

	aiStub := &stubAIExtractor{err: errors.New("provider timeout")}
	fallback := &stubFallback{result: &DocumentExtraction{Engine: "tesseract-5"}}
	extractor := NewAIDocumentExtractor(aiStub, fallback)

	doc, err := extractor.Extract(context.Background(), "card.jpg", "image/jpeg", strings.NewReader("bytes"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if doc.Engine != "tesseract-5" {
		t.Fatalf("expected fallback on AI error, got %q", doc.Engine)
	}
}

func TestAIDocumentExtractorErrorsWithoutFallback(t *testing.T) {
	t.Parallel()

	aiStub := &stubAIExtractor{err: errors.New("provider timeout")}
	extractor := NewAIDocumentExtractor(aiStub, nil)

	if _, err := extractor.Extract(context.Background(), "card.jpg", "image/jpeg", strings.NewReader("bytes")); err == nil {
		t.Fatal("expected error when AI fails and no fallback exists")
	}
}
