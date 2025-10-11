package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IngestSource describes metadata about the data source for a batch.
type IngestSource struct {
	Slug string
	Name string
}

// IngestBatch contains the records to persist for a tenant.
type IngestBatch struct {
	Source  IngestSource
	Records []map[string]any
}

// IngestResult reports the outcome of writing a batch.
type IngestResult struct {
	Created     int
	Linked      int
	NeedsReview int
}

// IngestWriter persists ingest batches for a tenant.
type IngestWriter interface {
	SaveBatch(ctx context.Context, tenantID string, batch IngestBatch) (IngestResult, error)
}

// MongoIngestWriter implements IngestWriter backed by MongoDB.
type MongoIngestWriter struct {
	db *mongo.Database
}

// IngestWriter returns an IngestWriter bound to the MongoStore database.
func (s *MongoStore) IngestWriter() IngestWriter {
	return &MongoIngestWriter{db: s.db}
}

// SaveBatch writes the ingest records and upserts the source metadata.
func (w *MongoIngestWriter) SaveBatch(ctx context.Context, tenantID string, batch IngestBatch) (IngestResult, error) {
	if w == nil || w.db == nil {
		return IngestResult{}, errors.New("store: ingest writer not configured")
	}

	slug := strings.TrimSpace(batch.Source.Slug)
	if slug == "" {
		return IngestResult{}, errors.New("store: source slug required")
	}

	if len(batch.Records) == 0 {
		return IngestResult{}, errors.New("store: no records to ingest")
	}

	now := time.Now().UTC()
	batchID := primitive.NewObjectID().Hex()

	sourceFilter := bson.M{
		"tenantId": tenantID,
		"slug":     slug,
	}

	sourceUpdate := bson.M{
		"$set": bson.M{
			"tenantId":  tenantID,
			"slug":      slug,
			"name":      batch.Source.Name,
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}

	if _, err := w.db.Collection("sources").UpdateOne(ctx, sourceFilter, sourceUpdate, options.Update().SetUpsert(true)); err != nil {
		return IngestResult{}, fmt.Errorf("store: upsert source: %w", err)
	}

	rawDocs := make([]interface{}, len(batch.Records))
	for i, rec := range batch.Records {
		rawDocs[i] = bson.M{
			"tenantId":    tenantID,
			"sourceSlug":  slug,
			"payload":     rec,
			"status":      RawStatusPending,
			"ingestBatch": batchID,
			"createdAt":   now,
			"updatedAt":   now,
		}
	}

	if _, err := w.db.Collection("raw").InsertMany(ctx, rawDocs); err != nil {
		return IngestResult{}, fmt.Errorf("store: insert raw records: %w", err)
	}

	return IngestResult{
		Created:     len(batch.Records),
		Linked:      0,
		NeedsReview: 0,
	}, nil
}
