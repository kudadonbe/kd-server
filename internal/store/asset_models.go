package store

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Asset represents a fixed asset in the assets collection.
type Asset struct {
	ID              primitive.ObjectID `bson:"_id"`
	AssetNo         string             `bson:"assetNo"`
	TenantID        string             `bson:"tenantId"`
	Office          string             `bson:"office"`
	Year            int                `bson:"year"`
	CategoryNo      string             `bson:"categoryNo"`
	CategoryName    string             `bson:"categoryName,omitempty"`
	SubCategoryNo   string             `bson:"subCategoryNo,omitempty"`
	SubCategoryName string             `bson:"subCategoryName,omitempty"`
	TypeNumber      string             `bson:"typeNumber"`
	TypeName        string             `bson:"typeName,omitempty"`
	SerialIncrement string             `bson:"serialIncrement"`

	Description1 string `bson:"description1"`
	Description2 string `bson:"description2,omitempty"`
	UseOfAsset   string `bson:"useOfAsset,omitempty"`
	Location     string `bson:"location"`
	Quantity     int    `bson:"quantity"`

	Vendor             string     `bson:"vendor,omitempty"`
	Manufacturer       string     `bson:"manufacturer,omitempty"`
	IsPurchasedNew     bool       `bson:"isPurchasedNew"`
	AcquisitionDate    time.Time  `bson:"acquisitionDate"`
	UseCommenceDate    *time.Time `bson:"useCommenceDate,omitempty"`
	CountryOfOrigin    string     `bson:"countryOfOrigin,omitempty"`
	TypeBrand          string     `bson:"typeBrand,omitempty"`
	OriginalValue      float64    `bson:"originalValue"`
	Currency           string     `bson:"currency"`
	UsefulLifeYears    int        `bson:"usefulLifeYears,omitempty"`
	DepreciationMethod string     `bson:"depreciationMethod,omitempty"`
	ResidualValue      float64    `bson:"residualValue,omitempty"`

	BusinessArea   string `bson:"businessArea,omitempty"`
	CostCentre     string `bson:"costCentre,omitempty"`
	CreationFormNo string `bson:"creationFormNo,omitempty"`

	RequestedBy      *AssetApproval `bson:"requestedBy,omitempty"`
	AuthorizedBy     *AssetApproval `bson:"authorizedBy,omitempty"`
	SAPAssetMasterID string         `bson:"sapAssetMasterId,omitempty"`

	Status     string         `bson:"status"`
	BarcodeID  string         `bson:"barcodeId,omitempty"`
	Attributes map[string]any `bson:"attributes,omitempty"`

	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
	CreatedBy string    `bson:"createdBy,omitempty"`
}

// AssetApproval represents approval workflow information.
type AssetApproval struct {
	UserID      string    `bson:"userId"`
	Name        string    `bson:"name"`
	Designation string    `bson:"designation"`
	Date        time.Time `bson:"date"`
}

// AssetLink represents a connection between an asset and a source system.
type AssetLink struct {
	ID         primitive.ObjectID `bson:"_id"`
	AssetNo    string             `bson:"assetNo"`
	TenantID   string             `bson:"tenantId"`
	Source     string             `bson:"source"`
	ExternalID string             `bson:"externalId"`
	Payload    map[string]any     `bson:"payload,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
}

// AssetHistory represents an audit trail event for an asset.
type AssetHistory struct {
	ID           primitive.ObjectID `bson:"_id"`
	AssetNo      string             `bson:"assetNo"`
	TenantID     string             `bson:"tenantId"`
	EventType    string             `bson:"eventType"`
	FromLocation string             `bson:"fromLocation,omitempty"`
	ToLocation   string             `bson:"toLocation,omitempty"`
	PerformedBy  string             `bson:"performedBy"`
	Notes        string             `bson:"notes,omitempty"`
	Timestamp    time.Time          `bson:"timestamp"`
}
