package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListIdentityDocuments returns tenant-scoped identity documents, newest first.
func (s *MongoStore) ListIdentityDocuments(ctx context.Context, tenantID, personID string, limit int) ([]IdentityDocument, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	filter := bson.M{"tenantId": strings.TrimSpace(tenantID)}
	if personID = strings.TrimSpace(personID); personID != "" {
		filter["personId"] = personID
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursor, err := s.db.Collection("identity_documents").Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("store: list identity documents: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	documents := make([]IdentityDocument, 0)
	for cursor.Next(ctx) {
		var document IdentityDocument
		if err := cursor.Decode(&document); err != nil {
			return nil, fmt.Errorf("store: decode identity document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("store: identity document cursor: %w", err)
	}
	return documents, nil
}

// SaveIdentityDocument creates or updates a document and records an immutable snapshot.
func (s *MongoStore) SaveIdentityDocument(ctx context.Context, document IdentityDocument, actor string) (*IdentityDocument, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	personCount, err := s.db.Collection("people").CountDocuments(ctx, bson.M{
		"tenantId": document.TenantID,
		"personId": document.PersonID,
	}, options.Count().SetLimit(1))
	if err != nil {
		return nil, fmt.Errorf("store: verify identity document person: %w", err)
	}
	if personCount == 0 {
		return nil, errors.New("store: linked person not found")
	}

	now := time.Now().UTC()
	isNew := strings.TrimSpace(document.DocumentID) == ""
	if isNew {
		document.ID = primitive.NewObjectID()
		document.DocumentID = "doc_" + primitive.NewObjectID().Hex()
		document.Version = 1
		document.CreatedAt = now
	} else {
		var existing IdentityDocument
		err = s.db.Collection("identity_documents").FindOne(ctx, bson.M{
			"tenantId":   document.TenantID,
			"documentId": document.DocumentID,
		}).Decode(&existing)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("store: identity document not found")
			}
			return nil, fmt.Errorf("store: find identity document: %w", err)
		}
		document.ID = existing.ID
		document.CreatedAt = existing.CreatedAt
		document.Version = existing.Version + 1
	}
	document.UpdatedAt = now

	if isNew {
		if _, err := s.db.Collection("identity_documents").InsertOne(ctx, document); err != nil {
			return nil, fmt.Errorf("store: insert identity document: %w", err)
		}
	} else {
		if _, err := s.db.Collection("identity_documents").ReplaceOne(ctx, bson.M{
			"tenantId":   document.TenantID,
			"documentId": document.DocumentID,
		}, document); err != nil {
			return nil, fmt.Errorf("store: update identity document: %w", err)
		}
	}

	history := IdentityDocumentHistory{
		ID:         primitive.NewObjectID(),
		TenantID:   document.TenantID,
		DocumentID: document.DocumentID,
		PersonID:   document.PersonID,
		EventType:  map[bool]string{true: "created", false: "updated"}[isNew],
		Actor:      strings.TrimSpace(actor),
		Version:    document.Version,
		Snapshot:   document,
		CreatedAt:  now,
	}
	if _, err := s.db.Collection("identity_document_history").InsertOne(ctx, history); err != nil {
		return nil, fmt.Errorf("store: insert identity document history: %w", err)
	}

	s.rebuildEntityIndexBestEffort(ctx, document.TenantID, document.PersonID)
	return &document, nil
}

// VerifyIdentityDocument promotes a document to verified, recording the actor,
// and versions the change through SaveIdentityDocument (history + reindex).
func (s *MongoStore) VerifyIdentityDocument(ctx context.Context, tenantID, documentID, verifiedBy string) (*IdentityDocument, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	documentID = strings.TrimSpace(documentID)
	if tenantID == "" || documentID == "" {
		return nil, errors.New("store: tenant and document ID are required")
	}

	var doc IdentityDocument
	err := s.db.Collection("identity_documents").FindOne(ctx, bson.M{
		"tenantId":   tenantID,
		"documentId": documentID,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("store: identity document not found")
		}
		return nil, fmt.Errorf("store: find identity document: %w", err)
	}

	doc.VerificationStatus = "verified"
	doc.VerifiedBy = strings.TrimSpace(verifiedBy)
	return s.SaveIdentityDocument(ctx, doc, verifiedBy)
}
