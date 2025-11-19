# KD-Server Client Integration Guide

This guide explains how to integrate with KD-Server as a client application or tenant.

---

## Overview

KD-Server is a multi-tenant entity resolution system. As a client/tenant, you:
1. **Ingest** raw records from your systems
2. **Resolve** records to unified person entities using deterministic matching
3. **Lookup** resolved persons by phone or email
4. **Review** records that need manual decisions

---

## Getting Started

### 1. Tenant Onboarding

Contact your KD-Server administrator to:
1. Create your tenant (you'll receive a **tenant slug**)
2. Issue an API key (you'll receive a **JWT token**)

You'll receive credentials in this format:
```env
KD_TENANT=your-tenant-slug
KD_API_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Store these securely** - never commit to version control.

### 2. Base Configuration

**API Base URL**: `http://your-kd-server:8080/v1` (or HTTPS in production)

**Required Headers** for all authenticated endpoints:
```http
Authorization: Bearer <your-jwt-token>
X-KD-Tenant: <your-tenant-slug>
Content-Type: application/json
```

---

## Authentication

### How Authentication Works

1. JWT token contains a `tenant` claim
2. `X-KD-Tenant` header must match the token's tenant claim
3. All data is isolated by tenant - you only see your own data

### Token Structure

Your JWT token is signed with HS256 and contains:
```json
{
  "tenant": "your-tenant-slug",
  "iat": 1234567890,
  "exp": 1234567890
}
```

### Testing Your Credentials

```bash
# Health check (no auth required)
curl http://localhost:8080/v1/healthz

# Version check (requires auth)
curl -H "Authorization: Bearer YOUR_TOKEN" \
     -H "X-KD-Tenant: YOUR_TENANT" \
     http://localhost:8080/v1/version
```

---

## API Endpoints

### 1. Health Check

**No authentication required**

```http
GET /v1/healthz
```

**Response** (200 OK):
```json
{
  "ok": true
}
```

---

### 2. Ingest Records

Import raw records from your source systems.

```http
POST /v1/ingest
```

**Request Body**:
```json
{
  "source": {
    "slug": "your-app-name",
    "name": "Your App Display Name"
  },
  "records": [
    {
      "id": "customer-123",
      "name": "Jane Doe",
      "email": "jane.doe@example.com",
      "phone": "+1234567890",
      "national_id": "ABC123456",
      "custom_field": "any additional data"
    }
  ]
}
```

**Important Fields**:
- `id` or `external_id` - Your system's unique identifier for this record
- `email` - Will be used for matching (lowercased, trimmed)
- `phone` - Should be E.164 format (e.g., `+1234567890`)
- `national_id` - National ID number for deterministic matching
- Any other fields will be stored in the payload

**Response** (200 OK):
```json
{
  "created": 10,
  "linked": 4,
  "needs_review": 1
}
```

**Response Fields**:
- `created` - New raw records stored
- `linked` - Records immediately linked to existing persons
- `needs_review` - Records that need manual review

**Error Responses**:
- `400` - Invalid payload
- `401` - Invalid/missing token
- `403` - Tenant mismatch

---

### 3. Resolve Records

Process pending records using deterministic matching rules.

```http
POST /v1/resolve
```

**Request Body** (optional):
```json
{
  "batch_size": 100
}
```

If no body provided, defaults to 100 records per batch.

**Matching Rules**:
1. If `national_id` matches → link to existing person
2. Else if `email` matches → link to existing person
3. Else if `phone` matches → link to existing person
4. Else if none match but at least one identifier present → create new person
5. Else if no identifiers → mark `needs_review`

**Response** (200 OK):
```json
{
  "resolved": 5,
  "needs_review": 1,
  "skipped": 0
}
```

**Response Fields**:
- `resolved` - Records successfully matched/created
- `needs_review` - Records lacking identifiers
- `skipped` - Records already processed

**Best Practice**: Run resolve after each ingest batch, or on a scheduled job.

---

### 4. Lookup by Phone

Retrieve a unified person record by phone number.

```http
GET /v1/lookup/phone/{e164}
```

**Path Parameter**:
- `e164` - Phone number in E.164 format (URL-encoded if needed)

**Example**:
```bash
GET /v1/lookup/phone/+1234567890
```

**Response** (200 OK):
```json
{
  "person": {
    "personId": "p_abc123",
    "tenantId": "your-tenant",
    "nationalId": "ABC123456",
    "primaryEmail": "jane.doe@example.com",
    "primaryPhone": "+1234567890",
    "attributes": {},
    "createdAt": "2025-11-20T10:00:00Z",
    "updatedAt": "2025-11-20T10:00:00Z"
  },
  "links": [
    {
      "source": "your-app-name",
      "externalId": "customer-123",
      "payload": {
        "id": "customer-123",
        "name": "Jane Doe",
        "email": "jane.doe@example.com"
      },
      "createdAt": "2025-11-20T10:00:00Z",
      "updatedAt": "2025-11-20T10:00:00Z"
    }
  ]
}
```

**Error Responses**:
- `404` - Person not found
- `401` - Invalid/missing token
- `403` - Tenant mismatch

---

### 5. Lookup by Email

Retrieve a unified person record by email.

```http
GET /v1/lookup/email/{email}
```

**Path Parameter**:
- `email` - Email address (URL-encoded)

**Example**:
```bash
GET /v1/lookup/email/jane.doe@example.com
```

**Response**: Same format as phone lookup (see above)

---

### 6. Review Queue

List records that need manual review (no identifiers).

```http
GET /v1/review?status=needs_review
```

**Query Parameters**:
- `status` - Filter by status (default: `needs_review`)

**Response** (200 OK):
```json
{
  "items": [
    {
      "id": "507f1f77bcf86cd799439011",
      "source_slug": "your-app-name",
      "payload": {
        "name": "John Smith",
        "address": "123 Main St"
      },
      "status": "needs_review"
    }
  ]
}
```

---

### 7. Review Decision

Submit a decision for a record in the review queue.

```http
POST /v1/review/{id}/decision
```

**Path Parameter**:
- `id` - MongoDB ObjectID of the raw record

**Request Body**:
```json
{
  "decision": "accept"
}
```

**Decision Values**:
- `"accept"` - Accept the record (manual processing)
- `"reject"` - Reject/skip the record

**Response** (200 OK):
```json
{
  "status": "updated"
}
```

**Note**: Current implementation is a stub - Phase 2 will add fuzzy matching and suggestions.

---

## Data Flow Examples

### Example 1: New Customer Onboarding

```javascript
// Step 1: Ingest new customer from your system
const ingestResult = await fetch('http://localhost:8080/v1/ingest', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'X-KD-Tenant': 'YOUR_TENANT',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    source: { slug: 'ecommerce', name: 'E-commerce Platform' },
    records: [{
      id: 'order-789',
      email: 'customer@example.com',
      phone: '+1234567890',
      name: 'John Customer',
      order_total: 299.99
    }]
  })
}).then(r => r.json());

console.log(ingestResult); // { created: 1, linked: 0, needs_review: 0 }

// Step 2: Resolve to link with existing person or create new
const resolveResult = await fetch('http://localhost:8080/v1/resolve', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'X-KD-Tenant': 'YOUR_TENANT',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ batch_size: 100 })
}).then(r => r.json());

console.log(resolveResult); // { resolved: 1, needs_review: 0, skipped: 0 }

// Step 3: Lookup the unified person record
const person = await fetch('http://localhost:8080/v1/lookup/email/customer@example.com', {
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'X-KD-Tenant': 'YOUR_TENANT'
  }
}).then(r => r.json());

console.log(person.person.personId); // p_abc123
console.log(person.links.length);    // Shows all systems where this person exists
```

---

### Example 2: Cross-System Lookup

Your customer service rep needs to see all records for a customer who called in:

```javascript
// Customer calls in with phone number
const phone = '+1234567890';

const result = await fetch(`http://localhost:8080/v1/lookup/phone/${encodeURIComponent(phone)}`, {
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'X-KD-Tenant': 'YOUR_TENANT'
  }
}).then(r => r.json());

// Now you have:
// - Unified person record (result.person)
// - All source system records (result.links)
//   - CRM record
//   - E-commerce order history
//   - Support ticket system records
//   - etc.

result.links.forEach(link => {
  console.log(`${link.source}: ${link.externalId}`);
  console.log(link.payload); // Full original record
});
```

---

## Best Practices

### 1. Identifier Quality

**Phone Numbers**:
- Use E.164 format: `+[country code][number]`
- Example: `+12125551234` (not `(212) 555-1234`)
- Validate format before ingesting

**Email Addresses**:
- Lowercase and trim before sending
- KD-Server will do this, but better to standardize early

**National IDs**:
- Use consistent format per country
- Remove spaces/dashes if your format varies

### 2. Batch Processing

- Ingest in batches of 100-1000 records
- Run resolve after each ingest batch
- Don't wait for resolve to complete before ingesting next batch

### 3. Error Handling

```javascript
try {
  const result = await kdApi.ingest(source, records);
} catch (error) {
  if (error.response) {
    // Server responded with error
    console.error('Status:', error.response.status);
    console.error('Error:', error.response.data.error);
  } else {
    // Network error
    console.error('Network error:', error.message);
  }
}
```

### 4. Rate Limiting

- No hard limits currently, but be respectful
- For bulk imports, consider throttling to ~10 requests/second
- Run large resolves in background jobs

### 5. Monitoring

Track these metrics:
- `ingest.created` - Total records ingested
- `ingest.needs_review` - Records lacking identifiers
- `resolve.resolved` - Successfully matched/created persons
- `lookup 404s` - Searches that found nothing

---

## Data Privacy & Security

### What KD-Server Stores

**Raw Collection**:
- Entire payload you send (including custom fields)
- Source information
- Status and timestamps

**People Collection**:
- `personId` (generated UUID)
- `nationalId`, `primaryEmail`, `primaryPhone` (identifiers)
- `attributes` (merged from all sources)

**Links Collection**:
- Mapping between `personId` and your `externalId`
- Copy of original payload for reference

### Tenant Isolation

- All queries are automatically scoped to your `tenantId`
- You cannot see other tenants' data
- MongoDB indexes enforce tenant boundaries

### PII Considerations

- KD-Server logs DO NOT contain PII (no emails, phones, IDs in logs)
- Logs contain: `tenantId`, `path`, `method`, `status`, `latency`
- Store API keys securely (environment variables, secrets manager)

---

## Troubleshooting

### "Missing tenant header" (400)

```bash
# Missing X-KD-Tenant header
✗ curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/version

# Fixed:
✓ curl -H "Authorization: Bearer TOKEN" \
       -H "X-KD-Tenant: your-tenant" \
       http://localhost:8080/v1/version
```

### "Invalid token" (401)

- Check token is not expired
- Verify token is signed with correct `JWT_SIGNING_KEY`
- Ensure `Bearer ` prefix in Authorization header

### "Tenant mismatch" (403)

- Token's `tenant` claim must match `X-KD-Tenant` header
- Contact admin to verify your tenant slug

### "Person not found" (404)

- Person hasn't been resolved yet - run `/v1/resolve`
- Identifier format mismatch (check phone E.164 format)
- Person exists under different identifier (try email if phone fails)

### CORS Errors (Browser Only)

If calling from browser, KD-Server may need CORS headers:
```go
// Server-side fix needed in kd-server
w.Header().Set("Access-Control-Allow-Origin", "https://your-ui-domain.com")
w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-KD-Tenant, Content-Type")
```

---

## Advanced Usage

### Scheduled Resolve Jobs

Run resolve on a schedule to continuously process pending records:

```bash
# Cron job example (every 5 minutes)
*/5 * * * * curl -X POST \
  -H "Authorization: Bearer $KD_API_KEY" \
  -H "X-KD-Tenant: $KD_TENANT" \
  -H "Content-Type: application/json" \
  -d '{"batch_size": 500}' \
  http://localhost:8080/v1/resolve
```

### Webhooks (Future)

Phase 2 may add webhooks for:
- New person created
- Person updated
- Review item needs attention

### Bulk Export (Future)

Phase 2 may add:
- `GET /v1/people` - List all persons
- `GET /v1/export` - Bulk export for analytics

---

## Getting Help

- **API Spec**: See `/api/openapi.yaml` in kd-server repo
- **Server Logs**: Check with your KD-Server admin
- **Issues**: Report bugs to kd-server maintainers

---

## Quick Reference

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/v1/healthz` | GET | No | Health check |
| `/v1/version` | GET | Yes | API version |
| `/v1/ingest` | POST | Yes | Import records |
| `/v1/resolve` | POST | Yes | Process pending records |
| `/v1/lookup/phone/{e164}` | GET | Yes | Find person by phone |
| `/v1/lookup/email/{email}` | GET | Yes | Find person by email |
| `/v1/review` | GET | Yes | List review queue |
| `/v1/review/{id}/decision` | POST | Yes | Submit review decision |

**Auth Headers**:
```
Authorization: Bearer <jwt>
X-KD-Tenant: <tenant-slug>
```
