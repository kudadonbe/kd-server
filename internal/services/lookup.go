package services

import (
	"context"
	"errors"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
	"go.mongodb.org/mongo-driver/mongo"
)

// PersonView represents the payload returned to clients for lookup responses.
type PersonView struct {
	Person store.Person `json:"person"`
	Links  []store.Link `json:"links"`
}

// LookupService exposes lookup functionality by identifier.
type Lookup interface {
	ByEmail(ctx context.Context, tenantID, email string) (*PersonView, error)
	ByPhone(ctx context.Context, tenantID, phone string) (*PersonView, error)
}

type LookupService struct {
	store *store.MongoStore
}

// NewLookupService creates a new LookupService instance.
func NewLookupService(mongoStore *store.MongoStore) *LookupService {
	return &LookupService{store: mongoStore}
}

// ByEmail fetches a person view by email.
func (s *LookupService) ByEmail(ctx context.Context, tenantID, email string) (*PersonView, error) {
	if strings.TrimSpace(email) == "" {
		return nil, errors.New("services: email required")
	}
	return s.fetch(ctx, tenantID, func(ctx context.Context, tenant string) (*store.Person, []store.Link, error) {
		return s.store.FindPersonWithLinksByEmail(ctx, tenant, strings.ToLower(strings.TrimSpace(email)))
	})
}

// ByPhone fetches a person view by phone.
func (s *LookupService) ByPhone(ctx context.Context, tenantID, phone string) (*PersonView, error) {
	if strings.TrimSpace(phone) == "" {
		return nil, errors.New("services: phone required")
	}
	return s.fetch(ctx, tenantID, func(ctx context.Context, tenant string) (*store.Person, []store.Link, error) {
		return s.store.FindPersonWithLinksByPhone(ctx, tenant, phone)
	})
}

func (s *LookupService) fetch(ctx context.Context, tenantID string, finder func(context.Context, string) (*store.Person, []store.Link, error)) (*PersonView, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: lookup store not configured")
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, errors.New("services: tenant required")
	}

	person, links, err := finder(ctx, tenantID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	return &PersonView{Person: *person, Links: links}, nil
}

var _ Lookup = (*LookupService)(nil)
