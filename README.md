# KD-Server

KD-Server is a Go + MongoDB backend that maintains a single source of truth for entities across tenant-specific integrations. Start in `GUIDE.md` for the roadmap and `AGENTS.md` for contributor expectations. Part 1 will introduce the HTTP server skeleton.

## Environment Configuration

Copy `.env.example` to `.env` and adjust the values for your environment:

```
cp .env.example .env
```

- `MONGO_URI` points to your MongoDB instance (defaults to `mongodb://localhost:27017` for local development).
- `MONGO_DB` is the database name the API uses (defaults to `kdserver`).

When you run `make run` or start the API manually, these variables configure the Mongo connection used during startup.
