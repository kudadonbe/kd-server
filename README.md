# KD-Server

KD-Server is a Go + MongoDB backend that maintains a single source of truth for entities across tenant-specific integrations. Start in `GUIDE.md` for the roadmap and `AGENTS.md` for contributor expectations. Part 1 will introduce the HTTP server skeleton.

## Environment Configuration

Copy `.env.example` to `.env` and adjust the values for your environment:

```
cp .env.example .env
```

- `MONGO_URI` points to your MongoDB instance (defaults to `mongodb://localhost:27017` for local development).
- `MONGO_DB` is the database name the API uses (defaults to `kdserver`).
- `JWT_SIGNING_KEY` holds the HS256 secret used to validate Bearer tokens; set a unique value per environment.

When you run `make run` or start the API manually, these variables configure the Mongo connection used during startup.

## Authentication

Non-health API routes require two headers:

- `Authorization: Bearer <jwt>` where the token is signed with `JWT_SIGNING_KEY` and contains a `tenant` claim.
- `X-KD-Tenant: <tenant-slug>` matching the `tenant` claim in the token.

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
