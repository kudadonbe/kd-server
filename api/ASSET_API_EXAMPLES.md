# Asset Management API Examples

## Base URL
```
http://localhost:8080/v1
```

## Authentication
All endpoints require:
- **Authorization Header**: `Bearer {jwt-token}`
- **Tenant Header**: `X-KD-Tenant: {tenant-slug}`

---

## 1. Ingest Assets

### Endpoint
```
POST /v1/ingest/assets
```

### Request Example
```json
{
  "source": {
    "slug": "neelan-portal",
    "name": "NeelanPortal Import"
  },
  "records": [
    {
      "office": "B15",
      "categoryNo": "02",
      "subCategoryNo": "13",
      "typeNumber": "073",
      "description1": "Computer Table",
      "description2": "Wooden desk with keyboard tray",
      "useOfAsset": "Student computer lab workstation",
      "location": "B15 - Hulhumale School Computer Lab",
      "quantity": 5,
      "vendor": "Office Supplies Pvt Ltd",
      "manufacturer": "Local Furniture Co",
      "isPurchasedNew": true,
      "acquisitionDate": "2024-01-15",
      "useCommenceDate": "2024-02-01",
      "countryOfOrigin": "Maldives",
      "typeBrand": "Standard Computer Desk",
      "originalValue": 5000.00,
      "currency": "MVR",
      "usefulLifeYears": 10,
      "depreciationMethod": "straight-line",
      "residualValue": 500.00,
      "businessArea": "Education",
      "costCentre": "CC001",
      "creationFormNo": "MG/PR02/2024/0123",
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
      }
    }
  ]
}
```

### Response Example
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

### curl Example
```bash
curl -X POST http://localhost:8080/v1/ingest/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d @asset_ingest.json
```

---

## 2. Lookup Asset by Number

### Endpoint
```
GET /v1/lookup/asset/{assetNo}
```

### Request Example
```
GET /v1/lookup/asset/B15-2024-02-073-0001
```

### Response Example
```json
{
  "asset": {
    "assetNo": "B15-2024-02-073-0001",
    "tenantId": "moe",
    "office": "B15",
    "year": 2024,
    "categoryNo": "02",
    "categoryName": "Furniture, Fixtures and Fittings",
    "subCategoryNo": "13",
    "subCategoryName": "College Furniture",
    "typeNumber": "073",
    "typeName": "Computer Table",
    "serialIncrement": "0001",
    "description1": "Computer Table",
    "description2": "Wooden desk with keyboard tray",
    "useOfAsset": "Student computer lab workstation",
    "location": "B15 - Hulhumale School Computer Lab",
    "quantity": 1,
    "vendor": "Office Supplies Pvt Ltd",
    "manufacturer": "Local Furniture Co",
    "isPurchasedNew": true,
    "acquisitionDate": "2024-01-15",
    "useCommenceDate": "2024-02-01",
    "countryOfOrigin": "Maldives",
    "typeBrand": "Standard Computer Desk",
    "originalValue": 5000.00,
    "currency": "MVR",
    "usefulLifeYears": 10,
    "depreciationMethod": "straight-line",
    "residualValue": 500.00,
    "businessArea": "Education",
    "costCentre": "CC001",
    "creationFormNo": "MG/PR02/2024/0123",
    "status": "active",
    "barcodeId": "BC-B15-2024-02-073-0001",
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
    "createdAt": "2024-01-15T10:00:00Z",
    "updatedAt": "2024-01-15T10:00:00Z",
    "createdBy": "USER001"
  },
  "links": [
    {
      "source": "neelan-portal",
      "externalId": "NP-2024-001",
      "payload": {},
      "createdAt": "2024-01-15T10:00:00Z",
      "updatedAt": "2024-01-15T10:00:00Z"
    }
  ]
}
```

### curl Example
```bash
curl -X GET http://localhost:8080/v1/lookup/asset/B15-2024-02-073-0001 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

---

## 3. Query Assets with Filters

### Endpoint
```
GET /v1/assets
```

### Request Examples

#### By Office
```
GET /v1/assets?office=B15&limit=20&offset=0
```

#### By Category
```
GET /v1/assets?categoryNo=02
```

#### By Type
```
GET /v1/assets?typeNumber=073
```

#### By Date Range
```
GET /v1/assets?acquisitionDateFrom=2024-01-01&acquisitionDateTo=2024-12-31
```

#### Combined Filters
```
GET /v1/assets?office=B15&categoryNo=02&status=active&limit=50
```

### Response Example
```json
{
  "assets": [
    {
      "assetNo": "B15-2024-02-073-0001",
      "categoryName": "Furniture, Fixtures and Fittings",
      "typeName": "Computer Table",
      "description1": "Computer Table",
      "location": "B15 - Hulhumale School Computer Lab",
      "originalValue": 5000.00,
      "currency": "MVR",
      "status": "active",
      "acquisitionDate": "2024-01-15",
      "createdAt": "2024-01-15T10:00:00Z"
    },
    {
      "assetNo": "B15-2024-02-073-0002",
      "categoryName": "Furniture, Fixtures and Fittings",
      "typeName": "Computer Table",
      "description1": "Computer Table",
      "location": "B15 - Hulhumale School Computer Lab",
      "originalValue": 5000.00,
      "currency": "MVR",
      "status": "active",
      "acquisitionDate": "2024-01-15",
      "createdAt": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 150,
  "limit": 20,
  "offset": 0
}
```

### curl Example
```bash
curl -X GET "http://localhost:8080/v1/assets?office=B15&categoryNo=02&limit=20" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

---

## 4. Get Asset Categories

### Endpoint
```
GET /v1/assets/categories
```

### Response Example
```json
{
  "categories": [
    {
      "code": "01",
      "name": "Land and Building",
      "description": "Land parcels, residential & non-residential buildings",
      "subcategories": [
        {
          "code": "10",
          "name": "Land",
          "types": [
            {
              "code": "035",
              "name": "Agricultural"
            },
            {
              "code": "036",
              "name": "Airports"
            },
            {
              "code": "046",
              "name": "Offices"
            }
          ]
        },
        {
          "code": "11",
          "name": "Residential Buildings",
          "types": [
            {
              "code": "061",
              "name": "Flats"
            },
            {
              "code": "062",
              "name": "Other Residential Buildings"
            }
          ]
        }
      ]
    },
    {
      "code": "02",
      "name": "Furniture, Fixtures and Fittings",
      "description": "Office, school, hospital furniture",
      "subcategories": [
        {
          "code": "13",
          "name": "College Furniture",
          "types": [
            {
              "code": "073",
              "name": "Computer Table"
            },
            {
              "code": "074",
              "name": "Desk / Table"
            },
            {
              "code": "075",
              "name": "Chair"
            }
          ]
        }
      ]
    }
  ]
}
```

### curl Example
```bash
curl -X GET http://localhost:8080/v1/assets/categories \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

---

## 5. Get Types for Specific Category

### Endpoint
```
GET /v1/assets/categories/{catNo}/types
```

### Request Example
```
GET /v1/assets/categories/02/types
```

### Response Example
```json
{
  "categoryNo": "02",
  "categoryName": "Furniture, Fixtures and Fittings",
  "subcategories": [
    {
      "code": "13",
      "name": "College Furniture",
      "types": [
        {
          "code": "073",
          "name": "Computer Table"
        },
        {
          "code": "074",
          "name": "Desk / Table"
        },
        {
          "code": "075",
          "name": "Chair"
        },
        {
          "code": "076",
          "name": "Bookshelves / Rack (cupboard)"
        }
      ]
    },
    {
      "code": "15",
      "name": "Office Furniture",
      "types": [
        {
          "code": "079",
          "name": "Executive Chair"
        },
        {
          "code": "089",
          "name": "Executive Table"
        },
        {
          "code": "090",
          "name": "Conference Table"
        }
      ]
    }
  ]
}
```

### curl Example
```bash
curl -X GET http://localhost:8080/v1/assets/categories/02/types \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid category number: must be 01-09 or 290"
}
```

### 401 Unauthorized
```json
{
  "error": "Missing or invalid bearer token"
}
```

### 403 Forbidden
```json
{
  "error": "Tenant mismatch: token tenant does not match X-KD-Tenant header"
}
```

### 404 Not Found
```json
{
  "error": "Asset not found: B15-2024-02-073-9999"
}
```

---

## Validation Rules

### Asset Number Format
```regex
^[A-Z0-9]{2,4}-\d{4}-(0[1-9]|290)-\d{3}-\d{4}$
```

Examples:
- ✅ `B15-2024-02-073-0001`
- ✅ `MLE-2024-04-199-0042`
- ✅ `ADU-2023-290-001-0001`
- ❌ `B1-2024-02-073-0001` (office too short)
- ❌ `B15-2024-10-073-0001` (invalid category)
- ❌ `B15-2024-02-73-0001` (type not 3 digits)

### Required Fields
- `office` (2-4 alphanumeric)
- `categoryNo` (01-09 or 290)
- `typeNumber` (3 digits)
- `description1` (max 200 chars)
- `originalValue` (positive number)
- `acquisitionDate` (ISO 8601 date, not future)
- `location` (max 200 chars)

### Optional but Recommended
- `subCategoryNo`
- `description2`
- `vendor`
- `manufacturer`
- `usefulLifeYears`
- `requestedBy` / `authorizedBy`

---

## Testing with requests.rest

Add to `/requests.rest`:

```http
### Ingest Assets
POST http://localhost:8080/v1/ingest/assets
Content-Type: application/json
Authorization: Bearer {{jwt_token}}
X-KD-Tenant: {{tenant}}

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
      "originalValue": 5000.00,
      "acquisitionDate": "2024-01-15",
      "location": "Lab A",
      "quantity": 1
    }
  ]
}

### Lookup Asset
GET http://localhost:8080/v1/lookup/asset/B15-2024-02-073-0001
Authorization: Bearer {{jwt_token}}
X-KD-Tenant: {{tenant}}

### Query Assets
GET http://localhost:8080/v1/assets?office=B15&categoryNo=02
Authorization: Bearer {{jwt_token}}
X-KD-Tenant: {{tenant}}

### Get Categories
GET http://localhost:8080/v1/assets/categories
Authorization: Bearer {{jwt_token}}
X-KD-Tenant: {{tenant}}

### Get Category Types
GET http://localhost:8080/v1/assets/categories/02/types
Authorization: Bearer {{jwt_token}}
X-KD-Tenant: {{tenant}}
```

---

For complete API specification, see `/api/openapi.yaml`
