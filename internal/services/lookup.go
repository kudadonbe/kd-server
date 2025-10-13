package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
	"go.mongodb.org/mongo-driver/mongo"
)

// PersonView represents the payload returned to clients for lookup responses.
type PersonView struct {
	Person PersonPayload `json:"person"`
	Links  []LinkPayload `json:"links"`
}

// PersonPayload represents a resolved person in API responses.
type PersonPayload struct {
	PersonID     string         `json:"person_id"`
	TenantID     string         `json:"tenant_id"`
	NationalID   string         `json:"national_id,omitempty"`
	PrimaryEmail string         `json:"primary_email,omitempty"`
	PrimaryPhone string         `json:"primary_phone,omitempty"`
	Attributes   map[string]any `json:"attributes,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// LinkPayload represents a link record in API responses.
type LinkPayload struct {
	Source     string         `json:"source"`
	ExternalID string         `json:"external_id"`
	Payload    map[string]any `json:"payload,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
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

	return &PersonView{
		Person: toPersonPayload(person),
		Links:  toLinkPayloads(links),
	}, nil
}

var _ Lookup = (*LookupService)(nil)

func toPersonPayload(p *store.Person) PersonPayload {
	if p == nil {
		return PersonPayload{}
	}
	return PersonPayload{
		PersonID:     p.PersonID,
		TenantID:     p.TenantID,
		NationalID:   p.NationalID,
		PrimaryEmail: p.PrimaryEmail,
		PrimaryPhone: p.PrimaryPhone,
		Attributes:   p.Attributes,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func toLinkPayloads(links []store.Link) []LinkPayload {
	result := make([]LinkPayload, 0, len(links))
	for _, link := range links {
		result = append(result, LinkPayload{
			Source:     link.Source,
			ExternalID: link.ExternalID,
			Payload:    link.Payload,
			CreatedAt:  link.CreatedAt,
			UpdatedAt:  link.UpdatedAt,
		})
	}
	return result
}
