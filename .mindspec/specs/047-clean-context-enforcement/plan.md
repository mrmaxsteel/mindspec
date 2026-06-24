---
adr_citations:
    - id: ADR-0006
      sections:
        - ADR Fitness
    - id: ADR-0019
      sections:
        - ADR Fitness
approved_at: "2026-02-26T12:00:45Z"
approved_by: user
last_updated: "2026-02-26T15:30:00Z"
spec_id: 047-clean-context-enforcement
status: Approved
version: 4
---

# Plan: 047-clean-context-enforcement — Clean Context Enforcement for Bead Starts

## Overview

When an agent implements multiple beads sequentially in one session, stale context from prior beads degrades output quality. This plan adds a clear gate (`needs_clear` flag) that blocks `mindspec next` after `mindspec complete`, forcing a context reset before the next bead starts. It also replaces the dead spec-scoped context pack system with a bead-scoped context primer that produces focused, lean output for each implementation bead.

**Key design decisions**:

1. **One context path, bead-scoped only.** The existing `contextpack.Build` generates spec-scoped context packs written to disk — but nothing reads them back. This plan removes the dead spec-scoped path entirely and replaces the contextpack package with a bead-focused primer emitted to stdout.

2. **Lean output.** The old context pack included full domain docs (overview, architecture, interfaces, runbook), full ADR bodies, and neighbor interfaces. For bead scope, we include only: bead description, spec requirements + acceptance criteria slice, plan work chunk, key file paths, ADR decision sections (not full bodies), and brief domain overviews.

3. **Primer is always recoverable.** The bead primer is emitted to stdout, never written to disk. To handle interruptions and session recovery, `mindspec instruct` also emits the bead primer when in implement mode with an active bead. This means the primer is available at two points: `mindspec next` (initial transition) and `mindspec instruct` / SessionStart hook (recovery). No file needed.

The implementation has four beads:

1. **State + clear gate** (Bead 1): `needs_clear` flag, gate logic in complete and next
2. **Bead context primer** (Bead 2): Replace contextpack with bead-scoped primer, wire into both `next` and `instruct`
3. **Multi-agent emit-only** (Bead 3): `--emit-only` flag for team lead usage
4. **Hook enforcement** (Bead 4): PreToolUse hook, SessionStart hook update

## ADR Fitness

- **ADR-0006** (Protected main with PR-based merging): Sound. This spec does not change branching or merge behavior. The clear gate operates entirely within the existing worktree lifecycle — it gates context, not code flow. No divergence.
- **ADR-0019** (Deterministic worktree enforcement): Sound. This spec extends the enforcement pattern from ADR-0019 (three layers: git hook, CLI guard, agent hook) to a new concern — context hygiene. The PreToolUse hook (R4) follows the same exit-code-2 blocking pattern established for worktree enforcement. No divergence.

## Testing Strategy

- **Unit tests**: Each bead includes tests for its package. State flag round-trip, gate logic (block/force/skip), primer rendering, instruct-mode primer emission, emit-only path.
- **Integration**: `make test` passes after each bead. `make build` succeeds.
- **Manual verification**: Complete a bead with another ready → `needs_clear` set → `mindspec next` blocked → `/clear` + session restart → `mindspec next` succeeds with bead primer. Also: session restart mid-bead → `mindspec instruct` re-emits the bead primer.

## Bead 1: State flag and clear gate logic

**Provenance**: R1 (Single-agent clear gate), R5 (Graceful degradation / --force)

**Steps**
1. Add `NeedsClear bool` field to `State` struct in `internal/state/state.go` with JSON tag `"needs_clear,omitempty"`.
2. Add `ClearNeedsClear(root string) error` helper in `internal/state/state.go` — reads state, sets `NeedsClear = false`, writes state. This is what the SessionStart hook will call.
3. Add `state clear-flag` subcommand in `cmd/mindspec/state.go` that calls `state.ClearNeedsClear(root)`. CLI entry point for the hook.
4. Modify `internal/complete/complete.go` `Run()`: after the `case state.ModeImplement:` branch in the state advance switch, read state back, set `NeedsClear = true`, and write. Must come after `setModeFn` since it constructs a fresh `State{}`.
5. Modify `cmd/mindspec/next.go` RunE: after finding root and before the clean tree check, read state. If `NeedsClear` is true and `--force` is not set, exit with error: `"Context clear required. Run /clear to reset your context, then retry.\nUse --force to bypass."`. If `--force` is set, print a warning and continue.
6. Add `--force` flag to `nextCmd` in `init()`.
7. Add unit tests:
   - `internal/state/state_test.go`: `NeedsClear` round-trip (set true, read back, clear, read back)
   - `internal/complete/complete_test.go`: verify `NeedsClear` is set when `advanceState` returns `ModeImplement`
   - Test that `NeedsClear` is NOT set when advancing to review/idle/plan

**Verification**
- [ ] `go test ./internal/state/...` passes with NeedsClear round-trip
- [ ] `go test ./internal/complete/...` passes with NeedsClear assertion
- [ ] `mindspec complete` on a bead with a successor → `state.json` shows `"needs_clear": true`
- [ ] `mindspec next` with `needs_clear: true` → exits with error
- [ ] `mindspec next --force` with `needs_clear: true` → proceeds with warning
- [ ] `make test` passes

**Depends on**
None

## Bead 2: Bead context primer (replace spec-scoped context pack)

**Provenance**: R2 (Bead context primer), R6 (Context budget estimation)

This bead removes the dead spec-scoped context pack system and replaces it with a bead-scoped primer emitted to stdout. The primer is available at two points: `mindspec next` (initial bead start) and `mindspec instruct` (session recovery / SessionStart hook). This ensures the primer is always recoverable after interruptions without needing a file on disk.

**Steps**
1. **Remove dead spec-scoped code paths:**
   - Remove the context pack generation from `internal/approve/spec.go` (Step 5: lines 63-71 — the `contextpack.Build` + `WriteToFile` call).
   - Remove `cmd/mindspec/context.go` (the `context pack` subcommand). Will be replaced with a bead-scoped command.
   - Remove `ContextPack.WriteToFile()` from `internal/contextpack/builder.go`.
   - Remove the mode constants (`ModeSpec`, `ModePlan`, `ModeImplement`) and the mode-tiered `addDomainSections` logic from `internal/contextpack/builder.go`.
   - Remove `ContextPack`, `PackSection`, `ProvenanceEntry` structs and `ContextPack.Render()`.
   - Remove the old `Build(root, specID, mode)` function.
   - Keep the reusable helpers: `ParseSpec`, `ReadDomainDocs`, `ParseContextMap`, `ResolveNeighbors`, `ScanADRs`, `FilterADRs`. These are still useful for assembling bead context.

2. **Build the bead primer in `internal/contextpack/`:**
   - Add `BeadPrimer` struct: `BeadID`, `BeadTitle`, `BeadDescription`, `SpecID`, `Requirements` (from spec), `AcceptanceCriteria` (from spec), `PlanWorkChunk` (from plan), `FilePaths []string`, `ADRDecisions []ADRDecision`, `DomainOverviews []DomainOverview`, `EstimatedTokens int`.
   - Add `ADRDecision` struct: `ID string`, `Decision string`.
   - Add `DomainOverview` struct: `Domain string`, `Overview string`.
   - Add `BuildBeadPrimer(root, specID, beadID string) (*BeadPrimer, error)`:
     a. Call `bead.Show(beadID)` to get title and description.
     b. Read `spec.md` — extract `## Requirements` and `## Acceptance Criteria` sections (find header, collect until next `## ` header). Reuse the scanning pattern from `ParseSpec`.
     c. Read `plan.md` — extract the `## Bead ...` section matching the bead title (scan for `## Bead` headers, match by substring from the bead title).
     d. Extract file paths from the plan work chunk (scan for lines containing `internal/`, `cmd/`, `_test.go`, etc.).
     e. Scan ADRs via existing `ScanADRs` + `FilterADRs`, but extract only the `## Decision` section from each (add `ExtractSection(content, heading string) string` helper — find heading, collect until next `## `).
     f. Read domain overviews via existing `ReadDomainDocs` (overview only, skip architecture/interfaces/runbook).
     g. Render via `RenderBeadPrimer`, then set `EstimatedTokens = len(rendered) / 4`.
   - Add `RenderBeadPrimer(p *BeadPrimer) string` — produces markdown:
     ```
     # Bead Context: <BeadTitle>
     **Spec**: <SpecID> | **Bead**: <BeadID> | **~N tokens**

     ## Scope
     <bead description>

     ## Requirements
     <extracted from spec>

     ## Acceptance Criteria
     <extracted from spec>

     ## Work Chunk
     <extracted bead section from plan>

     ## Key File Paths
     - path/to/file.go
     ...

     ## ADR Decisions
     - **ADR-NNNN**: <decision text>
     ...

     ## Domain Context
     ### <domain>
     <overview text>
     ...
     ```

3. **Wire into `mindspec next`:** Modify `cmd/mindspec/next.go` Step 8 — after claiming the bead and updating state, if we have a specID and beadID, call `contextpack.BuildBeadPrimer(root, specID, beadID)`, render, and print to stdout. If the build fails (missing spec, missing plan, bead CLI error), fall back to the existing `emitInstruct(root)`.

4. **Wire into `mindspec instruct`:** Modify `internal/instruct/instruct.go` `BuildContext()` — when `mode == implement` and `ActiveBead != ""` and `ActiveSpec != ""`, call `contextpack.BuildBeadPrimer(root, activeSpec, activeBead)`. If successful, set a new `BeadPrimer string` field on the `Context` struct with the rendered primer output. In `Render()`, when `BeadPrimer` is non-empty, append it after the implement template output (before the Beads context section). This way:
   - `mindspec next` emits the primer at bead start
   - `mindspec instruct` (called by SessionStart hook) re-emits the primer on session recovery
   - Both paths use the same `BuildBeadPrimer` → `RenderBeadPrimer` code

5. **Replace `mindspec context pack` with `mindspec context bead`:** New `cmd/mindspec/context.go` with a `context bead <spec-id> --bead <bead-id>` subcommand that calls `BuildBeadPrimer` and prints to stdout. For manual/debugging use.

6. **Update tests:**
   - Replace `internal/contextpack/builder_test.go`: remove spec/plan/implement mode tests, `WriteToFile` test. Add tests for `BuildBeadPrimer` with mock spec.md and plan.md on disk — verify section extraction, ADR decision extraction, file path extraction, token estimate > 0, graceful degradation when spec.md or plan.md is missing.
   - Keep existing tests for `ParseSpec`, `ReadDomainDocs`, `ParseContextMap`, `ResolveNeighbors`, `ScanADRs`, `FilterADRs`.
   - Update `internal/approve/spec_test.go` if it references context pack generation.
   - Add test in `internal/instruct/instruct_test.go`: when mode=implement with activeBead+activeSpec, verify `BuildContext` populates `BeadPrimer` and `Render` includes it in output.

7. **Clean up stale `context-pack.md` files:** Add a note to doctor checks that existing `context-pack.md` files in spec directories are stale and can be deleted. Do not auto-delete.

**Verification**
- [ ] `go test ./internal/contextpack/...` passes with bead primer tests
- [ ] `mindspec next` emits a bead-specific context primer (bead description, spec requirements, acceptance criteria, plan work chunk, file paths, ADR decisions)
- [ ] `mindspec instruct` in implement mode with an active bead re-emits the same bead primer
- [ ] Primer includes estimated token count
- [ ] Primer does not include domain runbooks, architecture docs, interfaces, or full ADR bodies
- [ ] `mindspec context bead <spec> --bead <id>` produces bead-scoped output to stdout
- [ ] Primer gracefully degrades to generic instruct when spec/plan/bead unavailable
- [ ] `approve spec` no longer generates or writes a context pack file
- [ ] Session recovery: after interruption mid-bead, SessionStart → `mindspec instruct` re-emits primer
- [ ] `make test` passes
- [ ] `make build` succeeds

**Depends on**
Bead 1

## Bead 3: Multi-agent emit-only mode

**Provenance**: R3 (Multi-agent context handoff)

**Steps**
1. Add `--emit-only` flag to `nextCmd` in `cmd/mindspec/next.go` `init()`.
2. In RunE, when `--emit-only` is set:
   - Skip the clear gate check (emit-only is for fresh agents that have no prior context)
   - Query ready beads as normal
   - Accept an optional positional argument as an explicit bead ID. If provided, use `bead.Show(id)` to fetch it instead of querying ready beads.
   - Skip `ClaimBead`, `EnsureWorktree`, state update, and recording
   - Call `contextpack.BuildBeadPrimer(root, specID, beadID)` to produce the primer
   - Print primer to stdout and return
3. Add `FetchBeadByID(id string) (BeadInfo, error)` to `internal/next/beads.go` — calls `bd show <id> --json` and parses a single `BeadInfo`. Needed for the explicit bead ID path.
4. Add unit tests:
   - Test emit-only path prints primer and does not claim bead
   - Test emit-only with explicit bead ID

**Verification**
- [ ] `mindspec next --emit-only` outputs primer to stdout, bead remains unclaimed
- [ ] `mindspec next --emit-only <bead-id>` outputs primer for the specified bead
- [ ] `go test ./internal/next/...` passes
- [ ] `make test` passes

**Depends on**
Bead 2

## Bead 4: Hook enforcement and SessionStart integration

**Provenance**: R4 (Hook enforcement for clear gate)

**Steps**
1. Extend `internal/setup/claude.go` `wantedHooks()`: add a `PreToolUse` entry with matcher `"Bash"`:
   - The hook command reads `.mindspec/state.json` via `jq`
   - Checks if `needs_clear` is `true`
   - Checks if the input command contains `mindspec next` but NOT `--force`
   - If both conditions met, exit 2 with message: `"needs_clear is set. Run /clear to reset your context, then retry mindspec next. Use --force to bypass."`
   - If `needs_clear` is false or command doesn't match, exit 0
2. Update the SessionStart hook command in `internal/setup/claude.go`: prepend `mindspec state clear-flag 2>/dev/null;` before the existing `mindspec instruct` call. This clears the flag on every session start (which happens after `/clear`).
3. Run `mindspec setup claude` to install the updated hooks (document this in verification).
4. Add unit tests:
   - `internal/setup/claude_test.go`: verify `wantedHooks()` includes the new Bash PreToolUse hook
   - Verify SessionStart command includes `state clear-flag`

**Verification**
- [ ] `mindspec setup claude` installs the PreToolUse Bash hook
- [ ] PreToolUse hook blocks `mindspec next` when `needs_clear` is true
- [ ] PreToolUse hook does NOT block `mindspec next --force`
- [ ] PreToolUse hook does NOT block other Bash commands when `needs_clear` is true
- [ ] SessionStart hook clears `needs_clear` flag on session start
- [ ] After `/clear` + session restart, `needs_clear` is false and `mindspec next` proceeds
- [ ] `make test` passes
- [ ] All existing tests pass (`make test`)
- [ ] New unit tests cover clear gate logic and primer generation

**Depends on**
Bead 1

## Dependency Graph

```
Bead 1 (state flag + clear gate)
  ├── Bead 2 (bead context primer, wire into next + instruct)
  │     └── Bead 3 (emit-only mode)
  └── Bead 4 (hook enforcement)
```

Beads 2 and 4 can be worked in parallel since they share only Bead 1 as a dependency. Bead 3 depends on Bead 2 (needs the primer builder).

## Provenance

| Acceptance Criterion | Bead | Verification |
|:---------------------|:-----|:-------------|
| `complete` sets `needs_clear: true` when next bead ready | Bead 1 | state.json shows flag after complete |
| `next` refuses when `needs_clear` set | Bead 1 | Exit with error and instruction |
| `next --force` bypasses clear gate | Bead 1 | Proceeds with warning |
| After `/clear` + SessionStart, `needs_clear` reset | Bead 4 | `state clear-flag` in SessionStart hook |
| `next` emits bead-specific primer | Bead 2 | Primer output includes description, spec slice, plan slice, paths, ADRs |
| `instruct` re-emits primer in implement mode | Bead 2 | SessionStart recovery path emits primer |
| `next --emit-only` outputs primer without claiming | Bead 3 | Bead remains unclaimed, primer on stdout |
| PreToolUse hook blocks `next` when `needs_clear` set | Bead 4 | Hook exits 2, agent sees instruction |
| Primer includes estimated token count | Bead 2 | Token count line in primer output |
| All existing tests pass | All | `make test` green after each bead |
| New unit tests cover gate + primer | Bead 1, 2 | Test files in state, complete, contextpack, instruct packages |

## Risk Notes

- **`setModeFn` overwrites state**: `state.SetMode` constructs a fresh `State{}` without `NeedsClear`. Bead 1 must read state *after* `setModeFn` writes, then set the flag. The two-write sequence (SetMode → read → set flag → write) is safe because complete runs single-threaded.
- **Section extraction is best-effort**: If spec.md or plan.md has non-standard headers, the primer falls back gracefully to generic instruct output. Acceptable for v1.
- **PreToolUse hook matching**: The Bash hook matches `mindspec next` by substring in the command input. Could false-positive on commands like `echo "mindspec next"` — acceptable because the hook only blocks when `needs_clear` is also true, which is a narrow window.
- **`state clear-flag` race**: The SessionStart hook clears the flag before `mindspec instruct` runs. Safe because only `complete` sets the flag.
- **Removing spec-scoped context pack**: Existing `context-pack.md` files on disk become stale. No code reads them, so this is cosmetic — doctor can flag them for cleanup.
- **`instruct` calling `BuildBeadPrimer`**: This adds a `bead.Show()` CLI call to the instruct path, which means `bd` must be available. If `bd` is missing, the primer gracefully degrades to the generic implement template — same as today. The `bead.Show()` call is fast (single issue lookup).
