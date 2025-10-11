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

// PersonIdentifiers groups deterministic match keys.
type PersonIdentifiers struct {
	NationalID string
	Email      string
	Phone      string
}

// FindPersonByIdentifiers searches for a person using deterministic identifiers.
func (s *MongoStore) FindPersonByIdentifiers(ctx context.Context, tenantID string, identifiers PersonIdentifiers) (*Person, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	filters := make([]bson.M, 0, 3)
	if identifiers.NationalID != "" {
		filters = append(filters, bson.M{"nationalId": identifiers.NationalID})
	}
	if identifiers.Email != "" {
		filters = append(filters, bson.M{"primaryEmail": identifiers.Email})
	}
	if identifiers.Phone != "" {
		filters = append(filters, bson.M{"primaryPhone": identifiers.Phone})
	}

	if len(filters) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	filter := bson.M{
		"tenantId": tenantID,
		"$or":      filters,
	}

	var person Person
	err := s.db.Collection("people").FindOne(ctx, filter).Decode(&person)
	if err != nil {
		return nil, err
	}
	return &person, nil
}

// CreatePerson inserts a new person document.
func (s *MongoStore) CreatePerson(ctx context.Context, tenantID string, identifiers PersonIdentifiers, attrs map[string]any) (*Person, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	now := time.Now().UTC()
	person := Person{
		ID:           primitive.NewObjectID(),
		TenantID:     tenantID,
		PersonID:     primitive.NewObjectID().Hex(),
		NationalID:   identifiers.NationalID,
		PrimaryEmail: identifiers.Email,
		PrimaryPhone: identifiers.Phone,
		Attributes:   attrs,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := s.db.Collection("people").InsertOne(ctx, person); err != nil {
		return nil, fmt.Errorf("store: insert person: %w", err)
	}

	return &person, nil
}

// UpdatePersonIdentifiers updates existing identifier fields when new values are provided.
func (s *MongoStore) UpdatePersonIdentifiers(ctx context.Context, personID string, identifiers PersonIdentifiers) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}

	update := bson.M{"$set": bson.M{"updatedAt": time.Now().UTC()}}
	set := update["$set"].(bson.M)

	if strings.TrimSpace(identifiers.NationalID) != "" {
		set["nationalId"] = identifiers.NationalID
	}
	if strings.TrimSpace(identifiers.Email) != "" {
		set["primaryEmail"] = identifiers.Email
	}
	if strings.TrimSpace(identifiers.Phone) != "" {
		set["primaryPhone"] = identifiers.Phone
	}

	_, err := s.db.Collection("people").UpdateOne(ctx, bson.M{"personId": personID}, update)
	if err != nil {
		return fmt.Errorf("store: update person identifiers: %w", err)
	}
	return nil
}

// UpsertLink creates or updates a link document.
func (s *MongoStore) UpsertLink(ctx context.Context, tenantID, personID, source, externalID string, payload map[string]any) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}

	if strings.TrimSpace(source) == "" || strings.TrimSpace(externalID) == "" {
		return errors.New("store: link source and external id required")
	}

	now := time.Now().UTC()
	filter := bson.M{
		"tenantId":   tenantID,
		"personId":   personID,
		"source":     source,
		"externalId": externalID,
	}

	update := bson.M{
		"$set": bson.M{
			"tenantId":   tenantID,
			"personId":   personID,
			"source":     source,
			"externalId": externalID,
			"payload":    payload,
			"updatedAt":  now,
		},
		"$setOnInsert": bson.M{
			"_id":       primitive.NewObjectID(),
			"createdAt": now,
		},
	}

	_, err := s.db.Collection("links").UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("store: upsert link: %w", err)
	}
	return nil
}

// FindPersonWithLinksByEmail retrieves a person and associated links using primary email.
func (s *MongoStore) FindPersonWithLinksByEmail(ctx context.Context, tenantID, email string) (*Person, []Link, error) {
	return s.findPersonWithLinks(ctx, bson.M{"tenantId": tenantID, "primaryEmail": email})
}

// FindPersonWithLinksByPhone retrieves a person and associated links using primary phone.
func (s *MongoStore) FindPersonWithLinksByPhone(ctx context.Context, tenantID, phone string) (*Person, []Link, error) {
	return s.findPersonWithLinks(ctx, bson.M{"tenantId": tenantID, "primaryPhone": phone})
}

func (s *MongoStore) findPersonWithLinks(ctx context.Context, filter bson.M) (*Person, []Link, error) {
	if s == nil || s.db == nil {
		return nil, nil, errors.New("store: mongo not configured")
	}

	var person Person
	err := s.db.Collection("people").FindOne(ctx, filter).Decode(&person)
	if err != nil {
		return nil, nil, err
	}

	cursor, err := s.db.Collection("links").Find(ctx, bson.M{
		"tenantId": person.TenantID,
		"personId": person.PersonID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("store: find links: %w", err)
	}
	defer cursor.Close(ctx)

	var links []Link
	for cursor.Next(ctx) {
		var link Link
		if err := cursor.Decode(&link); err != nil {
			return nil, nil, fmt.Errorf("store: decode link: %w", err)
		}
		links = append(links, link)
	}

	if err := cursor.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: cursor error: %w", err)
	}

	return &person, links, nil
}
