---
status: Approved
spec_id: 007-beads-tooling
version: "1.0"
last_updated: 2026-02-12
approved_at: 2026-02-12T12:00:00Z
approved_by: user
adr_citations:
  - id: ADR-0002
    sections: ["Spec Beads Pattern", "Active Workset Discipline", "Parallelism Compatibility"]
  - id: ADR-0004
    sections: ["Go as v1 CLI Implementation Language"]
work_chunks:
  - id: 1
    title: "bdcli wrapper + preflight checks"
    scope: "internal/bead/bdcli.go, internal/bead/bdcli_test.go"
    verify:
      - "BeadInfo JSON round-trips correctly for bd output format"
      - "Preflight() fails with actionable message when .beads/ missing"
      - "Preflight() fails when bd not on PATH"
      - "Create() constructs correct bd create args including --parent when provided"
      - "make test passes"
    depends_on: []
  - id: 2
    title: "Spec bead creation"
    scope: "internal/bead/spec.go, internal/bead/spec_test.go"
    verify:
      - "Creates bead with [SPEC <id>] title prefix and structured description"
      - "Rejects unapproved specs with descriptive error"
      - "Description ≤400 chars"
      - "Returns existing bead on re-run (idempotent)"
      - "make test passes"
    depends_on: [1]
  - id: 3
    title: "Plan bead creation"
    scope: "internal/bead/plan.go, internal/bead/plan_test.go"
    verify:
      - "Parses work_chunks YAML correctly"
      - "Creates one bead per chunk with [IMPL <spec-id>.<id>] title"
      - "Wires dependencies via bd dep add"
      - "WriteGeneratedBeadIDs() preserves existing frontmatter"
      - "No duplicates on re-run"
      - "make test passes"
    depends_on: [1]
  - id: 4
    title: "Worktree management"
    scope: "internal/bead/worktree.go, internal/bead/worktree_test.go"
    verify:
      - "Parses multi-worktree porcelain output correctly"
      - "Matches worktree by path or branch convention"
      - "Refuses creation when bead not in_progress or tree dirty"
      - "make test passes"
    depends_on: [1]
  - id: 5
    title: "Hygiene audit"
    scope: "internal/bead/hygiene.go, internal/bead/hygiene_test.go"
    verify:
      - "Identifies stale, orphaned, oversized beads"
      - "Reports total open count vs recommended max"
      - "--fix defaults to dry-run, only executes with --yes"
      - "make test passes"
    depends_on: [1]
  - id: 6
    title: "CLI wiring + doc-sync"
    scope: "cmd/mindspec/bead.go, cmd/mindspec/root.go, docs/templates/plan.md, CLAUDE.md, docs/core/CONVENTIONS.md"
    verify:
      - "./bin/mindspec bead --help shows all four subcommands"
      - "Exit codes: 0 success, 1 validation, 2 bd error"
      - "CLAUDE.md and CONVENTIONS.md updated"
      - "make build && make test passes"
    depends_on: [2, 3, 4, 5]
---

# Plan: Spec 007 — Beads Integration Conventions + Tooling

**Spec**: [spec.md](spec.md)

---

## Design Notes

### Package Architecture

New `internal/bead/` package, self-contained with zero imports from other `internal/` packages (except `workspace` for path helpers). This avoids coupling to `internal/next/` which has different concerns (work selection vs bead lifecycle).

`BeadInfo` struct is defined independently in `internal/bead/bdcli.go` — structurally identical to `next.BeadInfo` but scoped to this package. If deduplication becomes important later, a shared types package can be introduced.

### Testability via execCommand Pattern

All `bd` and `git` CLI invocations go through a package-level variable:

```go
var execCommand = exec.Command
```

Tests override this to capture arguments or return canned output. This is a standard Go pattern for testing exec calls without requiring real `bd` or `git` in the test environment.

### Bead Title Conventions (Idempotency Keys)

- Spec beads: `[SPEC <spec-id>] <title>` — e.g., `[SPEC 006-validate] Workflow Validation`
- Impl beads: `[IMPL <spec-id>.<chunk-id>] <chunk-title>` — e.g., `[IMPL 007-beads-tooling.1] bdcli wrapper`

Idempotent lookup uses `bd search "[SPEC <spec-id>]" --json --status=open`. The bracket prefix ensures reliable matching. `--status=open` avoids matching closed beads from previous runs.

### Structured Spec Bead Description

```
Summary: <goal, first sentence, ≤120 chars>
Spec: docs/specs/<id>/spec.md
Domains: <comma-separated>
```

Total cap: 400 chars. This enforces ADR-0002's "concise index entries" rule.

### Structured Impl Bead Description

```
Scope: <chunk scope>
Verify:
- <v1>
- <v2>
Plan: docs/specs/<id>/plan.md
```

Total cap: 800 chars.

### Plan Frontmatter Writeback

`WriteGeneratedBeadIDs()` writes bead IDs under a `generated:` key to separate machine-written metadata from human-authored plan content. This does not invalidate plan approval.

Strategy: read file → extract frontmatter into `map[string]interface{}` via yaml.v3 → set `generated.bead_ids` → re-marshal frontmatter → splice back preserving body content after closing `---`.

### Dependency Wiring

Uses `bd dep add <blocked> <blocker>` (not `--blocks`). If chunk 2 depends on chunk 1, call `bd dep add <bead-2-id> <bead-1-id>`.

### Hygiene Heuristics

- **Stale**: `updated_at` older than threshold (default 7 days, `--stale-days` flag)
- **Orphaned**: title has no `[SPEC` or `[IMPL` prefix (beads created outside `mindspec bead` conventions)
- **Oversized**: description > 400 chars for `[SPEC` beads, > 800 for others

### Exit Code Convention

- 0: success (created, found existing, or clean report)
- 1: validation failure (unapproved spec/plan, missing prerequisites, dirty tree)
- 2: Beads CLI error (bd command failed)

---

## Bead 007-A: bdcli wrapper + preflight checks

**Scope**: `internal/bead/bdcli.go`, `internal/bead/bdcli_test.go` — thin wrapper around `bd` CLI, shared by all subcommands

**Steps**:
1. Create `internal/bead/bdcli.go` with `BeadInfo` struct (ID, Title, Description, Status, Priority, IssueType, Owner, CreatedAt, UpdatedAt) and `var execCommand = exec.Command`
2. Implement `Preflight(root string) error` — checks git repo, `.beads/` exists, `bd` on PATH; returns first failure with remediation message
3. Implement `Create(title, desc, issueType string, priority int, parent string) (*BeadInfo, error)` — builds and runs `bd create`, parses JSON
4. Implement `Search(query string) ([]BeadInfo, error)` — `bd search <query> --json --status=open`
5. Implement helpers: `Show(id)`, `ListOpen()`, `DepAdd(blocked, blocker)`, `Update(id, status)`
6. Write tests: JSON parsing, Preflight failures, argument construction via execCommand mock

**Verification**:
- [ ] `BeadInfo` JSON round-trips correctly for `bd` output format
- [ ] `Preflight()` fails with actionable message when `.beads/` missing
- [ ] `Preflight()` fails when `bd` not on PATH
- [ ] `Create()` constructs correct `bd create` args including `--parent` when provided
- [ ] `Search()` passes `--status=open` to avoid matching closed beads
- [ ] `make test` passes

**Depends on**: nothing

---

## Bead 007-B: Spec bead creation

**Scope**: `internal/bead/spec.go`, `internal/bead/spec_test.go`

**Steps**:
1. Create `CreateSpecBead(root, specID string) (*BeadInfo, error)` — full orchestration
2. Implement approval validation: read spec.md, check for `Status: APPROVED` or `**Status**: APPROVED` in Approval section
3. Implement content extraction: title from `# Spec NNN:` heading, goal summary (≤120 chars), domains from `## Impacted Domains`
4. Build structured description (`Summary:/Spec:/Domains:`) capped at 400 chars
5. Idempotent lookup: `Search("[SPEC <spec-id>]")` before creating; return existing if found
6. Create via `Create("[SPEC <spec-id>] <title>", description, "feature", 2, "")`
7. Write tests: approved → creates, unapproved → error, description format/cap, idempotent re-run

**Verification**:
- [ ] Creates bead with `[SPEC <id>]` title prefix and structured description
- [ ] Rejects unapproved specs with descriptive error
- [ ] Handles both `Status: APPROVED` and `**Status**: APPROVED` formats
- [ ] Description ≤400 chars
- [ ] Returns existing bead on re-run (idempotent)
- [ ] `make test` passes

**Depends on**: 007-A

---

## Bead 007-C: Plan bead creation

**Scope**: `internal/bead/plan.go`, `internal/bead/plan_test.go`

**Steps**:
1. Define `WorkChunk`, `PlanMeta`, `Generated` structs for YAML parsing
2. Implement `ParsePlanMeta(planPath string) (*PlanMeta, error)` — extract YAML between `---` fences, unmarshal with yaml.v3
3. Implement `CreatePlanBeads(root, specID string) (map[int]string, error)` — validate approved + work_chunks present, find spec bead as parent, loop chunks with idempotent lookup, create missing, wire deps, return mapping
4. Build impl bead descriptions (`Scope:/Verify:/Plan:`) capped at 800 chars
5. Implement `WriteGeneratedBeadIDs(planPath string, mapping map[int]string) error` — round-trip frontmatter via `map[string]interface{}`, set `generated.bead_ids`, splice back
6. Write tests: parsing, approval rejection, missing work_chunks, idempotent creation, dep wiring, frontmatter writeback

**Verification**:
- [ ] Parses `work_chunks` YAML correctly
- [ ] Creates one bead per chunk with `[IMPL <spec-id>.<id>]` title
- [ ] Wires dependencies via `bd dep add`
- [ ] Sets spec bead as parent when found
- [ ] Rejects unapproved plans and missing `work_chunks`
- [ ] `WriteGeneratedBeadIDs()` preserves existing frontmatter
- [ ] No duplicates on re-run
- [ ] `make test` passes

**Depends on**: 007-A

---

## Bead 007-D: Worktree management

**Scope**: `internal/bead/worktree.go`, `internal/bead/worktree_test.go`

**Steps**:
1. Define `WorktreeEntry` struct (Path, Branch, HEAD)
2. Implement `ParseWorktreeList() ([]WorktreeEntry, error)` — parses `git worktree list --porcelain`
3. Implement `FindWorktree(beadID string) (string, error)` — matches path ending in `worktree-<beadID>` or branch `bead/<beadID>`
4. Implement `CreateWorktree(root, beadID string) (string, error)` — validates bead `in_progress`, validates clean tree, runs `git worktree add`
5. Write tests: porcelain parsing, matching, refusal on dirty tree / wrong status

**Verification**:
- [ ] Parses multi-worktree porcelain output correctly
- [ ] Matches worktree by path or branch convention
- [ ] Returns empty string when no match found
- [ ] Refuses creation when bead not `in_progress`
- [ ] Refuses creation when tree is dirty
- [ ] `make test` passes

**Depends on**: 007-A

---

## Bead 007-E: Hygiene audit

**Scope**: `internal/bead/hygiene.go`, `internal/bead/hygiene_test.go`

**Steps**:
1. Define `HygieneReport` struct (Stale, Orphaned, Oversized `[]BeadInfo`, TotalOpen, RecommendedMax int)
2. Implement `AuditWorkset(staleDays int) (*HygieneReport, error)` — ListOpen(), categorize each bead
3. Implement `FormatReport(r *HygieneReport) string` — sections per category with suggested `bd` commands
4. Implement `FixHygiene(dryRun bool) ([]string, error)` — finds `done`-labeled beads, closes if not dryRun
5. Write tests: stale/orphan/oversized detection, report formatting, dry-run behavior

**Verification**:
- [ ] Identifies stale beads based on configurable threshold
- [ ] Identifies orphaned beads (no convention prefix)
- [ ] Identifies oversized descriptions (>400 spec, >800 impl)
- [ ] Reports total open count vs recommended max
- [ ] `--fix` defaults to dry-run, only executes with `--yes`
- [ ] `make test` passes

**Depends on**: 007-A

---

## Bead 007-F: CLI wiring + doc-sync

**Scope**: `cmd/mindspec/bead.go`, `cmd/mindspec/root.go`, `docs/templates/plan.md`, `CLAUDE.md`, `docs/core/CONVENTIONS.md`

**Steps**:
1. Create `cmd/mindspec/bead.go`: parent `beadCmd` + four child commands, following `validate.go` pattern
2. Wire `beadSpecCmd` (ExactArgs(1)): findRoot → Preflight → CreateSpecBead → print ID
3. Wire `beadPlanCmd` (ExactArgs(1)): findRoot → Preflight → CreatePlanBeads → WriteGeneratedBeadIDs → print mapping
4. Wire `beadWorktreeCmd` (ExactArgs(1), `--create` flag): findRoot → Preflight → FindWorktree or CreateWorktree
5. Wire `beadHygieneCmd` (NoArgs, `--stale-days`, `--fix`, `--yes` flags): findRoot → Preflight → AuditWorkset → FormatReport; if --fix: FixHygiene
6. Register `beadCmd` in `root.go` init()
7. Update `docs/templates/plan.md`: add `work_chunks` example and `generated:` placeholder
8. Update `CLAUDE.md`: add `mindspec bead` to command table and Build & Run
9. Update `docs/core/CONVENTIONS.md`: bead title conventions, `work_chunks` format, `generated.bead_ids`

**Verification**:
- [ ] `./bin/mindspec bead --help` shows all four subcommands
- [ ] `./bin/mindspec bead spec 006-validate` creates or returns existing spec bead
- [ ] `./bin/mindspec bead hygiene` produces audit report
- [ ] Exit codes: 0 success, 1 validation, 2 bd error
- [ ] `docs/templates/plan.md` includes `work_chunks` block
- [ ] `CLAUDE.md` and `CONVENTIONS.md` updated
- [ ] `make build && make test` passes

**Depends on**: 007-B, 007-C, 007-D, 007-E

---

## Dependency Graph

```
007-A (bdcli wrapper + preflight)
  ├── 007-B (spec bead creation)
  ├── 007-C (plan bead creation)
  ├── 007-D (worktree management)
  └── 007-E (hygiene audit)
        ↓ all four
      007-F (CLI wiring + doc-sync)
```

B/C/D/E are parallelizable after A.

---

## End-to-End Verification

```bash
make build && make test
./bin/mindspec bead --help
./bin/mindspec bead spec 006-validate    # creates or returns existing
./bin/mindspec bead spec 006-validate    # idempotent — same ID
./bin/mindspec bead hygiene              # audit report
./bin/mindspec bead hygiene --fix        # dry-run output
```
