# KD-Server

KD-Server is a Go + MongoDB backend that maintains a single source of truth for entities across tenant-specific integrations. Start in `GUIDE.md` for the roadmap and `AGENTS.md` for contributor expectations. Part 1 will introduce the HTTP server skeleton.

## Environment Configuration

Copy `.env.example` to `.env` and adjust the values for your environment:

```
cp .env.example .env
```

- `MONGO_URI` points to your MongoDB instance (defaults to `mongodb://localhost:27017` for local development).
- `MONGO_DB` is the database name the API uses (defaults to `kdserver`).
- `JWT_SIGNING_KEY` holds the HS256 secret used to validate JWT bearer tokens; set a unique value per environment.
- `PORT` selects the API port. The example uses `18080` for local development; deployments should set the port required by their environment.
- `KD_ADMIN_USERNAME` and `KD_ADMIN_PASSWORD` protect the browser-based tenant administration console at `/admin`.

The API and admin TUI automatically load `.env` from the current working directory for local development. Existing process environment variables take precedence over values in `.env`.

## Web Administration

Open `http://localhost:18080/admin` and sign in with the configured admin username and password to list, create, and rename tenants, issue API keys, and revoke keys. Tenant slugs remain immutable so existing data and API keys continue to work. Admin credentials are separate from tenant credentials and must not be shared with tenant applications.

## Authentication

Non-health API routes require two headers. The bearer credential can be either a TUI-issued API key or an HS256 JWT:

- `Authorization: Bearer <api-key>` where the key was issued for the requested tenant, or `Bearer <jwt>` where the JWT contains a matching `tenant` claim.
- `X-KD-Tenant: <tenant-slug>` matching the API key tenant or JWT tenant claim.

## Ingest Workflow

`POST /v1/ingest` accepts a JSON payload that looks like:

```json
{
  "source": { "slug": "system", "name": "System Import" },
  "records": [
    { "id": "abcd", "name": "Jane Example" }
  ]
}
```

The request is authenticated via the headers above. Ingested records are written to the `raw` collection and the response reports counts:

```json
{ "created": 1, "linked": 0, "needs_review": 0 }
```

## Resolve & Lookup

- `POST /v1/resolve` processes pending ingest records using deterministic matches (national ID, email, phone). Provide an optional `batch_size` to control work per call.
- `GET /v1/lookup/phone/{e164}` and `GET /v1/lookup/email/{email}` return the resolved person record plus links stored for the given tenant.

## Review Queue (Stub)

`GET /v1/review?status=needs_review` lists raw records that could not be matched deterministically. Use `POST /v1/review/{id}/decision` with `{ "decision": "accept" | "reject" }` to update their status.

## Admin TUI

Run `make tui` (or `go run ./cmd/tui`) to open the admin console for tenant onboarding:

1. Create a tenant (provides slug + name).
2. Issue a new API key — the secret is displayed once alongside an `.env` snippet:

   ```
   KD_TENANT=tenant-slug
   KD_API_KEY=key_xxx.secret
   ```

3. Revoke API keys that are exposed or no longer needed.
4. Share active credentials only with the team deploying the specific tenant.
