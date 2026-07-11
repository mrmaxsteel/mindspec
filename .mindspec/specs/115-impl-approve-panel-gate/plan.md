---
adr_citations:
    - ADR-0037
    - ADR-0030
    - ADR-0035
    - ADR-0023
approved_at: "2026-07-11T08:02:58Z"
approved_by: user
bead_ids:
    - mindspec-fgmg.1
    - mindspec-fgmg.2
    - mindspec-fgmg.3
spec_id: 115-impl-approve-panel-gate
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/lifecycle/orphans.go
        - internal/lifecycle/orphans_test.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_advisory_test.go
        - internal/executor/mindspec_executor.go
        - internal/executor/finalize_worktree_only_test.go
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/approve/orphan_gate_test.go
    - depends_on: []
      id: 3
      key_file_paths:
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - internal/setup/claude.go
        - .claude/skills/ms-impl-approve/SKILL.md
        - .agents/skills/ms-impl-approve/SKILL.md
        - internal/lifecycle/ownership_test.go
---
# Plan: 115-impl-approve-panel-gate

Close the last un-gated merge path in the binary: `mindspec impl approve`
gains a pre-terminal REFUSAL gate (R1 orphan detection + the round-7
worktree-enumeration merge-prevention leg + R3 durable-obligation backstop,
with R2 advisory slot naming), fail-closed on every cleanly-signalled infra
error, never false-refusing a cleanly-deleted branch, routing all settlement
through the ONE existing gate home, `mindspec complete <bead>`. All line
references below are pinned against branch HEAD `8020b3e2`.

## Decomposition and land order

Three beads. Dependency edges are declared ONLY where a bead consumes another
bead's produced state:

- **Bead 1 (substrate + structural pins)** — the error-preserving enumeration
  core + epic-scoped closed-bead export in `internal/lifecycle`, the exported
  check-only obligation predicate + gate-roots export in `internal/complete`,
  and the `internal/executor` `:396-398` comment fix + the AC12 structural
  pin. Zero behavior change for every existing consumer.
- **Bead 2 (the gate)** — depends on Bead 1: it consumes Bead 1's exported
  `lifecycle.ScanOrphanedClosedBeads` core, `lifecycle.ClosedEpicBeadIDs`
  enumeration, `complete.CheckPendingObligations` predicate, and the exported
  panel-roots helper. All four legs of the refusal land here, with every
  `TestApproveImpl_*` suite (AC1-AC7, AC11, AC13) and the AC4 call-order
  anchor.
- **Bead 3 (docs + ownership pin)** — `depends_on: []`, genuinely
  independent: the ADR-0037 amendment and skill text are contract-level
  (drafted deliberately without detection internals), the AC9 discriminator
  phrase (`closed without`) is spec-pinned rather than code-derived, and the
  AC10 ownership test parses OWNERSHIP.yaml files already committed on the
  branch. **Recommended land order is still 1 → 2 → 3** (orchestration
  preference, not a data edge): landing docs last lets the docs-bead panel
  verify the prose against shipped refusal behavior.

Longest serial chain: **2** (Bead 1 → Bead 2), within the ≤3 bound.

**Bead 2 is NOT split (decomposition decision, stated for the panel).** The
candidate 2a/2b split (2a = R1 orphan + worktree-enum legs + ordering +
hatches + AC11/AC13; 2b = R2 advisory + R3 obligation, AC5/AC6) was
considered and REJECTED on the merge heuristic: both halves edit the same
production file (`internal/approve/impl.go`, one contiguous gate block) and
the same test fixtures/seams — far past the >50% file-overlap merge
threshold, so the split would buy a second panel round on a same-file serial
diff while making the intermediate 2a state carry a gate that silently lacks
its R3 unique coverage (a closed bead with a recorded obligation and a
deleted branch would pass 2a's gate — a hole 114's history says not to ship
even intra-branch). One production file, one insertion point, one seam
family; the twelve named tests are volume, not coupling. If the Bead-2 panel
signals genuine scope strain, the 2a/2b line above is the sanctioned split
and keeps the chain at 3.

**Reviewer/fixer scratch discipline (inherit into every bead brief and
reviewer prompt):** reviewers and fixers MUST use ABSOLUTE `/tmp` scratch
paths (or `t.TempDir()` inside Go tests) for any file they create, and must
NEVER write relative `.mindspec/` (or any relative repo) paths — the agent
harness resets cwd between bash calls, and a relative write from a reviewer
has previously corrupted SIBLING worktrees, which `mindspec complete` then
auto-committed past review. Reviewer verdict paths must be ABSOLUTE. Verify
the bead worktree is CLEAN (`git status --porcelain` empty) before every
`mindspec complete`.

**Toolchain note:** run the CI-matched `gofmt` (go.mod pins go 1.23.0 —
Go 1.19+ gofmt reformats doc comments). In doc comments and skill prose,
avoid backtick code spans containing shell-escape sequences (single-quote
escapes and the like).

**Repo-wide negative fence (every bead brief):** the round-6
deleted-classifier pin — `! grep -rn 'BranchExistsE' --include='*.go' .` and
`! grep -rn 'show-ref' --include='*.go' internal/` must stay 0-hit (verified
0 at `8020b3e2`); `gitutil.BranchExists` (`internal/gitutil/gitops.go:94-100`)
and ALL its callers stay byte-unchanged. NO `internal/gitutil` change lands
anywhere in 115. A fixer "helpfully" re-armoring the probe with an exit-code
classifier is a spec violation (the proven-impossible rounds-2..5 spiral —
the 114 delete>patch lesson).

**AC discriminator SHAs are restated as-written.** The spec's proofs anchor
to `eb6a2ed1`, `77d8a1a9`, `18cc59d9`, and `bcb255eb` (spec-branch history).
Bead briefs must NOT "fix" them to newer SHAs — the RED-on-revert claims are
made against those exact commits.

## Bead 1: Detection substrate + structural pins (lifecycle error-preserving core + epic enumeration export, complete predicate + roots export, executor comment fix + AC12 pin)

**Goal:** land every shared primitive the Bead-2 gate consumes and every
structural pin that is independent of the gate, with zero behavior change
for existing consumers.

**Scope**

`internal/lifecycle/orphans.go` (+ tests), `internal/complete/panel_advisory.go`
(+ tests), `internal/executor/mindspec_executor.go` (comment only) + one new
executor test file. Three packages, but each change is small, test-heavy, and
behavior-preserving for existing consumers. The mixed workflow+execution
surface is fine at gate time because the spec's `## Impacted Domains`
declares BOTH (the 114 round-3 O3 lesson: the divergence gate derives its
candidate domains from the spec, not from bead briefs).

**Steps**

1. **`internal/lifecycle/orphans.go` — the error-preserving core.** Refactor
   `FindOrphanedClosedBeads` (`:74-112`) into an exported error-preserving
   core `ScanOrphanedClosedBeads(specID, workdir, excludeBeadID string)
   ([]Orphan, error)` that PROPAGATES the three cleanly-signalled infra
   legs — epic-lookup (`findEpicBySpecIDFn`, `:28`), `bd`-list
   (`listClosedBeadsFn`, `:29-39`), ancestry (`isAncestorFn` →
   `gitutil.IsAncestor`, `gitops.go:446`, which discriminates exit 1 = not an
   ancestor from any other failure = non-nil error) — while
   `FindOrphanedClosedBeads` remains the fail-open wrapper (now delegating to
   the core and swallowing its error) with **byte-identical behavior** for
   `complete`/`next`/`doctor` (their best-effort parity contract doc,
   `orphans.go:71-73`, survives verbatim; no signature change — three call
   sites: `internal/complete/complete.go:111` seam, `cmd/mindspec/next.go:459`,
   `internal/doctor/orphaned_beads.go`). The branch-existence trigger stays
   the UNCHANGED bool `branchExistsFn = gitutil.BranchExists` (`orphans.go:40`,
   consumed at `:95`) in BOTH core and wrapper: `false` (absent or
   probe-infra-failure) = no trigger, by design (round-6 C+B).
   `Orphan.RecoveryCommand()` (`:54`) unchanged.
   **Plus the epic-scoped enumeration export (the worktree-enum leg's
   same-enumeration guarantee):** export
   `ClosedEpicBeadIDs(specID string) ([]string, error)` wrapping exactly the
   existing `findEpicBySpecIDFn` + `listClosedBeadsFn` pair (`orphans.go:75-83`)
   — the SAME `bd list --parent <epic> --status=closed` enumeration the
   orphan scan uses — and re-express the core's own enumeration over it
   (single home, so the gate's worktree-enum leg and the orphan scan can
   never disagree on the epic's closed-bead set). Error-preserving (returns
   the epic-lookup / bd-list error); Bead 2's gate consumes it fail-closed.
2. **`internal/lifecycle` tests.** New
   `TestScanOrphanedClosedBeads_ErrorPreserving` including the round-3 G2
   MIXED-list parity case: with the ancestry seam erroring for bead A while a
   LATER bead B in the list is a provable orphan, the core returns the error
   (so the gate refuses) while the fail-open `FindOrphanedClosedBeads`
   wrapper still returns the later provable orphan(s) — byte-identical
   wrapper output for complete/next/doctor, not merely "same on an all-error
   list". Existing wrapper tests untouched (AC1's bare
   `go test ./internal/lifecycle` is the package no-regression pin).
3. **`internal/complete/panel_advisory.go` — export the check-only
   obligation predicate (R3's single-home reuse).** Extract the coverage
   discipline `reconcilePendingRefutations` (`:519-584`) embodies — the
   fail-closed `decodePendingEntries` (def `:310`, reconcile read `:529`) +
   `decodeRefutations` (def `:329`, read `:546`) + (slot, round)-exact
   `pendingEntryKey` coverage (`:295`, applied at `:554-566`) — WITHOUT the
   `writePanelRefutedMetadata` settle step (`:578`) — into an exported
   check-only predicate, pinned shape:
   `CheckPendingObligations(beadID string, getMeta func(string) (map[string]interface{}, error)) error`
   — dependency-injected metadata reader, so `internal/approve` passes its
   own fail-closed `bead.GetMetadata` seam and approve tests stub it without
   reaching into complete's `completeGetMetadataFn`. Returns nil when the
   bead has no recorded pending entries or every entry is (slot, round)-covered
   by a durable `panel_refuted_entries` record; returns an error naming the
   uncovered slot@round, and an error (never decode-as-empty) on a metadata
   read error, a present-but-corrupt entries value, or a shape-invalid entry
   (empty slot / round < 1). **The shared core is a new UNEXPORTED helper
   that computes the set of uncovered (slot, round) entries — NOT the
   exported `CheckPendingObligations` predicate itself.** The two wrappers
   diverge on what they do with that set: the exported check-only predicate
   returns the uncovered set as an ERROR (and errors on any read / corrupt /
   shape-invalid input — it never settles), whereas
   `reconcilePendingRefutations` CONSUMES the set to SETTLE uncovered entries
   by writing `panel_refuted` metadata (`panel_advisory.go:558-582`).
   Re-express BOTH as thin wrappers over that unexported
   compute-uncovered-set helper — do NOT make reconcile call the erroring
   predicate — so complete's own behavior is provably unchanged (no
   semantic test change in `internal/complete`).
4. **`internal/complete/panel_advisory.go` — export the layout-aware panel
   root scan (R2's single-home reuse).** `panelGateRoots` (doc `:605`, def
   `:624`) is unexported; R2's advisory decoration needs the same layout-aware
   scan the authoritative gate uses. Export a thin wrapper (pinned name:
   `PanelGateRoots(root, wtPath, specID string) []string`) rather than
   reimplementing the root order in `internal/approve` — single-home, and it
   rides the same `approve → complete` import edge R3's predicate already
   adds. No behavior change; existing callers untouched.
   **Discriminator caution:** the pre-existing test
   `TestPanelGateRoots_LayoutAware` already contains the substring
   `PanelGateRoots`, so any bead-brief existence discriminator for this
   export MUST use a func-def-scoped grep (e.g.
   `grep -q 'func PanelGateRoots' internal/complete/panel_advisory.go`) or a
   file-scoped grep — never a bare `grep -q 'PanelGateRoots'`, which is
   GREEN today and would not be RED-on-revert.
5. **`internal/complete` tests.** New `TestPendingObligationPredicate`
   (AC6's predicate pin: uncovered → error naming slot@round;
   (slot, round)-exact covered → nil; read error / corrupt entries /
   shape-invalid (empty slot, round < 1) → error, never decode-as-empty;
   no-pending → nil) AND new `TestCompleteRun_RegatesAlreadyClosedOrphan`
   (AC7(a), package-local per the spec's import-cycle argument — an
   in-`package complete` test may not call `ApproveImpl` once R3 adds the
   `approve → complete` edge): an already-closed orphan bead with an
   unresolved latest-round RC → `complete.Run` Blocks at step 2.25 (gate call
   `complete.go:387`) BEFORE step-4's already-closed tolerance
   (`complete.go:576-579`); after the RC is durably refuted per 114 (marker +
   reconciliation), `complete.Run` succeeds with the "already closed" warning
   and merges. This pins EXISTING behavior — no production change for it.
6. **`internal/executor/mindspec_executor.go` — comment-only fix at
   `:396-398`.** Drop the "same regression-only rule as CompleteBead" parity
   phrasing (the phrase sits alone on `:398`); name `guardMergeLayout`'s
   directional layout-regression scope specifically (o4fd's NOTE resolution:
   no panel gate, no obligation reconciliation, no orphan check fires on this
   path — the comment must stop implying completion-gate parity). NO
   behavioral executor change (OQ1 resolved REFUSE, OQ4 resolved no-seam).
7. **`internal/executor` tests.** New
   `TestFinalizeEpic_MergesOnlyWorktreeRealBranches` (AC12, Fact 2, real-git,
   subtests, placed beside the `finalize_orphan_test.go` /
   `merge_conflict_test.go` conventions): (a) an unmerged `bead/<id>` branch
   ref with NO worktree is never merged (`FinalizeEpic`, `:349`, consumes
   `g.WorktreeOps.List()` at `:383`); (b) a `bead/<id>` TAG shadowing a
   deleted branch is not enumerated as a `bead/` branch (a tag-only checkout
   is DETACHED with no branch line → the `HasPrefix(e.Branch,
   workspace.BeadBranchPrefix)` filter at `:385` drops it); (c) a dangling
   `bead/<id>` symref cannot be `git worktree add`-ed at all. Also pins the
   blp6 constraint (spec Non-Goals): any future merge-loop change must keep
   the worktree-enumerated-real-branch invariant and the `WorktreeOps.List()`
   enumeration source.

**Verification**

- [ ] `go build ./... && go test ./internal/lifecycle ./internal/complete ./internal/executor` — all green with NO semantic expectation change in existing tests (mechanical fixture additions only).
- [ ] `grep -q 'func TestScanOrphanedClosedBeads_ErrorPreserving' internal/lifecycle/*_test.go && go test ./internal/lifecycle -run 'TestScanOrphanedClosedBeads_ErrorPreserving' -v` (AC1's lifecycle half, incl. the mixed-list parity subtest).
- [ ] `grep -q 'func TestPendingObligationPredicate' internal/complete/*_test.go && go test ./internal/complete -run 'TestPendingObligationPredicate' -v` (AC6's predicate half).
- [ ] `grep -q 'func TestCompleteRun_RegatesAlreadyClosedOrphan' internal/complete/*_test.go && go test ./internal/complete -run 'TestCompleteRun_RegatesAlreadyClosedOrphan' -v` (AC7(a)).
- [ ] AC8 pair: `go test ./internal/executor && git diff main -- internal/executor/mindspec_executor.go | grep -c '^-.*rule as CompleteBead'` ≥ 1 AND `grep -c 'rule as CompleteBead' internal/executor/mindspec_executor.go` returns 0.
- [ ] AC12 pair: `grep -q 'func TestFinalizeEpic_MergesOnlyWorktreeRealBranches' internal/executor/*_test.go && go test ./internal/executor -run 'TestFinalizeEpic_MergesOnlyWorktreeRealBranches' -v`.
- [ ] Negative fence: `! grep -rn 'BranchExistsE' --include='*.go' . && ! grep -rn 'show-ref' --include='*.go' internal/`; `git diff main -- internal/gitutil/` is empty.
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- The error-preserving core returns the epic-lookup / bd-list / ancestry
  error while the fail-open wrapper stays byte-identical for
  complete/next/doctor, pinned on a MIXED list (AC1 lifecycle half).
- The exported check-only predicate is fail-closed on read/decode/shape and
  exact on (slot, round) coverage (AC6 predicate half).
- `complete.Run` re-gates an already-closed orphan and converges after a
  durable refutation (AC7(a) — existing behavior, now pinned).
- The executor comment no longer claims CompleteBead rule parity, with zero
  semantic test churn (AC8), and Fact 2 is pinned RED-on-revert (AC12).

**Depends on**
None (first bead).

## Bead 2: The impl-approve refusal gate (R1 orphan + worktree-enum legs, R2 advisory slot naming, R3 obligation backstop; AC11/AC13 pins, AC4 call-order anchor)

**Goal:** `ApproveImpl` gains the pre-terminal refusal gate — fail-closed on
the three cleanly-signalled detection legs, on the worktree-enumeration leg's
own infra, and on the R3 obligation/plan-enumeration legs; never
false-refusing a cleanly-deleted branch; placed so a refusal performs no epic
close, no phase write, no merge, no push.

**Scope**

`internal/approve/impl.go` (the gate + its `guard.NewFailure` messages + new
seams), `internal/approve/impl_test.go` (call-order anchor + mechanical seam
defaults), one new gate test file (suggested `orphan_gate_test.go`). No other
production package changes — everything the gate consumes was exported in
Bead 1 or already exists (`bead.WorktreeList`, `bead.GetMetadata`,
`panel.ForBead`/`panel.Tally`/`Result.UnresolvedVerdicts`).

**Steps**

1. **Insertion point (HC-4).** Insert the gate AFTER the last read-only gate
   (the ADR-divergence block ends with the `adrResult.HasFailures()` refusal
   at `impl.go:310-313`) and BEFORE the Spec 092 deferred phase-reconcile
   write (`:327-340`), MUTATION (1/3) epic close (`:343-348`), the
   `mindspec_phase=done` write (`:352-357`), the CommitCount preflight
   (`:364`), and `exec.FinalizeEpic` (`:372`) — so a refusal performs no epic
   close, no phase write, no merge, no push. (The one write that legitimately
   precedes it is `phase.EnsureMigrated`'s one-shot migration reconcile at
   `impl.go:140`, which precedes EVERY `ApproveImpl` gate today.) Update the
   call-order doc comment at `impl.go:114-131` to list the new gate.
   **New seams — `implXxxFn` convention (matching the existing
   `implRunBDFn`/`implPhaseMetadataFn` vars at `impl.go:27-45`), all
   defaulting BENIGN so every existing test passes untouched (AC2(b)/AC5
   discipline):**
   - `implScanOrphansFn = lifecycle.ScanOrphanedClosedBeads`
   - `implClosedEpicBeadIDsFn = lifecycle.ClosedEpicBeadIDs`
   - `implWorktreeListFn = bead.WorktreeList`
   - `implIsAncestorFn = gitutil.IsAncestor`
   - `implGetMetadataFn = bead.GetMetadata`
   - `implCheckObligationsFn = complete.CheckPendingObligations`
   **Import-edge note (spec O2 round-7, decided here):** this adds THREE new
   import edges to `internal/approve` — `→ lifecycle`, `→ complete` (the R3
   predicate + R2 roots), and `→ gitutil` (the worktree-enum leg's ancestry
   check). All verified acyclic at `8020b3e2` (`complete` never imports
   `approve`; `gitutil` and `lifecycle` are leaves relative to `approve`).
   The `approve → gitutil` edge is taken DIRECTLY via the `implIsAncestorFn`
   seam rather than routed through a lifecycle helper: the spec itself names
   `gitutil.IsAncestor` for this leg, `internal/lifecycle` already consumes
   the same function the same way (`orphans.go:41`), and ADR-0030's boundary
   concern is enforcement-in-the-EXECUTOR, not gitutil use by an enforcement
   package — no executor git-I/O enters `internal/approve`.
2. **Leg 1 — R1 orphan scan (error-preserving core, fail-closed).** Call
   `implScanOrphansFn(specID, root, "")`. Any infra error (epic-lookup,
   bd-list, ancestry — the three cleanly-signalled legs) →
   `guard.NewFailure` naming the failed enumeration step (fail-closed: an
   unreadable store cannot prove the epic is settled). Any orphan →
   `guard.NewFailure` that (a) names the bead ID, its unmerged `bead/<id>`
   branch, and the spec branch, (b) states it was closed without
   `mindspec complete`, and (c) ends with the recovery line
   `orphan.RecoveryCommand()` = `mindspec complete <bead>` (ADR-0035;
   `internal/guard/recovery_convention_test.go` enforces the final-line
   shape). Epic-scoped by construction (the core enumerates only the
   finalizing spec's epic) — a different spec's orphan neither triggers nor
   suppresses.
3. **Leg 2 — the worktree-enumeration merge-prevention leg (round-7 Option
   B, AC13).** Enumerate `implWorktreeListFn()` (`bd worktree list --json`,
   `internal/bead/bdcli.go:202-217` — the SAME production source behind
   `defaultWorktreeOps.List()`, `mindspec_executor.go:44-46`, that
   `FinalizeEpic` consumes at `:383`). For every entry whose `Branch` carries
   the `bead/` prefix (`workspace.BeadBranchPrefix`, `worktree.go:21` — the
   same filter as `:385`) and whose bead ID is in
   `implClosedEpicBeadIDsFn(specID)` (epic-scoped: the same enumeration the
   orphan scan uses, so a different spec's worktree neither triggers nor
   suppresses — blp6 unchanged), check `implIsAncestorFn(root, e.Branch,
   specBranch)` and REFUSE if the branch is NOT an ancestor of the spec
   branch — regardless of what the branch-existence probe reported (this leg
   never consults `branchExistsFn`). Fail-CLOSED on its own infra: a
   `WorktreeList` error, a `ClosedEpicBeadIDs` error, or an ancestry error →
   `guard.NewFailure` naming the failed step (deliberate asymmetry vs
   `FinalizeEpic` itself, whose List/ancestry failures merge-SKIP — the safe
   direction there). The refusal message follows the same (a)/(b)/(c) shape
   as leg 1, and its `mindspec complete <bead>` final recovery line is always
   runnable (the branch demonstrably exists — its worktree is enumerated).
   Because the gate and the merge loop key off the SAME enumeration source, a
   transient ref-probe failure can no longer hide a merge candidate the loop
   will see (closes the round-6 G2 race). The residual two-call window
   (worktree CREATED between the gate's enumeration and `FinalizeEpic`'s) is
   the spec's disclosed concurrent-mutation residual — no code answers it.
4. **Leg 3 — R3 durable-obligation backstop (fail-closed on data AND
   enumeration).** Enumerate plan beads via `readPlanBeadIDs`
   (`impl.go:452`) — **fail-CLOSED**: a `readPlanBeadIDs` error (missing
   `plan.md`, missing/corrupt frontmatter, empty `bead_ids`) REFUSES naming
   the unreadable plan path, unlike gate-1/3's silent `if planErr == nil`
   skip at `:226-227`, which is UNCHANGED for gate 1/3 itself. Per bead:
   `implCheckObligationsFn(beadID, implGetMetadataFn)` — an uncovered
   obligation, a metadata read error, a corrupt entries value, or a
   shape-invalid entry → refuse (never decode-as-empty; a bead with no
   recorded pending entries is a no-op). **Recovery line is
   branch-state-truthful (round-2 G3):** branch exists
   (`gitutil.BranchExists` via a seam-consistent check) →
   `mindspec complete <bead>`; branch absent → the restoration-prerequisite
   recourse (restore the `bead/<id>` branch ref, THEN
   `mindspec complete <bead>`), because a bare complete dies at the step-3.5
   merge-base (`complete.go:492-495`) before reaching the step-3.75
   reconciliation — the message must never present a command known to fail.
5. **R2 — advisory slot naming (decoration on legs 1-2, never
   load-bearing).** For a refused orphan, best-effort read the registered
   panel via Bead 1's exported `complete.PanelGateRoots(root, wtPath,
   specID)` + `panel.ForBead` + `panel.Tally` + `Result.UnresolvedVerdicts()`
   (114 R1 keeps `VoteDecision` in lockstep, so the named slots can never
   contradict the gate's vote portion) and ADD the unresolved
   REQUEST_CHANGES slot(s) to the refusal message. Strictly advisory: an
   unreadable, missing, or removed panel omits the slot line — never a pass,
   never a crash. No gate decision computed, no metadata written; the message
   never prints `MINDSPEC_SKIP_PANEL` (HC-7) nor any paste-able refutation
   incantation (no `refut` substring).
   **Hatches:** `MINDSPEC_SKIP_PANEL` (`internal/panel/gate.go:30`) and
   `enforcement.panel_gate: false` bypass NO leg of this gate — it is a
   4gsz-class lifecycle-bypass guard (sibling of `complete` step 1.6,
   `complete.go:307-321`), not the panel-gate decision. The hatches keep
   their exact 114 semantics inside the recovery `complete` run.
6. **Call-order anchor (AC4).** Extend `TestApproveImplCallOrder`
   (`impl_test.go:714`, AST-based) with a new anchor referencing the gate's
   call site by the core's name (`ScanOrphanedClosedBeads` — ZERO code hits
   at `eb6a2ed1`: the operative grep is file-scoped to
   `internal/approve/impl_test.go`, no `func`/symbol match in the graded
   scope; the only repo-wide match is the spec's own prose), asserting it
   sits after the last read-only gate
   and BEFORE the deferred phase-reconcile write, MUTATION (1/3), and
   `FinalizeEpic`. `TestApproveImpl_HappyPath` (`:75`) and
   `TestApproveImpl_FinalizeEpicCalled` (`:299`) get NO semantic change
   (AC2(b) pin) — the new seams' benign defaults guarantee it.
7. **New named tests** (all in `internal/approve`, suggested home
   `orphan_gate_test.go`; every one absent at the spec's discriminator SHAs):
   `TestApproveImpl_OrphanRefuses` (AC1a — refusal content + epic-close fn,
   phase-metadata fn, `FinalizeEpic` all seam-recorded UNCALLED);
   `TestApproveImpl_OrphanInfraErrorFailsClosed` (AC1b — the three seam-error
   legs); `TestApproveImpl_OrphanExemptions` (AC2 subtests: ancestor
   no-refuse; other-epic no-refuse; clean-path regression pin);
   `TestApproveImpl_DeletedBranchNoRefusal` (AC2d — the round-3 G2
   anti-false-refusal pin: genuinely absent branch → proceeds to
   `FinalizeEpic` exactly as the clean path);
   `TestApproveImpl_HatchDoesNotBypassOrphan` (AC3 — both hatches; message
   never contains `MINDSPEC_SKIP_PANEL`); `TestApproveImpl_AdvisorySlotNaming`
   (AC5 subtests: slot named with readable panel; identical refusal minus the
   slot line with the panel dir removed; no `refut` substring);
   `TestApproveImpl_ObligationBackstop` (AC6 subtests a-d incl. the
   branch-less restoration-prerequisite recourse and the hatch non-bypass);
   `TestApproveImpl_CorruptPlanRefuses` (AC6e — refuse naming the plan path,
   never the gate-1/3 silent skip); `TestApproveImpl_PassesAfterOrphanSettled`
   (AC7b — post-settle ancestor state passes the gate and proceeds to
   `FinalizeEpic`); `TestApproveImpl_UnreadableRefStoreAbortsPreScan` (AC11,
   Fact 1 — real-git `chmod 000 .git/refs/heads` on a loose-ref fixture, or
   the stub executor's `MergeBase` forced to the equivalent exit-128 error:
   `ApproveImpl` errors at/before `exec.MergeBase("main", specBranch)`,
   `impl.go:249`, BEFORE the orphan scan, epic-close/phase/FinalizeEpic
   seams all UNCALLED); `TestApproveImpl_WorktreeEnumRefusesDespiteProbeMiss`
   (AC13 subtests: (a) the race — orphan scan seam-forced to report no
   orphans while the worktree-list seam enumerates the closed epic bead's
   non-ancestor `bead/<id>` worktree → refuses via the worktree-enum leg,
   nothing mutated; (b) worktree-list seam error → refuses naming the step;
   (c) no false positives — enumerated ancestor branch, and an enumerated
   worktree of a different spec's bead, do NOT trigger).

**Verification**

- [ ] Every AC1-AC7, AC11, AC13 proof EXACTLY as written in the spec's
  Acceptance Criteria (existence `grep -q 'func Test<Name>'` chained with the
  exact-named `go test -run '<Name>'` — the round-3 G3 discipline; no
  package-wide run stands in for a named run).
- [ ] AC4: `grep -q 'ScanOrphanedClosedBeads' internal/approve/impl_test.go && go test ./internal/approve -run 'TestApproveImplCallOrder' -v`.
- [ ] `go build ./... && go test ./internal/approve ./internal/executor ./internal/complete ./internal/lifecycle` — full sweep green; existing `TestApproveImpl*` suites pass with zero semantic modification (mechanical seam-default additions only).
- [ ] Negative fence + `git diff main -- internal/gitutil/` empty + `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- An orphaned closed bead (branch exists, not an ancestor) refuses with the
  bead/branch named and `mindspec complete <bead>` as the final line, with
  no epic close, phase write, merge, or push (AC1a); each of the three
  cleanly-signalled infra errors refuses fail-closed while the fail-open
  wrapper is untouched (AC1b).
- Ancestor branches, other-epic orphans, and genuinely-deleted branches never
  refuse; the clean path reaches `FinalizeEpic` byte-identically (AC2).
- Hatches bypass nothing (AC3); the gate sits pre-terminal, pinned AST-wise
  (AC4); advisory slot naming is never load-bearing (AC5).
- The obligation backstop is fail-closed on data and enumeration with
  branch-state-truthful recovery (AC6); a settled orphan converges (AC7b).
- Fact 1 (AC11) and the worktree-enum merge-prevention leg (AC13) are pinned
  RED-on-revert.

**Depends on**
Bead 1 (consumes `lifecycle.ScanOrphanedClosedBeads`,
`lifecycle.ClosedEpicBeadIDs`, `complete.CheckPendingObligations`,
`complete.PanelGateRoots` — none exist before Bead 1; AC4's anchor greps the
literal core name, INTRODUCED by Bead 1 and consumed here).

## Bead 3: Docs + ownership pin — ADR-0037 dated amendment, ms-impl-approve skill (embedded literal + both materialized copies), AC10 ownership test

**Goal:** the contract and the operator-facing skill say what the binary now
does, and the `internal/lifecycle` ownership claim is pinned
exactly-one-domain = workflow, RED-on-revert.

**Scope**

`.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md`,
`internal/setup/claude.go` (embedded literal), the two materialized skill
copies, plus one new Go structural test in `internal/lifecycle`. Docs content
is pre-drafted in
`/Users/Max/.claude/jobs/06840a22/tmp/spec-115-docs-predraft.md` (contract-level
prose only — no detection internals; the bead transcribes and re-grounds it).
`.mindspec/domains/workflow/OWNERSHIP.yaml` needs NO edit — the
`internal/lifecycle/**` claim is already committed on the spec branch
(verified at `8020b3e2`: `workflow/OWNERSHIP.yaml:9`, sole claimant across
all domain files).

**Steps**

1. **ADR-0037 dated amendment.** Append to the §1 amendment chain (directly
   after the 2026-07-10 spec-114 refutations block), following the file's
   `> **Amendment (YYYY-MM-DD, spec NNN — label):**` convention, stamped with
   the bead's actual land date: the contract's reach is **every lifecycle
   verb that can merge a bead branch** — `mindspec impl approve` REFUSES to
   finalize (exit non-zero: no epic close, no phase write, no merge, no push)
   while any closed bead under the spec's epic lacks proof of panel
   settlement (closed without `mindspec complete`, or carrying a durable
   `refutation_pending` obligation not covered by a durable `panel_refuted`
   record); the **single settlement surface remains `mindspec complete`** (no
   second gate home — `impl approve` never computes an Allow/Block decision,
   never writes panel audit metadata, never applies a refutation); the §7
   hatches keep their exact semantics — they except the GATE DECISION inside
   the recovery `complete` run, never the durable obligation, and they do NOT
   bypass this refusal (a lifecycle-bypass guard, not the gate decision).
   Follow the docs pre-draft's Artifact 1 text; keep it at contract level
   (no BranchExists/worktree/ancestry internals).
2. **ms-impl-approve skill — all three surfaces (AC9).** Append the
   pre-draft's Artifact 2 section ("If approval refuses: a bead was closed
   without `mindspec complete`") to (a) the `ms-impl-approve` entry inside
   the binary-embedded `lifecycleSkillFiles()` literal
   (`internal/setup/claude.go:681`, entry at `:736`, using the surrounding
   entries' backtick-escape convention), (b)
   `.claude/skills/ms-impl-approve/SKILL.md`, and (c)
   `.agents/skills/ms-impl-approve/SKILL.md`. Document the refusal, the
   `mindspec complete <bead>` recovery, the R3 branch-less
   restoration-prerequisite recourse, and that skip/abandon hatches do not
   bypass. (Round-1 correction stands: there is NO
   `plugins/mindspec/skills/ms-impl-approve/` — do not create one.)
   **`mindspec-sxjc` drift note:** the three copies have pre-existing drift
   (the literal says `mindspec impl approve` + full session-close protocol +
   `managed-by: mindspec` marker; both materialized copies say
   `mindspec approve impl`, differ on step 5, and LACK the marker — filed as
   `mindspec-sxjc`, P3). This bead's REQUIRED edit is the minimal
   AC9-compliant append to each file as-is; reconciling the verb-order /
   step-5 / marker drift is `mindspec-sxjc`'s scope — if the bead reconciles
   it opportunistically, it must first check the missing `managed-by` marker's
   effect on the shipped-vs-user-modified detection in
   `internal/setup/skills.go` (`installSkills`/`matchesShipped`) — note
   `refreshManagedSkill` exists only inside a doc comment
   (`internal/setup/claude.go:679-680`), not as a function — and
   say so in completion notes; do NOT silently normalize.
3. **AC10 structural ownership test (round-7: the proof made
   RED-on-revert).** The former `grep -l` one-liner is GREEN at the review
   SHA (the ownership edit landed rounds ago — a pre-applied artifact
   discriminates nothing), so the DISCRIMINATING proof is a NEW named
   structural test **`TestLifecycleOwnershipExactlyOneWorkflowClaimant`** in
   `internal/lifecycle` (new file, suggested `ownership_test.go` — disjoint
   from Bead 1's `orphans_test.go` additions, so no cross-bead file
   collision): locate the repo root, parse every
   `.mindspec/domains/*/OWNERSHIP.yaml`, assert the `internal/lifecycle/**`
   path pattern is claimed by EXACTLY ONE domain AND that domain is workflow.
   A third-domain claim appearing, or the workflow claim reverting, turns it
   RED. The spec assigns this test to the docs/ownership bead explicitly
   (AC10: "goes green only when the docs/ownership bead adds the test
   alongside the (already-present) claim").

**Verification**

- [ ] AC9: `grep -n 'impl approve' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` ≥ 1 hit (0 at `8020b3e2`, verified) AND `grep -c 'closed without' internal/setup/claude.go .claude/skills/ms-impl-approve/SKILL.md .agents/skills/ms-impl-approve/SKILL.md` ≥ 1 per file (all 0 today, verified).
- [ ] AC10: `grep -q 'func TestLifecycleOwnershipExactlyOneWorkflowClaimant' internal/lifecycle/*_test.go && go test ./internal/lifecycle -run 'TestLifecycleOwnershipExactlyOneWorkflowClaimant' -v && go test ./internal/lifecycle`. The shell one-liner `[ "$(grep -l 'internal/lifecycle' .mindspec/domains/*/OWNERSHIP.yaml)" = ".mindspec/domains/workflow/OWNERSHIP.yaml" ]` is retained as a quick non-discriminating regression pin only.
- [ ] `go build ./...` (the literal edit must compile) + `gofmt -l ./cmd ./internal` prints nothing (mind the doc-comment code-span/gofmt gotcha in the literal).
- [ ] The three ms-impl-approve texts carry the identical new section (content-identical for the appended block; pre-existing sxjc drift outside it is tolerated unless the bead reconciles it with a stated decision).
- [ ] `mindspec validate spec 115-impl-approve-panel-gate` passes (advisory WARN acceptable).

**Acceptance Criteria**

- ADR-0037 carries the dated 115 amendment (reach = every merging lifecycle
  verb; refusal is not a gate decision; hatches unaffected in reach), and the
  skill refusal text is present in the embedded literal and both materialized
  copies (AC9).
- `internal/lifecycle/**` is claimed by exactly one domain (workflow), pinned
  by the new structural test, RED-on-revert (AC10).

**Depends on**
None (no produced-state consumption; see the land-order preference in
Decomposition — land after Bead 2 so the docs panel verifies prose against
shipped behavior).

## ADR Fitness

- **ADR-0037 (panel gate as enforced contract) — AMENDED; remains the right
  home.** 115 extends the contract's REACH, not its enforcement home: the
  amended §1 lineage (099's relocation → 102's "single authoritative
  enforcement point" → 114's any-unresolved-RC-blocks + refutation protocol)
  gains one more dated block — every lifecycle verb that can merge a bead
  branch is inside the contract, and `impl approve`'s refusal routes
  settlement back to the single surface. A new ADR was considered and
  rejected for the same reason as in 114: the single-home property is itself
  part of the contract, and recording the reach extension anywhere else would
  recreate the two-sources drift the ADR exists to prevent. The §7 hatch
  semantics are RESTATED, not changed (hatches except the gate decision
  inside the recovery run, never the durable obligation, and never this
  refusal). No divergence; the amendment is Bead 3's job.
- **ADR-0030 (executor boundary) — best choice, unchanged; it DECIDES the
  design.** The REFUSE-not-RUN enforcement model is ADR-0030 applied:
  `internal/executor` is the git/process I/O boundary and must not house
  enforcement — and it structurally cannot (import cycle: complete →
  executor). The refusal therefore lives in the enforcement package
  `internal/approve`, pre-terminal, alongside the existing gates; the
  worktree-enumeration leg deliberately consumes `bead.WorktreeList()` (a
  bd-domain QUERY through `internal/bead`, already imported by approve) and
  `gitutil.IsAncestor` (a leaf utility, consumed the same way
  `internal/lifecycle` already does) rather than adding any executor seam —
  OQ4's no-seam resolution keeps `FinalizeEpic` behaviorally untouched, so
  no executor git-I/O enters `internal/approve` and no import cycle forms.
  No amendment needed.
- **ADR-0035 (agent error contract) — best choice, unchanged.** Every new
  refusal is a `guard.NewFailure` whose FINAL line is a GENUINE recovery:
  `mindspec complete <bead>` (via `Orphan.RecoveryCommand()`) for the
  orphan-with-branch and worktree-enumerated cases (where the command is
  demonstrably runnable), and the restoration-prerequisite recourse for the
  R3 branch-less case (where a bare `mindspec complete` would die at the
  step-3.5 merge-base, `complete.go:492-495` — ADR-0035 requires the recovery
  be truthful, so the message names the prerequisite). Infra-error refusals
  name the failed enumeration step. All conform to the convention
  `internal/guard/recovery_convention_test.go` enforces; the contract covers
  new failure modes by construction — no amendment.
- **ADR-0023 (beads as single state authority) — best choice, unchanged;
  115 is an application of its refinement.** A bead's `closed` STATUS is not
  proof of settlement: the gate treats status as a trigger for verification,
  never as evidence, and the authoritative record it consults is the durable
  `refutation_pending_entries`/`panel_refuted_entries` obligation on bead
  METADATA (114 R2) — exactly ADR-0023's division of labor (bd metadata is
  the durable state authority; the panel artifact stays removable). R3's
  fail-closed read discipline reuses 114's `bead.GetMetadata` and the same
  decode/coverage core (exported check-only, no second implementation). No
  amendment.

No ADR divergence is proposed anywhere in this plan; no ADR is superseded.

## Testing Strategy

- **Unit — `internal/approve` (the gate suite is the core surface).** Eleven
  new named `TestApproveImpl_*` functions plus the extended AST call-order
  anchor (Bead 2 step 7 enumerates them; the spec's Validation Proofs section
  is the canonical name list). All seam-driven through the `implXxxFn`
  package-var convention with BENIGN defaults, so the entire pre-existing
  approve suite passes with zero semantic modification (the AC2(b)/AC5
  discipline from 114). Fault injection per seam: `implScanOrphansFn`
  (orphan present / each of the three infra errors / forced no-orphan for the
  AC13 race), `implWorktreeListFn` (enumerated non-ancestor / error / ancestor
  / other-spec entries), `implIsAncestorFn` (false / error),
  `implClosedEpicBeadIDsFn` (membership / error), `implGetMetadataFn` (read
  error / corrupt), `implCheckObligationsFn` (uncovered / covered / error),
  and the stub `executor.Executor`'s `MergeBase` forced to exit-128 for AC11
  (or the real-git `chmod 000 .git/refs/heads` loose-ref fixture). Mutation
  non-occurrence is always seam-RECORDED (epic-close fn, phase-metadata fn,
  `FinalizeEpic` uncalled), never inferred.
- **Unit — `internal/lifecycle`.** `TestScanOrphanedClosedBeads_ErrorPreserving`
  drives the EXISTING seams (`findEpicBySpecIDFn`, `listClosedBeadsFn`,
  `branchExistsFn`, `isAncestorFn`, `orphans.go:28-41`) to prove the core
  propagates each cleanly-signalled error while the wrapper stays fail-open —
  pinned on the MIXED list (error for one bead, provable orphan later) so
  wrapper parity is byte-identical, not merely same-on-all-error. The bare
  `go test ./internal/lifecycle` run is the consumer no-regression pin
  (complete/next/doctor behavior unchanged). Bead 3 adds the AC10 structural
  ownership test (parses all OWNERSHIP.yaml files; exactly-one-claimant =
  workflow).
- **Unit — `internal/complete`.** `TestPendingObligationPredicate` pins the
  exported check-only predicate's fail-closed decode + (slot, round)-exact
  coverage against injected metadata readers; `reconcilePendingRefutations`'s
  existing suite (114) keeps passing unchanged, proving the re-expression
  over the shared core is behavior-preserving.
- **Unit/structural — `internal/executor`.** `TestFinalizeEpic_MergesOnlyWorktreeRealBranches`
  (AC12) is real-git: no-worktree branch ref, tag-shadow, dangling symref
  subtests beside the `finalize_orphan_test.go` / `merge_conflict_test.go`
  conventions. AC8 is the no-regression sweep + the comment-fix diff-grep
  pair — zero semantic expectation changes in the existing executor suite.
- **Integration-style — AC7 recovery convergence across the complete↔approve
  boundary.** Split across the package boundary because R3's
  `approve → complete` import edge makes an in-`package complete` call of
  `ApproveImpl` a cycle: (a) `TestCompleteRun_RegatesAlreadyClosedOrphan`
  (package-local in `internal/complete`, Bead 1) proves the recourse re-gates
  and converges after a durable refutation; (b)
  `TestApproveImpl_PassesAfterOrphanSettled` (`internal/approve`, Bead 2)
  proves the post-settle state passes the gate. Together they pin the full
  refuse → `mindspec complete <bead>` → re-gate → settle → approve loop.
- **RED-on-revert discipline.** Every named new test is ABSENT at the spec's
  review SHAs (`eb6a2ed1` / `77d8a1a9` / `18cc59d9` / `bcb255eb`, per AC),
  and every AC proof chains an existence discriminator
  (`grep -q 'func Test<Name>'`) with the exact-named `go test -run '<Name>'`
  — `go test -run` exits 0 on a no-match, so the grep is what makes each
  proof fail before the test lands. No package-wide run stands in for a
  named run.
- **Negative fences (every bead).** `! grep -rn 'BranchExistsE'
  --include='*.go' .` and `! grep -rn 'show-ref' --include='*.go' internal/`
  stay 0-hit; `git diff main -- internal/gitutil/` stays empty; the full
  four-package sweep (`go build ./... && go test ./internal/approve
  ./internal/executor ./internal/complete ./internal/lifecycle`) plus
  CI-matched `gofmt -l` closes every bead.
- **Shared test infrastructure (named, reused, never forked):** the
  `implXxxFn` seam-var pattern (`internal/approve/impl.go:27-45`), the
  lifecycle seam vars (`orphans.go:28-41`), the approve stub-executor
  fixtures behind `TestApproveImpl_HappyPath`/`_FinalizeEpicCalled`
  (`impl_test.go:75/:299`), the AST harness of `TestApproveImplCallOrder`
  (`impl_test.go:714`), the 114 metadata seam pattern
  (`completeGetMetadataFn`/`completeMergeMetadataFn`), and the real-git
  executor fixtures of `finalize_orphan_test.go`/`merge_conflict_test.go`.

## Provenance

Legend: compound ACs whose proof spans the Bead 1 → Bead 2 substrate/consumer
dependency edge (AC1, AC6, AC7) name their COMPLETION-OWNER bead in the Bead
column (the later bead, which fully satisfies the AC), with the prerequisite
substrate half annotated inline in that same cell — no proof test is authored
in two beads; only the AC→test mapping spans the edge.

| Spec AC | Bead | Verification step (named, runnable) |
|---|---|---|
| AC1 (orphan refuses + infra fail-closed + wrapper parity) | Bead 2 (lifecycle half lands in Bead 1) | `TestApproveImpl_OrphanRefuses` + `TestApproveImpl_OrphanInfraErrorFailsClosed` (approve) + `TestScanOrphanedClosedBeads_ErrorPreserving` (lifecycle, mixed-list); the spec's chained AC1 proof incl. the bare `go test ./internal/lifecycle` no-regression pin |
| AC2 (exemptions: ancestor, other-epic, clean path, deleted-branch normal path) | Bead 2 | `TestApproveImpl_OrphanExemptions` + `TestApproveImpl_DeletedBranchNoRefusal` + untouched `TestApproveImpl_HappyPath`/`_FinalizeEpicCalled` pins + the `git diff eb6a2ed1` no-deletion check, per the AC2 proof |
| AC3 (hatches do not bypass the refusal) | Bead 2 | `TestApproveImpl_HatchDoesNotBypassOrphan` (env + config hatch; no `MINDSPEC_SKIP_PANEL` in the message) |
| AC4 (call order, RED-on-revert) | Bead 2 | `TestApproveImplCallOrder` extended with the `ScanOrphanedClosedBeads` anchor; `grep -q 'ScanOrphanedClosedBeads' internal/approve/impl_test.go && go test ./internal/approve -run 'TestApproveImplCallOrder' -v` |
| AC5 (advisory slot naming, never load-bearing) | Bead 2 | `TestApproveImpl_AdvisorySlotNaming` (slot named / panel-removed identical refusal / no `refut` substring) |
| AC6 (obligation backstop + corrupt-plan fail-closed) | Bead 2 (predicate half lands in Bead 1) | `TestApproveImpl_ObligationBackstop` + `TestApproveImpl_CorruptPlanRefuses` (approve) + `TestPendingObligationPredicate` (complete) |
| AC7 (recovery convergence, split across the package boundary) | Bead 2 ((a) lands in Bead 1) | `TestCompleteRun_RegatesAlreadyClosedOrphan` (complete, Bead 1) + `TestApproveImpl_PassesAfterOrphanSettled` (approve, Bead 2) |
| AC8 (executor no-regression + comment fix) | Bead 1 | `go test ./internal/executor` + `git diff main -- internal/executor/mindspec_executor.go \| grep -c '^-.*rule as CompleteBead'` ≥ 1 + 0 remaining occurrences |
| AC9 (ADR amendment + skill refusal text, all three surfaces) | Bead 3 | `grep -n 'impl approve' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` ≥ 1 + `grep -c 'closed without'` ≥ 1 in the literal and both copies (all RED at `8020b3e2`, verified) |
| AC10 (exactly-one-claimant ownership, RED-on-revert) | Bead 3 | `TestLifecycleOwnershipExactlyOneWorkflowClaimant` (new structural test, `internal/lifecycle`); the shell one-liner retained as a non-discriminating pin only |
| AC11 (Fact 1: whole-store failure aborts pre-scan) | Bead 2 | `TestApproveImpl_UnreadableRefStoreAbortsPreScan` (errors at/before `exec.MergeBase`, `impl.go:249`; mutation seams uncalled) |
| AC12 (Fact 2: FinalizeEpic merges only worktree-enumerated real branches) | Bead 1 | `TestFinalizeEpic_MergesOnlyWorktreeRealBranches` (real-git: no-worktree ref / tag shadow / dangling symref) |
| AC13 (worktree-enum merge-prevention leg — the round-6 G2 race) | Bead 2 | `TestApproveImpl_WorktreeEnumRefusesDespiteProbeMiss` (race refusal / fail-closed list error / no false positives) |
