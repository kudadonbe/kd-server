package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
)

const (
	IdentityDocumentTypeMaldivesNationalID = "maldives_national_id"
	VerificationStatusUnverified           = "unverified"
	VerificationStatusVerified             = "verified"
	VerificationStatusRejected             = "rejected"
)

// IdentityDocumentStore persists identity documents and their history.
type IdentityDocumentStore interface {
	ListIdentityDocuments(ctx context.Context, tenantID, personID string, limit int) ([]store.IdentityDocument, error)
	SaveIdentityDocument(ctx context.Context, document store.IdentityDocument, actor string) (*store.IdentityDocument, error)
}

// IdentityDocuments exposes identity-document administration operations.
type IdentityDocuments interface {
	List(ctx context.Context, tenantID, personID string, limit int) ([]store.IdentityDocument, error)
	Save(ctx context.Context, document store.IdentityDocument, actor string) (*store.IdentityDocument, error)
}

// IdentityDocumentService validates and manages identity-document records.
type IdentityDocumentService struct {
	store IdentityDocumentStore
}

// NewIdentityDocumentService creates an identity-document service.
func NewIdentityDocumentService(documentStore IdentityDocumentStore) *IdentityDocumentService {
	return &IdentityDocumentService{store: documentStore}
}

// List returns identity documents for one tenant and optionally one person.
func (s *IdentityDocumentService) List(ctx context.Context, tenantID, personID string, limit int) ([]store.IdentityDocument, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: identity document store not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("services: tenant required")
	}
	return s.store.ListIdentityDocuments(ctx, tenantID, strings.TrimSpace(personID), limit)
}

// Save validates and persists an identity document.
func (s *IdentityDocumentService) Save(ctx context.Context, document store.IdentityDocument, actor string) (*store.IdentityDocument, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: identity document store not configured")
	}

	document.TenantID = strings.TrimSpace(document.TenantID)
	document.PersonID = strings.TrimSpace(document.PersonID)
	document.DocumentID = strings.TrimSpace(document.DocumentID)
	document.DocumentType = IdentityDocumentTypeMaldivesNationalID
	document.CountryCode = "MDV"
	document.NationalID = strings.ToUpper(strings.TrimSpace(document.NationalID))
	document.SerialNumber = strings.TrimSpace(document.SerialNumber)
	document.Name.English = strings.TrimSpace(document.Name.English)
	document.Name.Dhivehi = strings.TrimSpace(document.Name.Dhivehi)
	document.CommonName.English = strings.TrimSpace(document.CommonName.English)
	document.CommonName.Dhivehi = strings.TrimSpace(document.CommonName.Dhivehi)
	document.Sex = strings.ToUpper(strings.TrimSpace(document.Sex))
	document.Source = strings.TrimSpace(document.Source)
	document.ExtractionMethod = strings.TrimSpace(document.ExtractionMethod)
	document.VerificationStatus = strings.ToLower(strings.TrimSpace(document.VerificationStatus))
	document.VerifiedBy = strings.TrimSpace(document.VerifiedBy)
	document.FrontImageRef = strings.TrimSpace(document.FrontImageRef)
	document.BackImageRef = strings.TrimSpace(document.BackImageRef)

	if document.TenantID == "" || document.PersonID == "" {
		return nil, errors.New("services: tenant and person ID are required")
	}
	if document.NationalID == "" {
		return nil, errors.New("services: national ID is required")
	}
	if document.Name.English == "" && document.Name.Dhivehi == "" {
		return nil, errors.New("services: at least one name is required")
	}
	if document.Source == "" {
		document.Source = "manual-admin"
	}
	if document.ExtractionMethod == "" {
		document.ExtractionMethod = "manual"
	}
	if document.VerificationStatus == "" {
		document.VerificationStatus = VerificationStatusUnverified
	}
	if !validVerificationStatus(document.VerificationStatus) {
		return nil, errors.New("services: invalid verification status")
	}
	if document.DateOfBirth != "" && !validISODate(document.DateOfBirth) {
		return nil, errors.New("services: date of birth must use YYYY-MM-DD")
	}
	if document.ExpiryDate != "" && !validISODate(document.ExpiryDate) {
		return nil, errors.New("services: expiry date must use YYYY-MM-DD")
	}

	return s.store.SaveIdentityDocument(ctx, document, strings.TrimSpace(actor))
}

func validISODate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validVerificationStatus(status string) bool {
	switch status {
	case VerificationStatusUnverified, VerificationStatusVerified, VerificationStatusRejected:
		return true
	default:
		return false
	}
}

var _ IdentityDocuments = (*IdentityDocumentService)(nil)
