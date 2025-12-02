package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

// Asset ingest request/response types

type assetIngestRequest struct {
	Source  ingestSourceRequest  `json:"source"`
	Records []assetRecordRequest `json:"records"`
}

type assetRecordRequest struct {
	Office             string                `json:"office"`
	CategoryNo         string                `json:"categoryNo"`
	SubCategoryNo      string                `json:"subCategoryNo,omitempty"`
	TypeNumber         string                `json:"typeNumber"`
	Description1       string                `json:"description1"`
	Description2       string                `json:"description2,omitempty"`
	UseOfAsset         string                `json:"useOfAsset,omitempty"`
	Location           string                `json:"location"`
	Quantity           int                   `json:"quantity,omitempty"`
	Vendor             string                `json:"vendor,omitempty"`
	Manufacturer       string                `json:"manufacturer,omitempty"`
	IsPurchasedNew     bool                  `json:"isPurchasedNew"`
	AcquisitionDate    string                `json:"acquisitionDate"`
	UseCommenceDate    string                `json:"useCommenceDate,omitempty"`
	CountryOfOrigin    string                `json:"countryOfOrigin,omitempty"`
	TypeBrand          string                `json:"typeBrand,omitempty"`
	OriginalValue      float64               `json:"originalValue"`
	Currency           string                `json:"currency,omitempty"`
	UsefulLifeYears    int                   `json:"usefulLifeYears,omitempty"`
	DepreciationMethod string                `json:"depreciationMethod,omitempty"`
	ResidualValue      float64               `json:"residualValue,omitempty"`
	BusinessArea       string                `json:"businessArea,omitempty"`
	CostCentre         string                `json:"costCentre,omitempty"`
	CreationFormNo     string                `json:"creationFormNo,omitempty"`
	RequestedBy        *assetApprovalRequest `json:"requestedBy,omitempty"`
	AuthorizedBy       *assetApprovalRequest `json:"authorizedBy,omitempty"`
	SAPAssetMasterID   string                `json:"sapAssetMasterId,omitempty"`
	Attributes         map[string]any        `json:"attributes,omitempty"`
}

type assetApprovalRequest struct {
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Date        string `json:"date"`
}

type assetIngestResponse struct {
	Created      int                `json:"created"`
	AssetNumbers []string           `json:"assetNumbers"`
	Errors       []assetIngestError `json:"errors"`
}

type assetIngestError struct {
	RecordIndex int    `json:"recordIndex"`
	Field       string `json:"field"`
	Message     string `json:"message"`
}

// Asset view response types

type assetViewResponse struct {
	Asset *assetResponse      `json:"asset"`
	Links []assetLinkResponse `json:"links"`
}

type assetResponse struct {
	AssetNo            string                 `json:"assetNo"`
	TenantID           string                 `json:"tenantId"`
	Office             string                 `json:"office"`
	Year               int                    `json:"year"`
	CategoryNo         string                 `json:"categoryNo"`
	CategoryName       string                 `json:"categoryName,omitempty"`
	SubCategoryNo      string                 `json:"subCategoryNo,omitempty"`
	SubCategoryName    string                 `json:"subCategoryName,omitempty"`
	TypeNumber         string                 `json:"typeNumber"`
	TypeName           string                 `json:"typeName,omitempty"`
	SerialIncrement    string                 `json:"serialIncrement"`
	Description1       string                 `json:"description1"`
	Description2       string                 `json:"description2,omitempty"`
	UseOfAsset         string                 `json:"useOfAsset,omitempty"`
	Location           string                 `json:"location"`
	Quantity           int                    `json:"quantity"`
	Vendor             string                 `json:"vendor,omitempty"`
	Manufacturer       string                 `json:"manufacturer,omitempty"`
	IsPurchasedNew     bool                   `json:"isPurchasedNew"`
	AcquisitionDate    string                 `json:"acquisitionDate"`
	UseCommenceDate    string                 `json:"useCommenceDate,omitempty"`
	CountryOfOrigin    string                 `json:"countryOfOrigin,omitempty"`
	TypeBrand          string                 `json:"typeBrand,omitempty"`
	OriginalValue      float64                `json:"originalValue"`
	Currency           string                 `json:"currency"`
	UsefulLifeYears    int                    `json:"usefulLifeYears,omitempty"`
	DepreciationMethod string                 `json:"depreciationMethod,omitempty"`
	ResidualValue      float64                `json:"residualValue,omitempty"`
	BusinessArea       string                 `json:"businessArea,omitempty"`
	CostCentre         string                 `json:"costCentre,omitempty"`
	CreationFormNo     string                 `json:"creationFormNo,omitempty"`
	Status             string                 `json:"status"`
	BarcodeID          string                 `json:"barcodeId,omitempty"`
	RequestedBy        *assetApprovalResponse `json:"requestedBy,omitempty"`
	AuthorizedBy       *assetApprovalResponse `json:"authorizedBy,omitempty"`
	SAPAssetMasterID   string                 `json:"sapAssetMasterId,omitempty"`
	Attributes         map[string]any         `json:"attributes,omitempty"`
	CreatedAt          string                 `json:"createdAt"`
	UpdatedAt          string                 `json:"updatedAt"`
	CreatedBy          string                 `json:"createdBy,omitempty"`
}

type assetApprovalResponse struct {
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Date        string `json:"date"`
}

type assetLinkResponse struct {
	Source     string         `json:"source"`
	ExternalID string         `json:"externalId"`
	Payload    map[string]any `json:"payload,omitempty"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
}

type assetListResponse struct {
	Assets []assetResponse `json:"assets"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// assetIngestHandler handles POST /v1/ingest/assets
func assetIngestHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		var req assetIngestRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Convert request to service records
		serviceRecords := make([]services.AssetIngestRecord, len(req.Records))
		for i, rec := range req.Records {
			acqDate, err := parseDate(rec.AcquisitionDate)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "invalid acquisitionDate format, use ISO 8601 (YYYY-MM-DD)",
				})
				return
			}

			var useCommenceDate *time.Time
			if rec.UseCommenceDate != "" {
				ucd, err := parseDate(rec.UseCommenceDate)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{
						"error": "invalid useCommenceDate format, use ISO 8601 (YYYY-MM-DD)",
					})
					return
				}
				useCommenceDate = &ucd
			}

			serviceRecords[i] = services.AssetIngestRecord{
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
				reqDate, _ := parseDate(rec.RequestedBy.Date)
				serviceRecords[i].RequestedBy = &store.AssetApproval{
					UserID:      rec.RequestedBy.UserID,
					Name:        rec.RequestedBy.Name,
					Designation: rec.RequestedBy.Designation,
					Date:        reqDate,
				}
			}

			if rec.AuthorizedBy != nil {
				authDate, _ := parseDate(rec.AuthorizedBy.Date)
				serviceRecords[i].AuthorizedBy = &store.AssetApproval{
					UserID:      rec.AuthorizedBy.UserID,
					Name:        rec.AuthorizedBy.Name,
					Designation: rec.AuthorizedBy.Designation,
					Date:        authDate,
				}
			}
		}

		result, err := cfg.AssetService.IngestAssets(r.Context(), tenantID, req.Source.Slug, serviceRecords, "system")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Convert service result to response
		resp := assetIngestResponse{
			Created:      result.Created,
			AssetNumbers: result.AssetNumbers,
			Errors:       make([]assetIngestError, len(result.Errors)),
		}

		for i, e := range result.Errors {
			resp.Errors[i] = assetIngestError{
				RecordIndex: e.RecordIndex,
				Field:       e.Field,
				Message:     e.Message,
			}
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetLookupHandler handles GET /v1/lookup/asset/{assetNo}
func assetLookupHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		// Extract asset number from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/lookup/asset/")
		assetNo := strings.TrimSpace(path)

		if assetNo == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset number required"})
			return
		}

		asset, links, err := cfg.AssetService.GetAsset(r.Context(), tenantID, assetNo)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if asset == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "asset not found"})
			return
		}

		resp := assetViewResponse{
			Asset: convertAssetToResponse(asset),
			Links: make([]assetLinkResponse, len(links)),
		}

		for i, link := range links {
			resp.Links[i] = assetLinkResponse{
				Source:     link.Source,
				ExternalID: link.ExternalID,
				Payload:    link.Payload,
				CreatedAt:  link.CreatedAt.Format(time.RFC3339),
				UpdatedAt:  link.UpdatedAt.Format(time.RFC3339),
			}
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetsQueryHandler handles GET /v1/assets
func assetsQueryHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		query := r.URL.Query()

		filter := store.AssetQueryFilter{
			TenantID:   tenantID,
			Office:     query.Get("office"),
			CategoryNo: query.Get("categoryNo"),
			TypeNumber: query.Get("typeNumber"),
			Status:     query.Get("status"),
			Limit:      20,
			Offset:     0,
		}

		// Parse limit
		if limitStr := query.Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
				filter.Limit = limit
			}
		}

		// Parse offset
		if offsetStr := query.Get("offset"); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
				filter.Offset = offset
			}
		}

		// Parse date filters
		if fromStr := query.Get("acquisitionDateFrom"); fromStr != "" {
			if from, err := parseDate(fromStr); err == nil {
				filter.AcquisitionDateFrom = &from
			}
		}

		if toStr := query.Get("acquisitionDateTo"); toStr != "" {
			if to, err := parseDate(toStr); err == nil {
				filter.AcquisitionDateTo = &to
			}
		}

		assets, total, err := cfg.AssetService.QueryAssets(r.Context(), filter)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		resp := assetListResponse{
			Assets: make([]assetResponse, len(assets)),
			Total:  total,
			Limit:  filter.Limit,
			Offset: filter.Offset,
		}

		for i, asset := range assets {
			resp.Assets[i] = *convertAssetToResponse(&asset)
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// Helper functions

func convertAssetToResponse(asset *store.Asset) *assetResponse {
	resp := &assetResponse{
		AssetNo:            asset.AssetNo,
		TenantID:           asset.TenantID,
		Office:             asset.Office,
		Year:               asset.Year,
		CategoryNo:         asset.CategoryNo,
		CategoryName:       asset.CategoryName,
		SubCategoryNo:      asset.SubCategoryNo,
		SubCategoryName:    asset.SubCategoryName,
		TypeNumber:         asset.TypeNumber,
		TypeName:           asset.TypeName,
		SerialIncrement:    asset.SerialIncrement,
		Description1:       asset.Description1,
		Description2:       asset.Description2,
		UseOfAsset:         asset.UseOfAsset,
		Location:           asset.Location,
		Quantity:           asset.Quantity,
		Vendor:             asset.Vendor,
		Manufacturer:       asset.Manufacturer,
		IsPurchasedNew:     asset.IsPurchasedNew,
		AcquisitionDate:    asset.AcquisitionDate.Format("2006-01-02"),
		CountryOfOrigin:    asset.CountryOfOrigin,
		TypeBrand:          asset.TypeBrand,
		OriginalValue:      asset.OriginalValue,
		Currency:           asset.Currency,
		UsefulLifeYears:    asset.UsefulLifeYears,
		DepreciationMethod: asset.DepreciationMethod,
		ResidualValue:      asset.ResidualValue,
		BusinessArea:       asset.BusinessArea,
		CostCentre:         asset.CostCentre,
		CreationFormNo:     asset.CreationFormNo,
		Status:             asset.Status,
		BarcodeID:          asset.BarcodeID,
		SAPAssetMasterID:   asset.SAPAssetMasterID,
		Attributes:         asset.Attributes,
		CreatedAt:          asset.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          asset.UpdatedAt.Format(time.RFC3339),
		CreatedBy:          asset.CreatedBy,
	}

	if asset.UseCommenceDate != nil {
		resp.UseCommenceDate = asset.UseCommenceDate.Format("2006-01-02")
	}

	if asset.RequestedBy != nil {
		resp.RequestedBy = &assetApprovalResponse{
			UserID:      asset.RequestedBy.UserID,
			Name:        asset.RequestedBy.Name,
			Designation: asset.RequestedBy.Designation,
			Date:        asset.RequestedBy.Date.Format("2006-01-02"),
		}
	}

	if asset.AuthorizedBy != nil {
		resp.AuthorizedBy = &assetApprovalResponse{
			UserID:      asset.AuthorizedBy.UserID,
			Name:        asset.AuthorizedBy.Name,
			Designation: asset.AuthorizedBy.Designation,
			Date:        asset.AuthorizedBy.Date.Format("2006-01-02"),
		}
	}

	return resp
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

// assetHistoryHandler handles GET /v1/assets/{assetNo}/history
func assetHistoryHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		// Extract asset number from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
		path = strings.TrimSuffix(path, "/history")
		assetNo := strings.TrimSpace(path)

		if assetNo == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset number required"})
			return
		}

		history, err := cfg.AssetService.GetAssetHistory(r.Context(), tenantID, assetNo)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		resp := map[string]interface{}{
			"assetNo": assetNo,
			"history": history,
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetTransferRequest represents a transfer request.
type assetTransferRequest struct {
	ToLocation  string `json:"toLocation"`
	PerformedBy string `json:"performedBy"`
	Notes       string `json:"notes,omitempty"`
}

// assetTransferHandler handles POST /v1/assets/{assetNo}/transfer
func assetTransferHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		// Extract asset number from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
		path = strings.TrimSuffix(path, "/transfer")
		assetNo := strings.TrimSpace(path)

		if assetNo == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset number required"})
			return
		}

		var req assetTransferRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		if req.ToLocation == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "toLocation is required"})
			return
		}

		if req.PerformedBy == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "performedBy is required"})
			return
		}

		err := cfg.AssetService.TransferAsset(r.Context(), tenantID, assetNo, req.ToLocation, req.PerformedBy, req.Notes)
		if err != nil {
			if err.Error() == "asset not found" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		resp := map[string]interface{}{
			"assetNo":    assetNo,
			"status":     "transferred",
			"toLocation": req.ToLocation,
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetDisposeRequest represents a disposal request.
type assetDisposeRequest struct {
	Reason      string  `json:"reason"`
	PerformedBy string  `json:"performedBy"`
	SaleValue   float64 `json:"saleValue,omitempty"`
	Notes       string  `json:"notes,omitempty"`
}

// assetDisposeHandler handles POST /v1/assets/{assetNo}/dispose
func assetDisposeHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		// Extract asset number from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
		path = strings.TrimSuffix(path, "/dispose")
		assetNo := strings.TrimSpace(path)

		if assetNo == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset number required"})
			return
		}

		var req assetDisposeRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		if req.Reason == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reason is required"})
			return
		}

		if req.PerformedBy == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "performedBy is required"})
			return
		}

		err := cfg.AssetService.DisposeAsset(r.Context(), tenantID, assetNo, req.Reason, req.PerformedBy, req.SaleValue, req.Notes)
		if err != nil {
			if err.Error() == "asset not found" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		resp := map[string]interface{}{
			"assetNo": assetNo,
			"status":  "disposed",
			"reason":  req.Reason,
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetCategoriesHandler handles GET /v1/assets/categories
func assetCategoriesHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		if cfg.ClassificationService == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "classification service not available"})
			return
		}

		categories, err := cfg.ClassificationService.GetAllCategories(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		resp := map[string]interface{}{
			"categories": categories,
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// assetCategoryTypesHandler handles GET /v1/assets/categories/{catNo}/types
func assetCategoryTypesHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := TenantFromContext(r.Context())
		if !ok || strings.TrimSpace(tenantID) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tenant context missing"})
			return
		}

		if cfg.ClassificationService == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "classification service not available"})
			return
		}

		// Extract category number from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/assets/categories/")
		path = strings.TrimSuffix(path, "/types")
		catNo := strings.TrimSpace(path)

		if catNo == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category number required"})
			return
		}

		category, err := cfg.ClassificationService.GetCategory(r.Context(), catNo)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if category == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}

		resp := map[string]interface{}{
			"categoryNo":    category.Code,
			"categoryName":  category.Name,
			"subcategories": category.Subcategories,
		}

		writeJSON(w, http.StatusOK, resp)
	})
}
