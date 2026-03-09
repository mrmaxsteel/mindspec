---
adr_citations:
    - id: ADR-0023
      sections:
        - §3
approved_at: "2026-03-08T23:47:29Z"
approved_by: user
bead_ids:
    - mindspec-83zn.6
    - mindspec-83zn.7
    - mindspec-83zn.8
    - mindspec-83zn.9
    - mindspec-83zn.10
last_updated: "2026-03-08"
spec_id: 079-location-agnostic-commands
status: Approved
version: 1
---
# Plan: 079-location-agnostic-commands

## Overview

Five beads: two parallelizable foundations, two dependent command rewrites, one domain docs bead.

```
Bead 1: ResolveSpecPrefix (resolve layer)
Bead 2: CompleteBead worktree fix (execution layer)   ← parallel with 1
Bead 3: mindspec next location-agnostic (workflow layer) ← depends on 1
Bead 4: mindspec complete location-agnostic (workflow layer) ← depends on 1, 2
Bead 5: Domain docs — workflow/execution separation    ← depends on 3, 4
```

## ADR Fitness

- ADR-0023: State derivation from beads queries — this plan extends that principle so CWD is never required for state resolution

---

## Bead 1: `ResolveSpecPrefix` — Integer prefix resolution for `--spec`

**Domain**: Workflow Layer (`internal/resolve/`)

**Problem**: `--spec=077` doesn't work. `ResolveTarget()` passes the flag through verbatim, but `FindEpicBySpecID()` does exact match on the full slug (e.g. `077-execution-layer-interface`). Users must type the full slug.

**Steps**

1. Add `ResolveSpecPrefix(prefix string) (string, error)` to `internal/resolve/target.go`:
   - Call `phase.DiscoverActiveSpecs()` to get all active specs
   - If `prefix` already contains `-` (full spec ID), return as-is
   - If `prefix` is numeric-only (e.g. `"077"`), pad to 3 digits, match against `specID[:3]` for each active spec
   - If exactly one match, return the full spec ID
   - If zero matches, fall back to querying ALL epics (not just active) via existing `queryEpics()` path
   - If multiple matches (shouldn't happen — spec numbers are unique), error with candidates

2. Update `ResolveTarget()` — when `specFlag != ""`, run it through `ResolveSpecPrefix(specFlag)` before returning. This makes prefix resolution automatic for all callers.

3. Add tests to `internal/resolve/target_test.go`:
   - `ResolveSpecPrefix("077")` → `"077-execution-layer-interface"`
   - `ResolveSpecPrefix("077-execution-layer-interface")` → passthrough
   - `ResolveSpecPrefix("999")` → error (no match)

**Acceptance Criteria**
- [ ] `ResolveSpecPrefix("077")` returns the full spec ID matching prefix `077`
- [ ] `ResolveSpecPrefix("077-execution-layer-interface")` passes through unchanged
- [ ] `ResolveSpecPrefix("999")` returns an error when no spec matches
- [ ] `ResolveTarget(root, "077")` resolves the prefix before returning

**Verification**
- [ ] `go test ./internal/resolve/` passes with new prefix resolution tests in `target_test.go`

**Depends on**: None

---

## Bead 2: Fix `CompleteBead()` worktree removal (mindspec-qh1w)

**Domain**: Execution Layer (`internal/executor/`)

**Problem**: `CompleteBead()` in `git.go` calls `WorktreeRemoveFn(wtName)` and `DeleteBranchFn(beadBranch)` without ensuring CWD is outside the worktree. When invoked from inside the bead worktree, `git worktree remove` fails.

**Steps**

1. In `internal/executor/git.go`, `CompleteBead()` (lines 233-243): wrap the worktree removal + branch deletion in `withWorkingDir(g.Root, ...)`, matching the pattern already used by `FinalizeEpic()` at line 308.

2. The wrapped block should call `WorktreeRemoveFn` then `DeleteBranchFn` inside the closure, preserving the existing warning-on-error behavior but ensuring CWD is at repo root.

3. Add test to `internal/executor/git_test.go`: verify `CompleteBead` calls worktree remove after chdir by mocking `WorktreeRemoveFn` to capture the CWD at invocation time and asserting it equals `g.Root`.

**Acceptance Criteria**
- [ ] `CompleteBead()` calls `WorktreeRemoveFn` with CWD at repo root, not inside the bead worktree
- [ ] Bug mindspec-qh1w is closed

**Verification**
- [ ] `go test ./internal/executor/` passes with a test in `git_test.go` verifying CWD during worktree removal

**Depends on**: None

**Bug closure**: Close mindspec-qh1w

---

## Bead 3: `mindspec next` — Location-agnostic with multi-spec prompt

**Domain**: Workflow Layer (`cmd/mindspec/next.go`)

**Steps**

1. **Remove worktree scoping guard** (lines 58-90): Delete the entire `switch kind` block that enforces spec worktree CWD and errors from main/bead worktrees.

2. **Add `--spec` prefix resolution** at the top of RunE, before any other logic:
   ```go
   if specFlag != "" {
       resolved, err := resolve.ResolveSpecPrefix(specFlag)
       if err != nil {
           return err
       }
       specFlag = resolved
   }
   ```

3. **Multi-spec numbered prompt** — replace the hard error at step 1.5 (line 126). When `ResolveTarget` returns `ErrAmbiguousTarget` and no `--spec` flag, emit a numbered list and exit 1:
   ```go
   if _, ok := err.(*resolve.ErrAmbiguousTarget); ok {
       ambErr := err.(*resolve.ErrAmbiguousTarget)
       fmt.Fprintf(os.Stderr, "Multiple active specs have ready beads:\n\n")
       for i, s := range ambErr.Active {
           fmt.Fprintf(os.Stderr, "  %d. %s  (phase: %s)\n", i+1, s.SpecID, s.Mode)
       }
       fmt.Fprintf(os.Stderr, "\nAsk the user which spec to work on, then re-run:\n")
       fmt.Fprintf(os.Stderr, "  mindspec next --spec=<number>\n")
       os.Exit(1)
   }
   ```

4. **Bead worktree warning** — change the hard error at line 88-89 to an informational note:
   ```go
   if kind == workspace.WorktreeBead {
       fmt.Fprintf(os.Stderr, "Note: you're in a bead worktree. Run `mindspec complete <bead-id>` when done.\n")
   }
   ```

5. **Update unmerged-bead guard message** (line 366): change recovery instruction from `--spec=<specID>` to `mindspec complete <bead-id>`.

**Acceptance Criteria**
- [ ] `mindspec next` succeeds when CWD is main repo root (with one active spec)
- [ ] `mindspec next` with multiple active specs and no `--spec` emits a numbered list and exits non-zero
- [ ] `mindspec next --spec=079` targets spec 079 even when CWD is inside a different spec worktree
- [ ] Running from a bead worktree emits an informational note (not a hard error)

**Verification**
- [ ] `go build ./cmd/mindspec/` succeeds
- [ ] `go test ./cmd/mindspec/` passes (if cmd-level tests exist)

**Depends on**: Bead 1

---

## Bead 4: `mindspec complete` — Location-agnostic, impl-only, required bead ID

**Domain**: Workflow Layer (`cmd/mindspec/complete.go`, `internal/complete/complete.go`)

**Steps**

### `cmd/mindspec/complete.go`

1. **Remove worktree scoping guard** (lines 48-66): Delete the entire `switch kind` block.

2. **Require bead ID as positional arg**: Change `Args: cobra.ArbitraryArgs` to `Args: cobra.MinimumNArgs(1)`. Update `Use` to `"complete <bead-id> [commit message...]"`. First arg is always bead ID, remaining args are commit message:
   ```go
   beadID = args[0]
   if len(args) > 1 {
       commitMsg = strings.Join(args[1:], " ")
   }
   ```

3. **Resolve `--spec` prefix**: Same pattern as Bead 3.

4. **Auto-chdir before calling complete.Run**: After resolving bead/spec, chdir to spec worktree or main root:
   ```go
   specWtPath := state.SpecWorktreePath(root, specID)
   if fi, err := os.Stat(specWtPath); err == nil && fi.IsDir() {
       os.Chdir(specWtPath)
   } else {
       os.Chdir(root)
   }
   ```

### `internal/complete/complete.go`

5. **Remove backward-compat shim and auto-resolve** (lines 54-58, 75-95): Delete the spec-as-bead shim (`if beadID != "" && validate.SpecID(beadID) == nil`), the `if beadID == ""` auto-resolve blocks, and the `findRecentClosed` fallback. `beadID` is always provided.

7. **Add impl-only guard**: After resolving the spec, verify the epic phase:
   ```go
   epicID, err := phase.FindEpicBySpecID(specID)
   if err == nil && epicID != "" {
       epicPhase, phaseErr := phase.DerivePhase(epicID)
       if phaseErr == nil && epicPhase != state.ModeImplement && epicPhase != state.ModeReview {
           return nil, fmt.Errorf("bead %s belongs to spec %s which is in '%s' phase.\nmindspec complete is for implementation beads only.", beadID, specID, epicPhase)
       }
   }
   ```

8. **Fix state advancement (mindspec-tzh8)**: Rewrite `advanceState()` to use parent-scoped queries instead of title-based search:
   ```go
   func advanceState(specID string) (mode, nextBead string) {
       epicID, err := phase.FindEpicBySpecID(specID)
       if err != nil || epicID == "" {
           return state.ModeIdle, ""
       }

       // Query truly ready beads (unblocked + open)
       out, err := runBDFn("ready", "--parent", epicID, "--json")
       if err == nil {
           var ready []bead.BeadInfo
           if json.Unmarshal(out, &ready) == nil && len(ready) > 0 {
               return state.ModeImplement, ready[0].ID
           }
       }

       // Check for any open children (blocked or otherwise)
       out, err = listJSONFn("--parent", epicID, "--status=open")
       if err == nil {
           var open []bead.BeadInfo
           if json.Unmarshal(out, &open) == nil && len(open) > 0 {
               return state.ModePlan, ""
           }
       }

       return state.ModeReview, ""
   }
   ```
   Key fix: use `listJSONFn("--parent", epicID, "--status=open")` instead of `runBDFn("search", implPrefix, ...)` to check for remaining open beads. The search-by-title approach was fragile and didn't respect parent scoping.

**Acceptance Criteria**
- [ ] `mindspec complete <bead-id> "msg"` succeeds when CWD is main repo root
- [ ] `mindspec complete <bead-id>` succeeds when CWD is inside the bead worktree being completed
- [ ] `mindspec complete` with no positional arg errors with usage guidance
- [ ] `mindspec complete <bead-id>` on a non-impl bead errors with phase guidance
- [ ] After completing the last unblocked bead with blocked siblings, state advances to `plan` (not `implement`)
- [ ] Bugs mindspec-qh1w and mindspec-tzh8 are closed

**Verification**
- [ ] `go test ./internal/complete/` passes with updated tests for required bead ID, impl-only guard, and state advancement

**Depends on**: Bead 1, Bead 2

**Bug closures**: Close mindspec-qh1w (belt-and-suspenders with Bead 2), close mindspec-tzh8

---

## Bead 5: Domain docs — Workflow/Execution layer separation

**Domain**: Documentation (`docs/domains/`)

**Problem**: The `workflow` domain docs currently bundle everything — worktree management, git operations, beads integration, mode system, approval gates. There is no `execution` domain. The Executor interface (Spec 077) formalized a boundary that the docs don't reflect. The existing domain docs are also outdated (still reference `state.json`, planned interfaces from early specs).

**Steps**

1. **Create `execution` domain** at `.mindspec/docs/domains/execution/`:

   - `overview.md`: Owns all git, worktree, and filesystem operations. The "how" layer — performs operations delegated by the workflow layer. Key packages: `internal/executor/`, `internal/gitutil/`.
   - `architecture.md`: Document the Executor interface pattern, `withWorkingDir` safety, function injection for testability, `GitExecutor` as the concrete implementation.
   - `interfaces.md`: Document the `Executor` interface (current methods: `InitSpecWorkspace`, `HandoffEpic`, `DispatchBead`, `CompleteBead`, `FinalizeEpic`, `Cleanup`, `IsTreeClean`, `DiffStat`, `CommitCount`, `CommitAll`). Document that workflow packages call these methods and MUST NOT import `internal/gitutil/` directly.

2. **Update `workflow` domain** to reflect the separation:

   - `overview.md`: Remove worktree management from "what this domain owns". Add "delegates execution to the Executor interface". Reference the execution domain.
   - `architecture.md`: Update worktree isolation section to reference executor. Add the workflow/execution boundary invariant.
   - `interfaces.md`: Update to reflect current state (ADR-0023 replaced `state.json`, beads adapter is now `internal/bead/`, worktree lifecycle is now via `Executor`). Remove "Planned" markers from implemented interfaces.

3. **Update AGENTS.md**: Add a section documenting the two-layer architecture and the import boundary rule (`approve/`, `complete/`, `next/` → `executor.Executor`, never `gitutil` directly).

**Acceptance Criteria**
- [ ] `execution` domain exists at `.mindspec/docs/domains/execution/` with `overview.md`, `architecture.md`, `interfaces.md`
- [ ] `workflow` domain docs no longer claim ownership of worktree/git operations
- [ ] AGENTS.md documents the workflow/execution layer boundary and the import rule

**Verification**
- [ ] `go test ./internal/doctor/` passes (doctor validates domain doc structure)

**Depends on**: Bead 3, Bead 4 (docs should reflect final implementation)

---

## Testing Strategy

- Unit tests per bead (mock-based for executor, resolve)
- LLM harness regression: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM -timeout 10m`
- Manual validation: run `mindspec next` and `mindspec complete` from main repo root

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `next` from main with one active spec | Bead 3 verification |
| `next` multi-spec numbered prompt | Bead 3 verification |
| `next --spec=077` prefix resolution | Bead 1 + Bead 3 verification |
| `next --spec` overrides CWD | Bead 3 verification |
| `complete <bead-id>` from main | Bead 4 verification |
| `complete` from inside bead worktree | Bead 2 + Bead 4 verification |
| `complete` impl-only guard | Bead 4 verification |
| `complete` no args → error | Bead 4 verification |
| Blocked bead → plan mode | Bead 4 verification |
| LLM harness regression | All beads |
| Workflow/execution domain docs separation | Bead 5 |
