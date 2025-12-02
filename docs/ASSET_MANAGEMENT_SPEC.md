# Asset Management Specification for KD-Server

## Overview

This document defines the asset management data model and business rules for KD-Server, based on:
- **IPSAS 17**: Property, Plant, and Equipment standards (Maldives Ministry of Finance)
- **MoF Asset Master Data Form**: Standard government asset registration format
- **NeelanPortal Classification**: Hierarchical asset type taxonomy

---

## Asset Number Format

### Structure
```
{office}-{year}-{catNo}-{typeNumber}-{serialIncrement}
```

### Example
```
B15-2024-02-073-0001
│   │    │   │    │
│   │    │   │    └─── Serial Increment (0001-9999)
│   │    │   └──────── Type Number (3-digit code from classification)
│   │    └──────────── Category Number (01-09, 290)
│   └───────────────── Year of Acquisition (YYYY)
└───────────────────── Office/Location Code
```

### Field Specifications

| Field | Format | Description | Example |
|-------|--------|-------------|---------|
| `office` | 2-4 alphanumeric | Office or location identifier | `B15`, `MLE`, `ADU` |
| `year` | YYYY | Year of acquisition | `2024` |
| `catNo` | 2 digits | Main category (01-09, 290) | `02` |
| `typeNumber` | 3 digits | Specific asset type from hierarchy | `073` |
| `serialIncrement` | 4 digits | Auto-incrementing serial per type | `0001` |

---

## Asset Classification Hierarchy

### Level 1: Main Categories

| Code | Name | Description |
|------|------|-------------|
| `01` | Land and Building | Land parcels, residential & non-residential buildings |
| `02` | Furniture, Fixtures and Fittings | Office, school, hospital furniture |
| `03` | Equipment | IT hardware, machinery, medical equipment |
| `04` | Vehicular Equipment and Vehicles | Cars, trucks, boats, construction equipment |
| `05` | Tools, Instrument and Apparatus | Hand tools, instruments, apparatus |
| `06` | Copy Rights and Patents | Licenses, intellectual property |
| `07` | Heritage Assets | Cultural, historical artifacts |
| `08` | Lagoons and Islands | Natural resources |
| `09` | Reference Books & Exhibition | Books, exhibition goods |
| `290` | Non Asset Inventory | Stock items not capitalized |

### Level 2: Subcategories (Examples)

#### Category 01 - Land and Building
- `10` - Land
- `11` - Residential Buildings
- `12` - Non Residential Buildings

#### Category 02 - Furniture, Fixtures and Fittings
- `13` - College Furniture
- `14` - Hospital and Health Center Furniture
- `15` - Office Furniture
- `16` - School Furniture
- `17` - Household Furniture
- `18` - Other Fittings

#### Category 03 - Equipment
- `19` - Hospital Equipment and Machinery
- `20` - Laboratory Equipment and Machinery
- `21` - Office Equipment and Machinery
- `22` - Communication Infrastructure
- `23` - IT-Related Hardware

#### Category 04 - Vehicular Equipment and Vehicles
- `24` - Vehicular Equipment
- `25` - Motor Vehicles
- `26` - Ships and Boats
- `27` - Other Vessels
- `28` - Aerospace Equipment

#### Category 05 - Tools, Instrument and Apparatus
- `29` - Apparatus
- `30` - Instruments
- `31` - Tools

### Level 3: Type Codes (Selected Examples)

#### Land Types (Subcategory 10)
- `035` - Agricultural
- `036` - Airports
- `046` - Offices
- `050` - Public Schools
- `052` - Residential Area

#### Office Furniture (Subcategory 15)
- `079` - Executive Chair
- `080` - High back chair
- `089` - Executive Table
- `090` - Conference Table
- `097` - Shelves/cupboards

#### IT Hardware (Subcategory 23)
- `150` - Computers
- `151` - Printers
- `152` - Projectors
- `155` - Servers
- `156` - Switches

#### Motor Vehicles (Subcategory 25)
- `198` - Buses
- `199` - Cars
- `203` - Pick up
- `204` - Trucks
- `205` - Vans

---

## Required Data Fields

### Core Identification
```json
{
  "assetNo": "B15-2024-02-073-0001",
  "creationFormNo": "MG/PR02/2024/0123",
  "tenantId": "moe",
  "businessArea": "Education",
  "costCentre": "CC001"
}
```

### Classification
```json
{
  "categoryNo": "02",
  "categoryName": "Furniture, Fixtures and Fittings",
  "subCategoryNo": "13",
  "subCategoryName": "College Furniture",
  "typeNumber": "073",
  "typeName": "Computer Table",
  "assetClass": "423001"
}
```

### General Information
```json
{
  "description1": "Computer Table",
  "description2": "Wooden desk with keyboard tray",
  "useOfAsset": "Student computer lab workstation",
  "location": "B15 - Hulhumale School Computer Lab",
  "quantity": 10
}
```

### Origin Information
```json
{
  "vendor": "Office Supplies Pvt Ltd",
  "manufacturer": "Local Furniture Co",
  "isPurchasedNew": true,
  "acquisitionDate": "2024-01-15",
  "useCommenceDate": "2024-02-01",
  "countryOfOrigin": "Maldives",
  "typeBrand": "Standard Computer Desk",
  "originalValue": 5000.00,
  "currency": "MVR"
}
```

### Depreciation Information
```json
{
  "usefulLifeYears": 10,
  "depreciationMethod": "straight-line",
  "residualValue": 500.00,
  "reasonForLifeChange": null
}
```

### Workflow/Approval
```json
{
  "requestedBy": {
    "userId": "USER001",
    "name": "Ahmed Ali",
    "designation": "Admin Officer",
    "date": "2024-01-10"
  },
  "authorizedBy": {
    "userId": "MGR001",
    "name": "Fatima Hassan",
    "designation": "Director",
    "date": "2024-01-12"
  },
  "sapAssetMasterId": "SAP001234"
}
```

### Audit Fields
```json
{
  "createdAt": "2024-01-15T10:00:00Z",
  "updatedAt": "2024-01-15T10:00:00Z",
  "createdBy": "USER001",
  "status": "active",
  "barcodeId": "BC-B15-2024-02-073-0001"
}
```

---

## Asset Number Generation Rules

### Auto-Increment Logic
1. Query existing assets with same `{office}-{year}-{catNo}-{typeNumber}` prefix
2. Find maximum `serialIncrement` value
3. Increment by 1, zero-pad to 4 digits
4. Validate uniqueness before commit

### Example Sequence
```
B15-2024-02-073-0001  ← First computer table in B15 for 2024
B15-2024-02-073-0002  ← Second computer table
B15-2024-02-073-0003  ← Third computer table
...
B15-2024-02-079-0001  ← First executive chair (different type)
```

### MongoDB Index Strategy
```javascript
// Compound index for efficient sequence generation
db.assets.createIndex({ 
  "office": 1, 
  "year": 1, 
  "categoryNo": 1, 
  "typeNumber": 1, 
  "serialIncrement": -1 
})

// Unique constraint on full asset number
db.assets.createIndex({ "assetNo": 1 }, { unique: true })

// Tenant isolation
db.assets.createIndex({ "tenantId": 1, "assetNo": 1 })
```

---

## Validation Rules

### Asset Number Validation
```regex
^[A-Z0-9]{2,4}-\d{4}-(0[1-9]|290)-\d{3}-\d{4}$
```

### Business Rules
1. **Office Code**: Must exist in tenant's office registry
2. **Year**: Cannot be future year; max 100 years in past
3. **Category**: Must be valid (01-09, 290)
4. **Type Number**: Must exist in classification hierarchy
5. **Serial**: Auto-generated, cannot be manually set
6. **Quantity**: Positive integer, generates multiple assets if > 1
7. **Original Value**: Must be > 0 for capitalized assets
8. **Acquisition Date**: Cannot be future date

---

## Lookup & Resolution

### Deterministic Match Keys
- **Asset Number**: Exact match on `assetNo`
- **Barcode**: Exact match on `barcodeId`
- **SAP ID**: Match on `sapAssetMasterId`

### Query Patterns
```javascript
// By asset number
GET /v1/lookup/asset/{assetNo}

// By office and category
GET /v1/assets?office=B15&categoryNo=02

// By type
GET /v1/assets?typeNumber=073

// By date range
GET /v1/assets?acquisitionDate[gte]=2024-01-01&acquisitionDate[lte]=2024-12-31
```

---

## Integration with Existing KD-Server

### Ingest Endpoint
```
POST /v1/ingest/assets
Content-Type: application/json
Authorization: Bearer {jwt}
X-KD-Tenant: {tenant}

{
  "source": {
    "slug": "neelan-portal",
    "name": "NeelanPortal Import"
  },
  "records": [
    {
      "office": "B15",
      "categoryNo": "02",
      "typeNumber": "073",
      "description1": "Computer Table",
      "quantity": 5,
      "originalValue": 5000.00,
      ...
    }
  ]
}
```

### Response
```json
{
  "created": 5,
  "assetNumbers": [
    "B15-2024-02-073-0001",
    "B15-2024-02-073-0002",
    "B15-2024-02-073-0003",
    "B15-2024-02-073-0004",
    "B15-2024-02-073-0005"
  ],
  "errors": []
}
```

---

## MongoDB Collections

### `assets` Collection
Core asset entities with tenant isolation.

```javascript
{
  _id: ObjectId,
  assetNo: "B15-2024-02-073-0001",
  tenantId: "moe",
  office: "B15",
  year: 2024,
  categoryNo: "02",
  subCategoryNo: "13",
  typeNumber: "073",
  serialIncrement: "0001",
  
  // General info
  description1: "Computer Table",
  description2: "Wooden desk",
  location: "Lab A",
  quantity: 1,
  
  // Financial
  originalValue: 5000.00,
  currency: "MVR",
  usefulLifeYears: 10,
  
  // Dates
  acquisitionDate: ISODate("2024-01-15"),
  createdAt: ISODate("2024-01-15T10:00:00Z"),
  updatedAt: ISODate("2024-01-15T10:00:00Z"),
  
  // Status
  status: "active",
  
  // Attributes (flexible)
  attributes: {}
}
```

### `asset_links` Collection
Links to source systems (similar to existing `links` for people).

```javascript
{
  _id: ObjectId,
  assetNo: "B15-2024-02-073-0001",
  tenantId: "moe",
  source: "sap-system",
  externalId: "SAP001234",
  payload: {
    // Original source data
  },
  createdAt: ISODate,
  updatedAt: ISODate
}
```

### `asset_history` Collection
Audit trail for transfers, disposals, revaluations.

```javascript
{
  _id: ObjectId,
  assetNo: "B15-2024-02-073-0001",
  tenantId: "moe",
  eventType: "transfer",
  fromLocation: "Lab A",
  toLocation: "Lab B",
  performedBy: "USER001",
  notes: "Relocated for renovation",
  timestamp: ISODate
}
```

---

## Phase 1 Implementation Tasks

### Part 14 - Asset Ingest Foundation
- [ ] Add `assets` collection with indexes
- [ ] Create asset number generator service
- [ ] Implement `POST /v1/ingest/assets` endpoint
- [ ] Add validation for category/type codes
- [ ] Support bulk creation for quantity > 1

### Part 15 - Asset Lookup & Query
- [ ] Implement `GET /v1/lookup/asset/{assetNo}`
- [ ] Add filtering by office, category, type
- [ ] Support date range queries
- [ ] Return linked source system data

### Part 16 - Asset Classification API
- [ ] Serve classification hierarchy as JSON
- [ ] `GET /v1/assets/categories` - list all categories
- [ ] `GET /v1/assets/categories/{catNo}/types` - list types
- [ ] Client apps can populate dropdowns

---

## Sample Office Codes

Common office location codes used in Maldives government:
- `MLE` - Male' City
- `ADU` - Addu City
- `FML` - Fuvahmulah
- `B15` - specific business unit/office
- `HDH` - Haa Dhaalu Atoll
- `THA` - Thaa Atoll

(Tenant-specific office registries managed per deployment)

---

## References

- **IPSAS 17 Policy**: `/docs/AssetMngPolicyMaldives.pdf`
- **MoF Asset Form**: `/docs/` (Asset Master Data Creation Form)
- **NeelanPortal**: https://github.com/kudadonbe/neelanPortal
- **Classification Source**: `neelanPortal/index.html`

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 0.1 | 2024-11-28 | Initial specification based on IPSAS, MoF, NeelanPortal |

---

*For implementation details, see roadmap Part 14-16 in `GUIDE.md`*
