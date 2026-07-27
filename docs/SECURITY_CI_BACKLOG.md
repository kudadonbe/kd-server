# Security & CI Improvement Backlog

Gaps identified while reviewing the repo for production/industry-standard readiness. None of these block current work; they're tracked here so they don't get lost. Roughly ordered by leverage (cheapest to fix now, most expensive to retrofit later, first).

## Security

- [ ] **JWT signing key has no rotation path.** `JWT_SIGNING_KEY` (`internal/auth`) is a single static HS256 secret with no `kid` support. If it leaks, every tenant's tokens are compromised until redeploy + full invalidation. Consider `kid`-based multi-key verification, or moving to asymmetric signing (RS256/ES256) so the verifying side never holds the signing secret.
- [ ] **No brute-force protection on admin console login.** `internal/http/admin_auth.go` uses constant-time credential comparison (good — stops timing attacks) but has no rate-limiting or lockout after repeated failed attempts.
- [ ] **No dependency vulnerability scanning.** Nothing runs `govulncheck` (or equivalent) against `go.mod`/`go.sum` to catch known CVEs in dependencies like `mongo-driver` or `jwt/v5`.
- [ ] **No secret-scanning.** No gitleaks/trufflehog pass in CI to catch an accidentally-committed real secret or PII sample. Matters more than average here given national ID and identity document handling.
- [ ] **No documented path to a real secret manager for staging/prod.** Secrets currently live in plain `.env` files (correctly gitignored) with no migration path to Vault/AWS/GCP Secrets Manager for non-dev environments.

## CI

- [ ] **Integration tests don't run in CI.** `*_integration_test.go` files (e.g. `tenant_integration_test.go`, `ingest_integration_test.go`) exercise real MongoDB, but `.github/workflows/ci.yml` has no MongoDB service container — these are effectively only run manually/locally today, leaving the store layer's CI coverage weaker than it looks.
- [ ] **No vulnerability/security scan step in the pipeline** (`govulncheck`, `gosec`, or CodeQL).
- [ ] **No test coverage tracking or threshold.** Nothing measures `go test -cover` output or gates on a minimum percentage, so a coverage regression wouldn't be caught.
- [ ] **Branch protection status on `main` is unverified from the codebase.** Whether the CI check is actually *required* before merge (vs. someone being able to push directly or merge with a failing check) is a GitHub repository setting — check Settings → Branches, not the code.

## Out of scope for the Admin TUI

Most of the above concerns the HTTP surface (JWT, admin console login, CI-gated PRs). The TUI (`docs/ADMIN_TUI.md`) intentionally bypasses tenant auth entirely by design — it is not a candidate for JWT/login hardening. It's listed here only because it's part of the same overall security posture review.
