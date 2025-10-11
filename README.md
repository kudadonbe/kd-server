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
