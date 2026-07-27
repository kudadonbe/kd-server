# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Cross-Station Workflow — READ FIRST (every agent, every station)

This repo is developed from **two machines** (a Windows station and a MacBook).
To stay in sync, every agent must follow these rules **without being told** each
time. Full details in `docs/CROSS_STATION_WORKFLOW.md`.

1. **At the start of a session, pull latest before doing anything.** Run
   `bash scripts/sync-start.sh` (fetches + fast-forwards the branch, refreshes
   local `main`). If that script is unavailable, do `git fetch --all --prune`
   and fast-forward the current branch manually. Never start work on stale code.
2. **At the end of a session, make sure the remote has everything.** Run
   `bash scripts/sync-end.sh` to push all committed work so the other station
   starts current. It never commits for you.
3. **One branch per feature, used from both stations.** Do not commit feature
   work to `main` from one station while the other is on a feature branch — that
   is what caused past divergence. Check `git status` / current branch first.
4. **Line endings are LF everywhere**, enforced by `.gitattributes`
   (`* text=auto eol=lf`). If Go files show up as "modified" right after a
   checkout, it is CRLF noise — confirm with `git diff --ignore-all-space`
   (empty = noise) and renormalize; do **not** commit it. On a fresh Windows
   checkout, set `core.autocrlf=false` and `core.eol=lf` once.
5. **Commit only after the change is tested** (see below); `sync-end.sh` pushes
   only what is already committed.
6. **Tag significant milestones as versions.** When a meaningful chunk of work
   is done (or before a large/risky change), cut an annotated semver tag with
   `bash scripts/tag-release.sh vX.Y.Z "milestone description"` so there's a
   permanent point to roll back to or branch from. MAJOR = breaking `/v1`
   change, MINOR = new backward-compatible features, PATCH = fixes. Keep
   `appVersion` in `cmd/api/main.go` in step with the latest tag.

## Project Overview

KD-Server is a multi-tenant Go + MongoDB backend that maintains a single source of truth for entities (people, organizations, assets) across tenant-specific integrations. It prevents data duplication by linking records from multiple sources (SchoolSync, Ekkurey, etc.) to resolved person entities.

**Core Flow**: Ingest → Resolve → Lookup
- Raw records are ingested from various sources
- Deterministic matching (national ID, email, phone) resolves records to person entities
- Lookups return unified person records plus their source links

## Development Commands

```bash
# First-time setup - install dev tools (gofumpt, golangci-lint)
make deps

# Format code (gofumpt + goimports) - run before every commit
make fmt

# Run linters (vet + golangci-lint)
make lint

# Run all tests
make test

# Run specific test
make test TESTARGS='-run TestIngestService'

# Start the HTTP API (uses .env for configuration)
make run

# Start the admin TUI for tenant management
make tui
```

**Pre-commit requirement**: Always run `make fmt lint test` before creating a PR.

## Architecture

### Layered Design

```
cmd/
├── api/     - HTTP server entrypoint
└── tui/     - Admin console for tenant/API key management

internal/
├── auth/    - JWT token verification (HS256)
├── http/    - HTTP handlers + middleware (auth, logging)
├── services/- Business logic orchestration
└── store/   - MongoDB data access layer

api/         - OpenAPI 3.1 spec (source of truth for contracts)
testdata/    - Test fixtures
```

**Directory boundaries are strict**:
- `internal/http`: HTTP concerns only (request/response, middleware)
- `internal/services`: Orchestrate business logic, call store methods
- `internal/store`: MongoDB operations only, no business logic

### Data Model

**MongoDB Collections**:
1. **raw** - Ingested records awaiting resolution (status: pending/resolved/needs_review)
2. **people** - Resolved entities with deterministic identifiers (nationalId, primaryEmail, primaryPhone)
3. **links** - Connections between person records and source system records
4. **tenants** - Tenant definitions (slug, name)
5. **keys** - API keys (hashed) for tenant authentication

**Tenant Isolation**: All data is scoped by `tenantId`. Auth middleware enforces that JWT `tenant` claim matches `X-KD-Tenant` header.

### Authentication Flow

All routes except `/v1/healthz` require:
1. `Authorization: Bearer <jwt>` - Token signed with `JWT_SIGNING_KEY` (HS256) containing `tenant` claim
2. `X-KD-Tenant: <slug>` - Must match token's tenant claim

Middleware in `internal/http/server.go:129` validates both and injects tenant into request context.

### Resolve Logic

Deterministic matching (`internal/services/resolve.go`):
1. Extract `national_id`, `email`, `phone` from raw record payload
2. If all three missing → mark `needs_review`
3. If any present → `FindPersonByIdentifiers` (queries indexed fields)
4. If person exists → update identifiers, create/update link
5. If person missing → create new person, create link
6. Update raw record status to `resolved` with `personId` reference

Phone numbers should be E.164 format; emails are lowercased and trimmed.

## Configuration

Copy `.env.example` to `.env`:
- `MONGO_URI` - MongoDB connection string (default: `mongodb://localhost:27017`)
- `MONGO_DB` - Database name (default: `kdserver`)
- `JWT_SIGNING_KEY` - HS256 secret for token validation (set unique per environment)

Known security/CI gaps (secret rotation, dependency scanning, CI coverage of integration tests, etc.) are tracked in `docs/SECURITY_CI_BACKLOG.md` — check it before assuming the current setup is production-hardened.

## Testing Patterns

- Use table-driven tests with Go's `testing` package
- Place tests beside implementation (`resolve_test.go` next to `resolve.go`)
- Integration tests that hit MongoDB live in `internal/store/*_integration_test.go`
- Load fixtures from `testdata/` when needed
- Name tests `TestComponentCase` (e.g., `TestResolveServiceExactMatch`)

## Coding Conventions

**From AGENTS.md**:
- Go formatting is mandatory (tabs, gofmt/goimports)
- Exported types: `CamelCase`; internals: `lowerCamel`
- Package names: short nouns (`auth`, `resolve`)
- JSON response structs: `snake_case` tags
- Structured logging must include `tenantId`, `path`, `latency`
- Never log PII (national IDs, emails, phone numbers)

**API-First Development**:
- All endpoints must be defined in `api/openapi.yaml` before implementation
- JSON responses must validate against OpenAPI schemas
- Use `requests.rest` for manual API testing during development

## Phased Roadmap

Project follows a phased roadmap (see `GUIDE.md`). Current implementation includes:
- **Parts 1-8**: HTTP server, MongoDB wiring, auth/tenant, ingest, resolve, lookup, review queue (stub), admin TUI

**One PR = One Part** - Keep changes focused and testable.

## Common Patterns

**Service initialization** (`cmd/api/main.go`):
```go
mongoStore := store.NewMongoStore(mongoClient, cfg.MongoDB)
ingestSvc := services.NewIngestService(mongoStore)
resolveSvc := services.NewResolveService(mongoStore)
```

**Extracting tenant from context** (`internal/http`):
```go
tenantID, ok := TenantFromContext(ctx)
if !ok {
    // handle missing tenant
}
```

**Error handling in handlers**:
- Return structured JSON errors via `writeJSON(w, statusCode, map[string]string{"error": "message"})`
- Match OpenAPI error response schemas

## Admin TUI

`cmd/tui` is a super-admin, single-operator console that talks to MongoDB directly via `internal/services`, intentionally bypassing the HTTP auth/tenant middleware. Full details in `docs/ADMIN_TUI.md`.

**Parity rule**: whenever a new domain service is added under `internal/services` that a super-admin would plausibly need to inspect or operate directly, extend `cmd/tui` in the same PR — don't let it lag behind the HTTP API/admin console.

## Branch Naming

Use `feat/<area>-<part>` or `fix/<area>-issue`:
- Example: `feat/ingest-part4`, `feat/resolve-part5`
- Link PRs to roadmap parts or issues in description
