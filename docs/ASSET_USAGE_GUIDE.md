# KD-Server Asset Management - Getting Started Guide

This guide shows you how to use the KD-Server Asset Management API.

## Table of Contents

1. [Setup & Authentication](#setup--authentication)
2. [Asset Ingestion](#asset-ingestion)
3. [Asset Lookup](#asset-lookup)
4. [Asset Queries](#asset-queries)
5. [Classification API](#classification-api)
6. [Asset Lifecycle](#asset-lifecycle)
7. [Complete Examples](#complete-examples)

---

## Setup & Authentication

### 1. Start the Server

```bash
# Set environment variables
export MONGO_URI=mongodb://localhost:27017
export MONGO_DB=kdserver
export JWT_SIGNING_KEY=your-secret-key-here
export PORT=8080

# Run the API server
make run
# OR
./cmd/api/main
```

### 2. Create a Tenant (via TUI)

```bash
# Start the admin TUI
make tui

# Select option 2 to create a tenant
# Example:
#   Name: Ministry of Education
#   Slug: moe
```

### 3. Get an API Key (via TUI)

```bash
# In the TUI, select option 3 to issue an API key
# Save the secret key - you'll need it for authentication
```

### 4. Set Your Credentials

```bash
# Add to your .env file
KD_TENANT=moe
KD_API_KEY=the_secret_from_step_3
```

---

## Asset Ingestion

### Basic Asset Creation

**Endpoint:** `POST /v1/ingest/assets`

**Request:**
```bash
curl -X POST http://localhost:8080/v1/ingest/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d '{
    "source": {
      "slug": "neelan-portal",
      "name": "NeelanPortal"
    },
    "records": [
      {
        "office": "B15",
        "categoryNo": "02",
        "typeNumber": "073",
        "description1": "Computer Table - Standard",
        "location": "Computer Lab A",
        "originalValue": 5000.00,
        "acquisitionDate": "2024-01-15",
        "isPurchasedNew": true,
        "quantity": 1
      }
    ]
  }'
```

**Response:**
```json
{
  "created": 1,
  "assetNumbers": [
    "B15-2024-02-073-0001"
  ],
  "errors": []
}
```

### Bulk Asset Creation

Create 5 identical items at once using the `quantity` field:

```bash
curl -X POST http://localhost:8080/v1/ingest/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d '{
    "source": {
      "slug": "procurement-system"
    },
    "records": [
      {
        "office": "B15",
        "categoryNo": "02",
        "subCategoryNo": "15",
        "typeNumber": "082",
        "description1": "Office Chair - Ergonomic",
        "location": "Office Floor 2",
        "vendor": "Furniture World Pvt Ltd",
        "manufacturer": "ErgoComfort",
        "originalValue": 3500.00,
        "currency": "MVR",
        "acquisitionDate": "2024-11-15",
        "isPurchasedNew": true,
        "usefulLifeYears": 5,
        "quantity": 5
      }
    ]
  }'
```

**Response:**
```json
{
  "created": 5,
  "assetNumbers": [
    "B15-2024-02-082-0001",
    "B15-2024-02-082-0002",
    "B15-2024-02-082-0003",
    "B15-2024-02-082-0004",
    "B15-2024-02-082-0005"
  ],
  "errors": []
}
```

### Complete Asset Record

Here's an example with all available fields:

```json
{
  "source": {
    "slug": "sap-system",
    "name": "SAP Asset Management"
  },
  "records": [
    {
      "office": "B15",
      "categoryNo": "03",
      "subCategoryNo": "23",
      "typeNumber": "150",
      "description1": "Dell OptiPlex 7090 Desktop",
      "description2": "Intel i7, 16GB RAM, 512GB SSD",
      "useOfAsset": "General Office Work",
      "location": "IT Department - Floor 3",
      "quantity": 1,
      "vendor": "Tech Solutions Maldives",
      "manufacturer": "Dell Inc.",
      "isPurchasedNew": true,
      "acquisitionDate": "2024-11-20",
      "useCommenceDate": "2024-11-25",
      "countryOfOrigin": "USA",
      "typeBrand": "Dell",
      "originalValue": 25000.00,
      "currency": "MVR",
      "usefulLifeYears": 5,
      "depreciationMethod": "straight-line",
      "residualValue": 2500.00,
      "businessArea": "Information Technology",
      "costCentre": "IT-DEPT-001",
      "creationFormNo": "AF-2024-001234",
      "requestedBy": {
        "userId": "USER001",
        "name": "Ahmed Ali",
        "designation": "IT Manager",
        "date": "2024-11-15"
      },
      "authorizedBy": {
        "userId": "MGR001",
        "name": "Fatima Hassan",
        "designation": "Director General",
        "date": "2024-11-18"
      },
      "sapAssetMasterId": "SAP-10001234",
      "attributes": {
        "serialNumber": "CN-0123456789",
        "warrantyExpiry": "2027-11-20",
        "serviceTag": "DELL-SVC-7090-001"
      }
    }
  ]
}
```

---

## Asset Lookup

### Get Single Asset

**Endpoint:** `GET /v1/lookup/asset/{assetNo}`

```bash
curl -X GET http://localhost:8080/v1/lookup/asset/B15-2024-02-073-0001 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

**Response:**
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
    "description1": "Computer Table - Standard",
    "location": "Computer Lab A",
    "quantity": 1,
    "isPurchasedNew": true,
    "acquisitionDate": "2024-01-15",
    "originalValue": 5000.00,
    "currency": "MVR",
    "status": "active",
    "barcodeId": "BC-B15-2024-02-073-0001",
    "createdAt": "2024-11-28T22:30:00Z",
    "updatedAt": "2024-11-28T22:30:00Z",
    "createdBy": "system"
  },
  "links": [
    {
      "source": "neelan-portal",
      "externalId": "NP-12345",
      "payload": {},
      "createdAt": "2024-11-28T22:30:00Z",
      "updatedAt": "2024-11-28T22:30:00Z"
    }
  ]
}
```

---

## Asset Queries

### Query All Assets

**Endpoint:** `GET /v1/assets`

```bash
curl -X GET "http://localhost:8080/v1/assets?limit=10&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Filter by Office

```bash
curl -X GET "http://localhost:8080/v1/assets?office=B15&limit=20" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Filter by Category

```bash
curl -X GET "http://localhost:8080/v1/assets?categoryNo=02" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Filter by Asset Type

```bash
curl -X GET "http://localhost:8080/v1/assets?categoryNo=02&typeNumber=073" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Filter by Status

```bash
curl -X GET "http://localhost:8080/v1/assets?status=active" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Date Range Query

```bash
curl -X GET "http://localhost:8080/v1/assets?acquisitionDateFrom=2024-01-01&acquisitionDateTo=2024-12-31" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

### Combined Filters with Pagination

```bash
curl -X GET "http://localhost:8080/v1/assets?office=B15&categoryNo=02&status=active&limit=50&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

**Response:**
```json
{
  "assets": [
    {
      "assetNo": "B15-2024-02-073-0001",
      "office": "B15",
      "categoryNo": "02",
      "categoryName": "Furniture, Fixtures and Fittings",
      "typeName": "Computer Table",
      "description1": "Computer Table - Standard",
      "status": "active",
      "originalValue": 5000.00,
      "location": "Computer Lab A"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

---

## Classification API

### Get All Categories

**Endpoint:** `GET /v1/assets/categories`

```bash
curl -X GET http://localhost:8080/v1/assets/categories \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

**Response:**
```json
{
  "categories": [
    {
      "code": "01",
      "name": "Land and Building",
      "description": "Land parcels, residential & non-residential buildings",
      "subcategories": [...]
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
            {"code": "073", "name": "Computer Table"},
            {"code": "074", "name": "Desk / Table"},
            {"code": "075", "name": "Chair"}
          ]
        }
      ]
    }
  ]
}
```

### Get Types for a Category

**Endpoint:** `GET /v1/assets/categories/{catNo}/types`

```bash
curl -X GET http://localhost:8080/v1/assets/categories/02/types \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

**Response:**
```json
{
  "categoryNo": "02",
  "categoryName": "Furniture, Fixtures and Fittings",
  "subcategories": [
    {
      "code": "13",
      "name": "College Furniture",
      "types": [
        {"code": "073", "name": "Computer Table"},
        {"code": "074", "name": "Desk / Table"},
        {"code": "075", "name": "Chair"},
        {"code": "076", "name": "Bookshelves / Rack (cupboard)"}
      ]
    },
    {
      "code": "15",
      "name": "Office Furniture",
      "types": [
        {"code": "079", "name": "Executive Chair"},
        {"code": "082", "name": "Office Chair"},
        {"code": "089", "name": "Executive Table"}
      ]
    }
  ]
}
```

---

## Asset Lifecycle

### Get Asset History

**Endpoint:** `GET /v1/assets/{assetNo}/history`

```bash
curl -X GET http://localhost:8080/v1/assets/B15-2024-02-073-0001/history \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe"
```

**Response:**
```json
{
  "assetNo": "B15-2024-02-073-0001",
  "history": [
    {
      "assetNo": "B15-2024-02-073-0001",
      "tenantId": "moe",
      "eventType": "created",
      "timestamp": "2024-11-28T22:30:00Z",
      "performedBy": "system",
      "notes": "Asset created via API"
    },
    {
      "assetNo": "B15-2024-02-073-0001",
      "tenantId": "moe",
      "eventType": "transfer",
      "fromLocation": "Computer Lab A",
      "toLocation": "Computer Lab B",
      "timestamp": "2024-11-29T10:15:00Z",
      "performedBy": "USER001",
      "notes": "Lab reorganization"
    }
  ]
}
```

### Transfer Asset

**Endpoint:** `POST /v1/assets/{assetNo}/transfer`

```bash
curl -X POST http://localhost:8080/v1/assets/B15-2024-02-073-0001/transfer \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d '{
    "toLocation": "Computer Lab B",
    "performedBy": "USER001",
    "notes": "Lab reorganization"
  }'
```

**Response:**
```json
{
  "assetNo": "B15-2024-02-073-0001",
  "status": "transferred",
  "toLocation": "Computer Lab B"
}
```

### Dispose Asset

**Endpoint:** `POST /v1/assets/{assetNo}/dispose`

```bash
curl -X POST http://localhost:8080/v1/assets/B15-2024-02-073-0001/dispose \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d '{
    "reason": "Damaged beyond repair",
    "performedBy": "MGR001",
    "saleValue": 0,
    "notes": "Water damage from ceiling leak"
  }'
```

**Response:**
```json
{
  "assetNo": "B15-2024-02-073-0001",
  "status": "disposed",
  "reason": "Damaged beyond repair"
}
```

---

## Complete Examples

### Example 1: Import Assets from Excel/CSV

After converting your Excel/CSV to JSON:

```bash
curl -X POST http://localhost:8080/v1/ingest/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-KD-Tenant: moe" \
  -d @assets.json
```

**assets.json:**
```json
{
  "source": {
    "slug": "excel-import-2024",
    "name": "Annual Asset Import 2024"
  },
  "records": [
    {
      "office": "B15",
      "categoryNo": "02",
      "typeNumber": "073",
      "description1": "Computer Table 1",
      "location": "Room 101",
      "originalValue": 5000.00,
      "acquisitionDate": "2024-01-15",
      "isPurchasedNew": true
    },
    {
      "office": "B15",
      "categoryNo": "02",
      "typeNumber": "082",
      "description1": "Office Chair 1",
      "location": "Room 101",
      "originalValue": 3500.00,
      "acquisitionDate": "2024-01-15",
      "isPurchasedNew": true
    }
  ]
}
```

### Example 2: Monthly Asset Report

```bash
#!/bin/bash

# Get all assets acquired in November 2024
curl -X GET "http://localhost:8080/v1/assets?acquisitionDateFrom=2024-11-01&acquisitionDateTo=2024-11-30&limit=1000" \
  -H "Authorization: Bearer $KD_API_KEY" \
  -H "X-KD-Tenant: $KD_TENANT" \
  -o november_assets.json

# Count by category
cat november_assets.json | jq '.assets | group_by(.categoryNo) | map({category: .[0].categoryName, count: length})'
```

### Example 3: Asset Lifecycle Workflow

```bash
#!/bin/bash

ASSET_NO="B15-2024-02-073-0001"
TOKEN="YOUR_JWT_TOKEN"
TENANT="moe"

# Step 1: Create asset
curl -X POST http://localhost:8080/v1/ingest/assets \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-KD-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"source":{"slug":"test"},"records":[{"office":"B15","categoryNo":"02","typeNumber":"073","description1":"Test Asset","location":"Lab A","originalValue":5000,"acquisitionDate":"2024-01-15","isPurchasedNew":true}]}'

# Step 2: Transfer after 6 months
curl -X POST http://localhost:8080/v1/assets/$ASSET_NO/transfer \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-KD-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"toLocation":"Lab B","performedBy":"USER001","notes":"Department relocation"}'

# Step 3: Check history
curl -X GET http://localhost:8080/v1/assets/$ASSET_NO/history \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-KD-Tenant: $TENANT"

# Step 4: Dispose after damage
curl -X POST http://localhost:8080/v1/assets/$ASSET_NO/dispose \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-KD-Tenant: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"reason":"Fire damage","performedBy":"MGR001","saleValue":0}'
```

---

## Common Asset Categories & Types

Here are the most commonly used categories:

| Code | Category | Example Types |
|------|----------|---------------|
| 02 | Furniture | Computer Table (073), Office Chair (082), Executive Table (089) |
| 03 | Equipment | Computers (150), Printers (151), Air Conditioner (122) |
| 04 | Vehicles | Cars (199), Motorcycles (202), Boats (207) |

**Full classification available at:** `GET /v1/assets/categories`

---

## Error Handling

### Common Errors

**400 Bad Request** - Invalid input
```json
{
  "error": "invalid asset number format: XYZ"
}
```

**401 Unauthorized** - Missing or invalid token
```json
{
  "error": "invalid token"
}
```

**404 Not Found** - Asset doesn't exist
```json
{
  "error": "asset not found"
}
```

### Validation Errors

When ingesting assets, validation errors are returned per record:

```json
{
  "created": 2,
  "assetNumbers": ["B15-2024-02-073-0001", "B15-2024-02-073-0002"],
  "errors": [
    {
      "recordIndex": 2,
      "field": "originalValue",
      "message": "original value must be greater than 0"
    }
  ]
}
```

---

## Tips & Best Practices

1. **Use Batch Imports**: Use the `quantity` field or batch multiple records for efficiency
2. **Include Source Info**: Always specify source system for traceability
3. **Validate Dates**: Use ISO 8601 format (YYYY-MM-DD)
4. **Check Categories**: Use the classification API to get valid codes
5. **Track History**: Use the history endpoint to audit asset lifecycle
6. **Filter Smartly**: Combine filters to reduce response size
7. **Paginate**: Use limit/offset for large result sets

---

## Next Steps

- Review the [Asset Management Specification](./ASSET_MANAGEMENT_SPEC.md)
- Check the [Quick Reference](./ASSET_QUICK_REFERENCE.md)
- See more [API Examples](../api/ASSET_API_EXAMPLES.md)
- Read the [OpenAPI Specification](../api/openapi.yaml)

---

## Support

For issues or questions:
- Review the documentation in `docs/`
- Check API examples in `api/`
- Refer to IPSAS 17 policy: `docs/AssetMngPolicyMaldives.pdf`
