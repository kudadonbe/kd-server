
# KD-Server Build Guide (for AI & Human Developers)

---

## 🌍 Purpose
KD-Server is a **Go + MongoDB** backend that keeps **one truth per real-world entity (person, organization, asset, etc.)**  
It links data from many apps (like SchoolSync, Ekkurey, etc.) without duplication.  
Everything is tenant-aware and outputs clean JSON responses.  
Markdown and AI-generated summaries will come in **Phase 2**.

---

## ⚙️ Stack & Core Principles
| Component | Choice | Notes |
|------------|---------|-------|
| Language | **Go** | Fast, small binary, perfect for local hosting |
| Database | **MongoDB** | Flexible documents, good for linking data |
| Output | **JSON only** | Markdown later (AI phase) |
| Interface | REST API (`/v1/...`) | OpenAPI spec defines all contracts |
| Admin | TUI (bubbletea or tview) | For tenant setup and review queue |
| Auth | JWT + `X-KD-Tenant` header | Each tenant has isolated data |
| Deployment | Linux VM | API + MongoDB on same host |
| Logging | Structured (tenantId, path, latency) | No PII in logs |

---

## 📂 Expected Repo Structure (agents will create)
```

api/         → OpenAPI spec + JSON Schemas
cmd/         → server main(), tui main()
internal/    → http, services, store, jobs, auth, config, logging
docs/        → architecture notes, DB collection docs, ADRs
testdata/    → sample CSV/JSON for tests
.github/     → CI and PR templates

```

---

## 🧭 Development Rules
- **API-first:** everything follows `api/openapi.yaml`.
- **One PR = One task.**
- **Directory boundaries:**  
  - `internal/http` → handlers  
  - `internal/services` → logic  
  - `internal/store` → Mongo code only  
  - `cmd/tui` → admin interface  
- Run `make fmt lint test` before any PR.  
- All responses must validate against JSON Schemas.

---

## 🧩 Phase 1 Roadmap (JSON-only)
Each part is small, testable, and independent.  
Move to the next part only when tests for the current one pass.

---

### **Part 0 — Repo & Groundwork**
**Goal:** Create repo layout and placeholders (folders, Makefile, CI, docs).  
**Test:** `make fmt lint test` runs without errors.  
**Done:** Base repo ready.

---

### **Part 1 — Boot & Health**
**Goal:** Minimal Go API that starts and replies.  
**Endpoints:**  
- `GET /v1/healthz` → `{ "ok": true }`  
- `GET /v1/version` → `{ "version": "1.0.0" }`  
**Done:** Server runs and logs cleanly.

---

### **Part 2 — MongoDB Wiring & Index Plan**
**Goal:** Connect to Mongo, define collections and indexes.  
**Collections:** people, links, sources, raw, tenants, keys.  
**Done:** Connection tested; indexes logged at startup.

---

### **Part 3 — Auth + Tenant**
**Goal:** Enforce tenant scope and JWT auth.  
- Header: `X-KD-Tenant`  
- Token: Bearer JWT (tenant claim)  
**Done:** All non-health routes require valid tenant and token.

---

### **Part 4 — Ingest (JSON rows, sync)**
**Goal:** Accept small JSON array and store raw items + source info.  
**Endpoint:** `POST /v1/ingest`  
**Done:** Sends back counts (created, linked, needsReview).

---

### **Part 5 — Resolve (Exact Match)**
**Goal:** Convert raw → people + links using deterministic rules.  
**Match rules:** nationalId / email / phone (E.164).  
**Endpoint:** `POST /v1/resolve`  
**Done:** Re-runs safely (idempotent).

---

### **Part 6 — Lookup (Phone / Email)**
**Goal:** Fetch person + links from KD-Core.  
**Endpoints:**  
- `GET /v1/lookup/phone/{e164}`  
- `GET /v1/lookup/email/{email}`  
**Done:** Returns correct person JSON from schema.

---

### **Part 7 — Review Queue (Stub)**
**Goal:** Create placeholder for fuzzy matches (Phase 2).  
**Endpoints:**  
- `GET /v1/review?status=needs_review`  
- `POST /v1/review/{id}/decision`  
**Done:** Works with mock data.

---

### **Part 8 — Admin TUI v0**
**Goal:** Terminal UI for tenant & key management.  
**Features:** list tenants, create new, issue key, show `.env` snippet.  
**Done:** Admin can onboard a new tenant in < 5 min.

---

### **Part 9 — Ingest CSV**
**Goal:** Parse CSV → JSON → same ingest path.  
**Done:** CSV and JSON share same logic.

---

### **Part 10 — Async Jobs (large files)**
**Goal:** Handle >10 k rows via job queue.  
**Endpoints:**  
- `POST /v1/ingest` → `{ jobId }` if big  
- `GET /v1/jobs/{jobId}` → progress  
**Done:** Big imports processed in background.

---

### **Part 11 — Per-App Namespaces**
**Goal:** Friendly URLs for adapters.  
Examples:  
- `/v1/apps/schoolsync/students/ingest`  
- `/v1/apps/ekkurey/customers/ingest`  
**Done:** Internally routes to same ingest logic.

---

### **Part 12 — Backup & Health Ops**
**Goal:** Add backups, system health metrics.  
**Done:** `mongodump` scheduled; `/v1/healthz` shows Mongo OK.

---

### **Part 13 — Security Baseline**
**Goal:**  
- Rate-limit auth endpoints  
- Mask PII fields  
- Add audit logs  
**Done:** Lint passes; tests verify safe output.

---

## ✅ Definition of Done for Phase 1
- JSON ingest → resolve → lookup works end-to-end.  
- No cross-tenant data leaks.  
- TUI can create tenant and API key.  
- Nightly backup script exists.  
- CI passes all tests.  
- Server runs on Linux VM; health check OK.

---

## 🚀 After Phase 1
- Phase 2 → Markdown + AI summarization.  
- Phase 3 → Cross-app intelligence & advanced linking.  
- **KYC / entity hub** (multi-signal search, scoped access, ID-card finalize) is tracked part-by-part in [`docs/KYC_ENTITY_HUB_ROADMAP.md`](docs/KYC_ENTITY_HUB_ROADMAP.md).

---

## 👥 Collaboration Setup
- **One PR = One Part** above.  
- **Contracts:** defined in `api/openapi.yaml`.  
- **Schemas:** in `api/schemas/*.json`.  
- **CI checks:** `fmt`, `lint`, `test`, `contract`.  
- **Branch names:** `feat/<area>-<part>` (e.g. `feat/ingest-part4`).

---

### Example Agent Prompt
> You are implementing **Part 5 – Resolve (Exact Match)** in the `kd-server` repo.  
> Use Go + MongoDB. Follow `api/openapi.yaml` and `GUIDE.md`.  
> Edit only `internal/services/resolve.go` and `internal/http/resolve.go`.  
> Add tests in `testdata/resolve_test.go`.  
> Run `make fmt lint test` before commit.

---

## 📘 End of Guide