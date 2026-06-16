---
adr_citations:
    - ADR-0030
    - ADR-0035
approved_at: "2026-06-16T21:05:27Z"
approved_by: user
bead_ids:
    - mindspec-3cj0.1
    - mindspec-3cj0.2
    - mindspec-3cj0.3
    - mindspec-3cj0.4
spec_id: 101-lifecycle-cli-ergonomics
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
    - depends_on: []
      id: 2
    - depends_on: []
      id: 3
    - depends_on: []
      id: 4
---
# Plan: 101-lifecycle-cli-ergonomics

## ADR Fitness

This plan applies two Accepted ADRs that together cover both Impacted Domains
(`workflow`, `execution`); a third (ADR-0034) is supporting context only.

- **ADR-0030 (Executor boundary, Accepted; Domain(s): execution)** — mutating
  git/process I/O routes through `internal/executor`; enforcement/command
  packages may not shell out raw. **Covers the `execution` domain.** Binds Bead 2
  (`release` removes the bead worktree via `WorktreeOps.Remove`, never a raw
  `git worktree remove`) and Bead 4 (the new fetch / default-branch-detect /
  branch-from-remote operations stay behind the `executor → gitutil` seam rather
  than an `exec.Command("git", ...)` in `cmd/mindspec/`).
- **ADR-0035 (Agent error contract, Accepted; Domain(s): workflow, execution,
  core)** — guard/lifecycle failures must tell the agent what to RUN, and emitted
  output must be legible and paste-safe. **Covers the `workflow` domain.** Binds
  Bead 3 (a claim failure surfaces bd's real stderr instead of a misleading
  generic prefix so the agent can act on the true cause) and Bead 2 (the
  `release` usage/failure messages follow the recovery-line convention; the
  emitted recovery never contains a raw destructive `bd update ... --metadata`).

ADR-0034 (Ceremony collapse, Accepted; Domain(s): workflow) is *supporting*
context — it explains how a claim sets `in_progress` and the derived state cursor
(ADR-0023) that Bead 2's `release` must reverse. ADR-0034's `Domain(s)` does
intersect the impacted `workflow` domain, so citing it would not trip
`adr-cite-irrelevant`; it is deliberately left out of `adr_citations` because the
coverage-relevant set is exactly ADR-0030 (execution) + ADR-0035 (workflow), and
citations are kept to the minimal covering set.

## Decomposition rationale

The spec's four requirements (R1 `mfe0`, R2 `hrmw`, R3 `jaeg`, R4 `k9a8`) are
**four independently-triaged defects with no shared runtime output** — no bead
consumes another bead's emitted value, so every `depends_on` is empty. This maps
cleanly to **4 beads, one per requirement**, which the mindspec plan heuristics
endorse:

- **Independence / target 3–5:** four genuinely independent fixes → four chunks,
  inside the 3–5 sweet spot. Merging any two would couple unrelated defects (e.g.
  the `next`-help fix to the `release` verb) for no integration benefit.
- **No serial chain:** the dependency graph is four roots, depth 1 — well under
  the serial-chain ≤ 3 guard. They can be cycled in any order or in parallel.
- **No trivial-work splinters:** each bead carries real, separable impl + a
  distinct RED→GREEN test surface in a different package; none is a one-line
  splinter that should fold into a sibling.
- **Merge-ORDER signals, NOT `depends_on` edges (mirrors spec 100's `plan.go`):**
  two file overlaps exist but neither is a data dependency, so neither becomes an
  edge — they are recorded as a preferred merge order so two subagents don't
  collide on the same file:
  - **Bead 2 and Bead 4 both edit `internal/executor/mindspec_executor.go`**, but
    DIFFERENT functions: Bead 2 adds a release/worktree-removal path; Bead 4 edits
    `InitSpecWorkspace` (the branch base, ~L85). No line overlap. **Preferred
    merge order: Bead 4 before Bead 2** (Bead 4's `InitSpecWorkspace` edit is
    localized; Bead 2 adds new surface area around it), but order is advisory —
    the functions don't touch.
  - **Bead 2 and Bead 3 both relate to `internal/next/beads.go`:** Bead 3 edits
    `ClaimBead`; Bead 2 needs a `bd update --status open`/clear-assignee mutation.
    To avoid a collision, **Bead 2's bd-state mutation lands in the new
    `cmd/mindspec/release.go` (or a new `internal/release` helper), NOT in
    `beads.go`** — Bead 3 keeps `beads.go` to itself. Recorded as a merge-order /
    file-placement preference, not an edge.

### Dependency graph (`work_chunks`)

```
Bead 1 (mfe0, R1)  depends_on: []
Bead 2 (hrmw, R2)  depends_on: []
Bead 3 (jaeg, R3)  depends_on: []
Bead 4 (k9a8, R4)  depends_on: []
```

Preferred (advisory) merge order to minimize file-touch overlap:
**Bead 1 → Bead 3 → Bead 4 → Bead 2** (substantive `release` bead last, after
its `executor` co-editor Bead 4 and its `beads.go` neighbor Bead 3 have landed).

## Testing Strategy

Every test is hermetic and CI-runnable: `go build ./...` plus filtered
`go test -run <Name> -timeout 120s ./<pkg>/...`. No test uses the slow
`internal/harness` LLM machinery. Each bead writes its RED test(s) FIRST (assert
the new behavior, watch it fail against current code), then the GREEN impl, then
re-runs the filtered gate. The exact reusable test seams:

- `internal/next` unit tests: inject `runBDCombFn` / `runBDFn` package vars
  (`internal/next/beads.go` L26–30) — no `bd` on PATH required.
- `cmd/mindspec` help/golden tests: `buildMindspecBinary(t)` + subprocess +
  grep, the pattern in `cmd/mindspec/help_golden_test.go`.
- `internal/doctor` checks: `findCheck(r, name)` over a `*Report`, a temp
  `beadsRoot(t, initGit)`, and `t.Setenv("PATH", ...)` for the multi-bd check —
  the harness in `internal/doctor/beads_test.go`.
- `internal/executor` real-repo tests: `newRepoExecutor(t)` (executor_test.go
  L112) for a real temp-git executor + `fakeWorktreeOps`; the cwd-deletion
  pattern of `TestWithWorkingDir_RemovedCwdRemainsAtDirSilently` (L994).
- `internal/gitutil` tests: the `var execCommand = exec.Command` seam
  (gitops.go L22) swapped via `swapExec(t, stdout, exitCode)` (gitops_test.go
  L276) — stub git output, assert the captured argv. No real remote.

## Bead 1: R1 — `mindspec next <bead-id>` honors the positional on the claim path

Bead `mindspec-mfe0` (P2, GH #146.1).

**Changed files**
- `cmd/mindspec/next.go` — thread the positional `args` into the claim path
  (currently `args` reaches only `runEmitOnly`, L58); fix the long help (L28–37).
- `internal/next/select.go` — add a name-aware selection so a named bead is
  resolved from the already-fetched ready set instead of falling through to
  `items[0]`.

**Steps**
1. **RED:** in `internal/next` add `TestSelectWork*` (or a new
   `TestSelectWorkByName`) for the named-bead selector — a named ID that is NOT
   in the ready `items` returns an **error** (not `items[0]`); a named ID present
   returns exactly that item. In `cmd/mindspec` add a `TestNext*` golden/help
   test via `buildMindspecBinary` + grep asserting the `next` long help no longer
   says the positional is "accepted generally" and describes when it is honored
   (both claim path and `--emit-only`). Run; watch fail.
2. **GREEN (select):** add a `SelectWorkByName(items, name)` (or extend
   `SelectWork` with an optional name) that filters the already-fetched `items`
   for the named bead and errors with a clear "not found / not ready" message if
   absent — never `items[0]`.
3. **GREEN (cmd):** on the claim path (next.go ~L203–216), when a positional is
   present, route through the name-aware selector instead of `SelectWork(items,
   pick)`; suppress the "Defaulting to first item" line (L208–210) when a
   positional named the bead. Add the **positional-vs-`--pick` conflict** guard:
   if both a positional and a non-zero `--pick` are supplied, return a clear
   "supply exactly one selector" error before any claim.
4. **GREEN (help):** rewrite the L28–37 long help so it accurately states the
   positional is honored on the claim path (claim that bead) and under
   `--emit-only` (primer for that bead).
5. Gate: re-run the commands in **Verification** below.

**Verification**
- [ ] `go build ./...`
- [ ] `go test -run TestSelectWork -timeout 120s ./internal/next/...`
- [ ] `go test -run TestNext -timeout 120s ./cmd/mindspec/...`

**Acceptance Criteria**
- [ ] `go build ./...` succeeds.
- [ ] `TestSelectWork*`: named-but-not-ready → error, never `items[0]`; named &
  ready → that bead.
- [ ] `TestNext*` (cmd): positional claims that bead; positional + `--pick` →
  conflict error.
- [ ] `TestNext*` help/golden: long help no longer claims the positional is
  accepted generally and describes when it is honored.

**Depends on**: None (`depends_on: []`).

## Bead 2: R2 — `mindspec release <bead>` cleanly reverses a claim

Bead `mindspec-hrmw` (P3, GH #146.2). **The substantive bead.**

**Changed files**
- `cmd/mindspec/release.go` (new) — the `release` cobra command + the strict
  6-step reversal; its own dirty pre-gate; the `bd update --status open` /
  clear-assignee mutation (placed HERE or in a new `internal/release` helper, NOT
  in `internal/next/beads.go`, to avoid colliding with Bead 3).
- `cmd/mindspec/root.go` — register `releaseCmd` (alongside `nextCmd`, ~L228).
- `internal/executor/mindspec_executor.go` — expose the bead-worktree removal
  through `WorktreeOps.Remove` so `release` routes removal through the executor
  (ADR-0030), not a raw `git worktree remove`. (DIFFERENT function from Bead 4's
  `InitSpecWorkspace`; see merge-order note.)

**The strict 6-step order (spec R2 / Design Decisions):**
1. resolve the repo **root** and the bead **worktree path**;
2. **dirty-check** the bead worktree via `next.CheckDirtyTree`
   (`internal/next/guard.go` L110) — refuse (non-zero, no removal) unless
   `--force`. `--force` is a **mindspec-level PRE-GATE** (decide before calling
   `Remove`), NOT a bd-flag passthrough — `bead.WorktreeRemove` is hardcoded
   `--force` at the bd-CLI layer;
3. **`WorktreeOps.Remove` the bead worktree FIRST** (before any bd/state
   mutation);
4. **`os.Chdir(root)` immediately after removal** — MIRROR `complete.go` ~L576:
   chdir to the resolved MAIN repo root (NOT the bead worktree, which may now be
   deleted) so no subsequent bd/git subprocess runs from a deleted cwd
   (spec-092 Req 3c / `mindspec-qxsy` bug class);
5. **`bd update <id> --status open` + clear assignee LAST**, with the bd-state
   mutation's commit reconcile handled the same way `complete`/`next` do
   (`MINDSPEC_ALLOW_MAIN`-style re-export/auto-commit on the default branch);
6. **rewind the state cursor only if it currently points at the released bead**
   — per ADR-0023 the cursor is DERIVED from bd's single `in_progress` child, so
   step 5 self-rewinds the derived cursor; the explicit step syncs the
   `mindspec_phase` metadata cache. Releasing a non-active bead leaves the cursor
   untouched.

The **remove-first / set-open-last** order is deliberate: a partial failure
leaves the recoverable "still-claimed, worktree-gone" state (re-running `release`
or `mindspec next` recovers) rather than an "open + stale-worktree-collision"
state.

**Steps**
1. **RED (ordering + dirty-refuse/force):** in `cmd/mindspec` add `TestRelease*`
   using `&MindspecExecutor{WorktreeOps: fake}` (the `internal/executor`
   injected-fake pattern) recording calls. Assert: `release <bead>` sets the bead
   to `open` with assignee cleared, `WorktreeOps.Remove` is called for the bead
   worktree, the cursor is rewound; a **dirty** worktree refuses (non-zero, NO
   `Remove`) without `--force` and proceeds **with** `--force`; removal goes
   through the executor (no raw `git worktree remove`). Run; watch fail.
2. **RED (cwd-safety AC, spec-092 Req 3c):** add a `TestRelease*Cwd*` using a
   REAL temp-git executor (`newRepoExecutor`, executor_test.go L112; pattern of
   `TestWithWorkingDir_RemovedCwdRemainsAtDirSilently` L994) with the process cwd
   set INSIDE the bead worktree. Assert that after removal `os.Getwd()` recovers
   to root AND a post-removal bd read (injected via the `runBDCombFn`/`runBDFn`
   seam) succeeds. This REQUIRES a real executor — the fake `WorktreeOps` does
   not actually delete the dir, so it structurally cannot catch the footgun.
3. **GREEN:** implement `release.go` per the 6 steps; add/route the
   `WorktreeOps.Remove` path in the executor; register `releaseCmd` in root.go;
   put the `bd update --status open` mutation in `release.go`/`internal/release`
   (not `beads.go`); follow ADR-0035 for paste-safe usage/failure messages.
4. Gate: re-run the commands in **Verification** below.

**Verification**
- [ ] `go build ./...`
- [ ] `go test -run TestRelease -timeout 120s ./cmd/mindspec/...`
- [ ] `go test -run TestRelease -timeout 120s ./internal/executor/...` (if a
  real-repo cwd test lands in the executor package)

**Acceptance Criteria**
- [ ] `go build ./...` succeeds.
- [ ] `TestRelease*` (fake `WorktreeOps`): bead → `open`, assignee cleared,
  `WorktreeOps.Remove` called, cursor rewound; removal via executor only.
- [ ] `TestRelease*` (dirty): refuse without `--force` (no removal), proceed with
  `--force`.
- [ ] `TestRelease*Cwd*` (real temp-git executor, cwd inside worktree): cwd
  recovers to root after removal; post-removal bd read succeeds.

**Depends on**: None (`depends_on: []`). Advisory: merge AFTER Bead 4 (shared
file `mindspec_executor.go`, different functions) and Bead 3 (keeps `beads.go`
out of this bead).

## Bead 3: R3 — surface bd's real stderr on claim failure + two doctor checks

Bead `mindspec-jaeg` (P3, GH #146.3).

**Changed files**
- `internal/next/beads.go` — `ClaimBead` (L162–168): drop the misleading
  "may already be claimed" generic prefix; surface bd's captured
  `CombinedOutput` verbatim (still wrapped with enough context to know a CLAIM
  failed), so a stale-binary schema line like `column "depends_on_id" could not
  be found` is legible (ADR-0035 paste-safe).
- `internal/doctor/beads.go` — two NEW additive `Check` entries (alongside
  `checkBdVersionFloor` ~L284): (a) **bd-binary-vs-DB schema drift**; (b)
  **more than one `bd` on PATH** (the real root cause was a stale
  `~/.local/bin/bd` shadowing Homebrew). Warn-on-problem, OK/skip otherwise —
  never false-warn.

**Steps**
1. **RED (ClaimBead):** add `TestClaimBead*` in `internal/next` injecting a fake
   `runBDCombFn` that fails with a schema-drift stderr; assert the returned error
   CONTAINS bd's real text (e.g. `depends_on_id`) and is NOT flattened to the
   bare "may already be claimed" string. Run; watch fail.
2. **RED (doctor):** add `TestCheckBd*` in `internal/doctor` for the two new
   checks — schema-mismatch simulation → `Warn`; two `bd` on a fabricated
   `t.Setenv("PATH", ...)` → `Warn`; happy path → `OK`/skip (no false warn).
   Use `findCheck(r, name)` + a `beadsRoot(t, ...)`. Run; watch fail.
3. **GREEN:** rewrite `ClaimBead`'s error to pass bd's real output through; add
   the two `Check` functions in `internal/doctor/beads.go` and wire them into the
   beads report assembly.
4. Gate: re-run the commands in **Verification** below.

**Verification**
- [ ] `go build ./...`
- [ ] `go test -run TestClaimBead -timeout 120s ./internal/next/...`
- [ ] `go test -run TestCheckBd -timeout 120s ./internal/doctor/...`

**Acceptance Criteria**
- [ ] `go build ./...` succeeds.
- [ ] `TestClaimBead*`: error contains bd's real stderr (`depends_on_id`), not
  flattened to "may already be claimed".
- [ ] `TestCheckBd*`: schema mismatch → `Warn`; two `bd` on PATH → `Warn`; happy
  → `OK`/skip (no false warn).

**Depends on**: None (`depends_on: []`). Owns `internal/next/beads.go` for this
spec — Bead 2 keeps its bd mutation OUT of this file (merge-order note).

## Bead 4: R4 — `mindspec spec create` branches from `origin/<default-branch>`

Bead `mindspec-k9a8` (P3, GH #76).

**Changed files**
- `internal/gitutil/gitops.go` — NEW helpers behind the executor → gitutil seam:
  a fetch, a default-branch detector (try `git symbolic-ref
  refs/remotes/origin/HEAD`; empty/garbage → `git remote show origin`), and a
  branch-from-remote-ref. Reuse `HasRemote` (L225) and the
  `rejectOptionLike`/`execCommand` conventions.
- `internal/executor/mindspec_executor.go` — `InitSpecWorkspace` (~L85): replace
  the unconditional `gitutil.CreateBranch(specBranch, "HEAD")` with: if a remote
  exists, fetch then branch from `origin/<detected-default>`; on ANY
  fetch/detect error (offline, auth, no-remote) fall back to local `HEAD` and
  emit a WARN — a fetch error is NEVER a hard `spec create` failure. (DIFFERENT
  function from Bead 2's executor edit; see merge-order note.)

**Steps**
1. **RED (detect + branch-from-remote):** add `TestInitSpecWorkspace*` (or a new
   gitutil-helper test) using `swapExec(t, stdout, exitCode)` (gitops_test.go
   L276) over the `execCommand` seam. Stub `symbolic-ref` to a NON-`main` branch
   (e.g. `refs/remotes/origin/develop`); assert the captured `git branch` argv
   references `origin/develop` (default DETECTED, not hardcoded `main`). Stub a
   non-zero fetch/detect exit; assert the WARN fallback to local `HEAD`. Add an
   empty/garbage `symbolic-ref` stub; assert it falls THROUGH to
   `git remote show origin` (cached ref unparseable → miss, not a default). Run;
   watch fail.
2. **GREEN:** add the gitutil fetch / default-branch-detect / branch-from-remote
   helpers; rewire `InitSpecWorkspace` to the detect→fetch→branch-from-origin
   flow with the offline-WARN fallback, all behind the executor → gitutil seam
   (ADR-0030 — no raw `exec.Command("git", ...)` in `cmd/mindspec/`).
3. Gate: re-run the commands in **Verification** below.

**Verification**
- [ ] `go build ./...`
- [ ] `go test -run TestInitSpecWorkspace -timeout 120s ./internal/executor/...`
- [ ] `go test -run Test -timeout 120s ./internal/gitutil/...` (filtered to the
  new helper tests)

**Acceptance Criteria**
- [ ] `go build ./...` succeeds.
- [ ] gitutil/executor test: with a remote, spec branch created from
  `origin/<detected-default>` after a fetch — default DETECTED, not hardcoded.
- [ ] empty/garbage `symbolic-ref` falls through to `git remote show origin`.
- [ ] offline / non-zero fetch: falls back to local `HEAD` + emits a WARN
  (never a hard failure).

**Depends on**: None (`depends_on: []`). Advisory: merge BEFORE Bead 2 (shared
file `mindspec_executor.go`, different functions).

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By |
|---|---|---|
| R1 — positional claims that bead; named-not-ready → error (not `items[0]`) | Bead 1 | `TestSelectWork*` + `TestNext*` (cmd) |
| R1 — `next` long help no longer says positional "accepted generally" | Bead 1 | `TestNext*` help/golden (`buildMindspecBinary`) |
| R2 — `release` sets `open`+clears assignee, `WorktreeOps.Remove` called, cursor rewound; executor-only removal | Bead 2 | `TestRelease*` (fake `WorktreeOps`) |
| R2 — dirty worktree refuses w/o `--force`, proceeds with `--force` | Bead 2 | `TestRelease*` (dirty) |
| R2 (cwd-safety, spec-092 Req 3c) — cwd recovers to root after removal; post-removal bd read succeeds | Bead 2 | `TestRelease*Cwd*` (real temp-git `newRepoExecutor`) |
| R3 — `ClaimBead` error contains bd's real stderr (`depends_on_id`), not flattened | Bead 3 | `TestClaimBead*` (`runBDCombFn` inject) |
| R3 — schema-drift → `Warn`; two `bd` on PATH → `Warn`; happy → `OK`/skip | Bead 3 | `TestCheckBd*` (`findCheck` + `t.Setenv("PATH")`) |
| R4 — spec branch from `origin/<detected-default>` after fetch (not hardcoded) | Bead 4 | `TestInitSpecWorkspace*` / gitutil (`swapExec`) |
| R4 — offline / no-origin → local `HEAD` + WARN (never hard fail) | Bead 4 | `TestInitSpecWorkspace*` / gitutil (`swapExec`, non-zero exit) |
