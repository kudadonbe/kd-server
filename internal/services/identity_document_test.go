package services

import (
	"context"
	"testing"

	"github.com/kudadonbe/kd-server/internal/store"
)

func TestIdentityDocumentServiceSave(t *testing.T) {
	t.Parallel()

	documentStore := &stubIdentityDocumentStore{}
	service := NewIdentityDocumentService(documentStore)
	document, err := service.Save(context.Background(), store.IdentityDocument{
		TenantID:        " fc ",
		PersonID:        " person-1 ",
		NationalID:      " a000001 ",
		Name:            store.LocalizedText{English: " Sample Person "},
		DateOfBirth:     "2000-01-02",
		ExpiryDate:      "2030-01-02",
		BloodGroup:      "A+",
		FieldConfidence: map[string]float64{"national_id": 0.99},
	}, " admin ")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if document.TenantID != "fc" || document.PersonID != "person-1" {
		t.Fatalf("unexpected identity scope: %#v", document)
	}
	if document.NationalID != "A000001" {
		t.Fatalf("unexpected normalized national ID: %s", document.NationalID)
	}
	if document.DocumentType != IdentityDocumentTypeMaldivesNationalID || document.CountryCode != "MDV" {
		t.Fatalf("unexpected document classification: %#v", document)
	}
	if document.Source != "manual-admin" || document.ExtractionMethod != "manual" {
		t.Fatalf("unexpected provenance defaults: %#v", document)
	}
	if document.VerificationStatus != VerificationStatusUnverified {
		t.Fatalf("unexpected verification status: %s", document.VerificationStatus)
	}
	if documentStore.actor != "admin" {
		t.Fatalf("unexpected history actor: %s", documentStore.actor)
	}
}

func TestIdentityDocumentServiceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewIdentityDocumentService(&stubIdentityDocumentStore{})
	tests := []struct {
		name     string
		document store.IdentityDocument
	}{
		{name: "missing tenant", document: validIdentityDocument("", "person-1")},
		{name: "missing person", document: validIdentityDocument("fc", "")},
		{name: "missing national ID", document: func() store.IdentityDocument {
			document := validIdentityDocument("fc", "person-1")
			document.NationalID = ""
			return document
		}()},
		{name: "invalid date", document: func() store.IdentityDocument {
			document := validIdentityDocument("fc", "person-1")
			document.DateOfBirth = "02/01/2000"
			return document
		}()},
		{name: "impossible date", document: func() store.IdentityDocument {
			document := validIdentityDocument("fc", "person-1")
			document.DateOfBirth = "2000-99-99"
			return document
		}()},
		{name: "invalid verification", document: func() store.IdentityDocument {
			document := validIdentityDocument("fc", "person-1")
			document.VerificationStatus = "approved"
			return document
		}()},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := service.Save(context.Background(), test.document, "admin"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func validIdentityDocument(tenantID, personID string) store.IdentityDocument {
	return store.IdentityDocument{
		TenantID:   tenantID,
		PersonID:   personID,
		NationalID: "A000001",
		Name:       store.LocalizedText{English: "Sample Person"},
	}
}

type stubIdentityDocumentStore struct {
	document store.IdentityDocument
	actor    string
}

func (s *stubIdentityDocumentStore) ListIdentityDocuments(context.Context, string, string, int) ([]store.IdentityDocument, error) {
	return []store.IdentityDocument{s.document}, nil
}

func (s *stubIdentityDocumentStore) SaveIdentityDocument(_ context.Context, document store.IdentityDocument, actor string) (*store.IdentityDocument, error) {
	document.DocumentID = "doc_test"
	document.Version = 1
	s.document = document
	s.actor = actor
	return &document, nil
}
