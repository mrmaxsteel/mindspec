---
approved_at: "2026-03-03T22:22:33Z"
approved_by: user
bead_ids:
    - mindspec-pg0w.1
spec_id: 061-spec-list
status: Approved
version: "1"
---
# Plan: 061-spec-list

## ADR Fitness

- **ADR-0023** (accepted): Spec list derives lifecycle phase from beads via `FindEpicBySpecID()` + `DerivePhase()` — consistent with ADR-0023's mandate that all phase derivation comes from beads.

## Testing Strategy

- **Unit tests**: Table-driven tests for `speclist.List()` using a temp directory with mock spec dirs and stubbed beads queries.
- **Integration**: `make test` passes; manual verification via `mindspec spec list` on real project.

## Bead 1: Implement spec list command

Add the `mindspec spec list` subcommand with table and JSON output.

**Steps**
1. Create `internal/speclist/speclist.go`:
   - `SpecEntry` struct: `SpecID`, `Status` (from frontmatter), `Phase` (from beads)
   - `List(root string) ([]SpecEntry, error)`: scan `.mindspec/docs/specs/`, read each `spec.md` frontmatter for status, call `phase.FindEpicBySpecID()` + `phase.DerivePhase()` for phase, sort by spec number
2. Create `internal/speclist/speclist_test.go`: table-driven tests with temp dirs and stubbed beads
3. Create `cmd/mindspec/spec_list.go`: register `list` subcommand under `specCmd`, call `speclist.List()`, format as table or JSON based on `--json` flag
4. Register in `cmd/mindspec/root.go` or existing spec command group

**Verification**
- [ ] `go test ./internal/speclist/...` passes
- [ ] `make test` passes
- [ ] `mindspec spec list` prints table output
- [ ] `mindspec spec list --json` prints valid JSON

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `mindspec spec list` prints human-readable table | Bead 1 manual verification |
| Each spec shows frontmatter status and beads phase | Bead 1 unit tests |
| `--json` outputs valid JSON array | Bead 1 unit tests + manual verification |
| Output sorted by spec number | Bead 1 unit tests |
| `go test ./internal/speclist/...` passes | Bead 1 verification |
