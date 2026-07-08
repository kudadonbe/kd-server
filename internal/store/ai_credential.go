package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AICredential stores one tenant's AI provider credential. The API key is held
// only as ciphertext; the store never sees or returns plaintext.
type AICredential struct {
	ID          primitive.ObjectID `bson:"_id"`
	TenantID    string             `bson:"tenantId"`
	Provider    string             `bson:"provider"`
	KeyHint     string             `bson:"keyHint"`
	Model       string             `bson:"model,omitempty"`
	Ciphertext  []byte             `bson:"ciphertext"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
	ValidatedAt *time.Time         `bson:"validatedAt,omitempty"`
}

// GetAICredential returns the credential for a tenant+provider, or (nil, nil)
// when none exists.
func (s *MongoStore) GetAICredential(ctx context.Context, tenantID, provider string) (*AICredential, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	var cred AICredential
	err := s.db.Collection("ai_credentials").
		FindOne(ctx, bson.M{"tenantId": tenantID, "provider": provider}).
		Decode(&cred)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: get ai credential: %w", err)
	}
	return &cred, nil
}

// SaveAICredential upserts a tenant+provider credential and returns the stored
// document.
func (s *MongoStore) SaveAICredential(ctx context.Context, cred AICredential) (*AICredential, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	now := time.Now().UTC()
	filter := bson.M{"tenantId": cred.TenantID, "provider": cred.Provider}
	update := bson.M{
		"$set": bson.M{
			"keyHint":     cred.KeyHint,
			"model":       cred.Model,
			"ciphertext":  cred.Ciphertext,
			"validatedAt": cred.ValidatedAt,
			"updatedAt":   now,
		},
		"$setOnInsert": bson.M{
			"_id":       primitive.NewObjectID(),
			"tenantId":  cred.TenantID,
			"provider":  cred.Provider,
			"createdAt": now,
		},
	}

	var out AICredential
	err := s.db.Collection("ai_credentials").FindOneAndUpdate(
		ctx,
		filter,
		update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&out)
	if err != nil {
		return nil, fmt.Errorf("store: save ai credential: %w", err)
	}
	return &out, nil
}

// ListAICredentials returns all credentials for a tenant, ordered by provider.
func (s *MongoStore) ListAICredentials(ctx context.Context, tenantID string) ([]AICredential, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}

	cur, err := s.db.Collection("ai_credentials").Find(
		ctx,
		bson.M{"tenantId": tenantID},
		options.Find().SetSort(bson.D{{Key: "provider", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("store: list ai credentials: %w", err)
	}
	defer func() {
		_ = cur.Close(ctx)
	}()

	var creds []AICredential
	if err := cur.All(ctx, &creds); err != nil {
		return nil, fmt.Errorf("store: decode ai credentials: %w", err)
	}
	return creds, nil
}

// DeleteAICredential removes a tenant+provider credential. Deleting a missing
// credential is not an error.
func (s *MongoStore) DeleteAICredential(ctx context.Context, tenantID, provider string) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}

	if _, err := s.db.Collection("ai_credentials").DeleteOne(
		ctx,
		bson.M{"tenantId": tenantID, "provider": provider},
	); err != nil {
		return fmt.Errorf("store: delete ai credential: %w", err)
	}
	return nil
}
