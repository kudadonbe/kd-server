package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

// The asset ingest DTOs mirror internal/http/asset.go's assetIngestRequest
// so the same JSON file can be posted to the HTTP API or loaded by the TUI.

type assetIngestFileRequest struct {
	Source  assetIngestFileSource   `json:"source"`
	Records []assetIngestFileRecord `json:"records"`
}

type assetIngestFileSource struct {
	Slug string `json:"slug"`
	Name string `json:"name,omitempty"`
}

type assetIngestFileRecord struct {
	Office             string                   `json:"office"`
	CategoryNo         string                   `json:"categoryNo"`
	SubCategoryNo      string                   `json:"subCategoryNo,omitempty"`
	TypeNumber         string                   `json:"typeNumber"`
	Description1       string                   `json:"description1"`
	Description2       string                   `json:"description2,omitempty"`
	UseOfAsset         string                   `json:"useOfAsset,omitempty"`
	Location           string                   `json:"location"`
	Quantity           int                      `json:"quantity,omitempty"`
	Vendor             string                   `json:"vendor,omitempty"`
	Manufacturer       string                   `json:"manufacturer,omitempty"`
	IsPurchasedNew     bool                     `json:"isPurchasedNew"`
	AcquisitionDate    string                   `json:"acquisitionDate"`
	UseCommenceDate    string                   `json:"useCommenceDate,omitempty"`
	CountryOfOrigin    string                   `json:"countryOfOrigin,omitempty"`
	TypeBrand          string                   `json:"typeBrand,omitempty"`
	OriginalValue      float64                  `json:"originalValue"`
	Currency           string                   `json:"currency,omitempty"`
	UsefulLifeYears    int                      `json:"usefulLifeYears,omitempty"`
	DepreciationMethod string                   `json:"depreciationMethod,omitempty"`
	ResidualValue      float64                  `json:"residualValue,omitempty"`
	BusinessArea       string                   `json:"businessArea,omitempty"`
	CostCentre         string                   `json:"costCentre,omitempty"`
	CreationFormNo     string                   `json:"creationFormNo,omitempty"`
	RequestedBy        *assetIngestFileApproval `json:"requestedBy,omitempty"`
	AuthorizedBy       *assetIngestFileApproval `json:"authorizedBy,omitempty"`
	SAPAssetMasterID   string                   `json:"sapAssetMasterId,omitempty"`
	Attributes         map[string]any           `json:"attributes,omitempty"`
}

type assetIngestFileApproval struct {
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Date        string `json:"date"`
}

func (a *app) assetsMenu() {
	tenant := a.requireTenant()
	if tenant == nil {
		return
	}

	for {
		fmt.Println()
		fmt.Printf("Assets (tenant: %s):\n", tenant.Slug)
		fmt.Println(" 1) Ingest assets from JSON file")
		fmt.Println(" 2) Query assets")
		fmt.Println(" 3) Get asset by number")
		fmt.Println(" 4) Transfer asset")
		fmt.Println(" 5) Dispose asset")
		fmt.Println(" 6) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = ingestAssetsFromFile(a.ctx, a.assets, a.reader, tenant.Slug)
		case "2":
			err = queryAssets(a.ctx, a.assets, a.reader, tenant.Slug)
		case "3":
			err = getAsset(a.ctx, a.assets, a.reader, tenant.Slug)
		case "4":
			err = transferAsset(a.ctx, a.assets, a.reader, tenant.Slug)
		case "5":
			err = disposeAsset(a.ctx, a.assets, a.reader, tenant.Slug)
		case "6":
			return
		default:
			fmt.Println("Unknown option, please choose 1-6.")
			fmt.Println()
			continue
		}
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		}
	}
}

func ingestAssetsFromFile(ctx context.Context, assetService *services.AssetService, reader *bufio.Reader, tenantID string) error {
	path := prompt(reader, "Path to asset ingest JSON file")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	sourceSlug, records, err := parseAssetIngestFile(data)
	if err != nil {
		return err
	}

	createdBy := promptDefault(reader, "Created by", "tui-admin")

	result, err := assetService.IngestAssets(ctx, tenantID, sourceSlug, records, createdBy)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Created: %d\n", result.Created)
	if len(result.AssetNumbers) > 0 {
		fmt.Printf("Asset numbers: %s\n", strings.Join(result.AssetNumbers, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - record %d (%s): %s\n", e.RecordIndex, e.Field, e.Message)
		}
	}
	fmt.Println()
	return nil
}

// parseAssetIngestFile decodes an asset ingest JSON file into a source slug
// and a slice of AssetIngestRecord, matching internal/http/asset.go's mapping.
func parseAssetIngestFile(data []byte) (string, []services.AssetIngestRecord, error) {
	var req assetIngestFileRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return "", nil, fmt.Errorf("parse asset ingest file: %w", err)
	}

	records := make([]services.AssetIngestRecord, len(req.Records))
	for i, rec := range req.Records {
		acqDate, err := time.Parse("2006-01-02", rec.AcquisitionDate)
		if err != nil {
			return "", nil, fmt.Errorf("record %d: invalid acquisitionDate %q, use YYYY-MM-DD", i, rec.AcquisitionDate)
		}

		var useCommenceDate *time.Time
		if rec.UseCommenceDate != "" {
			ucd, err := time.Parse("2006-01-02", rec.UseCommenceDate)
			if err != nil {
				return "", nil, fmt.Errorf("record %d: invalid useCommenceDate %q, use YYYY-MM-DD", i, rec.UseCommenceDate)
			}
			useCommenceDate = &ucd
		}

		records[i] = services.AssetIngestRecord{
			Office:             rec.Office,
			CategoryNo:         rec.CategoryNo,
			SubCategoryNo:      rec.SubCategoryNo,
			TypeNumber:         rec.TypeNumber,
			Description1:       rec.Description1,
			Description2:       rec.Description2,
			UseOfAsset:         rec.UseOfAsset,
			Location:           rec.Location,
			Quantity:           rec.Quantity,
			Vendor:             rec.Vendor,
			Manufacturer:       rec.Manufacturer,
			IsPurchasedNew:     rec.IsPurchasedNew,
			AcquisitionDate:    acqDate,
			UseCommenceDate:    useCommenceDate,
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
			SAPAssetMasterID:   rec.SAPAssetMasterID,
			Attributes:         rec.Attributes,
		}

		if rec.RequestedBy != nil {
			reqDate, _ := time.Parse("2006-01-02", rec.RequestedBy.Date)
			records[i].RequestedBy = &store.AssetApproval{
				UserID:      rec.RequestedBy.UserID,
				Name:        rec.RequestedBy.Name,
				Designation: rec.RequestedBy.Designation,
				Date:        reqDate,
			}
		}

		if rec.AuthorizedBy != nil {
			authDate, _ := time.Parse("2006-01-02", rec.AuthorizedBy.Date)
			records[i].AuthorizedBy = &store.AssetApproval{
				UserID:      rec.AuthorizedBy.UserID,
				Name:        rec.AuthorizedBy.Name,
				Designation: rec.AuthorizedBy.Designation,
				Date:        authDate,
			}
		}
	}

	return req.Source.Slug, records, nil
}

func queryAssets(ctx context.Context, assetService *services.AssetService, reader *bufio.Reader, tenantID string) error {
	office := prompt(reader, "Office (blank = any)")
	categoryNo := prompt(reader, "Category No (blank = any)")
	typeNumber := prompt(reader, "Type Number (blank = any)")
	status := prompt(reader, "Status (blank = any)")
	limitStr := promptDefault(reader, "Limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return fmt.Errorf("invalid limit: %q", limitStr)
	}

	filter := store.AssetQueryFilter{
		TenantID:   tenantID,
		Office:     office,
		CategoryNo: categoryNo,
		TypeNumber: typeNumber,
		Status:     status,
		Limit:      limit,
	}

	assets, total, err := assetService.QueryAssets(ctx, filter)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Total matching: %d\n", total)
	if len(assets) == 0 {
		fmt.Println()
		return nil
	}
	fmt.Printf("%-24s %-15s %-10s %s\n", "Asset No", "Status", "Quantity", "Description")
	fmt.Println(strings.Repeat("-", 90))
	for _, asset := range assets {
		fmt.Printf("%-24s %-15s %-10d %s\n", asset.AssetNo, asset.Status, asset.Quantity, asset.Description1)
	}
	fmt.Println()
	return nil
}

func getAsset(ctx context.Context, assetService *services.AssetService, reader *bufio.Reader, tenantID string) error {
	assetNo := prompt(reader, "Asset number")
	asset, links, err := assetService.GetAsset(ctx, tenantID, assetNo)
	if err != nil {
		return err
	}
	if asset == nil {
		fmt.Println()
		fmt.Println("Asset not found.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Printf("Asset No:      %s\n", asset.AssetNo)
	fmt.Printf("Status:        %s\n", asset.Status)
	fmt.Printf("Description:   %s\n", asset.Description1)
	fmt.Printf("Location:      %s\n", asset.Location)
	fmt.Printf("Quantity:      %d\n", asset.Quantity)
	fmt.Printf("Original Value: %.2f %s\n", asset.OriginalValue, asset.Currency)
	fmt.Printf("Acquired:      %s\n", asset.AcquisitionDate.Format("2006-01-02"))

	if len(links) == 0 {
		fmt.Println("Links: none")
	} else {
		fmt.Println("Links:")
		for _, link := range links {
			fmt.Printf("  - %s / %s\n", link.Source, link.ExternalID)
		}
	}
	fmt.Println()
	return nil
}

func transferAsset(ctx context.Context, assetService *services.AssetService, reader *bufio.Reader, tenantID string) error {
	assetNo := prompt(reader, "Asset number")
	toLocation := prompt(reader, "New location")
	performedBy := promptDefault(reader, "Performed by", "tui-admin")
	notes := prompt(reader, "Notes (optional)")

	if err := assetService.TransferAsset(ctx, tenantID, assetNo, toLocation, performedBy, notes); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Asset %s transferred to %s.\n\n", assetNo, toLocation)
	return nil
}

func disposeAsset(ctx context.Context, assetService *services.AssetService, reader *bufio.Reader, tenantID string) error {
	assetNo := prompt(reader, "Asset number")
	reason := prompt(reader, "Disposal reason")
	performedBy := promptDefault(reader, "Performed by", "tui-admin")
	saleValueStr := promptDefault(reader, "Sale value (0 if none)", "0")
	saleValue, err := strconv.ParseFloat(saleValueStr, 64)
	if err != nil {
		return fmt.Errorf("invalid sale value: %q", saleValueStr)
	}
	notes := prompt(reader, "Notes (optional)")

	if err := assetService.DisposeAsset(ctx, tenantID, assetNo, reason, performedBy, saleValue, notes); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Asset %s disposed.\n\n", assetNo)
	return nil
}
