# Repository Guidelines

## Project Structure & Module Organization
KD-Server is a Go + MongoDB service. Source entrypoints live in `cmd/api` (HTTP) and `cmd/tui` (admin console). Put handlers in `internal/http`, orchestrating services in `internal/services`, and Mongo data access in `internal/store`. Keep shared packages narrow; wire dependencies via interfaces. Contracts stay in `api/openapi.yaml` with schemas in `api/schemas`. Use `docs/` for ADRs, `testdata/` for fixtures, and `.github/` for CI configs.

## Build, Test, and Development Commands
Run `make deps` once to install tooling. `make fmt` applies gofmt/goimports. `make lint` runs vet and golangci-lint. `make test` executes unit and integration suites; scope with `TESTARGS='-run Name'` when needed. `make run` starts the API using `.env`, while `make tui` opens the admin console. Always complete `make fmt lint test` before pushing.

## Coding Style & Naming Conventions
Go formatting (tabs) is mandatory; never hand-format. Exported types and functions use CamelCase, internals stay lowerCamel. Package names stay short nouns (`auth`, `resolve`). JSON response structs need snake_case tags. Keep files focused; split large handlers or services early. Include structured logs with `tenantId`, `path`, and `latency`, and avoid logging PII.

## Testing Guidelines
Write table-driven tests with Go’s `testing` package beside the code (`internal/services/resolve_test.go`). Integration tests that hit MongoDB belong in `internal/store` and load fixtures from `testdata/`. Name tests `TestComponentCase`. Cover ingest → resolve → lookup flows and add focused tests for bug regressions. Fail the build on missing coverage around auth or tenant boundaries.

## Commit & Pull Request Guidelines
Commit messages stay imperative (`Add tenant resolver`). Limit each branch to one roadmap part using `feat/<area>-<part>` or `fix/<area>-issue`. Pull requests must link issues or roadmap parts, summarize behavior changes, and paste the latest `make fmt lint test` output. Attach API responses or TUI screenshots when UX changes, and confirm the OpenAPI spec stays in sync.
