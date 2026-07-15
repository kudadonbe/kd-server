# KYC / Entity Hub Roadmap

Tracks the work that turns kd-server into a shared KYC/entity hub for many apps
(first consumer: **aqd** — ID-card entry + people search). Check items off as they
land. **One PR = one part.** Keep this file updated when a part completes.

- **Status legend:** `[ ]` todo · `[x]` done · `[~]` in progress
- **Last updated:** 2026-07-15
- **Build notes for the aqd consumer flow (extract → prefill KYC):** `AQD_KYC_INTEGRATION.md`

## Why

KYC-grade identity data is reused by many apps, so all integrity rules (normalize,
validate, resolve, verify) live server-side and are shared over `/v1` — never
re-implemented in a frontend where they'd drift or be bypassed. A person must be
findable from *any* partial signal (name part En/Dhivehi, DOB, current/old address,
national ID, email, phone, vehicle/property/business via linked records). Related
entities attach as generic **linked records** (the `links` collection = a Mongo
junction table), so new record types are searchable with no schema change.

Build order: **search → access → write/finalize → verify.**

---

## Phase 1 — Multi-signal entity search  `[~]` verified, commit pending

Approach: a denormalized per-`(tenant, person)` `entity_index` (rebuilt on write)
flattens every signal into a bounded, indexed `terms[]` array; Mongo filters
(index-bound), Go scores/ranks (explainable).

**Store**
- [x] `EntityIndex` / `EntitySummary` model — `internal/store/entity_index.go`
- [x] `SearchEntityIndex` (index-backed `$or`: terms `$in` + anchored-prefix regex + exact fields)
- [x] `RebuildEntityIndex` (people + identity docs + history old-addresses + link payload scalars)
- [x] `BackfillEntityIndex` + best-effort rebuild helper (non-fatal, logged)
- [x] `entity_index` indexes in `defaultIndexSpecs` + `mongo_test.go` map kept in sync
- [x] Write-path hooks: `CreatePerson`, `UpsertLink`, `SaveIdentityDocument`

**Service** — `internal/services/search.go`
- [x] `EntitySearch` interface + `EntitySearchService`
- [x] Additive ranking (nid 100 · email 60 · phone 50 · dob 40 · name-exact 20 · island 15 · name-prefix 10 · linked 8) with `matched_on` reasons

**HTTP** — `internal/http/search.go`
- [x] `POST /v1/search` (snake_case multi-signal body → ranked candidates)
- [x] `POST /v1/search/reindex` (backfill / drift repair)
- [x] Wiring: `Config.SearchService`, nil-guarded routes, `main.go` construction

**Gates**
- [x] `go build ./...`, `make fmt`, `make lint` (exit 0), `make test` green

**Owner verification (manual, via `search.rest`)** — run 2026-07-15 (fc tenant)
- [x] Seed (`/v1/ingest` + `/v1/resolve`), add an identity doc
- [x] `POST /v1/search/reindex` once (populate existing data)
- [x] Search: name prefix · national_id exact (100) · combined name+dob+island (75) ordering
- [x] Linked-payload term (`P1234` vehicle number) matches — extensibility proof
- [x] Dhivehi/Thaana prefix (`ޢަ` → name match) · tenant isolation (pem empty) · pagination
- [ ] Commit after OK (separate from the admin-HTMX commit)

---

## Phase 2 — Access model (scoped delegated tokens)  `[ ]`

The app authenticates its own user and passes the user's role/level; kd-server mints
a scoped session token on top of the app's API key.

- [ ] Token exchange: app API key + user role → short-lived scoped session token (JWT: tenant + scopes)
- [ ] External IdP bearer verification for browser-only tenants (per-tenant OIDC issuer/audience/JWKS; aqd = Firebase `securetoken.google.com/aqd-fc`) → restricted `extract` scope — `AQD_KYC_INTEGRATION.md` Part C
- [ ] CORS for browser tenants (per-tenant allowed origins, preflight on `/v1/*`) — `AQD_KYC_INTEGRATION.md` Part B
- [ ] Scopes enforced per route: `extract`, `search:read`, `records:write`, `verify`
- [ ] Common `/v1` services for all apps; optional app-specific services gated by an app claim
- [ ] Gate `/v1/search` as read; write/verify endpoints by their scopes

---

## Phase 3 — KYC write / finalize  `[ ]`

- [ ] Tenant-facing `/v1` extract endpoint (mirror admin extract; extract-only; per-tenant AI creds + rate limits) — `AQD_KYC_INTEGRATION.md` Part A
- [ ] Normalize + validate layer → structured per-field errors (national ID format, sex enum, ISO dates, names/address)
- [ ] Finalize: resolve-or-create person (`store.FindPersonByIdentifiers`/`CreatePerson`) → save identity document (versioned, audited)
- [ ] Server owns `verification_status` (default unverified) and `field_confidence` — clients cannot self-assert
- [ ] Gated by `records:write`

---

## Phase 4 — Verify transition  `[ ]`

- [ ] `POST /v1/identity-documents/{id}/verify` — promotes to verified, records authenticated actor as `verified_by`
- [ ] Gated by `verify` scope

---

## Deferred / notes

- **Images:** structured data + reference string only (no blob storage in kd-server now).
- **OpenAPI:** `api/openapi.yaml` is a stale placeholder — update contracts as these endpoints land.
- **Dhivehi:** v1 stores Thaana tokens verbatim + anchored prefix; Unicode NFC normalization is a follow-up.
- **Denormalization sync:** the search index is derived/best-effort on write; `POST /v1/search/reindex` reconciles drift.
- **Model gaps:** "old address" exists only if `identity_document_history` rows exist; `Person` has no name field (name search needs an identity doc).
