package services

import (
	"context"
	"errors"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReviewItem represents a raw document requiring human review.
type ReviewItem struct {
	ID         string         `json:"id"`
	SourceSlug string         `json:"source_slug"`
	Payload    map[string]any `json:"payload"`
	Status     string         `json:"status"`
}

// ReviewService provides review queue operations.
type Review interface {
	List(ctx context.Context, tenantID, status string, limit int) ([]ReviewItem, error)
	Decide(ctx context.Context, tenantID, rawID, decision string) error
}

type ReviewService struct {
	store *store.MongoStore
}

// NewReviewService creates a new ReviewService.
func NewReviewService(mongoStore *store.MongoStore) *ReviewService {
	return &ReviewService{store: mongoStore}
}

// List returns raw documents in a given status (needs_review only supported).
func (s *ReviewService) List(ctx context.Context, tenantID, status string, limit int) ([]ReviewItem, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: review store not configured")
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, errors.New("services: tenant required")
	}

	if strings.TrimSpace(status) == "" || status == store.RawStatusNeedsReview {
		raw, err := s.store.RawNeedsReview(ctx, tenantID, limit)
		if err != nil {
			return nil, err
		}

		items := make([]ReviewItem, 0, len(raw))
		for _, rec := range raw {
			items = append(items, ReviewItem{
				ID:         rec.ID.Hex(),
				SourceSlug: rec.SourceSlug,
				Payload:    rec.Payload,
				Status:     rec.Status,
			})
		}
		return items, nil
	}

	return nil, errors.New("services: unsupported review status")
}

// Decide updates a review item with a decision (accept/reject).
func (s *ReviewService) Decide(ctx context.Context, tenantID, rawID, decision string) error {
	if s == nil || s.store == nil {
		return errors.New("services: review store not configured")
	}
	if strings.TrimSpace(tenantID) == "" {
		return errors.New("services: tenant required")
	}
	if strings.TrimSpace(rawID) == "" {
		return errors.New("services: raw id required")
	}

	objectID, err := primitive.ObjectIDFromHex(rawID)
	if err != nil {
		return errors.New("services: invalid raw id")
	}

	var status string
	switch strings.ToLower(decision) {
	case "accept":
		status = store.RawStatusResolved
	case "reject":
		status = "rejected"
	default:
		return errors.New("services: decision must be 'accept' or 'reject'")
	}

	return s.store.UpdateRawStatus(ctx, tenantID, []primitive.ObjectID{objectID}, status, "")
}

var _ Review = (*ReviewService)(nil)
