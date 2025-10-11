package services

import (
	"context"
	"errors"
	"testing"

	"github.com/kudadonbe/kd-server/internal/store"
)

func TestIngestValidation(t *testing.T) {
	t.Parallel()

	svc := NewIngestService(&stubIngestWriter{})

	_, err := svc.Ingest(context.Background(), "", IngestCommand{
		Source:  IngestSource{Slug: "source"},
		Records: []map[string]any{{"id": "1"}},
	})
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}

	_, err = svc.Ingest(context.Background(), "tenant", IngestCommand{
		Source:  IngestSource{Slug: ""},
		Records: []map[string]any{{"id": "1"}},
	})
	if !errors.Is(err, ErrSourceSlugRequired) {
		t.Fatalf("expected ErrSourceSlugRequired, got %v", err)
	}

	_, err = svc.Ingest(context.Background(), "tenant", IngestCommand{
		Source:  IngestSource{Slug: "source"},
		Records: nil,
	})
	if !errors.Is(err, ErrNoRecords) {
		t.Fatalf("expected ErrNoRecords, got %v", err)
	}
}

func TestIngestSuccess(t *testing.T) {
	t.Parallel()

	writer := &stubIngestWriter{
		result: store.IngestResult{
			Created:     2,
			Linked:      1,
			NeedsReview: 0,
		},
	}
	svc := NewIngestService(writer)

	cmd := IngestCommand{
		Source: IngestSource{
			Slug: "source",
			Name: "Source Name",
		},
		Records: []map[string]any{
			{"id": "1"},
			{"id": "2"},
		},
	}

	res, err := svc.Ingest(context.Background(), "tenant", cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Created != 2 || res.Linked != 1 || res.NeedsReview != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	if writer.lastTenant != "tenant" {
		t.Fatalf("unexpected tenant passed to store: %s", writer.lastTenant)
	}

	if writer.lastBatch.Source.Slug != cmd.Source.Slug {
		t.Fatalf("unexpected slug passed to store: %s", writer.lastBatch.Source.Slug)
	}
}

type stubIngestWriter struct {
	result     store.IngestResult
	err        error
	lastTenant string
	lastBatch  store.IngestBatch
}

func (w *stubIngestWriter) SaveBatch(_ context.Context, tenantID string, batch store.IngestBatch) (store.IngestResult, error) {
	w.lastTenant = tenantID
	w.lastBatch = batch
	if w.err != nil {
		return store.IngestResult{}, w.err
	}
	return w.result, nil
}
