package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	RawStatusPending     = "pending"
	RawStatusResolved    = "resolved"
	RawStatusNeedsReview = "needs_review"
)

// ListRawByStatus returns raw documents for a tenant with the desired status.
func (s *MongoStore) ListRawByStatus(ctx context.Context, tenantID, status string, limit int) ([]RawRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	if limit <= 0 {
		limit = 100
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "createdAt", Value: 1}})

	cursor, err := s.db.Collection("raw").Find(ctx, bson.M{
		"tenantId": tenantID,
		"status":   status,
	}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("store: list raw: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var records []RawRecord
	for cursor.Next(ctx) {
		var rec RawRecord
		if err := cursor.Decode(&rec); err != nil {
			return nil, fmt.Errorf("store: decode raw record: %w", err)
		}
		records = append(records, rec)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("store: cursor error: %w", err)
	}

	return records, nil
}

// UpdateRawStatus updates the status and person mapping for the provided raw IDs.
func (s *MongoStore) UpdateRawStatus(ctx context.Context, tenantID string, ids []primitive.ObjectID, status, personID string) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}
	if len(ids) == 0 {
		return nil
	}

	update := bson.M{
		"$set": bson.M{
			"status":    status,
			"updatedAt": time.Now().UTC(),
		},
	}
	if personID != "" {
		update["$set"].(bson.M)["personId"] = personID
	}

	_, err := s.db.Collection("raw").UpdateMany(ctx, bson.M{
		"tenantId": tenantID,
		"_id":      bson.M{"$in": ids},
	}, update)
	if err != nil {
		return fmt.Errorf("store: update raw status: %w", err)
	}
	return nil
}

// RawNeedsReview returns raw documents flagged for review.
func (s *MongoStore) RawNeedsReview(ctx context.Context, tenantID string, limit int) ([]RawRecord, error) {
	return s.ListRawByStatus(ctx, tenantID, RawStatusNeedsReview, limit)
}

// InsertRawReview writes a raw record straight into the review queue
// (status needs_review). It is the entry point for captured-but-unconfirmed
// information — e.g. new fields a traced document surfaced that the server does
// not already hold. Returns the new record's hex id.
func (s *MongoStore) InsertRawReview(ctx context.Context, tenantID, sourceSlug string, payload map[string]any) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("store: mongo not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", errors.New("store: tenant required")
	}

	now := time.Now().UTC()
	rec := RawRecord{
		ID:          primitive.NewObjectID(),
		TenantID:    tenantID,
		SourceSlug:  strings.TrimSpace(sourceSlug),
		Payload:     payload,
		Status:      RawStatusNeedsReview,
		IngestBatch: "capture",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := s.db.Collection("raw").InsertOne(ctx, rec); err != nil {
		return "", fmt.Errorf("store: insert raw review: %w", err)
	}
	return rec.ID.Hex(), nil
}

// GetRaw returns a single raw record by tenant + hex id.
func (s *MongoStore) GetRaw(ctx context.Context, tenantID, rawID string) (*RawRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(rawID))
	if err != nil {
		return nil, errors.New("store: invalid raw id")
	}
	var rec RawRecord
	err = s.db.Collection("raw").FindOne(ctx, bson.M{
		"tenantId": strings.TrimSpace(tenantID),
		"_id":      objectID,
	}).Decode(&rec)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
