---
adr_citations: []
approved_at: "2026-03-03T23:48:34Z"
approved_by: user
bead_ids:
    - mindspec-sx1h.1
last_updated: "2026-03-03"
spec_id: 065-freshness-gate-messaging
status: Approved
version: 1
---
# Plan: 065-freshness-gate-messaging

## ADR Fitness

No ADRs are relevant to this change. This is a cosmetic update to error message strings with no architectural implications.

## Testing Strategy

Unit tests only. The existing `dispatch_test.go` tests for `--force` bypass behavior will be verified to still pass. No new tests needed since this is a message text change.

## Bead 1: Update freshness gate error messages

**Steps**
1. In `cmd/mindspec/next.go`, change the error format string to remove "Use --force to bypass" and use imperative "You MUST run /clear" language with "Do NOT attempt workarounds"
2. In `internal/hook/dispatch.go`, update both freshness gate block messages (session source stale + bead already claimed) with the same imperative language
3. Run `make build` to verify compilation
4. Run `make test` to verify no test regressions
5. Run `grep -r "Use --force to bypass" cmd/ internal/` to confirm no remaining instances

**Verification**
- [ ] `make build` exits 0
- [ ] `make test` exits 0
- [ ] `grep -r "Use --force to bypass" cmd/ internal/` returns no matches

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---|---|
| Error messages no longer mention `--force` | Bead 1, step 5 (grep) |
| Error messages use imperative "MUST" language | Bead 1, steps 1-2 (code review) |
| `--force` flag still works | Bead 1, step 4 (existing unit test) |
| Unit tests pass | Bead 1, step 4 |
| `make build` succeeds | Bead 1, step 3 |
