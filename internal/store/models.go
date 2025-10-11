package store

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RawRecord represents a document in the raw ingest collection.
type RawRecord struct {
	ID          primitive.ObjectID `bson:"_id"`
	TenantID    string             `bson:"tenantId"`
	SourceSlug  string             `bson:"sourceSlug"`
	Payload     map[string]any     `bson:"payload"`
	Status      string             `bson:"status"`
	PersonID    string             `bson:"personId,omitempty"`
	IngestBatch string             `bson:"ingestBatch"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty"`
}

// Person represents a resolved entity stored in the people collection.
type Person struct {
	ID           primitive.ObjectID `bson:"_id"`
	TenantID     string             `bson:"tenantId"`
	PersonID     string             `bson:"personId"`
	NationalID   string             `bson:"nationalId,omitempty"`
	PrimaryEmail string             `bson:"primaryEmail,omitempty"`
	PrimaryPhone string             `bson:"primaryPhone,omitempty"`
	Attributes   map[string]any     `bson:"attributes,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"`
}

// Link represents the connection between a person and a source record.
type Link struct {
	ID         primitive.ObjectID `bson:"_id"`
	TenantID   string             `bson:"tenantId"`
	PersonID   string             `bson:"personId"`
	Source     string             `bson:"source"`
	ExternalID string             `bson:"externalId"`
	Payload    map[string]any     `bson:"payload,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
}
