package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestMongoIngestWriter_SaveBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ingest integration test in short mode")
	}

	t.Parallel()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = defaultURI
	}

	dbName := "kdserver_test_ingest_" + time.Now().Format("20060102_150405")

	ctx := context.Background()
	mongoStore, err := Connect(ctx, Config{
		URI:      uri,
		Database: dbName,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Skipf("skipping ingest integration test, connect failed: %v", err)
	}
	defer func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoStore.db.Drop(dropCtx)
		closeCtx, cancelClose := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelClose()
		_ = mongoStore.Close(closeCtx)
	}()

	if err := mongoStore.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	payloadPaths := []string{
		filepath.Join("testdata", "ingest", "sample_batch.json"),
		filepath.Join("..", "testdata", "ingest", "sample_batch.json"),
		filepath.Join("..", "..", "testdata", "ingest", "sample_batch.json"),
	}

	var payloadBytes []byte
	var readErr error
	for _, path := range payloadPaths {
		payloadBytes, readErr = os.ReadFile(path)
		if readErr == nil {
			break
		}
	}
	if readErr != nil {
		t.Fatalf("read sample payload: %v", readErr)
	}

	var payload struct {
		Source  IngestSource     `json:"source"`
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	writer := mongoStore.IngestWriter()

	result, err := writer.SaveBatch(ctx, "tenant-ingest", IngestBatch{
		Source:  payload.Source,
		Records: payload.Records,
	})
	if err != nil {
		t.Fatalf("save batch: %v", err)
	}

	if result.Created != len(payload.Records) {
		t.Fatalf("expected created %d, got %d", len(payload.Records), result.Created)
	}

	countCtx, cancelCount := context.WithTimeout(ctx, 5*time.Second)
	defer cancelCount()

	rawCount, err := mongoStore.db.Collection("raw").CountDocuments(countCtx, bson.M{"tenantId": "tenant-ingest"})
	if err != nil {
		t.Fatalf("count raw documents: %v", err)
	}
	if rawCount != int64(len(payload.Records)) {
		t.Fatalf("expected %d raw documents, got %d", len(payload.Records), rawCount)
	}

	var sourceDoc bson.M
	sourceCtx, cancelSource := context.WithTimeout(ctx, 5*time.Second)
	defer cancelSource()

	if err := mongoStore.db.Collection("sources").FindOne(sourceCtx, bson.M{
		"tenantId": "tenant-ingest",
		"slug":     payload.Source.Slug,
	}).Decode(&sourceDoc); err != nil {
		t.Fatalf("find source doc: %v", err)
	}

	if sourceDoc["name"] != payload.Source.Name {
		t.Fatalf("expected source name %s, got %v", payload.Source.Name, sourceDoc["name"])
	}
}
