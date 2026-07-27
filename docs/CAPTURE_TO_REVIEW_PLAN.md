# Capture-New-Info-to-Review — build plan

**Goal:** when a document is traced (extracted) and yields information the server
doesn't already hold, capture the *new* info into the review queue rather than
losing it — and make review **accept** actually apply + verify the data, so the
server accumulates a verified source of truth over time.

## Decisions (owner-approved)
- **Who captures:** trusted callers (`records:write`) **and** public (`extract`-only) users.
- **Where the logic lives:** extend **finalize** (trusted) + a quarantined side-effect on **extract** (public).
- **Accept applies:** review `accept` runs resolve-or-create + save + verify (reviewer = `verified_by`).

## Safety model (binding)
- **Diff-driven:** only capture the *delta* vs the person's current identity document. No delta → save nothing.
- **Never overwrite verified data:** a traced value that conflicts with a **verified** record goes to `needs_review`, never auto-applied.
- **Public captures are quarantined:** tagged `trust: untrusted`, `source: public:<tenant>`; only `needs_review`; never auto-saved/verified; a `verify` reviewer gates them.
- **No info leak:** the extract response is unchanged — capturing never reveals whether the person exists or returns existing/verified data.
- **Best-effort:** a capture failure never breaks extract or finalize.
- **Caveat:** public captures are low-verifiability leads (no image stored) — staff treat them as tips to confirm/reject.

## Parts
- [x] **1 — Identity diff service** (`internal/services/identity_capture.go`): resolve person by national_id(+phone), load latest identity document, compute changed/new fields + `conflicts_verified`. `HasNewInfo()`.
- [x] **2 — Capture-to-review store** (`internal/store/raw.go`): `InsertRawReview(tenantID, sourceSlug, payload)` + `GetRaw` → `raw` record, status `needs_review`, payload carries `capture_type:"identity"`, `trust`, `resolved_person_id`, `is_new_person`, `conflicts_verified`, `changed_fields`, `document`.
- [x] **3 — Finalize becomes diff-aware** (`identity_finalize.go`): no delta → `unchanged` (no write); conflict with verified → capture to `needs_review` (don't overwrite), return `queued_for_review` (HTTP 202); else → save unverified. Returns `FinalizeResult`.
- [x] **4 — Extract quarantine capture** (`internal/http/identity_document.go`): for callers **without** `records:write`, after building the (unchanged) response, best-effort diff + `InsertRawReview` with `trust:untrusted` when there's new info and a national_id.
- [x] **5 — Review accept applies + verifies** (`services/review.go` + store): `Decide(accept)` on an identity capture reconstructs the document (bson round-trip), resolve-or-create + save + verify (reviewer as `verified_by`), sets raw `resolved` + personId. Non-identity raw keeps current behavior. Threads reviewer actor.
- [ ] **6 — Docs + version**: update `docs/API_USAGE.md` (done); owner test via `.rest`; then tag the milestone (`vX.Y.Z`).

**Status:** code complete — build + vet + lint + all existing tests pass. Awaiting owner manual test before commit.

## Manual verification (owner, via .rest)
1. Public extract of a brand-new national ID → `needs_review` item appears (trust untrusted); response unchanged.
2. Trusted finalize of new person → saved unverified (as today).
3. Trusted finalize matching an existing **verified** doc with a changed field → item in `needs_review`, verified doc untouched.
4. Review `accept` on (3) → person/doc updated + verified, reviewer recorded; raw → resolved.
5. Re-trace identical data → **no** new review item (no delta).
6. Tenant isolation + no-PII-in-logs still hold.
