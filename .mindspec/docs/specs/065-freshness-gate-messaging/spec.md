---
approved_at: "2026-03-03T23:48:05Z"
approved_by: user
status: Approved
---
# Spec 065-freshness-gate-messaging: Freshness Gate Messaging

## Goal

Make the session freshness gate error message forceful enough that LLM agents actually obey it instead of working around it.

## Background

When `mindspec next` hits the session freshness gate, the error message says "Run /clear to reset your context, then retry. Use --force to bypass." The agent reads this, ignores the `/clear` instruction, and tries `--force` or other workarounds. The `--force` escape hatch in the error message gives the agent an easy out.

The `--force` flag should remain available for human users but must not be advertised in the error message where agents will see it.

## Impacted Domains

- cli: error messages in `cmd/mindspec/next.go`
- hooks: error messages in `internal/hook/dispatch.go`
- tests: unit tests that assert on error message strings

## ADR Touchpoints

None — no architectural decisions affected.

## Requirements

1. Remove "Use --force to bypass" from all freshness gate error messages
2. Make the error message imperative: "You MUST run /clear" instead of "Run /clear"
3. Add explicit "Do NOT attempt workarounds" language
4. Keep the `--force` flag functional (undocumented escape hatch for humans)
5. Update any unit tests that assert on the old message strings

## Scope

### In Scope
- `cmd/mindspec/next.go` — error message on line 75
- `internal/hook/dispatch.go` — error messages on lines 149, 155
- `internal/hook/dispatch_test.go` — any tests asserting old message text

### Out of Scope
- Removing the `--force` flag itself
- Changes to the freshness gate logic

## Non-Goals

- Changing when the freshness gate triggers
- Adding new enforcement mechanisms

## Acceptance Criteria

- [ ] Error messages no longer mention `--force`
- [ ] Error messages use imperative "MUST" language
- [ ] `--force` flag still works when explicitly passed
- [ ] Unit tests pass with updated message strings
- [ ] `make build` succeeds

## Validation Proofs

- `make build`: exits 0
- `make test`: exits 0
- `grep -r "Use --force to bypass" cmd/ internal/`: no matches

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec