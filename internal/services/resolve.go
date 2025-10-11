package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ResolveOptions controls how many records are processed per request.
type ResolveOptions struct {
	BatchSize int
}

// ResolveResult summarizes the outcome of a resolve run.
type ResolveResult struct {
	Resolved    int
	NeedsReview int
	Skipped     int
}

// Resolver exposes resolve orchestration behaviour.
type Resolver interface {
	Resolve(ctx context.Context, tenantID string, opts ResolveOptions) (ResolveResult, error)
}

// ResolveService implements deterministic resolve rules.
type ResolveService struct {
	store *store.MongoStore
}

// NewResolveService builds a new ResolveService.
func NewResolveService(mongoStore *store.MongoStore) *ResolveService {
	return &ResolveService{store: mongoStore}
}

// Resolve updates pending raw records by matching deterministic identifiers.
func (s *ResolveService) Resolve(ctx context.Context, tenantID string, opts ResolveOptions) (ResolveResult, error) {
	if s == nil || s.store == nil {
		return ResolveResult{}, errors.New("services: resolve store not configured")
	}
	if strings.TrimSpace(tenantID) == "" {
		return ResolveResult{}, errors.New("services: tenant required")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}

	rawRecords, err := s.store.ListRawByStatus(ctx, tenantID, store.RawStatusPending, opts.BatchSize)
	if err != nil {
		return ResolveResult{}, err
	}

	var (
		reviewIDs []primitive.ObjectID
		result    ResolveResult
	)

	for _, rec := range rawRecords {
		ids := extractIdentifiers(rec.Payload)
		if ids.NationalID == "" && ids.Email == "" && ids.Phone == "" {
			reviewIDs = append(reviewIDs, rec.ID)
			result.NeedsReview++
			continue
		}

		person, findErr := s.store.FindPersonByIdentifiers(ctx, tenantID, ids)
		switch {
		case errors.Is(findErr, mongo.ErrNoDocuments):
			person, err = s.store.CreatePerson(ctx, tenantID, ids, rec.Payload)
			if err != nil {
				return result, err
			}
			result.Resolved++
		case findErr != nil:
			return result, findErr
		default:
			if err := s.store.UpdatePersonIdentifiers(ctx, person.PersonID, ids); err != nil {
				return result, err
			}
			result.Resolved++
		}

		externalID := firstNonEmpty(
			asString(rec.Payload["external_id"]),
			asString(rec.Payload["id"]),
			rec.ID.Hex(),
		)

		if err := s.store.UpsertLink(ctx, tenantID, person.PersonID, rec.SourceSlug, externalID, rec.Payload); err != nil {
			return result, err
		}

		if err := s.store.UpdateRawStatus(ctx, tenantID, []primitive.ObjectID{rec.ID}, store.RawStatusResolved, person.PersonID); err != nil {
			return result, err
		}
	}

	if len(reviewIDs) > 0 {
		if err := s.store.UpdateRawStatus(ctx, tenantID, reviewIDs, store.RawStatusNeedsReview, ""); err != nil {
			return result, err
		}
	}

	result.Skipped = len(rawRecords) - result.Resolved - result.NeedsReview
	return result, nil
}

func extractIdentifiers(payload map[string]any) store.PersonIdentifiers {
	return store.PersonIdentifiers{
		NationalID: asString(payload["national_id"]),
		Email:      strings.ToLower(strings.TrimSpace(asString(payload["email"]))),
		Phone:      strings.TrimSpace(asString(payload["phone"])),
	}
}

func asString(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var _ Resolver = (*ResolveService)(nil)
