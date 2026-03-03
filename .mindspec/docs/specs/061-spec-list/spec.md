---
approved_at: "2026-03-03T22:21:53Z"
approved_by: user
status: Approved
---
# Spec 061-spec-list: Spec List Command

## Goal

Add a `mindspec spec list` command that lists all specs with their current lifecycle phase, giving users a quick overview of project status.

## Background

There is currently no command to list specs. Users must manually `ls .mindspec/docs/specs/` or use `mindspec state show` (which only shows the active spec). With ADR-0023 eliminating focus files, lifecycle phase is now derived from beads — a list command can show each spec's phase by querying beads epics.

## Impacted Domains

- cli: new `spec list` subcommand under existing `spec` command group
- phase: reuses `DiscoverActiveSpecs()` for phase derivation

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Phase derived from beads, not focus/lifecycle files

## Requirements

1. `mindspec spec list` prints a table of all specs found under `.mindspec/docs/specs/`
2. Each row shows: spec ID, spec status (Draft/Approved from frontmatter), lifecycle phase (from beads, e.g. idle/spec/plan/implement/review)
3. Specs with no beads epic show phase as "idle" or "—"
4. Output is sorted by spec number ascending
5. `--json` flag outputs structured JSON for scripting

## Scope

### In Scope
- `cmd/mindspec/spec_list.go` — new subcommand
- `internal/speclist/speclist.go` — list + enrich logic
- Unit tests for the list/enrich logic

### Out of Scope
- Filtering by phase or status (can be added later)
- Spec creation/deletion from this command

## Non-Goals

- Interactive spec selection or navigation
- Changing spec state from the list command

## Acceptance Criteria

- [ ] `mindspec spec list` prints a human-readable table of all specs
- [ ] Each spec shows its frontmatter status and beads-derived phase
- [ ] `mindspec spec list --json` outputs valid JSON array
- [ ] Output sorted by spec number
- [ ] `go test ./internal/speclist/...` passes

## Validation Proofs

- `mindspec spec list`: Outputs table with at least one spec row
- `mindspec spec list --json | jq length`: Returns count matching `ls .mindspec/docs/specs/ | wc -l`

## Open Questions

None.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-03
- **Notes**: Approved via mindspec approve spec