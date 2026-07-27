package services

import (
	"context"
	"errors"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
	"go.mongodb.org/mongo-driver/bson"
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
	Decide(ctx context.Context, tenantID, rawID, decision, actor string) error
}

type ReviewService struct {
	store    *store.MongoStore
	finalize *IdentityFinalizeService
}

// NewReviewService creates a new ReviewService.
func NewReviewService(mongoStore *store.MongoStore) *ReviewService {
	return &ReviewService{store: mongoStore}
}

// SetFinalize attaches the finalize/verify service so that accepting an identity
// capture applies it (resolve-or-create + save + verify). Optional: without it,
// accept only flips the record's status.
func (s *ReviewService) SetFinalize(f *IdentityFinalizeService) {
	s.finalize = f
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

// Decide updates a review item with a decision (accept/reject). Accepting an
// identity capture applies it: resolve-or-create the person, save the document,
// and verify it, recording the reviewer (actor) as verified_by.
func (s *ReviewService) Decide(ctx context.Context, tenantID, rawID, decision, actor string) error {
	if s == nil || s.store == nil {
		return errors.New("services: review store not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return errors.New("services: tenant required")
	}
	if strings.TrimSpace(rawID) == "" {
		return errors.New("services: raw id required")
	}

	objectID, err := primitive.ObjectIDFromHex(rawID)
	if err != nil {
		return errors.New("services: invalid raw id")
	}

	switch strings.ToLower(decision) {
	case "reject":
		return s.store.UpdateRawStatus(ctx, tenantID, []primitive.ObjectID{objectID}, "rejected", "")
	case "accept":
		// fall through to acceptance handling below.
	default:
		return errors.New("services: decision must be 'accept' or 'reject'")
	}

	rec, err := s.store.GetRaw(ctx, tenantID, rawID)
	if err != nil {
		return err
	}

	// An identity capture becomes a verified document on accept (when the
	// finalize collaborator is wired). Anything else just resolves the record.
	if s.finalize != nil && capturePayloadType(rec.Payload) == captureTypeIdentity {
		personID, aerr := s.applyIdentityCapture(ctx, tenantID, actor, rec.Payload)
		if aerr != nil {
			return aerr
		}
		return s.store.UpdateRawStatus(ctx, tenantID, []primitive.ObjectID{objectID}, store.RawStatusResolved, personID)
	}

	return s.store.UpdateRawStatus(ctx, tenantID, []primitive.ObjectID{objectID}, store.RawStatusResolved, "")
}

// applyIdentityCapture reconstructs the captured document and applies it as a
// verified record, returning the resolved person id.
func (s *ReviewService) applyIdentityCapture(ctx context.Context, tenantID, actor string, payload map[string]any) (string, error) {
	if strings.TrimSpace(actor) == "" {
		return "", errors.New("services: reviewer identity required")
	}
	doc, err := decodeCapturedDocument(payload)
	if err != nil {
		return "", err
	}
	doc.TenantID = tenantID
	phone, _ := payload["phone"].(string)

	saved, err := s.finalize.saveUnverified(ctx, tenantID, actor, strings.TrimSpace(phone), doc)
	if err != nil {
		return "", err
	}
	verified, err := s.finalize.Verify(ctx, tenantID, saved.DocumentID, actor)
	if err != nil {
		return "", err
	}
	return verified.PersonID, nil
}

// capturePayloadType reads the capture_type marker off a raw review payload.
func capturePayloadType(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	t, _ := payload["capture_type"].(string)
	return t
}

// decodeCapturedDocument reconstructs the stored identity document from a raw
// review payload via a bson round-trip (payload["document"] is a bson sub-doc).
func decodeCapturedDocument(payload map[string]any) (store.IdentityDocument, error) {
	var doc store.IdentityDocument
	raw, ok := payload["document"]
	if !ok {
		return doc, errors.New("services: capture has no document")
	}
	encoded, err := bson.Marshal(raw)
	if err != nil {
		return doc, errors.New("services: encode captured document")
	}
	if err := bson.Unmarshal(encoded, &doc); err != nil {
		return doc, errors.New("services: decode captured document")
	}
	return doc, nil
}

var _ Review = (*ReviewService)(nil)
