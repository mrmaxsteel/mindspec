---
approved_at: "2026-03-08T16:35:44Z"
approved_by: user
status: Approved
---
# Spec 075-beads-as-context: Beads as Context Authority — Self-Contained Work Units with Decision Links

## Goal

Introduce a **decision access interface** that abstracts ADR storage behind a common API, keeping file-based markdown as the default backend while enabling a future beads/Dolt backend. All mindspec consumers (contextpack, validate, instruct, approve) access decisions through this interface, not through direct filesystem or beads calls.

This is the second step (after Spec 074) toward beads as the universal context substrate. It preserves the current developer experience (browse ADRs as markdown in VSCode) while laying the foundation for the enterprise knowledge layer described in the companion vision document.

## Background

### Current state

MindSpec maintains its own ADR system:
- `internal/adr/` — Go package: `parse.go`, `create.go`, `supersede.go`, `list.go`, `show.go` (+ tests)
- `cmd/mindspec/adr.go` — CLI subcommands (`adr create`, `adr list`, `adr show`)
- `.mindspec/docs/adr/ADR-NNNN.md` — 24 markdown files with convention-based metadata parsing
- `internal/contextpack/adr.go` — ADR scanning for context pack generation
- `internal/validate/plan.go` — ADR citation validation via file scanning

This system parses metadata from markdown conventions (`**Status**:`, `**Domain(s)**:`, `**Supersedes**:`), generates sequential IDs by scanning filenames, and manages supersede chains by mutating file content. Consumers access ADR files directly — `contextpack` reads files, `validate` scans files, `instruct` references file paths.

### The UX tension: git vs Dolt for decisions

The vision document (`~/enterprise-knowledge/vision-enterprise-knowledge.md`) proposes migrating ADRs to beads `decision` issues in Dolt. At enterprise scale this is clearly right — structured queryability, federation, dependency graphs. But the current developer experience is excellent: open `ADR-NNNN.md` in VSCode, hit preview, read nicely rendered markdown. This is fast, zero-infrastructure, works offline, and leverages the editor the developer is already in.

Moving ADRs to Dolt would break this workflow. The alternatives (terminal output via `bd show`, a VSCode extension, self-hosted DoltLab) are all worse for a solo developer or small team. See [ADR-0024](../../adr/ADR-0024.md) for the full analysis.

**Resolution**: Interface-first, file-based default. Consumers talk to an abstraction layer. The default implementation reads markdown files. A future implementation reads from beads. The switchover is a configuration change, not a code rewrite.

### What beads already provides (future backend)

Beads has a first-class `decision` issue type (`internal/types/types.go:431`):
- **Type aliases**: `--type adr` and `--type dec` both normalize to `decision`
- **Structured sections** enforced at creation: `## Decision`, `## Rationale`, `## Alternatives Considered`
- **`supersedes` dependency type** for version chains
- **`related` dependency type** for linking decisions to affected work
- **Queryable**: `bd list --type decision`, `bd search "keyword" --type decision`
- **Metadata**: arbitrary JSON for structured attributes (e.g., `{"domains": ["workflow", "git"]}`)
- **Dolt versioning**: cell-level history, branching, time-travel, sync

This is the target backend for enterprise use, but the file-based backend remains the default until the UX gap is closed.

### Relationship to Spec 074

Spec 074 snapshots ADR Decision sections as text into the bead `design` field at plan approval. This spec introduces the interface that Spec 074's snapshot logic should use for ADR access, and (in future phases) enables replacing the text snapshot with `related` dependency links to beads decision issues.

### Forward-looking: enterprise knowledge layer

This interface is the foundation for a portfolio-wide knowledge layer where decisions are shared across repositories via Dolt federation. See the companion vision document (`~/enterprise-knowledge/vision-enterprise-knowledge.md`) for the full architectural picture, and [ADR-0024](../../adr/ADR-0024.md) for why the interface-first approach was chosen.

## Impacted Domains

- adr: `internal/adr/` package refactored behind a decision access interface
- approve: Plan approval uses the interface for ADR lookups (replaces direct file access)
- validate: Plan ADR citation checks use the interface instead of file scans
- contextpack: `internal/contextpack/adr.go` uses the interface instead of direct file reads

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md): Extends beads-as-authority principle to architectural decisions
- [ADR-0024](../../adr/ADR-0024.md): ADR Storage Abstraction — interface-first, file-based default (governs this spec's approach)

## Requirements

### Decision access interface

1. A `DecisionStore` interface (or equivalent) must be defined with at minimum: `List(filter) []Decision`, `Get(id) Decision`, `Search(query) []Decision`
2. The `Decision` type must capture: ID, title, status, domains, content (full text or sections), supersedes/superseded-by links
3. The interface must be backend-agnostic — no file paths or beads IDs leak into the contract

### File-based backend (default)

4. A file-based implementation reads from `.mindspec/docs/adr/ADR-NNNN.md` markdown files
5. This implementation extracts existing code from `internal/adr/` (parse, list, show) and wraps it behind the interface
6. The file-based backend is the default — no configuration needed, works out of the box
7. The developer's VSCode browse-and-preview workflow is unchanged

### Consumer migration

8. `internal/contextpack/adr.go` must use the interface instead of direct file reads
9. `internal/validate/plan.go` must use the interface for ADR citation checks
10. `internal/approve/plan.go` must use the interface for ADR lookups at plan approval
11. `cmd/mindspec/adr.go` CLI subcommands must delegate to the interface

### Forward compatibility

12. The interface design must accommodate a future beads backend (`bd list --type decision`, `bd show <id>`) without breaking the contract
13. Decision metadata schema should support optional `domains` (string array) for domain-based filtering — this works for both backends
14. The beads backend implementation itself is **out of scope** — it will be added when the UX gap (browsability in editor) is addressed

## Scope

### In Scope
- New `DecisionStore` interface definition (new package or within `internal/adr/`)
- File-based backend implementation (refactoring existing `internal/adr/` code behind the interface)
- `cmd/mindspec/adr.go` — delegate to the interface
- `internal/contextpack/adr.go` — use the interface
- `internal/validate/plan.go` — use the interface for ADR citation checks
- `internal/approve/plan.go` — use the interface for ADR lookups

### Out of Scope
- Beads `decision` backend implementation (future work, when editor-browsability UX is addressed)
- ADR migration to beads (depends on beads backend)
- Enterprise knowledge layer / portfolio federation (future work, see vision doc)
- DAG navigation and cross-repo decision scoping (future work)
- Changes to spec.md or plan.md file formats
- Deleting `internal/adr/` — the code is refactored behind the interface, not deleted

## Non-Goals

- Replacing file-based ADRs with beads decisions (this spec introduces the interface; the backend swap is future work per ADR-0024)
- Cross-repo decision sharing (enterprise knowledge layer, future work)
- Deleting ADR markdown files — they remain the default storage format

## Acceptance Criteria

### Interface
- [ ] `DecisionStore` interface exists with `List`, `Get`, `Search` methods
- [ ] `Decision` type captures ID, title, status, domains, content, supersede links

### File-based backend
- [ ] File-based implementation passes all existing ADR tests
- [ ] `mindspec adr list` works unchanged (backed by interface)
- [ ] `mindspec adr show <id>` works unchanged (backed by interface)
- [ ] `mindspec adr create` works unchanged (backed by interface)
- [ ] ADR markdown files in `.mindspec/docs/adr/` remain the source of truth

### Consumer migration
- [ ] `internal/contextpack/adr.go` uses the interface — no direct `os.ReadFile` or file path construction
- [ ] `internal/validate/plan.go` uses the interface for ADR citation checks
- [ ] `internal/approve/plan.go` uses the interface for ADR lookups
- [ ] All existing tests pass with the refactored code

### Forward compatibility
- [ ] A second implementation of `DecisionStore` (e.g., a test mock or stub beads backend) can be swapped in without changing any consumer code

## Validation Proofs

- `go test ./internal/adr/...`: interface and file-based backend tests pass
- `go test ./internal/contextpack/...`: context pack ADR integration passes
- `go test ./internal/validate/...`: plan validation ADR citation checks pass
- `mindspec adr list`: output unchanged from current behavior
- `mindspec adr show ADR-0024`: renders the new ADR correctly

## Open Questions

- [x] Should the interface live in `internal/adr/` (refactored in place) or a new `internal/decision/` package (cleaner name for the dual-backend future)? **Resolved**: Keep in `internal/adr/`. The package already owns the types and code. A rename adds churn with no value until the beads backend exists. Can rename later if needed.
- [x] Should `Search` accept structured filters (by domain, status) or just a text query? **Resolved**: Text query only (`Search(query string) []ADR`). The existing `List` already supports status/domain filtering. Structured search is premature — matches `bd search` contract.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec