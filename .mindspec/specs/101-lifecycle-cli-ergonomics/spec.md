---
approved_at: "2026-06-16T20:55:42Z"
approved_by: user
status: Approved
---
# Spec 101-lifecycle-cli-ergonomics: Lifecycle CLI ergonomics: mindspec next claim/recovery/diagnostics + spec-create branch base

## Goal

Make four lifecycle CLI commands predictable so agents and humans stop
paying for surprising behavior with manual unwinding and stale work bases:

1. `mindspec next <bead-id>` must claim the **named** bead (or fail loudly),
   never silently claim `items[0]`.
2. A first-class recovery verb (`mindspec release <bead>`) must cleanly
   reverse a wrong claim — no hand-running `bd update ... --assignee ""` +
   `git worktree remove`.
3. `mindspec next` claim failures must surface bd's **real** stderr (so a
   stale-binary schema error is legible), and `mindspec doctor` must catch
   the two root causes (bd-binary-vs-DB schema drift; multiple `bd` on PATH).
4. `mindspec spec create` must branch from `origin/<default-branch>` (after
   a fetch, default branch detected — not hardcoded), so specs never start
   from a stale local base.

## Background

These are four independent, already-triaged defects (GH #146.1-3 and GH #76,
beads `mindspec-mfe0`, `mindspec-hrmw`, `mindspec-jaeg`, `mindspec-k9a8`).
They share a theme — the lifecycle CLI surprises its callers — but no shared
code path, so each ships as its own bead.

Verified in the current tree:

- **R1.** `cmd/mindspec/next.go` (RunE at L38; `runEmitOnly` call at L58)
  threads the positional `args` to the `--emit-only` path **only**. The
  claim path (L203-216) calls `next.SelectWork(items, pick)` with `--pick`
  alone; `internal/next/select.go` `SelectWork` returns `items[0]` when
  `pick == 0` (L26-27), and L208-210 even prints "Defaulting to first item".
  So `mindspec next mindspec-xxxx` (no `--emit-only`) ignores the positional
  and claims the wrong bead — creating a worktree, setting `in_progress`, and
  moving the state cursor. The `next` long help (L37) also wrongly implies the
  positional is accepted generally ("Accepts an optional positional bead ID")
  when it is only honored under `--emit-only`.
- **R2.** No `release`/`abandon`/`unclaim`/`drop` subcommand exists in
  `cmd/mindspec/` (the registered set in `cmd/mindspec/root.go` has no such
  verb). After a mis-claim the user hand-unwinds: `bd update <id> --status
  open --assignee ""` + `git worktree remove`, then relies on the next claim
  to overwrite the state cursor.
- **R3.** `internal/next/beads.go` `ClaimBead` (L162-168) runs
  `bd update --claim` via `CombinedOutput` and, on failure, wraps it as
  `"claim failed (may already be claimed): %s"`. The generic prefix masks the
  real cause — e.g. a stale `bd` emitting `column "depends_on_id" could not be
  found` reads as a benign "already claimed". `internal/doctor/beads.go`
  `checkBdVersionFloor` (L284-317) checks `bd --version >= 1.0.4` but **not**
  schema compatibility, and nothing checks for multiple `bd` binaries on PATH
  (the real root cause was a stale `~/.local/bin/bd` shadowing Homebrew).
- **R4.** `internal/executor/mindspec_executor.go` `InitSpecWorkspace`
  (L85-89) creates the spec branch with `gitutil.CreateBranch(specBranch,
  "HEAD")`, i.e. `git branch -- spec/<id> HEAD` (`internal/gitutil/gitops.go`
  `CreateBranch`, L88-102). A stale local default branch yields a spec
  branched off an out-of-date base. There is no default-branch detection
  helper in `internal/gitutil` today (only `HasRemote`, L225).

## Impacted Domains

- workflow: owns `cmd/**`, `internal/next/**`, and `internal/doctor/**`
  (per `.mindspec/domains/workflow/OWNERSHIP.yaml`). R1 (`cmd/mindspec/next.go`,
  `internal/next/select.go`), R2 (new `cmd/mindspec/release.go` + wiring in
  `cmd/mindspec/root.go`), and R3 (`internal/next/beads.go`,
  `internal/doctor/beads.go`) all land here.
- execution: owns `internal/executor/**` and `internal/gitutil/**`
  (per `.mindspec/domains/execution/OWNERSHIP.yaml`). R2's worktree
  removal routes through the executor's `WorktreeOps`, and R4's branch-base
  change lands in `internal/executor/mindspec_executor.go` +
  `internal/gitutil/gitops.go` (new fetch / default-branch / remote-ref helpers).
- core: owns `internal/redact/**` (per
  `.mindspec/domains/core/OWNERSHIP.yaml`); carries the spec-094
  redaction-allowlist registration of the new `release` command
  (`internal/redact/redact.go` — `redact.CommandTokens`), so the release verb's
  success/friction telemetry events are not silently dropped.

Every source file this spec changes is owned by one of these three declared
domains.

## ADR Touchpoints

- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) (Accepted; Domain(s):
  execution): mutating git/process I/O routes through `internal/executor`;
  enforcement packages may not shell out raw. **Covers the execution domain.**
  Binds R2 (worktree removal via `WorktreeOps.Remove`, not a raw `git worktree
  remove`) and R4 (the new fetch / default-branch-detect / branch-from-remote
  operations stay behind the executor → gitutil seam rather than a raw
  `exec.Command("git", ...)` in command code).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) (Accepted; Domain(s):
  workflow, execution, core): the agent error contract — guard/lifecycle
  failures must tell the agent what to RUN, and emitted output must be legible
  and safe to paste. **Covers the workflow and core domains** (the core domain's
  spec-094 release telemetry registration is in scope of this contract). Binds R3 (a claim failure
  must surface bd's real stderr instead of a misleading generic prefix so the
  agent can act on the true cause) and R2 (the `release` failure/usage messages
  follow the recovery-line convention; the emitted recovery is never a raw
  destructive `bd update ... --metadata`).
- [ADR-0034](../../adr/ADR-0034-ceremony-collapse.md) (Accepted; Domain(s):
  workflow): the single-bead lifecycle epic and `mindspec_phase` metadata
  cache — context for how a claim sets `in_progress` and the state cursor, and
  what R2's `release` must reverse. Supporting, not a per-domain coverage
  requirement.

All three Impacted Domains (workflow, execution, core) are covered by a cited
Accepted ADR (workflow → ADR-0035; execution → ADR-0030; core → ADR-0035,
whose Domain(s) include core). No coverage gap.

## Requirements

1. **R1 — `mindspec next <bead-id>` honors the positional on the claim path**
   (`mindspec-mfe0`, P2, GH #146.1). On the claim (non-`--emit-only`) path,
   a positional bead ID MUST cause `mindspec next` to claim **that** bead, or
   fail with a clear error if it is not found or not ready — never silently
   claim `items[0]`. The `next` long help MUST no longer claim the positional
   is accepted generally; it must accurately describe when the positional is
   honored.

2. **R2 — `mindspec release <bead>` cleanly reverses a claim**
   (`mindspec-hrmw`, P3, GH #146.2). A new `mindspec release <bead>`
   subcommand MUST reverse a claim by performing, **in this order**:
   (1) resolve the repo root and the bead worktree path;
   (2) dirty-check the bead worktree — if it has uncommitted changes, **refuse**
   unless `--force` is given;
   (3) remove the bead worktree via the executor (`WorktreeOps.Remove`), not a
   raw `git worktree remove`, **FIRST** (before any bd/state mutation);
   (4) `os.Chdir` to the resolved repo root **immediately after** removal, so no
   subsequent bd/git subprocess runs from a possibly-deleted cwd (the spec-092
   Req 3c / `mindspec-qxsy` cwd-deletion bug class — see Design Decision);
   (5) set the bead back to `open` and clear the assignee **LAST**;
   (6) rewind the state cursor **only if** it currently points at the released
   bead (a non-active release leaves the cursor untouched).
   The order is **remove-worktree-first, set-open-last** so a partial failure
   leaves a recoverable "still-claimed, worktree-gone" state (re-running
   `release` or `mindspec next` recovers) rather than an "open + stale-worktree-
   collision" state where a re-claim could collide on a leftover worktree path
   (see Design Decision). `--force` is a **mindspec-level pre-gate** (refuse
   before calling `Remove` when dirty), NOT a bd-flag passthrough, because
   `bead.WorktreeRemove` is hardcoded `--force` at the bd-CLI layer.

3. **R3 — surface bd's real stderr on claim failure + doctor schema/PATH
   checks** (`mindspec-jaeg`, P3, GH #146.3). `ClaimBead` failures MUST surface
   bd's real stderr (the captured `CombinedOutput`) rather than flattening it
   under a misleading "may already be claimed" prefix. `mindspec doctor` MUST
   add two checks: (a) bd-binary-vs-DB schema drift, and (b) more than one `bd`
   binary on PATH.

4. **R4 — `mindspec spec create` branches from `origin/<default-branch>`**
   (`mindspec-k9a8`, P3, GH #76). `InitSpecWorkspace` MUST create the spec
   branch from `origin/<default-branch>` after a fetch, with the default branch
   **detected** (not hardcoded `main`). When origin is absent or unreachable
   (offline), it MUST fall back to the current local `HEAD` and emit a WARN.
   The fetch / default-branch detection / branch-from-remote operations MUST
   route through the executor → `internal/gitutil` boundary.

## Design Decisions

- **R1 (honor vs warn): honor.** The positional names a specific bead the
  caller intends to work; the least-surprising behavior is to claim exactly
  that bead. `mindspec next <id>` resolves the named bead and claims it (after
  the existing ready/dirty/freshness guards), or errors if the bead is not
  found or not ready — it never falls through to `items[0]`. (Warn-and-claim-
  first was rejected: it still does the wrong destructive thing.) The help
  text is corrected to state the positional is honored on both the claim path
  (claim that bead) and `--emit-only` (primer for that bead).
  **Positional vs `--pick` precedence (both given): error on conflict.** A
  positional bead ID names a specific bead; `--pick` is an index into the ready
  set. If both are supplied they may disagree, so `mindspec next <id> --pick N`
  is rejected with a clear error rather than silently picking one — the caller
  must supply exactly one selector.

- **R2 (verb name): `mindspec release <bead>`, not `next --abandon`.**
  Release is a distinct, discoverable top-level verb (a flag on `next` would
  overload a command that otherwise *acquires* work). **Dirty-worktree
  handling: refuse by default, allow with `--force`.** Silently discarding
  uncommitted work is unacceptable; the default protects the human's dirt
  (consistent with the spec-092 anti-destructive stance). `--force` is the
  explicit, named opt-out, implemented as a **mindspec-level pre-gate**: because
  `bead.WorktreeRemove` (`internal/bead/bdcli.go`) is already hardcoded
  `bd worktree remove --force` at the bd-CLI layer, mindspec's `--force` cannot
  be a flag passthrough — `release` does its own dirty-check and refuses
  *before* calling `WorktreeOps.Remove` when the worktree is dirty and `--force`
  was not given.

- **R2 (cwd-safety mandate — spec-092 Req 3c / `mindspec-qxsy` bug class).**
  `release` removes the bead worktree, and `release` is expected to be invoked
  from *inside* that very worktree (the natural place an agent realizes it
  mis-claimed). After `WorktreeOps.Remove`, the process would be left in a
  deleted cwd, and every subsequent `bd`/git subprocess (the `bd update --status
  open` mutation and the cursor read/rewind) would silently degrade — the exact
  bug spec 092 fixed in `complete.go` with an explicit `os.Chdir(root)` *after*
  worktree removal. `release` MUST mirror that pattern: **chdir to the resolved
  main repo root (NOT the bead worktree) immediately after `WorktreeOps.Remove`,
  before any post-removal bd/state operation.** The chdir target is the resolved
  root so it never operates from a deleted directory.

- **R2 (ordering — remove-first, set-open-last; atomicity).** The reversal order
  is: resolve root + worktree path → dirty-check (refuse if dirty, no `--force`)
  → `WorktreeOps.Remove` the worktree FIRST → `os.Chdir(root)` → set bead
  `open` (clear assignee) LAST → rewind cursor (only if it points at the
  released bead). **Rationale:** the rejected order (bead→`open` first, then
  Remove) means a failed `Remove` *after* the bead is already `open` leaves an
  open, re-claimable bead pointing at a leftover (possibly dirty) worktree — and
  a subsequent `mindspec next` claim could collide on the existing worktree
  path. Remove-first/set-open-last instead leaves any partial failure in a
  recoverable **"still-claimed, worktree-gone"** state: a re-run of `release`
  (worktree already gone → proceed to set `open`) or `mindspec next` recovers
  cleanly, with no stale-worktree collision.

- **R2 (bd-state mutation reconcile).** The `bd update <id> --status open`
  (assignee cleared) mutation re-exports `issues.jsonl` and auto-commits, which
  on the default branch requires `MINDSPEC_ALLOW_MAIN`-style handling. The impl
  handles the bd-state mutation's commit reconciliation (same pattern
  `complete`/`next` already use), not just a bare `bd update` shell-out.

- **R2 (derived cursor).** Per ADR-0023 the state cursor is DERIVED from bd's
  single `in_progress` child, so setting the released bead back to `open`
  self-rewinds the derived cursor; an explicit cursor write is needed only for
  the `mindspec_phase` metadata cache, which may need a sync (see the resolved
  Open Question on cursor-rewind scope).

- **R3 (error surfacing): pass bd's real stderr through.** The generic
  "may already be claimed" prefix is removed in favor of returning bd's
  captured output verbatim (still wrapped with enough context to know it was a
  claim that failed), so a schema-drift line is legible. Per ADR-0035 the
  message stays paste-safe (no raw destructive recovery emitted). The two new
  doctor checks are additive `Check` entries in `internal/doctor/beads.go`
  (Warn-on-problem, OK/skip otherwise — never false-warn).

- **R4 (default-branch detection + offline fallback): detect via remote, fall
  back to local HEAD with a WARN.** The default branch is detected from the
  remote (e.g. `git remote show origin` / `git symbolic-ref
  refs/remotes/origin/HEAD`) — never hardcoded `main`. Flow: if a remote
  exists, `git fetch` then branch from `origin/<detected-default>`; if origin
  is absent or the fetch fails (offline), branch from local `HEAD` and emit a
  WARN naming the stale-base risk. All of this stays behind the executor →
  gitutil seam (ADR-0030), so command code never shells out raw.
  **Two robustness clarifications:** (a) an empty or garbage
  `git symbolic-ref refs/remotes/origin/HEAD` output MUST fall through to
  `git remote show origin` (the cached ref is not always populated, so an
  unparseable result is treated as a miss, not a default); (b) the offline
  fallback MUST fire on **any** fetch/detect error — offline, auth failure, or
  no-remote all funnel to "branch from local `HEAD` + WARN", and a fetch error
  is **never** a hard `spec create` failure. (An optional opt-out to branch
  from local HEAD deliberately, for intentional unpushed work, is recorded as a
  Non-Goal / possible follow-up below, not in scope here.)

## Scope

### In Scope
- `cmd/mindspec/next.go` — R1 (claim-path positional handling + help text).
- `internal/next/select.go` — R1 (selection when a specific bead is named).
- `cmd/mindspec/release.go` (new) + `cmd/mindspec/root.go` — R2 (register the verb).
- `internal/next/beads.go` — R3 (`ClaimBead` stderr surfacing).
- `internal/doctor/beads.go` — R3 (schema-drift + multiple-bd-on-PATH checks).
- `internal/executor/mindspec_executor.go` — R2 (release worktree removal via
  `WorktreeOps`) and R4 (`InitSpecWorkspace` branch base).
- `internal/gitutil/gitops.go` — R4 (fetch / default-branch detection /
  branch-from-remote helpers).

### Out of Scope
- The panel review gates and ownership-discovery gates (delivered by specs
  099/100); this spec touches only lifecycle CLI ergonomics.
- Any change to the `--emit-only` semantics beyond R1's help-text accuracy
  (its positional handling is already correct).
- The bd binary itself or its schema migrations — R3 only **diagnoses**
  drift, it does not migrate.

## Non-Goals

- This is **four independent CLI fixes**, not a redesign of the lifecycle
  state machine; no change to how `mindspec complete`/`approve` work.
- No change to the review-panel, ownership, or doc-sync gates.
- No new persisted state schema; R2 reuses the existing state-cursor and
  worktree mechanisms in reverse.
- **R4 has no intentional-local-base opt-out** (e.g. a `--from-local-head` flag
  for deliberately unpushed work). The offline fallback already branches from
  local `HEAD` with a WARN when the remote is unreachable; an explicit opt-out
  for *intentional* unpushed bases is a noted possible follow-up, not in scope.

## Acceptance Criteria

All ACs are hermetic and CI-runnable: `go build ./...` plus filtered
`go test -run <Name> -timeout 120s ./...` over `cmd/mindspec`, `internal/next`,
`internal/doctor`, `internal/executor`, `internal/gitutil`. No AC uses the
slow LLM harness (`internal/harness`).

- [ ] **R1.** `go build ./...` succeeds. A `cmd/mindspec` test
  (`rootCmd.SetArgs([]string{"next", "<named-bead>"})` + `Execute`, reusing the
  existing cmd harness with a fake bd/ready-set) asserts that passing a
  positional bead ID claims **that** bead, and a unit test on
  `internal/next.SelectWork` (or its named-bead replacement) asserts that a
  named-but-not-in-ready-set ID returns an error rather than `items[0]`.
- [ ] **R1.** A `cmd/mindspec` help/golden test asserts the `next` long help
  no longer states the positional is "accepted generally" and accurately
  describes when it is honored.
- [ ] **R2.** A `cmd/mindspec` test runs `release <bead>` against an
  injected executor with a fake `WorktreeOps` and asserts: the bead is set to
  `open` with assignee cleared, `WorktreeOps.Remove` is called for the bead
  worktree, and the state cursor is cleared/rewound. A second test asserts a
  **dirty** bead worktree causes `release` to refuse (non-zero, no removal)
  without `--force` and to proceed **with** `--force`. Worktree removal goes
  through the executor (no raw `git worktree remove`).
  *Testability note:* `WorktreeOps` lives on the concrete `*MindspecExecutor`,
  not the `Executor` interface / `MockExecutor`, so these tests realize an
  injected fake via `&MindspecExecutor{WorktreeOps: fake}` (as
  `internal/executor/executor_test.go` already does) — or, as an impl wiring
  choice, by adding a worktree-removal method to the executor interface.
- [ ] **R2 (cwd-safety, spec-092 Req 3c).** A test runs `release` with the
  process cwd set **inside** a *real* temp-git bead worktree (built with a
  `newRepoExecutor`-style real-repo executor, NOT the fake `WorktreeOps`),
  and asserts that after the worktree is removed the cwd is recovered
  (chdir-to-root) and a subsequent operation (e.g. the post-removal bd state
  read) succeeds. This AC requires a real executor because the fake-`WorktreeOps`
  AC above structurally CANNOT catch the cwd-deletion footgun — the fake does
  not actually remove the directory, so the process is never left in a deleted
  cwd.
- [ ] **R3.** A unit test on `internal/next.ClaimBead` (injecting a fake bd
  that fails with a schema-drift stderr) asserts the returned error contains
  bd's real stderr text (e.g. `depends_on_id`) and is NOT flattened to the
  bare "may already be claimed" string.
- [ ] **R3.** Unit tests on the two new `internal/doctor` checks assert: (a)
  a simulated bd-binary-vs-DB schema mismatch yields a `Warn` check, and (b)
  two `bd` binaries on a fabricated PATH yield a `Warn` check; the happy path
  yields `OK`/skip (no false warn).
- [ ] **R4.** A unit test on `internal/executor.InitSpecWorkspace` (or the new
  gitutil helper) asserts that, with a remote present, the spec branch is
  created from `origin/<detected-default-branch>` after a fetch — the default
  branch is detected, not hardcoded `main`. A second test asserts the
  offline/no-origin path falls back to local `HEAD` and emits a WARN.

## Validation Proofs

- `mindspec validate spec 101-lifecycle-cli-ergonomics`: 0 errors (a
  lifecycle-binding WARN before spec approve is acceptable). Confirms ParseSpec
  extracts the declared domains `[workflow execution core]` (not `[]`).
- `go build ./...`: succeeds.
- `go test -run TestNext -timeout 120s ./cmd/mindspec/...`,
  `go test -run TestSelectWork -timeout 120s ./internal/next/...`,
  `go test -run TestRelease -timeout 120s ./cmd/mindspec/...`,
  `go test -run TestClaimBead -timeout 120s ./internal/next/...`,
  `go test -run TestCheckBd -timeout 120s ./internal/doctor/...`,
  `go test -run TestInitSpecWorkspace -timeout 120s ./internal/executor/...`:
  the R1-R4 acceptance tests pass.

## Open Questions

- [x] R2 state-cursor rewind scope — **resolved**: `release` rewinds the
  cursor only when it points at the released bead; releasing a non-active bead
  leaves the cursor untouched (no surprise re-targeting). Per ADR-0023 the
  cursor is derived from bd's single `in_progress` child, so setting the bead
  back to `open` self-rewinds the derived cursor; the explicit step covers
  syncing the `mindspec_phase` metadata cache. The panel may revisit.
- [x] R4 default-branch detection source — **resolved**: try the cached
  `git symbolic-ref refs/remotes/origin/HEAD` first (cheap), fall back to
  `git remote show origin` on the fetch this requirement already performs. The
  panel may revisit.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-16
- **Notes**: Approved via mindspec approve spec