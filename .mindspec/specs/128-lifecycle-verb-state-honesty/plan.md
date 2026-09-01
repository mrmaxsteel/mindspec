---
status: Draft
spec_id: 128-lifecycle-verb-state-honesty
version: "1"
adr_citations:
  - ADR-0023
  - ADR-0025
  - ADR-0035
  - ADR-0037
  - ADR-0041
  - ADR-0042
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/phase/phaselock.go
      - internal/phase/phaselock_test.go
      - internal/phase/order.go
      - internal/phase/order_test.go
      - internal/phase/writer_inventory_test.go
      - cmd/mindspec/repair.go
      - cmd/mindspec/repair_test.go
      - internal/phase/migrate.go
      - internal/approve/plan.go
      - internal/approve/spec.go
      - internal/approve/impl.go
      - internal/approve/adopt.go
      - internal/complete/complete.go
      - cmd/mindspec/release.go
  - id: 2
    depends_on:
      - 1
    key_file_paths:
      - internal/approve/plan.go
      - internal/approve/plan_test.go
      - cmd/mindspec/next.go
      - cmd/mindspec/release.go
      - cmd/mindspec/release_test.go
      - internal/complete/complete.go
      - internal/phase/derive.go
      - internal/phase/derive_recovery_test.go
      - internal/phase/semantics_v2.go
      - internal/phase/semantics_v2_test.go
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/resolve/resolve.go
      - internal/resolve/resolve_test.go
      - cmd/mindspec/state.go
      - cmd/mindspec/state_test.go
      - internal/phase/derive.go
      - internal/workspace/workspace.go
      - .mindspec/adr/ADR-0023.md
  - id: 4
    depends_on:
      - 2
      - 3
    key_file_paths:
      - project-docs/user/guides/lifecycle-modes.md
      - internal/phase/agreement_table_test.go
      - .mindspec/adr/ADR-0023.md
  - id: 5
    depends_on:
      - 2
      - 3
    key_file_paths:
      - cmd/mindspec/instruct_tail.go
      - cmd/mindspec/instruct_tail_test.go
      - cmd/mindspec/spec.go
      - cmd/mindspec/complete.go
      - cmd/mindspec/verb_output_test.go
      - internal/complete/complete.go
      - internal/instruct/templates/indeterminate.md
      - internal/instruct/run.go
  - id: 6
    depends_on:
      - 1
    key_file_paths:
      - internal/complete/complete.go
      - internal/complete/complete_test.go
      - internal/validate/docsync.go
      - internal/validate/adr_divergence.go
      - internal/gitutil/gitops.go
      - internal/gitutil/snapshot.go
      - internal/gitutil/snapshot_test.go
      - internal/executor/executor.go
      - internal/executor/mindspec_executor.go
      - internal/executor/mock.go
      - .mindspec/adr/ADR-0041-gate-before-mutate.md
      - cmd/mindspec/adr0041_amendment_test.go
  - id: 7
    depends_on:
      - 1
    key_file_paths:
      - internal/next/guard.go
      - internal/next/guard_test.go
      - cmd/mindspec/next.go
      - internal/approve/spec.go
      - internal/approve/spec_test.go
      - .mindspec/adr/ADR-0025-jsonl-as-build-artifact.md
  - id: 8
    depends_on:
      - 2
    key_file_paths:
      - internal/next/select.go
      - internal/next/select_test.go
      - internal/next/beads.go
      - cmd/mindspec/next.go
      - cmd/mindspec/next_selection_test.go
      - internal/resolve/target.go
  - id: 9
    depends_on: []
    key_file_paths:
      - internal/bead/bdcli.go
      - internal/bead/bdcli_test.go
      - internal/next/beads.go
      - internal/next/beads_test.go
  - id: 10
    depends_on:
      - 4
      - 5
    key_file_paths:
      - internal/instruct/templates/idle.md
      - internal/instruct/templates/review.md
      - internal/instruct/templates_test.go
      - project-docs/user/guides/merge-safety.md
      - project-docs/user/guides/lifecycle-modes.md
---
# Plan: 128-lifecycle-verb-state-honesty

## Plan-level choices the spec delegates, resolved

Every resolution below was re-derived against the worktree tree at spec-approval
commit `1e41c7ce` (spec content `b7deb948`); the code claims cited were re-read
for this plan, not inherited from the spec text.

### OQ-1 — #186 mechanism: **(a) gates-before-commit reorder**, with worktree-inclusive gate measurement; ADR-0041 §1 amended as a priced reversal

**Chosen**: reorder the doc-sync and ADR-divergence gates to run BEFORE the
artifact-materialization commits (the `--commit-msg` `CommitAll` at
`internal/complete/complete.go:815` and the pathspec-scoped `.beads` sync
commit), measuring the bead worktree's *content* rather than its committed tip.

**Mechanism — the prospective-snapshot protocol (designed here, round-1 G1;
every primitive claim re-derived at HEAD)**. Today both gates late-bind the
range head — `beadHead` is the branch NAME resolved at `complete.go:634`, so
`ValidateDocsRange(root, base, beadHead, beadHead, exec)` (`complete.go:973`)
and `CheckADRDivergence(..., beadHead, beadHead)` (`:996`) resolve the ref at
git-command time, after the commits. A naive "diff base against the worktree"
replacement has three holes the round-1 gate found and this protocol closes:
`CommitAll` → `commitWithExport` commits with `git add -A`
(`mindspec_executor.go:1484` — untracked files INCLUDED) while every
working-tree diff primitive is plain `git diff` (`gitutil.DiffNameOnlyRef`,
`gitops.go:1352-1366`) — blind to untracked files; `commitWithExport` runs a
bd export BEFORE staging (`:1491`), mutating content between measurement and
commit; and `MindspecExecutor.ChangedFiles` is rooted at `g.Root`
(`mindspec_executor.go:1221`, with a cwd-rooted `DiffNameOnlyRef("", head)`
fallback at `:1210`), not the bead worktree. The protocol:

1. **Snapshot** (new gitutil primitive `SnapshotWorktreeTree(wtPath,
   excludePathspecs) (treeOID, error)`): in the bead worktree, stage the FULL
   prospective content into a TEMPORARY index — `GIT_INDEX_FILE=<tmp> git -C
   wtPath add -A -- . ':!<named-artifact>'` (tracked + untracked + deletions;
   the ADR-0025 named artifact is pathspec-EXCLUDED — the sync commit owns it
   and no gate reads it) — then `git write-tree` → tree OID `T`. The worktree,
   real index, and refs are untouched (a temp index + loose objects only).
2. **Measurement**: both gates measure `base → T` via a new tree-capable leg —
   executor interface gains `ChangedFilesTree(workdir, base, treeOID)
   ([]string, error)` (tree-vs-tree `git diff --name-only base T`, run `-C`
   the bead worktree; untracked files appear because `add -A` staged them into
   `T`; mock updated) — and every gate INPUT (OWNERSHIP manifests, domain
   enumeration, doc/ADR file contents) is read FROM `T` (`FileAtRef` with
   `T:path`), so attribution and content come from the same candidate tree the
   commit will produce (G1.3's executor-grounding face). A test pins that
   invoking `complete` from the main checkout still measures the bead
   worktree's snapshot, not `g.Root`'s state.
3. **Exactly-the-measured-tree guarantee, by construction**: on gate pass, the
   user materialization commit is created FROM `T` itself — plumbing
   `git commit-tree T -p <tip>` + fast-forward `update-ref` (new gitutil
   primitive; `CommitTreeMerge`/`UpdateRef` from spec 127 are the house
   precedent) — NOT by re-running `git add -A`. Consequences, each closing one
   G1.2 hole: a pre-commit hook cannot mutate the commit (plumbing runs no
   hooks); a worktree edit in the gate→commit interval cannot enter the commit
   (the commit is built from `T`, the edit stays uncommitted dirt for the next
   run); the bd export cannot enter it (the artifact is excluded from `T`; the
   export lands in the separate, pathspec-scoped artifact-sync commit the flow
   already has, which no gate measures — the artifact-only normalization
   treatment). "Committed tree ≡ measured tree" is thereby mechanical, not
   asserted: they are the same object.
4. Reconcile mode (`beadHead = landed.SHA`, `complete.go:704`) keeps its
   landed `M^1..M` measurement — no commits are authored on that path.

**Honest scope (round-1 G5.4)**: (a) closes the **gate-refusal class** — no
complete-authored commit can precede a fallible-gate refusal, so a refusal
always leaves the tip at `reviewed_head_sha` and the retry meets the same
gate. A POST-gate failure (bead→spec merge conflict, bd-close failure) still
occurs after the materialization commits, exactly as at HEAD — recovery there
is spec 127's preserved-merge/`--resolve-merge` and forward-reconcile
machinery, and that residual is out of this spec's scope and stated, not
papered over.

**Why (a) over (b)/(c)**:
- (b) rollback is a DOUBLE reversal (ADR-0041 §1 Reconcile "Never a rollback" +
  §2, exactly as the spec priced) and introduces rollback machinery adjacent to
  preserved merges — the hazard class spec 127 spent four rounds burning down.
  Rejected.
- (c) the paths-only tracker exemption closes only the tracker-sync instance of
  the wedge. R4.1 demands the retry never be stale-blocked by ANY
  complete-authored delta — and the `--commit-msg` `CommitAll` commit is
  complete-authored too; under (c) a gate refusal after a non-empty `CommitAll`
  still wedges the retry on stale-SHA. (a) closes the entire GATE-REFUSAL
  class by construction (the honest-scope boundary above — post-gate failures
  are out of scope and stated): no complete-authored commit exists before any
  fallible gate, so a refusal always leaves the tip at `reviewed_head_sha` and
  the retry meets the same gate. Rejected as primary; NOT taken as a backstop either — one
  mechanism, one amendment.

**Consequences the spec pre-priced, now settled**: ADR-0041 §1 is amended as an
explicit reversal (pre-drafted in bead 6, with a pinning test extending
`cmd/mindspec/adr0041_amendment_test.go`); ADR-0037 §4 is NOT amended;
`internal/panel` and `panel.GateFacts` are UNTOUCHED (the spec's conditional
Impacted-Domains/In-Scope entries resolve to not-touched); AC-6's diff-pathspec
sentence and AC-7(ii)–(iv) are tagged "(c) only" in the spec and therefore do
not exist; AC-7(i) (a post-review non-tracker commit still trips stale-SHA)
holds trivially — the stale-SHA gate itself is untouched — and stays pinned
red-on-revert. The panel-gate-before-`CommitAll` ordering
(`complete.go:710-715`) is unchanged.

### OQ-2 — draft-mode derivation: **the on-disk draft carve-out** (127-R1's proposal), as a narrow ADR-0023 amendment

**Chosen**: pre-epic draft discovery is filesystem-scoped — a spec is in draft
(`spec`) mode when its spec directory + `spec/<id>` branch exist and no
lifecycle epic exists; from `spec approve` (epic creation, the durable approval
record) onward, beads is the sole authority, unchanged.

**Rationale**: this is the row ADR-0023's own derivation table has always
promised ("No epic with matching spec_num → **spec**"), and the mechanism the
code already ships on one path — `ResolveContextFromDirWithCache`'s
`WorktreeSpec` branch derives `ModeSpec` from worktree identity with no bd call
(`derive.go:568-579`). The amendment closes C1-1 by scoping the authority
model's "all lifecycle discovery goes through beads" to post-approval
lifecycle, matching table and code. The alternatives lose: a bead-side draft
marker (ii) couples `spec create` to bd availability — today `spec create`
makes zero bd writes, and R1.4 just made bd-failure loud rather than
load-bearing; a separate honest field with mode kept beads-only (iii) leaves
`mode` itself lying by omission ("idle" while a draft is active), which is the
defect class this spec exists to remove, merely relabeled.

**Shape**: `resolve.ResolveModeWithCache`'s no-epic return (`resolve.go:57-67`)
gains the draft probe (workspace scan: spec dir + branch) before defaulting;
`state show` (`cmd/mindspec/state.go:80-97`) is recomposed so ONE derivation is
authoritative (R1.2): the context path's result is used for mode, and the
`ResolveMode`-vs-context disagreement is asserted away by test. Main-checkout
draft-only state → `{"mode":"spec","activeSpec":...}` when exactly one draft
exists; multiple drafts → the existing ambiguity listing (not a fabricated
single mode). Discovery *failure* → R1.4's non-zero refusal (bead 3 chooses the
refusal form over a 0-exit indeterminate object: `state show` is a
read-only query; failing loudly is strictly more honest and simpler than a
second success shape — the "explicit machine-distinguishable indeterminate
result" option is reserved for the instruct tail, which must not fail the
verb it trails).

### OQ-3 — plan-approve row: **option 1 — write `plan` at plan-approve; the first lifecycle claim advances stored to `implement`**

**Chosen**: `plan approve` writes `mindspec_phase: "plan"` (`plan.go:455`
value change); `mindspec next` gains a stored-phase advance to `implement`
immediately after a successful claim of a LIFECYCLE (task-typed) bead — a NEW
`mindspec_phase` writer that joins the R3.3 lock and is caught by the
writer-inventory test by construction; `release`'s step-6.5 sync takes the
sanctioned redesign leg (writes the CHILD-DERIVED phase via the guard-routed
internal caller — see the sanction boundary below); `complete`'s step-6.5 sync
keeps writing the derived `NextMode` (under the lock, from bead 1).

**Rationale**: option 1 makes stored and derived AGREE at every point of the
plan-approve → first-claim window, which dissolves the #248 warning in BOTH
fixture routes rather than suppressing it: after plan approve, stored=plan =
derived (all-open, `derive.go:387`, the settled row); after the first lifecycle
claim, stored=implement = derived (in_progress>0, `derive.go:370-371`); after
releasing the sole claimed bead, the sanctioned release sync returns stored to
plan = derived (all-open again — matching `ADR-0023.md:141`). The round-6
advisory's hazard evaporates under this option: in the complete→plan trigger-2
state (non-lifecycle child completed, lifecycle tasks all open, no lifecycle
claim ever made) stored is still `plan`, so `complete`'s step-6.5
`NextMode=plan` write is plan-over-plan — NOT a backward write, and no
sanctioning is needed for it. The agreement table's plan-approve row becomes a
TRUE three-surface agreement; the release row's plan leg becomes a true
TWO-surface agreement — its tail-less carve-out stands (round-1 G5.5's
correction: a tail-less row cannot be a three-surface anything). Option 2 would have preserved a
permanently quirky window (annotated rows, suppressed warnings, two unforced
backward writers needing sanction); option 3 stays CLOSED — no audited
refutation of settled fact 1 exists or is sought, and option 1 makes the
shipped system MATCH the settled row, the opposite of relitigating it.

**Costs, stated**: one new writer in `next` (lock + inventory + a
lifecycle-only condition: a non-lifecycle child claim advances nothing);
`repair phase`'s backward guard (R3.3) still ships unchanged — it protects
every OTHER window and the operator path. AC-2's option-2 carve-out branches
(the annotated stored-leads-derived fixtures) become dead arms: the fixtures
assert the true-agreement contract. AC-5(i)'s "or the warning is absent"
mechanism arm is the one that fires — the fixtures assert NO stored-vs-derived
warning is emitted on either route because there is no disagreement.

**Sanction boundary (round-3 G5, honored; API pinned per round-1 G4.2)**: the
ONLY unforced backward writer is `release`'s sync, which is the single
production caller of the exported `phase.SanctionedDerivedSync(root, epicID)`
— created in bead 1 beside the comparator, doc-commented as the only
sanctioned backward write surface, its exported-ness fenced by a call-site
enumeration test (the `impl adopt` house pattern) pinning exactly one caller;
it re-derives fresh child-derived evidence inside the lock's critical section
by construction. The operator `repair phase` still refuses backward without
`--force-backward` + reason (AC-5(viii) distinguishes them).

### OQ-5 — default-pick rule: **displayed row 1** (bd ready's priority-first order)

**Chosen**: with multiple ready beads and no `--pick`/positional, select
displayed row 1 — the head of `bd ready --parent`'s priority-respecting order,
which R6.2/AC-9 already settle as the display order. The printed line becomes
self-describing: `Defaulting to item 1 (bd ready priority order). Use --pick=N
or a bead ID to select a specific item.`

**Rationale**: plan order is already enforced where it is authoritative — the
`work_chunks` dependency edges gate READINESS (spec 097 R3), so beads that are
simultaneously ready are beads the plan itself declared order-free, and
priority is the remaining meaningful signal. A lowest-bead-number default would
re-derive "plan order" from ID lexicography — a second, weaker authority for a
fact the dependency edges already own, the exact dual-derivation anti-pattern
R6.5 struck prose-parsing for. It would also silently contradict an operator's
explicit priority edits (#187's own repro raised a priority expecting it to
matter).

### The two inherited advisories, carried

**(a) round-6 R3 (trigger-2 / OQ-3 interlock)**: discharged by the OQ-3
option-1 choice, as derived above — in the trigger-2 state stored is `plan`,
`complete`'s step-6.5 write is plan-over-plan (no backward write, nothing to
sanction), and AC-2's complete→plan trigger-2 fixture asserts TRUE agreement
(no annotated-row treatment needed; the option-2 arm is dead). The sanction
sentence stays true with exactly one sanctioned backward writer (release's
sync). Bead 4's fixture list names trigger-2 explicitly.

### Delegated dispositions the spec left to plan time, recorded (round-1 G5)

- **R2.4 — #245's exact 0.12.0 shape, re-examined at HEAD (executed
  2026-09-01, this round)**: the shape (success with zero result lines) is
  STRUCTURALLY UNREACHABLE at HEAD — `complete`'s RunE has exactly one
  success return, through `completeTail` (`cmd/mindspec/complete.go:145`),
  whose first statement prints `complete.FormatResult` (`:157`), whose first
  line is `"Bead %s closed."` (`internal/complete/complete.go:1380`); the
  real-wiring tests `TestCompleteTail_NoteIsLastStdoutLine` /
  `TestCompleteTail_NoNoteWhileCwdExists` were RUN this round and pass.
  Disposition: **fixed-by-spec-092** (the completeTail extraction), with
  AC-4's ordering pin and AC-14's `complete` row as the regression guards.
  (A full end-to-end complete could not be executed in the fix round —
  lifecycle verbs are barred here — so the executed evidence is the
  real-wiring test run plus the single-success-path call-graph derivation.)
- **AC-5(iv) — the retained warning's recovery line**: where the
  stored-vs-derived mismatch warning remains reachable (out-of-band `bd`
  edits; states outside the dissolved windows), its recovery line becomes
  DIRECTION-CONDITIONAL (bead 2, at the emission site `derive.go:190-193`):
  a FORWARD disagreement (derived ahead of stored) keeps
  `recovery: mindspec repair phase <spec>` — safe and correct there; a
  BACKWARD disagreement emits an inspection action
  (`recovery: bd show <epic-id>` + a one-line explanation), never the
  backward write — satisfying AC-5(iv)'s letter with belt (the line) and
  suspenders (bead 1's guard would refuse the write anyway).
- **R3.5 — phase.Cache noise reduction: TAKEN**, as a side effect of bead 3's
  `state.go` recomposition: one derivation authoritative means one
  `phase.Cache` constructed per invocation and passed through the existing
  `WithCache` variants — within R3.5's stated bar (no new exported API).
- **OQ-1 scope flow-through**: bead 10's `merge-safety.md` text and the
  ADR-0041 amendment describe the GATE-REFUSAL guarantee only; post-gate
  failure recovery remains the 127 machinery and the docs keep saying so.

**(b) AC-4b seam shape — decided**: a package-level function seam
`completeFetchChildrenFn = phase.FetchChildren` in `internal/complete`
(the exact house pattern of `completeMergeMetadataFn`, `complete.go:46`),
replacing the direct call at `complete.go:1434`; `advanceState` gains an error
return (`(mode, nextBead string, err error)`); `Run` maps a non-nil error to
`Result.NextModeIndeterminate = true` + reason — `FormatResult` renders the
indeterminate form and step-6.5 SKIPS its phase write entirely (no value
fabricated from a nil child set). Bead 5 implements; AC-4b asserts through the
real result path.

### Two plan-scope decisions the beads depend on

**R3.3 lock home**: `internal/phase/phaselock.go` — new file, journal-idiom
(`acquireFileLock`: `flock(2)` unix / O_EXCL-lockfile windows, mirroring
`internal/journal/lock.go:55-61`), lock file under `.mindspec/`
(gitignored-adjacent runtime state, exact name pinned in bead 1). `internal/
phase` is imported by every writer package (`internal/approve`,
`internal/complete`, `cmd/mindspec`, and `phase` itself for `migrate.go`), so
no new import edges are created; the lock helper exposes
`WithPhaseWriteLock(root string, fn func(stored string) (write string, ok bool, err error))`
— read-stored → caller decides → conditional write, all inside the critical
section. The backward comparator (`spec < plan < implement < review < done`)
lands beside it in `internal/phase/order.go`.

**AC-5 leg mapping**: AC-5's eight legs split across beads 1 and 2 at the
stated grain — (ii),(iii),(v),(vi),(vii) are lock/guard/inventory machinery
(bead 1); (i),(iv),(viii) are warning-behavior and sanction legs that need the
OQ-3 value semantics (bead 2). The AC as a whole is complete when bead 2
lands; the Provenance table records the split explicitly rather than forcing a
false single-bead mapping.

### Decomposition note (the three validate-plan WARN heuristics, answered)

`mindspec validate plan` emits three advisory WARNs on this plan: bead count
(10 vs the 3–5 heuristic), scope-redundancy R=0.09, and chain depth 4. All
three are deliberate: spec 127's own nine-slot gate SPLIT a 7-issue/5-bead
plan as over-compressed (O3 measured ~10 independently-reviewable
deliverables), and this spec covers six issue clusters with sixteen ACs —
each bead here is one independently-reviewable deliverable with its own AC
set, which is the sizing the gate history demands. The low scope-redundancy
score reflects clean seams — the sharing policy actually applied (round-1
G4.3): SAME-REGION overlap on a file gets a serializing dependency edge
(bead 8 → 2 on `next.go`'s claim block; bead 6 → 1 on `complete.go`'s
materialization region), while DISTINCT-REGION overlap relies on the serial
bead cycle's one-at-a-time merges (bead 5 vs 6 on `complete.go`) — and the
depth-4 chain
(1→2→4→10 / 1→2→5→10) costs nothing in practice: the bead cycle executes
serially, and beads 3, 7, 8, 9 are ready in parallel with the chain from the
start.

## ADR Fitness

- **ADR-0041** — **AMENDED (§1, a deliberate reversal — OQ-1(a))**: the
  doc-sync/ADR-divergence gates move from validate-the-committed-tip to
  validate-the-content-to-be-committed (worktree-inclusive measurement in the
  bead worktree), so every fallible gate precedes every complete-authored
  commit and "exit non-zero implies tip == reviewed_head_sha" holds
  unconditionally. §1's load-bearing rationale (the gates must not miss the
  just-committed work) is preserved by measurement equivalence: the commits
  materialize exactly the measured content. The amendment text is pre-drafted
  in bead 6 and pinned by extending `cmd/mindspec/adr0041_amendment_test.go`.
  §2/§3 (forward-reconcile, never rollback) are untouched — (a) introduces no
  rollback.
- **ADR-0023** — **AMENDED, three edits**: (1) the OQ-2 draft-discovery
  exception (pre-epic drafts are filesystem-discovered; beads-authoritative
  from epic creation onward) — bead 3; (2) the OQ-4-resolved editorial strike/
  supersede-annotation of the superseded `:107` and `:108` edge-case rows —
  bead 4; (3) the agreement table lands as/alongside the ADR's derivation
  documentation — bead 4. No derivation-rule row changes: OQ-3 option 1 makes
  the WRITERS match the existing rows.
- **ADR-0025** — **AMENDED (mandatory, both R5 directions)**: decision item
  1's unqualified "User-authored dirt still blocks" is scoped to dirt the
  claim can entangle (an empty set today — R5.1's derived enumeration); the
  named-artifact list model ("explicit and small … not a glob") is unchanged.
  **Scope fence (round-1 G6)**: the amendment is expressly limited to the
  worktree-creating `mindspec next` claim path — `mindspec approve`'s and
  every other dirty-tree guard's semantics are unchanged unless separately
  amended. Bead 7.
- **ADR-0037** — **NOT amended** (OQ-1 resolved to (a); §4 staleness and the
  `internal/panel` zero-subprocess confinement are untouched).
- **ADR-0035** — APPLIED: every new refusal/warning keeps the copy-pastable
  final recovery line (R1.4's discovery refusal, R3.6's replay refusal, the
  indeterminate tail's resolving action, `repair phase`'s backward refusal).
- **ADR-0042** — APPLIED: new messages render IDs via `idrender`, agent-
  writable text (bd stderr lines, bead titles, exclusion reasons) via
  `termsafe.Escape` (AC-11's rendering leg pins the bd-stderr case).

## Testing Strategy

- **Fixture repos over mocks** for lifecycle-state assertions: throwaway git
  repos + seeded bd state via the existing test seams
  (`planMergeMetadataFn`, `completeMergeMetadataFn`, `repairMergeMetadataFn`,
  `SetMergeMetadataForTest`, `runBDFn`-family), following the house pattern.
  The two #248 fixture routes (plan-approve-shaped and release-shaped) are one
  shared fixture builder used by beads 2 and 4.
- **Real-lock contention tests** (bead 1): contending goroutines on the real
  flock in a temp repo — no simulated locking.
- **Anti-drift tests are total, not sentinel**: the writer-inventory test
  (AC-5(vi)) greps/AST-walks for `mindspec_phase`-writing call sites and
  asserts lock coverage; the verb-classification test (AC-14) walks the
  registered cobra tree and requires every command classified
  (mutating-with-row or allowlisted-non-mutating with rationale).
- **`-short` discipline** (AC-13, settled fact 3): all new tests run under
  `go test ./... -short`; nothing added to `internal/harness`'s LLM-gated
  set. Every bead's verification includes the full `-short` run.
- **Red-on-revert**: each behavioral AC names its counterexample (the spec's
  captured outputs); fixtures assert the NEW behavior so reverting the fix
  turns them red.

## Bead 1: The stored-phase write lock, backward guard, and `repair phase` hardening

The root of the graph — R3.3's machinery, which four later beads consume.

**Steps**
1. `internal/phase/order.go` (new): the lifecycle order comparator
   (`spec < plan < implement < review < done`; unknown values compare
   incomparable → treated as backward for guard purposes, stated in the doc
   comment) + `internal/phase/order_test.go`. Beside it, the **sanctioned
   backward API** (round-1 G4.2 pin): exported
   `phase.SanctionedDerivedSync(root, epicID) error` — re-derives the
   child-derived phase INSIDE the lock's critical section and writes it,
   backward permitted, doc-commented as the ONLY sanctioned backward write
   surface; its exported-ness is fenced by a call-site enumeration test (the
   `impl adopt` house pattern) pinning exactly ONE production caller
   (`cmd/mindspec/release.go`, wired in bead 2). Bead 1 creates the API and
   the fence; bead 2 wires the caller.
2. `internal/phase/phaselock.go` (new): journal-idiom file lock
   (`flock(2)` unix / O_EXCL windows, per `internal/journal/lock.go:55-61`'s
   pattern) exposing `WithPhaseWriteLock(root, epicID, fn)` — reads stored
   phase inside the critical section, passes it to `fn`, performs the
   conditional `MergeMetadata` write `fn` returns, releases. **Operational
   design (round-1 G3), pinned here:** lock path = `.mindspec/phase.lock`;
   the managed `.gitignore` block gains that entry (beside the existing
   `.mindspec/session.json` line) and a test asserts lock acquisition leaves
   zero blockable git dirt; acquisition uses a BOUNDED wait (poll-with-
   deadline, default 10s, constant pinned in the file) — on timeout, an
   ADR-0035 refusal naming the lock path and a re-run recovery line (never a
   delete-the-lock instruction); unix crashed holders self-release (flock is
   kernel-held); the Windows O_EXCL path writes holder PID + RFC3339
   timestamp into the file and treats a file older than the pinned staleness
   bound as stale (remove + single retry, best-effort within the same-machine
   model, documented). Tests: real-goroutine contention (`-race`), timeout
   error propagation, crashed-holder recovery (kill the flock holder;
   next acquire succeeds), stale-file takeover (seam-driven for the Windows
   path), and the no-git-dirt assertion.
3. Wrap every existing `mindspec_phase` writer in the lock (values unchanged
   in this bead): `plan.go:455`, `spec.go:76`, `complete.go:1366`,
   `release.go:152`, `impl.go:577-579`, `impl.go:611-613`, `adopt.go:424/:437`,
   `migrate.go:59-61` (its absent-check moves inside the critical section),
   `repair.go:189-191`. The `spec.go:84` creation seed stays excluded (spec's
   stated rationale).
4. `cmd/mindspec/repair.go`: the backward-move guard — refuse a write that
   moves stored backward unless `--force-backward "<reason>"` (flag name
   pinned here) is passed; refusal carries an ADR-0035 recovery line
   (inspection action); forced success output names the recorded reason;
   the guard compares against the stored value read under the lock.
5. `internal/phase/writer_inventory_test.go` (new): the AC-5(vi) anti-drift
   test — every `MergeMetadata` call site writing `mindspec_phase` executes
   inside the lock's critical section; a new unlocked writer is red.
6. Legacy-migration contention test (AC-5(vii)) via `SetMergeMetadataForTest`
   (`migrate.go:21`) + a contending locked writer.

**Verification**
- [ ] `go build ./... && go test ./... -short` green
- [ ] Contention tests pass under `-race`

**Acceptance Criteria**
- AC-5(ii) backward refusal leaves stored unchanged, exits non-zero, recovery line
- AC-5(iii) forced write records AND prints the reason
- AC-5(v) concurrent-writer leg (final stored value is `review`)
- AC-5(vi) writer-inventory anti-drift
- AC-5(vii) legacy-migration contention

**Depends on**
None (bd edges wired from `work_chunks[1]`).

## Bead 2: OQ-3 option 1 — phase-write value semantics across the four verbs

**Steps**
1. `internal/approve/plan.go:455`: write `"plan"` (was `"implement"`), under
   the bead-1 lock.
2. `cmd/mindspec/next.go`: after a successful `ClaimBead` of a LIFECYCLE
   (task-typed or untyped — the ADR-0023 discriminator) bead, advance stored
   phase to `implement` under the lock (forward write; no-op when already
   `implement`). A non-lifecycle child claim advances nothing. This is a NEW
   writer: it registers in the bead-1 inventory by construction.
3. `cmd/mindspec/release.go`: the sanctioned redesign leg — `syncPhase`
   (`release.go:152`) becomes the ONE production caller of
   `phase.SanctionedDerivedSync` (created + call-site-fenced in bead 1),
   which re-derives the child-derived phase inside the critical section and
   writes it, backward permitted; the operator guard in `repair.go` is
   unaffected.
4. `internal/complete/complete.go:1366`: step-6.5 keeps writing derived
   `NextMode` under the lock (wrapped in bead 1; value semantics unchanged —
   in the trigger-2 state this is plan-over-plan, not backward).
5. Fixtures: the two #248 routes (plan-approve-shaped: `plan approve` → state
   probes; release-shaped: claim → release → state probes) assert stored ==
   derived at every probe point and that NO stored-vs-derived warning or
   `repair phase` recovery line is emitted (`derive.go:190-193` never fires on
   these states) — AC-5(i)'s "warning is absent" arm, both routes.
6. AC-5(viii): on the released all-open state, release's internal sync
   succeeded unforced (stored back to `plan`) while `mindspec repair phase`
   invoked on a constructed backward case still refuses without
   `--force-backward`.
7. **Upgrade reconciliation for legacy epics (round-1 G2)** —
   `internal/phase/semantics_v2.go` (new): `EnsurePhaseSemanticsV2(epicID)`,
   the one-shot, idempotent, lock-protected reconciliation mirroring
   ADR-0034's `EnsureMigrated` pattern and invoked from the same
   ahead-of-preflight call sites (`impl.go:277`, `plan.go:300`,
   `complete.go:522`) plus `next`'s claim path. **Durable discriminator**: the
   epic metadata marker `mindspec_phase_semantics: "v2"`. Marker absent AND
   the legacy signature holds (stored `implement`, ≥1 lifecycle child, all
   open, none closed, none in_progress — under old semantics that is the
   never-claimed plan-approve state; under v2 semantics stored would be
   `plan` there) → rewrite stored to `plan` + write the marker, one critical
   section. Marker absent, any other state (progressed, released-with-closed-
   siblings, review, done) → write the marker only, phase untouched. Marker
   present → no-op. The v2-unreachability claim is scoped honestly: an
   out-of-band raw `bd update` can still manufacture the signature post-
   marker, and the marker is exactly what prevents re-reconciling it.
   **Five fixtures**: upgrade-before-first-claim (legacy → `plan` + marker);
   upgrade-then-first-claim (claim advances to `implement`); non-lifecycle
   claim (no advance, marker written); release (returns to `plan`, marker
   present, no re-reconcile); already-progressed epics (each of
   some-closed / in_progress / review / done — phase untouched, marker
   written).

**Verification**
- [ ] `go build ./... && go test ./... -short` green
- [ ] Both fixture routes assert zero warning emissions on stderr

**Acceptance Criteria**
- AC-5(i) (both routes, warning-absent arm), AC-5(iv) (the general mismatch
  warning, where still reachable outside these windows, retains its safe final
  recovery line), AC-5(viii)

**Depends on**
Bead 1 (lock + guard + sanctioned entry).

## Bead 3: Draft-mode derivation and `state show` honesty (OQ-2, R1.1/R1.2/R1.4)

**Steps**
1. `internal/resolve/resolve.go`: `ResolveModeWithCache`'s no-epic path gains
   the on-disk draft probe (spec dir + `spec/<id>` branch via
   `internal/workspace`) → `state.ModeSpec` for a draft; bd/discovery ERRORS
   propagate instead of collapsing to idle (`resolve.go:60-63` today returns
   `ModeIdle, nil` on lookup error — split error from empty).
2. `cmd/mindspec/state.go`: recompose so the context derivation is
   authoritative for mode (R1.2) — drop the `ResolveMode`-for-mode /
   context-for-bead split (`state.go:80-97`); the fallback path no longer
   serializes an empty `ctx.Phase`: discovery failure → non-zero ADR-0035
   refusal naming the failed query (R1.4; the plan resolves R1.4's
   either/or to the refusal form for `state show`).
3. Main-checkout draft rendering: exactly one draft → `{"mode":"spec",
   "activeSpec":…,"specBranch":…}`; multiple drafts → the existing ambiguity
   listing; draft + active epic states unchanged.
4. `.mindspec/adr/ADR-0023.md`: the draft-discovery exception amendment
   (OQ-2's scoping paragraph).
5. Fixtures for AC-1's three legs, red-on-revert against the three captured
   2026-09-01 outputs.

**Verification**
- [ ] `go build ./... && go test ./... -short` green
- [ ] Live probe: `state show` in this very worktree reports `spec`, not
      `idle`-with-activeSpec

**Acceptance Criteria**
- AC-1(i), AC-1(ii), AC-1(iii)

**Depends on**
None.

## Bead 4: The per-verb agreement table, its row fixtures, and the ADR-0023 strike

**Steps**
1. The table lands in `project-docs/user/guides/lifecycle-modes.md` (new
   operator doc) AND is referenced from ADR-0023 (the ADR keeps derivation
   authority; the guide carries the per-verb view — location decision made).
2. `internal/phase/agreement_table_test.go`: one fixture test per row against
   the status-aware derivation surface (`DerivePhaseWithStatus`-class) + the
   `state show` output + tail template selection, with the carved rows:
   release (two surfaces, tail-less), terminal rows (epic `done` via
   `derive.go:200-202` + global idle via zero-open-epic discovery),
   complete→plan BOTH triggers (`derive.go:366` empty-lifecycle; `:387`
   all-open via non-lifecycle child — under OQ-3 option 1 both assert true
   agreement, the option-2 annotated arms being dead).
3. `.mindspec/adr/ADR-0023.md`: strike/supersede-annotate rows `:107` and
   `:108`.
4. Table/fixture cross-pin so editing one without the other is red (AC-2's
   closing clause).

**Verification**
- [ ] `go build ./... && go test ./... -short` green

**Acceptance Criteria**
- AC-2

**Depends on**
Beads 2 (option-1 semantics the rows assert) and 3 (honest `state show` the
rows assert).

## Bead 5: Verb output honesty — result-first, mode-agreeing tails, the indeterminate form

**Steps**
1. `cmd/mindspec/instruct_tail.go`: `emitInstruct` distinguishes indeterminate
   from idle (R2.3) — context error or empty phase renders the indeterminate
   block ("cannot determine mode: <reason>" + ADR-0035 final `recovery:` line;
   template `internal/instruct/templates/indeterminate.md`), NEVER `idle.md`.
2. `cmd/mindspec/spec.go`: spec-create tail renders spec-mode guidance (rides
   bead 3's draft derivation through `emitInstruct`); AC-3's pinning test
   (idle "Available Actions" absent from success output).
3. `internal/complete`: the AC-4b seam (`completeFetchChildrenFn`),
   `advanceState` error return, `Result.NextModeIndeterminate` + reason;
   `FormatResult` renders the indeterminate form; step-6.5 skips its write on
   indeterminate; ModePlan message reworded to fit both true triggers (exact
   wording landed here, satisfying the plan-time disposition).
4. `completeTail` branch test (AC-4's two mutually exclusive branches).
5. `cmd/mindspec/verb_output_test.go`: AC-14's table-driven result-before-
   guidance test over all nine verbs + the total cobra-tree classification
   (mutating-with-row vs rationale-carrying non-mutating allowlist).

**Verification**
- [ ] `go build ./... && go test ./... -short` green

**Acceptance Criteria**
- AC-3, AC-4, AC-4b, AC-14

**Depends on**
Beads 2 and 3 (the post-states the tails must agree with).

## Bead 6: Complete gate reorder (OQ-1(a)) — the prospective-snapshot protocol + ADR-0041 §1 amendment

**Steps**
1. `internal/gitutil`: the two snapshot primitives —
   `SnapshotWorktreeTree(wtPath, excludePathspecs)` (temp-index `add -A` with
   the named-artifact exclusion + `write-tree`; worktree/index/refs untouched,
   asserted by test) and the plumbing commit-from-tree
   (`commit-tree T -p <tip>` + fast-forward `update-ref`, following the
   spec-127 `CommitTreeMerge`/`UpdateRef` precedent).
2. `internal/executor`: interface + `MindspecExecutor` + mock gain
   `ChangedFilesTree(workdir, base, treeOID)` and tree-grounded input reads;
   the gate-input surfaces (OWNERSHIP manifests, domain enumeration, doc/ADR
   contents) read from `T:path`, never from `g.Root`'s checkout (closes the
   `ChangedFiles` `g.Root`-rooting and the cwd-rooted `DiffNameOnlyRef("",…)`
   fallback holes, `mindspec_executor.go:1210/:1221`).
3. `internal/validate`: `ValidateDocsRange`/`CheckADRDivergence` snapshot legs
   over `base → T` (BOTH gates, same protocol); committed-range behavior
   preserved for reconcile mode and the impl-approve whole-branch backstop.
4. `internal/complete/complete.go`: snapshot taken once after the panel gate;
   both gates measure it; on pass, the user commit is created from `T`
   (replacing the `CommitAll` porcelain path for this flow) and the artifact
   sync commit follows; panel gate stays first; reconcile path untouched.
5. `.mindspec/adr/ADR-0041-gate-before-mutate.md`: the §1 amendment REPLACES
   BOTH load-bearing statements (round-1 G1.4): (i) panel-before-`CommitAll`
   is RETAINED with its reviewed-SHA rationale; (ii) gates-after-
   materialization is REPLACED by prospective-snapshot-before-any-complete-
   authored-commit, stating the artifact-only normalization treatment (the
   export is excluded from `T` and measured by no gate) and why the gates
   cannot vacuously miss content (untracked files are staged into `T`; the
   commit IS `T`). No measured-equals-committed prose claim survives — the
   equality is the commit-tree construction. Extend
   `cmd/mindspec/adr0041_amendment_test.go`.
6. AC-6 + AC-7(i) fixtures: panel-pass at SHA A + dirty tracker +
   doc-sync-failing change → run 1 fails doc-sync with tip STILL at A; run 2
   with `--allow-doc-skew` is decided by doc-sync, never stale-SHA; and
   red-on-revert, a post-review non-tracker commit still trips stale-SHA
   (gate untouched).
7. Failure-injection fixture legs (round-1 G1.5), all four: (i) an UNTRACKED
   source/doc/ADR file in the worktree — the gate measures it and refuses
   before any commit; (ii) a pre-commit hook that adds/changes a source file —
   the committed tree is still ≡ `T` (plumbing runs no hooks); (iii) the
   tracker export changing content post-measurement — asserted confined to
   the excluded artifact path; (iv) a worktree mutation in the
   gate→commit interval — absent from the commit, present as dirt afterward.

**Verification**
- [ ] `go build ./... && go test ./... -short` green
- [ ] AC-6 fixture asserts `git rev-parse` tip unchanged across the refusal
- [ ] Cross-checkout test: `complete` invoked from the main checkout measures
      the bead worktree's snapshot

**Acceptance Criteria**
- AC-6 (convergence assertions; the (c)-only pathspec sentence vacated), AC-7
  ((i); legs (ii)–(iv) vacated as (c)-only)

**Depends on**
Bead 1 (the reorder repositions the sync-commit/`CommitAll` region whose
step-6.5 write bead 1 lock-wraps — same-file seam management; bead 5's
`advanceState`/`Result` changes live in a different region of `complete.go`,
and the serial bead cycle merges beads one at a time, so no edge to bead 5 is
needed).

## Bead 7: Proportional claim guard + `spec approve` safe replay + ADR-0025 amendment

**Steps**
1. `internal/next/guard.go` + `cmd/mindspec/next.go`: remove the
   invoking-worktree user-dirt block for the worktree-creating claim path
   (R5.1's derived-empty enumeration); keep the pinned in-code
   claim-written-paths list (empty beyond exempt artifacts) + the
   enumeration-pinning test + the seam-injected refusal leg preserving the
   `DirtyTreeFailure` contract.
2. `internal/approve/spec.go:76`: R3.6 absent-only backfill under the bead-1
   lock (write `plan` only when stored phase is empty); replay output
   distinguishes backfill / no-op / later-phase-untouched (the fail-closed
   alternative is NOT taken — a replay is a legitimate idempotent re-run;
   refusing it would break `spec approve`'s existing idempotence contract at
   `spec.go:73-74`).
3. `.mindspec/adr/ADR-0025-jsonl-as-build-artifact.md`: the mandatory
   clarifying amendment (decision item 1 scoped to entangleable dirt).
4. AC-8 and AC-16 fixtures (all AC-16 legs: implement/review/done variants,
   same-value plan replay, empty-phase backfill).

**Verification**
- [ ] `go build ./... && go test ./... -short` green

**Acceptance Criteria**
- AC-8 (i)(ii)(iii), AC-16

**Depends on**
Bead 1 (the lock the backfill uses).

## Bead 8: Deterministic selection — pick/emit parity, filter loudness, scope honesty (OQ-5)

**Steps**
1. `internal/next/select.go`: total pick semantics (single-row validation —
   the `select.go:20-22` early return goes; 0=unset; negative rejected);
   ONE selection function consumed by both paths. The shared ready-set
   CONSTRUCTION (query + spec filter + exclusion accounting, R6.1's "one
   set-construction") lands beside `QueryReady` in `internal/next/beads.go`
   (round-1 G4.3 — this is chunk 8's `beads.go` entry).
2. `cmd/mindspec/next.go`: `runEmitOnly` receives `--pick` and builds the
   SAME ordered set via the same construction as the claim path; the
   positional emit-only bypass keeps `FetchBeadByID` + prints the pinned
   bypass line; the OQ-5 default line ("Defaulting to item 1 (bd ready
   priority order)…"); filter loudness R6.3(a)/(b)/(c) — exclusion count +
   reason on every set-constructing path, loud error on empty-after-filter,
   the honest positional exclusion error.
3. `internal/resolve/target.go` + `cmd/mindspec/next.go`: R6.7 scope honesty —
   explicit `--spec` resolution failure (prefix no-match / epic lookup error /
   draft-no-epic) refuses on both paths; the silent global-`bd ready`
   fallbacks (`next.go:166-171`, `:417-424`) are removed; `ResolveSpecPrefix`
   full-ID passthrough gains the existence proof.
4. AC-9/AC-10/AC-10b/AC-15 fixtures (including the seam-recorded
   no-global-query assertion).

**Verification**
- [ ] `go build ./... && go test ./... -short` green

**Acceptance Criteria**
- AC-9, AC-10, AC-10b, AC-15

**Depends on**
Bead 2 (same-file serialization on `cmd/mindspec/next.go` after its
claim-advance write lands).

## Bead 9: bd stderr passthrough with safe rendering

**Steps**
1. `internal/bead/bdcli.go`: on `cmd.Output()` failure, surface
   `exec.ExitError.Stderr` content through the returned error (a bounded,
   line-capped excerpt — cap pinned here).
2. `internal/next/beads.go`: the `QueryReady`/`QueryReadyForEpic` wrapping
   errors carry the surfaced stderr; every surfaced line routes through
   `termsafe.Escape` (ADR-0042).
3. AC-11's two legs: content (schema-error line present) + rendering
   (multi-line, control-bearing stderr — escaped, no raw control bytes).

**Verification**
- [ ] `go build ./... && go test ./... -short` green

**Acceptance Criteria**
- AC-11

**Depends on**
None.

## Bead 10: Frozen-contract guidance, docs sweep, and the method gate — last by construction

**Steps**
1. `internal/instruct/templates/idle.md` + `review.md`: the R7 guidance —
   new implementation work (including implementing a frozen contract) goes
   through a new spec; the externally-merged predecessor's exit is named by
   its real invocation `mindspec impl adopt <spec-id> --reason "<why>"`;
   template test pins the command name against the registered cobra command
   (AC-12's rename-is-red clause).
2. `project-docs/user/guides/merge-safety.md`: R4.3's operator-visible
   updates for the bead-6 reorder, scoped to the GATE-REFUSAL class (a
   gate-refused complete leaves the tip at the reviewed SHA; plain re-run +
   the original gate's override is the whole recovery; POST-gate failures —
   merge conflicts, close failures — keep the 127 preserved-merge/forward-
   reconcile guidance unchanged, and the doc says so); `lifecycle-modes.md`
   cross-links.
3. Doc-sync gate coverage over every touched source; final whole-tree
   `go test ./... -short` as the AC-13 method gate.

**Verification**
- [ ] `go build ./... && go test ./... -short` green (the AC-13 assertion)
- [ ] `mindspec validate spec 128-lifecycle-verb-state-honesty` green

**Acceptance Criteria**
- AC-12, AC-13

**Depends on**
Beads 4 and 5 (the docs describe their shipped behavior).

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC-1 (i)(ii)(iii) | Bead 3 |
| AC-2 | Bead 4 |
| AC-3 | Bead 5 |
| AC-4 | Bead 5 |
| AC-4b | Bead 5 |
| AC-5 (ii)(iii)(v)(vi)(vii) | Bead 1 (lock/guard/inventory machinery) |
| AC-5 (i)(iv)(viii) | Bead 2 (OQ-3 value semantics; AC-5 complete at bead 2) |
| AC-6 | Bead 6 |
| AC-7 (i) ((ii)–(iv) vacated: OQ-1 resolved to (a), no exemption exists) | Bead 6 |
| AC-8 (i)(ii)(iii) | Bead 7 |
| AC-9 | Bead 8 |
| AC-10, AC-10b | Bead 8 |
| AC-11 | Bead 9 |
| AC-12 | Bead 10 |
| AC-13 | Bead 10 (final gate; every bead's verification runs it) |
| AC-14 | Bead 5 |
| AC-15 | Bead 8 |
| AC-16 | Bead 7 |
