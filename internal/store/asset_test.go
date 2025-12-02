package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestGenerateAssetNumber(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping asset integration test in short mode")
	}

	t.Parallel()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = defaultURI
	}

	dbName := "kdserver_test_asset_" + time.Now().Format("20060102_150405")

	ctx := context.Background()
	mongoStore, err := Connect(ctx, Config{
		URI:      uri,
		Database: dbName,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Skipf("skipping asset integration test, connect failed: %v", err)
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

	tenantID := "test-tenant"
	office := "B15"
	categoryNo := "02"
	typeNumber := "073"
	year := 2024

	// Generate first asset number
	assetNo1, err := mongoStore.GenerateAssetNumber(ctx, tenantID, office, categoryNo, typeNumber, year)
	if err != nil {
		t.Fatalf("GenerateAssetNumber failed: %v", err)
	}

	expected1 := "B15-2024-02-073-0001"
	if assetNo1 != expected1 {
		t.Errorf("expected %s, got %s", expected1, assetNo1)
	}

	// Create the asset
	asset1 := &Asset{
		AssetNo:         assetNo1,
		TenantID:        tenantID,
		Office:          office,
		Year:            year,
		CategoryNo:      categoryNo,
		TypeNumber:      typeNumber,
		SerialIncrement: "0001",
		Description1:    "Test Asset",
		Location:        "Test Location",
		OriginalValue:   1000.00,
		Currency:        "MVR",
		AcquisitionDate: time.Now(),
		Status:          "active",
		Quantity:        1,
		IsPurchasedNew:  true,
	}

	if err := mongoStore.CreateAsset(ctx, asset1); err != nil {
		t.Fatalf("CreateAsset failed: %v", err)
	}

	// Generate second asset number
	assetNo2, err := mongoStore.GenerateAssetNumber(ctx, tenantID, office, categoryNo, typeNumber, year)
	if err != nil {
		t.Fatalf("GenerateAssetNumber failed for second asset: %v", err)
	}

	expected2 := "B15-2024-02-073-0002"
	if assetNo2 != expected2 {
		t.Errorf("expected %s, got %s", expected2, assetNo2)
	}
}

func TestGetAssetByNumber(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping asset integration test in short mode")
	}

	t.Parallel()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = defaultURI
	}

	dbName := "kdserver_test_asset_get_" + time.Now().Format("20060102_150405")

	ctx := context.Background()
	mongoStore, err := Connect(ctx, Config{
		URI:      uri,
		Database: dbName,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Skipf("skipping asset integration test, connect failed: %v", err)
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

	tenantID := "test-tenant"
	assetNo := "B15-2024-02-073-0001"

	// Create test asset
	asset := &Asset{
		AssetNo:         assetNo,
		TenantID:        tenantID,
		Office:          "B15",
		Year:            2024,
		CategoryNo:      "02",
		TypeNumber:      "073",
		SerialIncrement: "0001",
		Description1:    "Test Computer Table",
		Location:        "Lab A",
		OriginalValue:   5000.00,
		Currency:        "MVR",
		AcquisitionDate: time.Now(),
		Status:          "active",
		Quantity:        1,
		IsPurchasedNew:  true,
	}

	if err := mongoStore.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("CreateAsset failed: %v", err)
	}

	// Retrieve the asset
	retrieved, err := mongoStore.GetAssetByNumber(ctx, tenantID, assetNo)
	if err != nil {
		t.Fatalf("GetAssetByNumber failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("expected asset to be found, got nil")
	}

	if retrieved.AssetNo != assetNo {
		t.Errorf("expected asset number %s, got %s", assetNo, retrieved.AssetNo)
	}

	if retrieved.Description1 != "Test Computer Table" {
		t.Errorf("expected description 'Test Computer Table', got %s", retrieved.Description1)
	}
}

func TestQueryAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping asset integration test in short mode")
	}

	t.Parallel()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = defaultURI
	}

	dbName := "kdserver_test_asset_query_" + time.Now().Format("20060102_150405")

	ctx := context.Background()
	mongoStore, err := Connect(ctx, Config{
		URI:      uri,
		Database: dbName,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Skipf("skipping asset integration test, connect failed: %v", err)
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

	tenantID := "test-tenant"

	// Create multiple assets
	assets := []Asset{
		{
			AssetNo:         "B15-2024-02-073-0001",
			TenantID:        tenantID,
			Office:          "B15",
			Year:            2024,
			CategoryNo:      "02",
			TypeNumber:      "073",
			SerialIncrement: "0001",
			Description1:    "Asset 1",
			Location:        "Location 1",
			OriginalValue:   1000.00,
			Currency:        "MVR",
			AcquisitionDate: time.Now(),
			Status:          "active",
			Quantity:        1,
			IsPurchasedNew:  true,
		},
		{
			AssetNo:         "B15-2024-02-073-0002",
			TenantID:        tenantID,
			Office:          "B15",
			Year:            2024,
			CategoryNo:      "02",
			TypeNumber:      "073",
			SerialIncrement: "0002",
			Description1:    "Asset 2",
			Location:        "Location 2",
			OriginalValue:   2000.00,
			Currency:        "MVR",
			AcquisitionDate: time.Now(),
			Status:          "active",
			Quantity:        1,
			IsPurchasedNew:  true,
		},
	}

	for i := range assets {
		if err := mongoStore.CreateAsset(ctx, &assets[i]); err != nil {
			t.Fatalf("CreateAsset failed: %v", err)
		}
	}

	// Query all assets for tenant
	filter := AssetQueryFilter{
		TenantID: tenantID,
		Limit:    10,
		Offset:   0,
	}

	results, total, err := mongoStore.QueryAssets(ctx, filter)
	if err != nil {
		t.Fatalf("QueryAssets failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Query by office
	filter.Office = "B15"
	results, total, err = mongoStore.QueryAssets(ctx, filter)
	if err != nil {
		t.Fatalf("QueryAssets with office filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2 with office filter, got %d", total)
	}

	// Query by category
	filter.Office = ""
	filter.CategoryNo = "02"
	results, total, err = mongoStore.QueryAssets(ctx, filter)
	if err != nil {
		t.Fatalf("QueryAssets with category filter failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2 with category filter, got %d", total)
	}
}
