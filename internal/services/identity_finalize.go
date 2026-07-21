package services

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/kudadonbe/kd-server/internal/store"
)

// FieldError is a per-field validation failure returned to the caller.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError carries one or more per-field errors (maps to HTTP 400).
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// IdentityFinalizeStore is the persistence the finalize/verify flow composes:
// resolve-or-create a person, then save/verify a versioned identity document.
type IdentityFinalizeStore interface {
	FindPersonByIdentifiers(ctx context.Context, tenantID string, ids store.PersonIdentifiers) (*store.Person, error)
	CreatePerson(ctx context.Context, tenantID string, ids store.PersonIdentifiers, attrs map[string]any) (*store.Person, error)
	SaveIdentityDocument(ctx context.Context, doc store.IdentityDocument, actor string) (*store.IdentityDocument, error)
	VerifyIdentityDocument(ctx context.Context, tenantID, documentID, verifiedBy string) (*store.IdentityDocument, error)
}

// IdentityFinalizeService turns reviewed extraction into a stored identity
// document: it resolves-or-creates the person and saves the document tagged
// unverified. The server owns verification_status — a client cannot self-assert
// verified; that is a separate, authorized Verify step.
type IdentityFinalizeService struct {
	store IdentityFinalizeStore
}

// NewIdentityFinalizeService builds the finalize/verify service.
func NewIdentityFinalizeService(s IdentityFinalizeStore) *IdentityFinalizeService {
	return &IdentityFinalizeService{store: s}
}

// Finalize normalizes and validates the document, resolves-or-creates the
// person by national ID (+ optional phone), then saves it as unverified.
func (s *IdentityFinalizeService) Finalize(ctx context.Context, tenantID, actor, phone string, doc store.IdentityDocument) (*store.IdentityDocument, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: identity finalize store not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("services: tenant required")
	}

	normalizeIdentityDocument(&doc)
	doc.TenantID = tenantID
	phone = strings.TrimSpace(phone)

	if verr := validateFinalizeDocument(doc); verr != nil {
		return nil, verr
	}

	// Resolve-or-create the person by deterministic identifiers.
	ids := store.PersonIdentifiers{NationalID: doc.NationalID, Phone: phone}
	person, err := s.store.FindPersonByIdentifiers(ctx, tenantID, ids)
	switch {
	case err == nil:
		doc.PersonID = person.PersonID
	case errors.Is(err, mongo.ErrNoDocuments):
		created, cerr := s.store.CreatePerson(ctx, tenantID, ids, map[string]any{
			"name":         doc.Name.English,
			"name_dhivehi": doc.Name.Dhivehi,
			"national_id":  doc.NationalID,
		})
		if cerr != nil {
			return nil, cerr
		}
		doc.PersonID = created.PersonID
	default:
		return nil, err
	}

	// The server owns verification: finalize always stores unverified. Promotion
	// to verified is the separate, authorized Verify step.
	doc.DocumentID = ""
	doc.VerificationStatus = VerificationStatusUnverified
	doc.VerifiedBy = ""
	if doc.Source == "" {
		doc.Source = "app-finalize"
	}
	if doc.ExtractionMethod == "" {
		doc.ExtractionMethod = "app-review"
	}

	return s.store.SaveIdentityDocument(ctx, doc, actor)
}

// Verify promotes a document to verified and records the authorized actor.
func (s *IdentityFinalizeService) Verify(ctx context.Context, tenantID, documentID, actor string) (*store.IdentityDocument, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: identity finalize store not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	documentID = strings.TrimSpace(documentID)
	actor = strings.TrimSpace(actor)
	if tenantID == "" || documentID == "" {
		return nil, errors.New("services: tenant and document ID are required")
	}
	if actor == "" {
		return nil, errors.New("services: verifier identity required")
	}
	return s.store.VerifyIdentityDocument(ctx, tenantID, documentID, actor)
}

func normalizeIdentityDocument(doc *store.IdentityDocument) {
	doc.DocumentType = IdentityDocumentTypeMaldivesNationalID
	doc.CountryCode = "MDV"
	doc.NationalID = strings.ToUpper(strings.TrimSpace(doc.NationalID))
	doc.Name.English = strings.TrimSpace(doc.Name.English)
	doc.Name.Dhivehi = strings.TrimSpace(doc.Name.Dhivehi)
	doc.CommonName.English = strings.TrimSpace(doc.CommonName.English)
	doc.CommonName.Dhivehi = strings.TrimSpace(doc.CommonName.Dhivehi)
	doc.Sex = strings.ToUpper(strings.TrimSpace(doc.Sex))
	doc.DateOfBirth = strings.TrimSpace(doc.DateOfBirth)
	doc.ExpiryDate = strings.TrimSpace(doc.ExpiryDate)
	doc.Source = strings.TrimSpace(doc.Source)
	doc.ExtractionMethod = strings.TrimSpace(doc.ExtractionMethod)
}

func validateFinalizeDocument(doc store.IdentityDocument) *ValidationError {
	var errs []FieldError
	if doc.NationalID == "" {
		errs = append(errs, FieldError{"national_id", "national ID is required"})
	}
	if doc.Name.English == "" && doc.Name.Dhivehi == "" {
		errs = append(errs, FieldError{"name", "at least one of English or Dhivehi name is required"})
	}
	if doc.Sex != "" && doc.Sex != "M" && doc.Sex != "F" {
		errs = append(errs, FieldError{"sex", "must be M or F"})
	}
	if doc.DateOfBirth != "" && !validISODate(doc.DateOfBirth) {
		errs = append(errs, FieldError{"date_of_birth", "must use YYYY-MM-DD"})
	}
	if doc.ExpiryDate != "" && !validISODate(doc.ExpiryDate) {
		errs = append(errs, FieldError{"expiry_date", "must use YYYY-MM-DD"})
	}
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}
