package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
)

var (
	// Asset number validation: {office}-{year}-{catNo}-{typeNumber}-{serial}
	assetNoRegex = regexp.MustCompile(`^[A-Z0-9]{2,4}-\d{4}-(0[1-9]|290)-\d{3}-\d{4}$`)

	validCategories = map[string]bool{
		"01": true, "02": true, "03": true, "04": true,
		"05": true, "06": true, "07": true, "08": true,
		"09": true, "290": true,
	}
)

// AssetIngestRecord represents a single asset record to be ingested.
type AssetIngestRecord struct {
	Office             string
	CategoryNo         string
	SubCategoryNo      string
	TypeNumber         string
	Description1       string
	Description2       string
	UseOfAsset         string
	Location           string
	Quantity           int
	Vendor             string
	Manufacturer       string
	IsPurchasedNew     bool
	AcquisitionDate    time.Time
	UseCommenceDate    *time.Time
	CountryOfOrigin    string
	TypeBrand          string
	OriginalValue      float64
	Currency           string
	UsefulLifeYears    int
	DepreciationMethod string
	ResidualValue      float64
	BusinessArea       string
	CostCentre         string
	CreationFormNo     string
	RequestedBy        *store.AssetApproval
	AuthorizedBy       *store.AssetApproval
	SAPAssetMasterID   string
	Attributes         map[string]any
}

// AssetIngestResult contains the results of an asset ingest operation.
type AssetIngestResult struct {
	Created      int
	AssetNumbers []string
	Errors       []AssetIngestError
}

// AssetIngestError represents an error for a specific record.
type AssetIngestError struct {
	RecordIndex int
	Field       string
	Message     string
}

// AssetService handles asset-related business logic.
type AssetService struct {
	store                 *store.MongoStore
	classificationService *ClassificationService
}

// NewAssetService creates a new asset service.
func NewAssetService(s *store.MongoStore) *AssetService {
	return &AssetService{store: s}
}

// SetClassificationService sets the classification service for name enrichment.
func (s *AssetService) SetClassificationService(cs *ClassificationService) {
	s.classificationService = cs
}

// IngestAssets processes multiple asset records and creates them in the system.
func (s *AssetService) IngestAssets(ctx context.Context, tenantID, sourceSlug string, records []AssetIngestRecord, createdBy string) (*AssetIngestResult, error) {
	result := &AssetIngestResult{
		AssetNumbers: make([]string, 0),
		Errors:       make([]AssetIngestError, 0),
	}

	currentYear := time.Now().Year()

	for i, rec := range records {
		// Validate record
		if err := s.validateAssetRecord(&rec, i, result); err != nil {
			continue // Validation errors added to result.Errors
		}

		// Determine year (use current year if not specified)
		year := rec.AcquisitionDate.Year()
		if year == 1 || year == 0 {
			year = currentYear
		}

		// Handle quantity > 1 (create multiple assets)
		quantity := rec.Quantity
		if quantity < 1 {
			quantity = 1
		}

		for q := 0; q < quantity; q++ {
			// Generate unique asset number
			assetNo, err := s.store.GenerateAssetNumber(ctx, tenantID, rec.Office, rec.CategoryNo, rec.TypeNumber, year)
			if err != nil {
				result.Errors = append(result.Errors, AssetIngestError{
					RecordIndex: i,
					Field:       "assetNo",
					Message:     fmt.Sprintf("failed to generate asset number: %v", err),
				})
				continue
			}

			// Extract serial increment from asset number
			serialIncrement := assetNo[len(assetNo)-4:]

			// Enrich with classification names if service available
			categoryName := ""
			subCategoryName := ""
			typeName := ""
			if s.classificationService != nil {
				cat, _ := s.classificationService.GetCategory(ctx, rec.CategoryNo)
				if cat != nil {
					categoryName = cat.Name
				}
				if rec.SubCategoryNo != "" {
					subCategoryName = s.classificationService.GetSubcategoryName(rec.CategoryNo, rec.SubCategoryNo)
				}
				typeName = s.classificationService.GetTypeName(rec.CategoryNo, rec.TypeNumber)
			}

			// Create asset
			asset := &store.Asset{
				AssetNo:            assetNo,
				TenantID:           tenantID,
				Office:             rec.Office,
				Year:               year,
				CategoryNo:         rec.CategoryNo,
				CategoryName:       categoryName,
				SubCategoryNo:      rec.SubCategoryNo,
				SubCategoryName:    subCategoryName,
				TypeNumber:         rec.TypeNumber,
				TypeName:           typeName,
				SerialIncrement:    serialIncrement,
				Description1:       rec.Description1,
				Description2:       rec.Description2,
				UseOfAsset:         rec.UseOfAsset,
				Location:           rec.Location,
				Quantity:           1, // Always 1 per asset record
				Vendor:             rec.Vendor,
				Manufacturer:       rec.Manufacturer,
				IsPurchasedNew:     rec.IsPurchasedNew,
				AcquisitionDate:    rec.AcquisitionDate,
				UseCommenceDate:    rec.UseCommenceDate,
				CountryOfOrigin:    rec.CountryOfOrigin,
				TypeBrand:          rec.TypeBrand,
				OriginalValue:      rec.OriginalValue,
				Currency:           rec.Currency,
				UsefulLifeYears:    rec.UsefulLifeYears,
				DepreciationMethod: rec.DepreciationMethod,
				ResidualValue:      rec.ResidualValue,
				BusinessArea:       rec.BusinessArea,
				CostCentre:         rec.CostCentre,
				CreationFormNo:     rec.CreationFormNo,
				RequestedBy:        rec.RequestedBy,
				AuthorizedBy:       rec.AuthorizedBy,
				SAPAssetMasterID:   rec.SAPAssetMasterID,
				Status:             "active",
				BarcodeID:          fmt.Sprintf("BC-%s", assetNo),
				Attributes:         rec.Attributes,
				CreatedBy:          createdBy,
			}

			if err := s.store.CreateAsset(ctx, asset); err != nil {
				result.Errors = append(result.Errors, AssetIngestError{
					RecordIndex: i,
					Field:       "asset",
					Message:     fmt.Sprintf("failed to create asset: %v", err),
				})
				continue
			}

			// Create asset link to source system
			link := &store.AssetLink{
				AssetNo:    assetNo,
				TenantID:   tenantID,
				Source:     sourceSlug,
				ExternalID: fmt.Sprintf("%s-%d", rec.Office, i),
				Payload:    make(map[string]any),
			}

			if err := s.store.CreateAssetLink(ctx, link); err != nil {
				// Log error but don't fail the entire ingest
				// The asset was created successfully
				_ = err // explicitly ignore
			}

			// Record creation in history
			history := &store.AssetHistory{
				AssetNo:     assetNo,
				TenantID:    tenantID,
				EventType:   "created",
				PerformedBy: createdBy,
				Notes:       fmt.Sprintf("Asset created from %s import", sourceSlug),
			}

			_ = s.store.CreateAssetHistory(ctx, history)

			result.AssetNumbers = append(result.AssetNumbers, assetNo)
			result.Created++
		}
	}

	return result, nil
}

// validateAssetRecord validates a single asset record.
func (s *AssetService) validateAssetRecord(rec *AssetIngestRecord, index int, result *AssetIngestResult) error {
	hasError := false

	// Required fields
	if rec.Office == "" {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "office",
			Message:     "office code is required",
		})
		hasError = true
	}

	if rec.CategoryNo == "" {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "categoryNo",
			Message:     "category number is required",
		})
		hasError = true
	} else if !validCategories[rec.CategoryNo] {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "categoryNo",
			Message:     "invalid category number (must be 01-09 or 290)",
		})
		hasError = true
	}

	if rec.TypeNumber == "" {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "typeNumber",
			Message:     "type number is required",
		})
		hasError = true
	}

	if rec.Description1 == "" {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "description1",
			Message:     "description is required",
		})
		hasError = true
	}

	if rec.Location == "" {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "location",
			Message:     "location is required",
		})
		hasError = true
	}

	if rec.OriginalValue <= 0 {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "originalValue",
			Message:     "original value must be greater than zero",
		})
		hasError = true
	}

	if rec.AcquisitionDate.IsZero() {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "acquisitionDate",
			Message:     "acquisition date is required",
		})
		hasError = true
	} else if rec.AcquisitionDate.After(time.Now()) {
		result.Errors = append(result.Errors, AssetIngestError{
			RecordIndex: index,
			Field:       "acquisitionDate",
			Message:     "acquisition date cannot be in the future",
		})
		hasError = true
	}

	// Set defaults
	if rec.Currency == "" {
		rec.Currency = "MVR"
	}

	if rec.DepreciationMethod == "" {
		rec.DepreciationMethod = "straight-line"
	}

	if hasError {
		return errors.New("validation failed")
	}

	return nil
}

// GetAsset retrieves an asset by its asset number.
func (s *AssetService) GetAsset(ctx context.Context, tenantID, assetNo string) (*store.Asset, []store.AssetLink, error) {
	// Validate asset number format
	if !assetNoRegex.MatchString(assetNo) {
		return nil, nil, fmt.Errorf("invalid asset number format: %s", assetNo)
	}

	asset, err := s.store.GetAssetByNumber(ctx, tenantID, assetNo)
	if err != nil {
		return nil, nil, fmt.Errorf("get asset: %w", err)
	}
	if asset == nil {
		return nil, nil, nil // Not found
	}

	links, err := s.store.GetAssetLinks(ctx, tenantID, assetNo)
	if err != nil {
		return nil, nil, fmt.Errorf("get asset links: %w", err)
	}

	return asset, links, nil
}

// QueryAssets queries assets with filters.
func (s *AssetService) QueryAssets(ctx context.Context, filter store.AssetQueryFilter) ([]store.Asset, int64, error) {
	return s.store.QueryAssets(ctx, filter)
}

// GetAssetHistory retrieves the history of an asset.
func (s *AssetService) GetAssetHistory(ctx context.Context, tenantID, assetNo string) ([]store.AssetHistory, error) {
	return s.store.GetAssetHistory(ctx, tenantID, assetNo)
}

// TransferAsset transfers an asset to a new location.
func (s *AssetService) TransferAsset(ctx context.Context, tenantID, assetNo, toLocation, performedBy, notes string) error {
	// Get the asset first
	asset, err := s.store.GetAssetByNumber(ctx, tenantID, assetNo)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset == nil {
		return errors.New("asset not found")
	}

	fromLocation := asset.Location

	// Update asset location
	asset.Location = toLocation
	asset.UpdatedAt = time.Now().UTC()

	// Update the asset (we need an update method)
	if err := s.store.UpdateAsset(ctx, asset); err != nil {
		return fmt.Errorf("update asset: %w", err)
	}

	// Record history
	history := &store.AssetHistory{
		AssetNo:      assetNo,
		TenantID:     tenantID,
		EventType:    "transfer",
		FromLocation: fromLocation,
		ToLocation:   toLocation,
		PerformedBy:  performedBy,
		Notes:        notes,
	}

	if err := s.store.CreateAssetHistory(ctx, history); err != nil {
		// Log but don't fail - asset was updated
		_ = err
	}

	return nil
}

// DisposeAsset marks an asset as disposed.
func (s *AssetService) DisposeAsset(ctx context.Context, tenantID, assetNo, reason, performedBy string, saleValue float64, notes string) error {
	// Get the asset first
	asset, err := s.store.GetAssetByNumber(ctx, tenantID, assetNo)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset == nil {
		return errors.New("asset not found")
	}

	// Update asset status
	asset.Status = "disposed"
	asset.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateAsset(ctx, asset); err != nil {
		return fmt.Errorf("update asset: %w", err)
	}

	// Record history
	historyNotes := fmt.Sprintf("Reason: %s", reason)
	if saleValue > 0 {
		historyNotes += fmt.Sprintf(" | Sale Value: %.2f", saleValue)
	}
	if notes != "" {
		historyNotes += " | " + notes
	}

	history := &store.AssetHistory{
		AssetNo:     assetNo,
		TenantID:    tenantID,
		EventType:   "dispose",
		PerformedBy: performedBy,
		Notes:       historyNotes,
	}

	if err := s.store.CreateAssetHistory(ctx, history); err != nil {
		// Log but don't fail - asset was updated
		_ = err
	}

	return nil
}
