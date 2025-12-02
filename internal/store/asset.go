package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GenerateAssetNumber generates a unique asset number in the format:
// {office}-{year}-{catNo}-{typeNumber}-{serialIncrement}
// Example: B15-2024-02-073-0001
func (s *MongoStore) GenerateAssetNumber(ctx context.Context, tenantID, office, categoryNo, typeNumber string, year int) (string, error) {
	if s == nil {
		return "", errors.New("store: nil MongoStore")
	}

	// Find the maximum serial increment for this asset type
	filter := bson.M{
		"tenantId":   tenantID,
		"office":     office,
		"year":       year,
		"categoryNo": categoryNo,
		"typeNumber": typeNumber,
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "serialIncrement", Value: -1}})

	var lastAsset Asset
	err := s.db.Collection("assets").FindOne(ctx, filter, opts).Decode(&lastAsset)

	var nextSerial int
	if err == mongo.ErrNoDocuments {
		// First asset of this type
		nextSerial = 1
	} else if err != nil {
		return "", fmt.Errorf("store: find last asset: %w", err)
	} else {
		// Parse existing serial and increment
		currentSerial, parseErr := strconv.Atoi(lastAsset.SerialIncrement)
		if parseErr != nil {
			return "", fmt.Errorf("store: parse serial increment %s: %w", lastAsset.SerialIncrement, parseErr)
		}
		nextSerial = currentSerial + 1
	}

	// Format: {office}-{year}-{catNo}-{typeNumber}-{serial}
	assetNo := fmt.Sprintf("%s-%04d-%s-%s-%04d", office, year, categoryNo, typeNumber, nextSerial)

	// Verify uniqueness
	count, err := s.db.Collection("assets").CountDocuments(ctx, bson.M{"assetNo": assetNo})
	if err != nil {
		return "", fmt.Errorf("store: check asset number uniqueness: %w", err)
	}
	if count > 0 {
		return "", fmt.Errorf("store: asset number %s already exists", assetNo)
	}

	return assetNo, nil
}

// CreateAsset inserts a new asset into the assets collection.
func (s *MongoStore) CreateAsset(ctx context.Context, asset *Asset) error {
	if s == nil {
		return errors.New("store: nil MongoStore")
	}

	if asset.ID.IsZero() {
		asset.ID = primitive.NewObjectID()
	}

	now := time.Now().UTC()
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = now
	}
	asset.UpdatedAt = now

	_, err := s.db.Collection("assets").InsertOne(ctx, asset)
	if err != nil {
		return fmt.Errorf("store: insert asset: %w", err)
	}

	return nil
}

// UpdateAsset updates an existing asset.
func (s *MongoStore) UpdateAsset(ctx context.Context, asset *Asset) error {
	if s == nil {
		return errors.New("store: nil MongoStore")
	}

	asset.UpdatedAt = time.Now().UTC()

	filter := bson.M{
		"tenantId": asset.TenantID,
		"assetNo":  asset.AssetNo,
	}

	update := bson.M{"$set": asset}

	result, err := s.db.Collection("assets").UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("store: update asset: %w", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("store: asset not found")
	}

	return nil
}

// GetAssetByNumber retrieves an asset by its asset number.
func (s *MongoStore) GetAssetByNumber(ctx context.Context, tenantID, assetNo string) (*Asset, error) {
	if s == nil {
		return nil, errors.New("store: nil MongoStore")
	}

	filter := bson.M{
		"tenantId": tenantID,
		"assetNo":  assetNo,
	}

	var asset Asset
	err := s.db.Collection("assets").FindOne(ctx, filter).Decode(&asset)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get asset by number: %w", err)
	}

	return &asset, nil
}

// AssetQueryFilter represents filters for querying assets.
type AssetQueryFilter struct {
	TenantID            string
	Office              string
	CategoryNo          string
	TypeNumber          string
	Status              string
	AcquisitionDateFrom *time.Time
	AcquisitionDateTo   *time.Time
	Limit               int
	Offset              int
}

// QueryAssets retrieves assets matching the given filters.
func (s *MongoStore) QueryAssets(ctx context.Context, filter AssetQueryFilter) ([]Asset, int64, error) {
	if s == nil {
		return nil, 0, errors.New("store: nil MongoStore")
	}

	query := bson.M{"tenantId": filter.TenantID}

	if filter.Office != "" {
		query["office"] = filter.Office
	}
	if filter.CategoryNo != "" {
		query["categoryNo"] = filter.CategoryNo
	}
	if filter.TypeNumber != "" {
		query["typeNumber"] = filter.TypeNumber
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}

	// Date range filter
	if filter.AcquisitionDateFrom != nil || filter.AcquisitionDateTo != nil {
		dateFilter := bson.M{}
		if filter.AcquisitionDateFrom != nil {
			dateFilter["$gte"] = *filter.AcquisitionDateFrom
		}
		if filter.AcquisitionDateTo != nil {
			dateFilter["$lte"] = *filter.AcquisitionDateTo
		}
		query["acquisitionDate"] = dateFilter
	}

	// Count total matching documents
	total, err := s.db.Collection("assets").CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("store: count assets: %w", err)
	}

	// Apply pagination
	opts := options.Find()
	if filter.Limit > 0 {
		opts.SetLimit(int64(filter.Limit))
	}
	if filter.Offset > 0 {
		opts.SetSkip(int64(filter.Offset))
	}
	opts.SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := s.db.Collection("assets").Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("store: find assets: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var assets []Asset
	if err := cursor.All(ctx, &assets); err != nil {
		return nil, 0, fmt.Errorf("store: decode assets: %w", err)
	}

	return assets, total, nil
}

// CreateAssetLink creates a link between an asset and a source system.
func (s *MongoStore) CreateAssetLink(ctx context.Context, link *AssetLink) error {
	if s == nil {
		return errors.New("store: nil MongoStore")
	}

	if link.ID.IsZero() {
		link.ID = primitive.NewObjectID()
	}

	now := time.Now().UTC()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}
	link.UpdatedAt = now

	_, err := s.db.Collection("asset_links").InsertOne(ctx, link)
	if err != nil {
		return fmt.Errorf("store: insert asset link: %w", err)
	}

	return nil
}

// GetAssetLinks retrieves all links for a specific asset.
func (s *MongoStore) GetAssetLinks(ctx context.Context, tenantID, assetNo string) ([]AssetLink, error) {
	if s == nil {
		return nil, errors.New("store: nil MongoStore")
	}

	filter := bson.M{
		"tenantId": tenantID,
		"assetNo":  assetNo,
	}

	cursor, err := s.db.Collection("asset_links").Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("store: find asset links: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var links []AssetLink
	if err := cursor.All(ctx, &links); err != nil {
		return nil, fmt.Errorf("store: decode asset links: %w", err)
	}

	return links, nil
}

// CreateAssetHistory adds a history event for an asset.
func (s *MongoStore) CreateAssetHistory(ctx context.Context, history *AssetHistory) error {
	if s == nil {
		return errors.New("store: nil MongoStore")
	}

	if history.ID.IsZero() {
		history.ID = primitive.NewObjectID()
	}

	if history.Timestamp.IsZero() {
		history.Timestamp = time.Now().UTC()
	}

	_, err := s.db.Collection("asset_history").InsertOne(ctx, history)
	if err != nil {
		return fmt.Errorf("store: insert asset history: %w", err)
	}

	return nil
}

// GetAssetHistory retrieves the history events for an asset.
func (s *MongoStore) GetAssetHistory(ctx context.Context, tenantID, assetNo string) ([]AssetHistory, error) {
	if s == nil {
		return nil, errors.New("store: nil MongoStore")
	}

	filter := bson.M{
		"tenantId": tenantID,
		"assetNo":  assetNo,
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := s.db.Collection("asset_history").Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("store: find asset history: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var history []AssetHistory
	if err := cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("store: decode asset history: %w", err)
	}

	return history, nil
}
