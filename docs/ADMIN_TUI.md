# Admin TUI

`cmd/tui` is a terminal admin console for KD-Server. It is a **super-admin, single-operator tool** — it connects directly to MongoDB via `internal/services` and intentionally bypasses the HTTP auth/tenant middleware (`authMiddleware` in `internal/http/server.go`). There is no login, no JWT, no API key check: whoever can run `make tui` against the configured `MONGO_URI` has full access to every tenant.

This is a deliberate design choice, not a gap. The TUI exists for one trusted operator running it locally or on the server host, to get administrative work done without going through the tenant-scoped HTTP surface. It is **not** a multi-user or remote-access tool, and should never be exposed over a network or given a login layer that implies otherwise.

## Parity rule

Whenever a new domain service is added under `internal/services` that a super-admin would plausibly need to inspect, run, or fix directly (a new ingest source, a new resolution mode, a new document type, etc.), **extend `cmd/tui` in the same PR**. This is an addition to the "One PR = one Part" rule in `CLAUDE.md`. The TUI previously lagged for over a month while the HTMX admin console (`feat/admin-htmx`) gained substantial new capability — this rule exists to stop that drift from happening again.

## Structure

Each domain lives in its own file under `cmd/tui/`, mirroring the `internal/services`/`internal/http` one-file-per-concern convention:

- `main.go` — entry point, top-level menu loop, tenant-context session, shared prompt helpers (`prompt`, `promptDefault`, `envOrDefault`, `selectTenant`).
- `tenants.go` — tenant and API key management.
- `ingest.go` — raw record ingest (from a JSON file) and resolve batch runs.
- `lookup.go` — lookup by email/phone.
- `review.go` — review queue listing and accept/reject decisions.
- `assets.go` — asset ingest (from a JSON file), query, transfer, dispose.
- `identity.go` — identity document list/view/verify/reject.

## Menu structure

```
1) Tenant & API Key Management
2) Ingest & Resolve
3) Lookup
4) Review Queue
5) Assets
6) Identity Documents
7) Switch tenant
8) Exit
```

Tenant & API Key Management is tenant-agnostic (it manages tenants themselves). Every other menu is scoped to a single tenant — selecting one of those menus prompts for a tenant first if none is set for the session, and "Switch tenant" is available at any time.

## Bulk input

Ingest and asset ingest both take structured batches, not single-field forms. The TUI prompts for a **JSON file path** rather than a field-by-field CLI form. The file's shape matches the equivalent HTTP API request body (`internal/http/ingest.go`, `internal/http/asset.go`'s `assetIngestRequest`), so the same file can be used against either interface.

## Identity documents

Identity documents are normally created via OCR extraction (`internal/services/document_extraction.go`), not manual entry — a full record has 15+ fields including bilingual (English/Dhivehi) name and address fields. The TUI supports **list, view, and verify/reject** only. Manual full-record creation stays an OCR/API job.

## Known limitation

Because the TUI bypasses tenant auth by design, it has no built-in audit trail of who ran which action — that's an accepted tradeoff for a single-operator tool. See `docs/SECURITY_CI_BACKLOG.md` for broader security/CI improvement areas (most of which are about the HTTP surface, not the TUI).
