package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	defaultURI     = "mongodb://localhost:27017"
	defaultTimeout = 10 * time.Second
)

// Config contains the parameters required to connect to MongoDB.
type Config struct {
	URI      string
	Database string
	Logger   *log.Logger
	Timeout  time.Duration
}

// MongoStore bundles the MongoDB client and database handle.
type MongoStore struct {
	client  *mongo.Client
	db      *mongo.Database
	logger  *log.Logger
	timeout time.Duration
}

// Connect establishes a MongoDB client and verifies connectivity.
func Connect(ctx context.Context, cfg Config) (*MongoStore, error) {
	if cfg.Database == "" {
		return nil, errors.New("store: database name is required")
	}

	uri := cfg.URI
	if strings.TrimSpace(uri) == "" {
		uri = defaultURI
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}

	pingCtx, cancelPing := context.WithTimeout(ctx, timeout)
	defer cancelPing()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(pingCtx)
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	store := &MongoStore{
		client:  client,
		db:      client.Database(cfg.Database),
		logger:  cfg.Logger,
		timeout: timeout,
	}

	if store.logger != nil {
		store.logger.Printf("mongo connected uri=%s database=%s", uri, cfg.Database)
	}

	return store, nil
}

// EnsureIndexes creates the default collection indexes and logs the results.
func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	if s == nil {
		return errors.New("store: EnsureIndexes on nil MongoStore")
	}

	specs := defaultIndexSpecs()

	for collection, models := range specs {
		if len(models) == 0 {
			continue
		}

		names, err := s.db.Collection(collection).Indexes().CreateMany(ctx, models)
		if err != nil {
			return fmt.Errorf("store: ensure indexes for %s: %w", collection, err)
		}

		if s.logger == nil {
			continue
		}

		for i, model := range models {
			var keyDoc bson.D
			if doc, ok := model.Keys.(bson.D); ok {
				keyDoc = doc
			}

			s.logger.Printf(
				"mongo index ensured collection=%s name=%s keys=%s",
				collection,
				names[i],
				describeKeys(keyDoc),
			)
		}
	}

	return nil
}

// Close disconnects the MongoDB client.
func (s *MongoStore) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}

	closeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.client.Disconnect(closeCtx)
}

func defaultIndexSpecs() map[string][]mongo.IndexModel {
	return map[string][]mongo.IndexModel{
		"people": {
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}},
				Options: options.Index().SetName("people_tenant_person").SetUnique(true),
			},
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "primaryEmail", Value: 1}},
				Options: options.Index().SetName("people_tenant_email"),
			},
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "primaryPhone", Value: 1}},
				Options: options.Index().SetName("people_tenant_phone"),
			},
		},
		"links": {
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "externalId", Value: 1}, {Key: "source", Value: 1}},
				Options: options.Index().SetName("links_external").SetUnique(true),
			},
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}},
				Options: options.Index().SetName("links_person"),
			},
		},
		"sources": {
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "slug", Value: 1}},
				Options: options.Index().SetName("sources_slug").SetUnique(true),
			},
		},
		"raw": {
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "ingestBatch", Value: 1}},
				Options: options.Index().SetName("raw_ingest_batch"),
			},
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "status", Value: 1}},
				Options: options.Index().SetName("raw_status"),
			},
		},
		"tenants": {
			{
				Keys:    bson.D{{Key: "slug", Value: 1}},
				Options: options.Index().SetName("tenants_slug").SetUnique(true),
			},
		},
		"keys": {
			{
				Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "keyId", Value: 1}},
				Options: options.Index().SetName("keys_tenant_key").SetUnique(true),
			},
			{
				Keys:    bson.D{{Key: "keyHash", Value: 1}},
				Options: options.Index().SetName("keys_hash"),
			},
		},
	}
}

func describeKeys(doc bson.D) string {
	if len(doc) == 0 {
		return ""
	}

	parts := make([]string, 0, len(doc))
	for _, elem := range doc {
		parts = append(parts, fmt.Sprintf("%s:%v", elem.Key, elem.Value))
	}
	return strings.Join(parts, ",")
}
