package store

import (
	"context"
	"errors"
	"fmt"
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
	defer cursor.Close(ctx)

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
