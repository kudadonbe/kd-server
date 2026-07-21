# KD-Server API Usage

Integration reference for apps consuming kd-server (first consumer: **aqd**).
Covers the live `/v1` surface. Contracts are snake_case JSON. This doc reflects
the current server; planned endpoints are marked **(planned)**.

> Source of truth for schemas is meant to be `api/openapi.yaml`, but that file is
> currently a stale placeholder — treat this doc as the working reference until
> the spec is refreshed.

## Base URL

- Local dev: `http://localhost:18080`
- All application endpoints live under `/v1`.

## Authentication

Every route except `GET /v1/healthz` requires **two** headers:

| Header | Value |
|---|---|
| `Authorization` | `Bearer <credential>` |
| `X-KD-Tenant` | tenant slug, e.g. `fc` |

`<credential>` is either:
- a **tenant API key** — `key_<id>.<secret>`, issued from the admin console
  (server-side secret; never ship it in a browser bundle), or
- an **HS256 JWT** whose `tenant` claim equals the `X-KD-Tenant` header.

Browser-safe per-user auth (verify the app user's own IdP token, scoped) is
**(planned — aqd Part C)**; until then, calls are server-to-server.

**Auth failures**

| Situation | Status |
|---|---|
| Missing `X-KD-Tenant` | `400 {"error":"missing tenant header"}` |
| Missing/blank `Authorization` | `401 {"error":"missing authorization"}` |
| Bad API key | `401 {"error":"invalid API key"}` |
| JWT tenant ≠ header | `403 {"error":"tenant mismatch"}` |

## Conventions

- **Errors** always: `{"error":"<message>"}` with the matching HTTP status.
- **Content type**: send `application/json` (except the multipart extract route).
- Unknown JSON fields are rejected (`400 invalid payload`).

---

## Health & version

```
GET /v1/healthz            → 200 {"ok":true}          (no auth)
GET /v1/version            → 200 {"version":"1.0.0"}  (auth required)
```

---

## Entity search  (shared, multi-signal)

Find a person from any partial signal. Read-only (POST because the body is
structured and routinely carries Dhivehi/Thaana).

```
POST /v1/search
{
  "q":            "free text (name En/Dv, or any linked-record term)",
  "name":         "…",
  "national_id":  "A000001",
  "email":        "…",
  "phone":        "+960…",
  "date_of_birth":"1990-01-01",
  "island":       "Male",
  "limit":  20,      // default 20, max 100
  "offset": 0
}
→ 200
{
  "candidates": [
    {
      "person_id": "…",
      "score": 100,
      "matched_on": [ {"field":"national_id","value":"A000001","points":100} ],
      "summary": {
        "name": {"english":"…","dhivehi":"…"},
        "common_name": {"english":"…","dhivehi":"…"},
        "national_id": "A000001",
        "date_of_birth": "1990-01-01",
        "island": {"english":"…","dhivehi":"…"},
        "primary_email": "…",
        "primary_phone": "…",
        "sources": ["fc"]
      }
    }
  ],
  "total": 1, "limit": 20, "offset": 0
}
```

Scoring (additive, higher = stronger): national_id 100 · email 60 · phone 50 ·
dob 40 · name-exact 20 · island 15 · name-prefix 10 · linked-term 8. At least one
signal is required, else `400 {"error":"at least one search signal is required"}`.

**Reindex** (cold-start / drift repair; write-path keeps it current otherwise):

```
POST /v1/search/reindex    → 200 {"reindexed": <n>}
```

Name/Dhivehi search only returns for a person that has an identity document
(a bare ingest/resolve person has no name field — search it by identifiers).

---

## Identity-document extraction  (suggestion-only)

Read a Maldivian ID card/PDF and return suggested fields. **Nothing is stored**;
`Cache-Control: no-store`. AI vision first, Tesseract OCR fallback. The tenant is
resolved from auth, so per-tenant AI credentials apply (else the server default).

```
POST /v1/identity-documents/extract
Content-Type: multipart/form-data      // field "document", ≤10MB, PDF/JPEG/PNG
→ 200
{
  "engine": "anthropic:<model>" | "tesseract-5",
  "pages_processed": 1,
  "national_id": "A123456",
  "name_english": "…",   "name_dhivehi": "…",
  "sex": "M",            "date_of_birth": "1990-01-01",
  "house_english": "…",  "house_dhivehi": "…",
  "island_english":"…",  "island_dhivehi":"…",
  "common_name_english": "…",
  "blood_group": "…", "expiry_date": "…", "serial_number": "…",
  "field_confidence": { "national_id": 0.95, … },
  "warnings": [ "Review every field against the original document before saving." ],
  "raw_text": "…"
}
```

| Situation | Status |
|---|---|
| No file / wrong field name | `400 {"error":"document file is required"}` |
| Not PDF/JPEG/PNG or >10MB | `400 {"error":"…"}` |
| Extraction failed | `422 {"error":"…"}` |
| GET (wrong method) | `405` |

---

## Identity-document finalize & verify  **(planned — aqd Part D)**

The write path that makes kd-server the source of truth. Extraction is
suggestion-only; saving is a separate, reviewed step.

- `POST /v1/identity-documents` — resolve-or-create person, save a **versioned**
  identity document. Stored **tagged `unverified`** (user-asserted); the server
  owns `verification_status` — a client cannot self-assert verified.
- `POST /v1/identity-documents/{id}/verify` — **authorized role only**; promotes
  to `verified` and records `verified_by`. This is what search-first treats as
  authoritative.

---

## Data pipeline (ingest → resolve → lookup → review)

For bulk/source integrations that feed person records.

```
POST /v1/ingest
{ "source": {"slug":"fc","name":"Family Court"},
  "records": [ {"id":"ext-1","name":"…","national_id":"A000001",
                "email":"…","phone":"+960…","address":"…", …any fields…} ] }
→ 200 {"created":1,"linked":0,"needs_review":0}
```
Arbitrary record fields are preserved on the link payload and become searchable
(no schema change needed).

```
POST /v1/resolve            { "batch_size": 100 }
→ 200 {"resolved":1,"needs_review":0,"skipped":0}

GET  /v1/lookup/email/{email}
GET  /v1/lookup/phone/{e164}      // URL-encode '+' as %2B
→ 200 { "person": { "person_id":"…","national_id":"…",
                    "primary_email":"…","primary_phone":"…","attributes":{…} },
        "links": [ {"source":"fc","external_id":"…","payload":{…}} ] }

GET  /v1/review?status=needs_review
POST /v1/review/{id}/decision      { "decision": "accept" | "reject" }
```

---

## Per-tenant AI credentials  (optional)

A tenant can store its own encrypted provider key so extraction bills to it.
Requires `AI_ENCRYPTION_KEY` on the server.

```
POST   /v1/ai/credentials            { "api_key":"sk-ant-…", "model":"claude-sonnet-5" }
GET    /v1/ai/credentials            → masked list
DELETE /v1/ai/credentials/{provider} // e.g. anthropic
```

---

## Assets  (separate domain)

Asset tracking endpoints (IPSAS 17). See `ASSET_MANAGEMENT_SPEC.md` and
`ASSET_QUICK_REFERENCE.md`. Available when the asset service is wired:

```
POST /v1/ingest/assets
GET  /v1/lookup/asset/{assetNo}
GET  /v1/assets                     // filtered/paginated query
GET  /v1/assets/{assetNo}/history
POST /v1/assets/{assetNo}/transfer
POST /v1/assets/{assetNo}/dispose
GET  /v1/assets/categories
GET  /v1/assets/categories/{categoryNo}
```

---

## Security notes (binding)

- Never log documents, OCR text, extracted values, tokens, or raw PII.
- Extraction responses are **suggestions** — a human reviews before save.
- `verification_status` is server-owned; clients cannot self-assert `verified`.
- Tenant isolation is enforced at both auth (key is tenant-locked) and query
  (every search/lookup is tenant-scoped).

## Manual testing

Local `.rest` files (gitignored, hold real keys): `search.rest`, `extract.rest`,
`test.rest`, `ai.rest`. Copy the pattern for new endpoints.
