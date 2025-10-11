package services

import (
	"context"
	"errors"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
)

// IngestSource describes the source metadata provided by clients.
type IngestSource struct {
	Slug string
	Name string
}

// IngestCommand captures all fields required to ingest a batch of records.
type IngestCommand struct {
	Source  IngestSource
	Records []map[string]any
}

// IngestResult summarizes the ingest outcome.
type IngestResult struct {
	Created     int
	Linked      int
	NeedsReview int
}

var (
	// ErrTenantRequired indicates the tenant context was not supplied.
	ErrTenantRequired = errors.New("services: tenant required")
	// ErrSourceSlugRequired indicates the source slug is missing.
	ErrSourceSlugRequired = errors.New("services: source slug required")
	// ErrNoRecords indicates the ingest payload had no records.
	ErrNoRecords = errors.New("services: at least one record required")
)

// Ingestor defines the ingest service behaviour used by HTTP handlers.
type Ingestor interface {
	Ingest(ctx context.Context, tenantID string, command IngestCommand) (IngestResult, error)
}

// IngestService implements ingest orchestration.
type IngestService struct {
	writer store.IngestWriter
}

// NewIngestService creates a new ingest service.
func NewIngestService(writer store.IngestWriter) *IngestService {
	return &IngestService{writer: writer}
}

// Ingest validates the command then persists the batch.
func (s *IngestService) Ingest(ctx context.Context, tenantID string, command IngestCommand) (IngestResult, error) {
	if strings.TrimSpace(tenantID) == "" {
		return IngestResult{}, ErrTenantRequired
	}

	if strings.TrimSpace(command.Source.Slug) == "" {
		return IngestResult{}, ErrSourceSlugRequired
	}

	if len(command.Records) == 0 {
		return IngestResult{}, ErrNoRecords
	}

	if s == nil || s.writer == nil {
		return IngestResult{}, errors.New("services: ingest writer not configured")
	}

	result, err := s.writer.SaveBatch(ctx, tenantID, store.IngestBatch{
		Source: store.IngestSource{
			Slug: command.Source.Slug,
			Name: command.Source.Name,
		},
		Records: command.Records,
	})
	if err != nil {
		return IngestResult{}, err
	}

	return IngestResult{
		Created:     result.Created,
		Linked:      result.Linked,
		NeedsReview: result.NeedsReview,
	}, nil
}

var _ Ingestor = (*IngestService)(nil)
