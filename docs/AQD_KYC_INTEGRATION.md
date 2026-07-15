# AQD KYC Integration — Build Notes

What kd-server must provide so **aqd** (Family Court Maldives, first consumer)
and other client apps can use the identity-document extraction service to help
users fill KYC forms. Written 2026-07-13; aligns with
`KYC_ENTITY_HUB_ROADMAP.md` (Phases 2–4). One PR per part.

## The consumer flow this enables

1. A signed-in aqd user opens their profile KYC card and uploads a photo/PDF
   of their Maldivian ID card (front, optionally back).
2. The browser posts the file to kd-server's extract endpoint.
3. kd-server returns suggested fields (AI vision first, Tesseract fallback —
   already implemented, suggestion-only, nothing stored).
4. The aqd UI prefills its KYC form; the **user reviews and saves**. Saving
   stays in the client app (aqd currently saves to its own Firestore user
   doc); kd-server extraction remains stateless.
5. Later (Phase 3/4): aqd calls finalize/verify and kd-server becomes the
   KYC source of truth instead of the app's own store.

## What already exists (do not rebuild)

- `services.AIDocumentExtractor` → Claude vision with Maldivian-ID schema,
  falls back to `LocalDocumentExtractor` (Tesseract eng+div, Poppler for PDF).
  Returns `DocumentExtraction` (snake_case JSON): `national_id`,
  `name_english`, `name_dhivehi`, `sex`, `date_of_birth`, `house_english`,
  `house_dhivehi`, `island_english`, `island_dhivehi`, `common_name_english`,
  `blood_group`, `expiry_date`, `serial_number`, `field_confidence`,
  `warnings`, `raw_text`.
- Admin-only HTTP surface: `POST /admin/api/identity-documents/extract`
  (multipart field `document`, 10 MB cap, PDF/JPEG/PNG sniffed via
  `http.DetectContentType`, `422` on extraction failure, `Cache-Control:
  no-store`). See `internal/http/admin.go`.
- Per-tenant AI credentials (`/v1/ai/credentials`, encrypted at rest with
  `AI_ENCRYPTION_KEY`); wiring in `cmd/api/main.go` composes AI + OCR
  fallback into `Config.DocumentExtractor`.
- `/v1` auth: `Authorization: Bearer <api-key | HS256 JWT>` +
  `X-KD-Tenant` header.

## Part A — Tenant-facing extract endpoint  (Phase 3, first checkbox)

`POST /v1/identity-documents/extract`

- Thin mirror of the admin extract case, behind `authMiddleware`. Extract
  only: no person lookup, no save, no raw-text persistence.
- Same multipart contract as admin (`document` field, 10 MB, PDF/JPEG/PNG),
  same `DocumentExtraction` response, same `no-store` header.
- Resolve the tenant from auth context and pass it to the AI service so
  per-tenant AI credentials (and later per-tenant quotas) apply. Note the
  current `AIDocumentExtractor.Extract` hardcodes `tenantID: ""` — thread the
  tenant through when adding this route.
- Rate-limit per tenant *and* per end-user identity (see Part C) — vision
  calls cost real money. Suggested: small token bucket, `429` with
  `Retry-After`.
- Add contract to `api/openapi.yaml` (currently stale; roadmap already flags
  this).

## Part B — CORS for browser tenants

aqd is a **static, client-only** web app (Firebase Hosting; no backend of its
own yet). The browser calls kd-server directly, so `/v1` needs CORS:

- Per-tenant allowed-origins list (tenant config in Mongo, editable in the
  admin console), e.g. aqd → `https://aqd-fc.web.app`,
  `http://localhost:3000`.
- Middleware answers `OPTIONS` preflight for `/v1/*`: allow methods
  `GET, POST`, headers `Authorization, X-KD-Tenant, Content-Type`; echo only
  allowlisted origins; `Vary: Origin`. No wildcard `*` — credentials are in
  headers.

## Part C — Browser-safe auth  (Phase 2)

The tenant API key is a server-side secret and **must never ship in a static
browser bundle**. Two acceptable modes:

1. **External IdP verification (recommended for aqd).** Tenant config stores
   an OIDC issuer + audience (for aqd: issuer
   `https://securetoken.google.com/aqd-fc`, audience `aqd-fc`, Google
   securetoken JWKS, RS256). `authMiddleware` gains a second path: if the
   bearer is not an API key/HS256 JWT, verify it against the tenant's
   configured IdP and grant a **restricted scope set** (start with
   `extract` only). The end-user's `sub`/`email` claim becomes the per-user
   rate-limit key and audit actor. No secrets in the browser; each user
   calls with their own short-lived Firebase ID token.
2. **Token exchange (roadmap Phase 2 as written).** The tenant's *backend*
   swaps its API key + user context for a short-lived scoped session JWT.
   Correct for tenants that have a server; aqd does not yet, so mode 1
   unblocks it. Build both behind the same scope-enforcement layer so
   routes only ever check scopes (`extract`, `search:read`, `records:write`,
   `verify`), not auth mode.

Dev shortcut (document, never deploy): a throwaway API key in the client
`.env.local` against `localhost:18080` is fine for local end-to-end testing
only; treat that key as burned afterwards.

## Part D — Later: finalize & verify (Phases 3–4)

Once extract + auth work end-to-end, add (already on the roadmap):
normalize/validate → structured per-field errors; finalize =
resolve-or-create person + versioned identity document (server owns
`verification_status` and `field_confidence`); then
`POST /v1/identity-documents/{id}/verify` for staff with the `verify` scope.
At that point aqd should read/write KYC through kd-server instead of its own
Firestore `users` doc (aqd currently stores `nationalId`, `nameDhivehi`,
`nameEnglish`, `addressDhivehi`, `addressEnglish` there as its minimum-KYC
gate for MAP session approval).

## aqd field mapping (client-side, for reference)

| DocumentExtraction | aqd minimum-KYC field |
|---|---|
| `national_id` | `nationalId` |
| `name_dhivehi` | `nameDhivehi` |
| `name_english` | `nameEnglish` |
| `house_dhivehi` + `island_dhivehi` | `addressDhivehi` (joined `، `) |
| `house_english` + `island_english` | `addressEnglish` (joined `, `) |

Remaining fields (`sex`, `date_of_birth`, `blood_group`, `expiry_date`,
`serial_number`, `common_name_english`) are used by aqd's fuller participant
profile and by kd-server's own identity-document record in Part D.

## Security & privacy (binding for every part)

- Never log uploaded documents, OCR text, extracted values, or tokens
  (existing rule in `IDENTITY_DOCUMENTS.md` — applies to the new route and
  middleware too).
- Extraction responses are suggestions; the reviewing human saves. Keep
  `Cache-Control: no-store` on every extract response.
- AI mode sends the card image to the configured provider (Anthropic).
  Tenants must be able to see which engine handled a request (`engine` field
  already in the response) and opt out to OCR-only by not configuring AI
  credentials.
- Per-user rate limiting is a privacy control as much as a cost control:
  it bounds bulk exfiltration through a leaked browser token.

## Config summary (new)

| Setting | Where | Purpose |
|---|---|---|
| Allowed CORS origins | tenant config | Part B |
| OIDC issuer / audience / JWKS URL | tenant config | Part C mode 1 |
| Scopes per auth mode | tenant config / token claims | Part C |
| Extract rate limits (tenant, user) | env or tenant config | Parts A/C |

Existing env stays as is (`MONGO_URI`, `JWT_SIGNING_KEY`,
`AI_ENCRYPTION_KEY`, …).

## Build order & gates

A → B → C (mode 1) unblocks aqd end-to-end; D follows. Each part: one PR,
`make fmt lint test` green, roadmap checkbox updated, `search.rest`-style
manual `.rest` file for owner verification (`extract.rest` with a sample
image is recommended — use synthetic/sample cards only, never a real
person's document in the repo).
