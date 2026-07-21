# KYC / Entity Hub Roadmap

Tracks the work that turns kd-server into a shared KYC/entity hub for many apps
(first consumer: **aqd** — ID-card entry + people search). Check items off as they
land. **One PR = one part.** Keep this file updated when a part completes.

- **Status legend:** `[ ]` todo · `[x]` done · `[~]` in progress
- **Last updated:** 2026-07-16 · Phase 1 shipped; Part A implemented (pending test/commit); smart-intake ladder + trust model added
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

## aqd enablement track (Parts A–D)  `[ ]`

Implementation-ordered slice of Phases 2–4, driven by **aqd** (static browser
app, Firebase Hosting). Detailed in `AQD_KYC_INTEGRATION.md`. Build order
**A → B → C(mode 1) → D** unblocks aqd end-to-end. One PR per part; each ends
green (`make fmt lint test`), ticks its box, and ships an `extract.rest`-style
manual test (sample cards only, never a real document).

**Part A — Tenant-facing extract endpoint**  (Phase 3, first checkbox)  `[~]`
- [x] Thread tenant into extraction: `DocumentExtractor.Extract` takes `tenantID`
      (was hardcoded `""`); updated `AIDocumentExtractor` (→ `ai.Extract`),
      `LocalDocumentExtractor` (ignores it), admin call site, existing test stubs
- [x] `POST /v1/identity-documents/extract` — mirror admin extract behind
      `authMiddleware`; multipart `document`, 10 MB, PDF/JPEG/PNG sniff, `422`
      on failure, `Cache-Control: no-store`; tenant from context; extract-only
      (no lookup, no save, no raw-text persistence)
- [x] Nil-guarded route wiring on `cfg.DocumentExtractor`
- [ ] Per-tenant rate limit (token bucket, `429` + `Retry-After`); per-user in Part C
- [ ] OpenAPI contract for the route
- [ ] `extract.rest` owner test with a sample card

**Part B — CORS for browser tenants**
- [ ] Per-tenant allowed-origins in tenant config (Mongo), editable in admin console
- [ ] CORS middleware on `/v1/*`: answer `OPTIONS` preflight; allow `GET, POST`,
      headers `Authorization, X-KD-Tenant, Content-Type`; echo only allowlisted
      origins; `Vary: Origin`; no wildcard `*`

**Part C — Browser-safe auth (mode 1: external IdP)**  (Phase 2)
- [ ] Tenant config: OIDC issuer + audience + JWKS URL (aqd = Firebase
      `securetoken.google.com/aqd-fc`, RS256)
- [ ] `authMiddleware` second path: verify tenant-IdP token → restricted scope
      set (start `extract` only); end-user `sub`/`email` → rate-limit key + audit actor
- [ ] Scope-enforcement layer (routes check scopes only: `extract`,
      `search:read`, `records:write`, `verify`), covering API-key + JWT + IdP modes
- [ ] (Later) mode 2 token exchange for tenants that have their own backend

**Part D — Finalize & verify**  (Phases 3–4)
- [ ] Normalize + validate → structured per-field errors (national ID, sex enum,
      ISO dates, names/address)
- [ ] Finalize: resolve-or-create person → versioned, audited identity document;
      server owns `verification_status` (default unverified) + `field_confidence`;
      gated `records:write`
- [ ] `POST /v1/identity-documents/{id}/verify` (scope `verify`) records `verified_by`

**Confirmed trust model** (2026-07-16)
- **Verification is by an authorized person only** — a privileged role. Clients
  can never self-assert `verified`; the server owns `verification_status`.
- **User edits/updates are saved but tagged** `unverified` (user-asserted):
  searchable, but labeled unconfirmed until an authorized person promotes them.
- Search-first treats **verified** records as authoritative; unverified matches
  may be offered but must be labeled.

---

## Living source of truth — smart intake ladder  `[ ]`

Goal: kd-server holds the current, verified identity data; every app's reviewed
corrections flow back so the next lookup is cheaper and better. Extraction spend
trends toward zero as the DB fills.

**Intake ladder (cheap → expensive):**
```
image → OCR (tesseract, free/local) → got searchable id (national_id ±dob)?
          ├─ yes → search kd-server → verified match? → RETURN it (no AI)
          │                            └─ no match ─────────────┐
          └─ no/blank ──────────────────────────────────────────► AI vision extract → return (fresh, unverified)
```
- [ ] Intake orchestrator service (composes OCR probe + `/v1/search` + AI extract);
      keep it ABOVE the extractor primitives, not inside them
- [ ] OCR probe hardening: validate national-ID **format** + **confidence-gate**
      before trusting a Tesseract read for the DB shortcut (avoid wrong-person match)
- [ ] Response marks provenance: `source: kd-server(verified) | ai(unverified)`,
      always presented as "is this you?" — never auto-accept
- [ ] Save only on user review+save (keeps unconfirmed AI guesses out of the DB)
- [ ] **Privacy scope (confirmed 2026-07-16, depends on Part C):** all search —
      including image→search return — requires an **authenticated end-user**
      identity (from any consuming app), never anonymous or a bare tenant key.
      Result **breadth follows the user's role/scope**: self-KYC users get only
      their own record; staff with `search:read` may search across people. The
      end-user identity is the audit actor + rate-limit key (a leaked token must
      not allow bulk PII pulls).
- [ ] (If cards carry an MRZ) add an MRZ reader as an even cheaper/reliable probe

**Revised build order:** A (done) → C (auth/scope) → D (save+verify) → intake
ladder → B (CORS, when the browser path is needed). The ladder is only worth
building once there is verified data to hit and auth to scope it.

**OCR choice:** Tesseract 5 stays for the free local probe (national ID is Latin,
reads fine; no image leaves the server; zero cost). It is weak on Thaana and on
phone photos — so AI owns full extraction, and the probe is format+confidence
gated. No cloud OCR (adds a data processor + cost for little Thaana gain).

---

## Deferred / notes

- **Images:** structured data + reference string only (no blob storage in kd-server now).
- **OpenAPI:** `api/openapi.yaml` is a stale placeholder — update contracts as these endpoints land.
- **Dhivehi:** v1 stores Thaana tokens verbatim + anchored prefix; Unicode NFC normalization is a follow-up.
- **Denormalization sync:** the search index is derived/best-effort on write; `POST /v1/search/reindex` reconciles drift.
- **Model gaps:** "old address" exists only if `identity_document_history` rows exist; `Person` has no name field (name search needs an identity doc).
