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
      - .gitignore
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
      - internal/approve/impl.go
      - cmd/mindspec/repair.go
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
   excludePathspecs) (treeOID, error)`): in the bead worktree, the temp index
   is **SEEDED FROM THE BEAD TIP FIRST** — `GIT_INDEX_FILE=<tmp> git -C
   wtPath read-tree <tip>` — then `add -A -- . ':!<named-artifact>'` stages
   the full prospective content (tracked + untracked + deletions) over that
   seed, then `git write-tree` → tree OID `T`. The seeding is load-bearing
   (round-2 G1, empirically reproduced this round): an EMPTY temp index plus
   the exclusion makes the tracked artifact ABSENT from `T`, and the commit
   built from `T` silently DELETES `.beads/issues.jsonl` from the branch;
   seeded, the exclusion means unchanged-at-tip — the artifact's tip
   blob/mode is retained in `T`, never dropped. The worktree, real index,
   and refs are untouched by the snapshot (a temp index + loose objects
   only).
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
   user materialization commit is created FROM `T` itself, as a three-part op
   with the real-index transition specified (round-2 G2, mechanics reproduced
   this round): (i) `C = git commit-tree T -p <tip>` (plumbing — no hooks);
   (ii) `git update-ref refs/heads/bead/<id> C <expected-old-tip>` — the
   expected-old-value operand makes a concurrent tip move FAIL the update
   (ADR-0035 refusal, nothing advanced beyond loose objects, re-run
   re-measures); (iii) `git reset --quiet --mixed` in `wtPath` — index-only
   (worktree untouched), refreshing the checked-out branch's REAL index to
   the new tip; without it, files newly tracked via `T` read as phantom
   staged deletions and pollute every status consumer (the residual-dirt
   WARN, `complete.go:928-933`, and fixture (iv)'s assertion) — reproduced:
   pre-reset status shows `D`+`??` for the same path, post-reset clean. A
   reset failure is non-rolled-back (forward-only), and the promised plain
   re-run ACTUALLY CONVERGES via the **WRITE-AHEAD materialization marker
   (round-3 G1; round-4's two-ref-transaction form REPLACED at round-5 G1 —
   replacement over patching, per house lesson)**. Why replaced:
   `update-ref --stdin` transactions give locked all-or-nothing DECISION
   semantics, but installation is per-ref — git's own documentation states
   "while each individual <ref> is updated or deleted atomically, a
   concurrent reader may still see a subset of the modifications"
   (git-update-ref(1), verified against git 2.51.2 this round) — so a crash
   mid-commit on the default files backend can leave ONE ref installed;
   evidence grade: authoritative documentation + the documented per-ref
   installation order, NOT a lab-reproduced crash window (fault-injecting
   git's commit phase is out of reach here, and the doc makes the
   reproduction unnecessary). The write-ahead protocol needs no multi-ref
   atomicity at all — SIMPLER, and its git-version floor drops to plain
   `update-ref`/`commit-tree`/`read-tree` (no floor beyond ancient git; the
   fixtures run explicitly on the default files backend). Per stage: build
   the commit OBJECT first, then durably publish the marker ref
   `refs/mindspec/materialized/<bead-id>` → the intended NEW tip, then
   advance the bead ref with expected-old — and because each intended tip is
   built with `-p <predecessor>`, the marker's single OID encodes BOTH the
   intended tip AND (as its first parent) the expected predecessor. (M):
   build `C` (from `T`, `-p A`) → marker → `C` → bead-ref `A→C`
   expected-old `A`. (R): index refresh — no ref change, idempotent.
   (S): build `D` by plumbing (C's tree + refreshed artifact blob,
   `-p C`) → marker → `D` → bead-ref `C→D` expected-old `C`, then the
   S-stage exit contract (round-5 G2): refresh the real index to `D` (the
   same index-only mixed reset), then classify remaining worktree dirt with
   the existing artifact/user classifier — GENUINE non-artifact dirt
   (interval edits) at this point REFUSES with a named-paths ADR-0035
   refusal BEFORE any merge path that can force-remove the bead worktree:
   an interval edit is preserved on disk behind a named refusal or an
   explicitly safe recovery, never left uncommitted through a terminal
   cleanup that removes its worktree. (G): merge — the bead ref no longer
   moves; a preserved conflict resumes INTO the 127 `--resolve-merge`
   machinery. (T): terminal marker clear on BOTH the
   ordinary and landed-merge-reconcile completion paths. **Re-entry
   discriminator (round-5 G3.2; made TOTAL under empty diffs at round-6
   G1)**: stage kind is encoded DURABLY IN THE COMMIT OBJECT itself — every
   `C` and `D` carries a pinned commit-message trailer (`Mindspec-Stage:
   materialization` / `Mindspec-Stage: artifact-sync`; exact key pinned in
   bead 6, read via the existing `CommitMessageBody` primitive) — never
   inferred from the tree diff, which is ambiguous when `C` is a clean-tree
   completion or `D` is a no-op sync. Empty stages are ELIDED (pinned
   decision, matching HEAD's no-op-commit behavior: a `T` equal to the
   tip's tree builds no `C`, an unchanged artifact builds no `D` — retaining
   empty trailer commits would stamp a contentless commit onto EVERY
   standard bead completion, since a subagent-committed clean tree is the
   norm), and the execution contract is therefore honestly **AT-LEAST-ONCE
   with per-operation idempotent replay (round-7 G1, contract (b) — the
   round-6 "exactly once" claim was unprovable after a post-no-op-S kill and
   is withdrawn)**. The per-operation proof obligations, stated: R's index
   refresh (`reset --quiet --mixed`) is a state-function of (tip, worktree)
   — replay converges to the identical index and touches no worktree or
   ref; S's export is ADR-0025's deterministic projection of Dolt state —
   replay converges to identical bytes for unchanged Dolt state and to the
   CURRENT projection otherwise, which is forward-correct for a cache; S's
   dirt classification is a pure read + refusal; the merge stage carries the
   127 preserved-merge/ancestry convergence machinery. "Not skipped" is
   guaranteed by routing (every path to the merge passes through R and the
   S-exit contract); "not looped" is bounded by convergence (each replay is
   a state-function of current state). **The elided-case transition graph is
   complete (round-7 G1)**: the FIRST marker publication of a run uses
   CREATE semantics whatever it points at — absent→`C` normally, absent→`D`
   when `C` elides (`D` then parents on `A`), with the same
   failed-publication refusal; when BOTH stages elide (clean tree AND
   unchanged artifact) there is NO marker and NO ref movement — the run
   proceeds from the gates through the S-exit contract to the merge, and a
   kill anywhere in that window re-enters as an ordinary run (tip still ==
   reviewed SHA, panel gate passes, gates re-measure) — the no-marker route. With marker `M*`
   present: tip == `M*` → the published stage COMPLETED — its kind is read
   from the trailer (`C` → continue at R then S; `D` → continue at the ONE
   authoritative S-resume route: index refresh to `D` + the genuine-dirt
   classifier, THEN merge — round-6 G3: no continue-at-the-merge shortcut
   exists on any re-entry), with preserved-merge/landed states identifying
   G/T; tip == `M*^` → the write-ahead is UNFINISHED — finish the bead-ref
   advance (expected-old = tip) and continue; tip ∉ {`M*`, `M*^`} → FOREIGN:
   delete the marker, proceed as a normal run, the stale-SHA gate evaluating
   exactly as today — the cannot-launder boundary is retained. **Marker
   publication is itself race-safe (round-6 G2)**: absent→`C` publishes via
   `update-ref` CREATE semantics (atomically fails if the ref exists),
   `C→D` via expected-old — per-ref CAS, deliberately NOT bead 1's phase
   lock (the named instrument choice: the materialization machinery stays
   uncoupled from the phase-metadata lock, so no lock-ordering edge exists
   between the two subsystems). A failed marker publication (unexpected
   existing value: another invocation in flight, or a leftover) REFUSES
   with an ADR-0035 line naming the marker — one invocation can never
   overwrite another's recovery evidence; a failed bead-ref advance leaves
   the already-published marker in place as write-ahead evidence for the
   next run's discriminator. Every run's
   early sequence, before the panel gate: (1) landed-merge reconciliation
   detection FIRST (spec-119 detection; if landed → reconcile path, clearing
   any orphaned marker — round-4 G1.3); (2) the discriminator above; resume
   skips re-snapshot, content gates, and the panel gate (the marker is
   published only after every gate passed). The full M→R→S→G→T boundary
   joins the ADR-0041 §3 mutation-point classification via the amendment.
   Consequences, each closing one
   G1.2 hole: a pre-commit hook cannot mutate the commit (plumbing runs no
   hooks); a worktree edit in the gate→commit interval cannot enter the
   commit (built from `T`; the edit stays ORDINARY UNSTAGED dirt after the
   mixed reset, never an index/HEAD split); the bd export cannot enter it
   (unchanged-at-tip in `T`; the export lands in the separate, pathspec-
   scoped artifact-sync commit the flow already has, which no gate measures —
   the artifact-only normalization treatment; `CommitPaths` is already
   pathspec-limited, `gitops.go:1699`). "Committed tree ≡ measured tree" is
   thereby mechanical, not asserted: they are the same object.
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
`--force-backward` + reason (AC-5(viii) distinguishes them). **Taxonomy
note (round-3 G5)**: `EnsurePhaseSemanticsV2`'s one-shot legacy rewrite
(`implement → plan` under the durable v2 marker) sits OUTSIDE the sanction
taxonomy — it is a semantics TRANSLATION of an unchanged underlying fact
from pre-v2 to v2 representation, performed once per epic and never again
(the marker), not a lifecycle backward move; the "exactly one sanctioned
backward writer" count refers to ongoing v2-semantics writers and is
unchanged by it.

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

**R3.3 lock home**: `internal/phase/phaselock.go` — new file, kernel-held on
both platforms (`flock(2)` unix / `LockFileEx` on a retained
no-`FILE_SHARE_DELETE` handle on Windows — round-4 G2: the journal package
contributes only its `withReportsLock` read-modify-write SHAPE,
`internal/journal/lock.go:55-61`, not its O_EXCL-lockfile mechanism), lock
file under `.mindspec/` (gitignored-adjacent runtime state, exact name
pinned in bead 1). `internal/
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

**Chunk/section reconciliation sweep (round-3 G4.1 — performed in full, not
patched)**: every chunk's `key_file_paths` was reconciled against its bead
section's named edit sites at round 3. Findings beyond the panel's ask:
chunk 1 was missing `.gitignore` (bead 1's lock-path entry) — added; chunk 2
was missing `cmd/mindspec/repair.go` (bead 2 step 7's reconciliation call
site) — added, per the panel; all other chunks matched their sections. This
class recurred three times because file additions were made ask-by-ask; the
full sweep is now the recorded baseline.

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
  §2 (forward-reconcile, never rollback) is unchanged — (a) introduces no
  rollback; §3 IS amended (round-5 G3.1, aligning with bead 6's actual
  disposition): it gains the M→R→S→G→T staged boundary classification with
  the write-ahead marker's forward-resume semantics.
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
   `impl adopt` house pattern) whose bead-1 form asserts ZERO production
   callers outside the allowlist `{cmd/mindspec/release.go}` — satisfiable at
   bead 1's own gate, before release is wired (round-2 G3.1); bead 2 TIGHTENS
   the same test to exactly-one when it wires the caller.
2. `internal/phase/phaselock.go` (new): kernel-held file lock — `flock(2)`
   on unix, `LockFileEx` on Windows (round-4 G2: the journal package's
   O_EXCL-lockfile idiom is NOT carried to this lock; only its
   `withReportsLock` read-modify-write shape is) — exposing
   `WithPhaseWriteLock(root, epicID, fn)` — reads stored
   phase inside the critical section, passes it to `fn`, performs the
   conditional `MergeMetadata` write `fn` returns, releases. **Operational
   design (round-1 G3), pinned here:** lock path = `.mindspec/phase.lock`;
   the managed `.gitignore` block gains that entry (beside the existing
   `.mindspec/session.json` line) and a test asserts lock acquisition leaves
   zero blockable git dirt; acquisition uses a BOUNDED wait (poll-with-
   deadline, default 10s, constant pinned in the file) — on timeout, an
   ADR-0035 refusal naming the lock path and a re-run recovery line (never a
   delete-the-lock instruction); unix crashed holders self-release (flock is
   kernel-held); the Windows path uses a **mechanically atomic primitive
   (round-3 G3 — the round-2 compare-and-delete + post-create-verify dance
   is REPLACED, not repaired)**: `LockFileEx` (exclusive byte-range lock) on
   a lock file opened with a RETAINED handle WITHOUT `FILE_SHARE_DELETE`,
   held for the whole critical section — atomicity and ownership are the
   OS's, release is owner-only (`UnlockFileEx` + `CloseHandle`), and a
   crashed holder's lock releases with its process, mirroring flock
   semantics. This moots the stale-file problem class entirely: the FILE is
   not the lock (the byte-range lock is), so an empty or partial leftover
   file never blocks — the round-2 crash-between-create-and-publish case is
   dispositioned as unreachable (no identity publication exists to crash
   before). No PID/timestamp content, no takeover, no deletion step. Tests:
   real-goroutine contention (`-race`), timeout error propagation,
   crashed-holder recovery (kill the holder; next acquire succeeds — both
   platforms' semantics, the Windows leg seam-driven on non-Windows CI), the
   barrier-controlled two-contender exclusivity test (exactly one inside the
   critical section, the loser blocked until release — subsuming the
   late-delete interleave the deleted design needed), a leftover-file leg
   (pre-existing empty/partial lock file: acquisition simply succeeds); plus
   the no-git-dirt assertion.
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
   `phase.SanctionedDerivedSync` (created + zero-caller-fenced in bead 1),
   which re-derives the child-derived phase inside the critical section and
   writes it, backward permitted; the operator guard in `repair.go` is
   unaffected. **This step also TIGHTENS bead 1's enumeration fence from
   zero-callers-with-allowlist to exactly-one (round-3 G4.2 — the tightening
   duty lives in the bead that executes it).**
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
   **READ paths are covered without casual lock-taking (round-2 G4)**: the
   legacy-signature predicate is exported (`phase.LegacyPhaseSignature(stored
   string, children []ChildInfo) bool`, same file) and consumed lock-free by
   the read-side consistency branch (`derive.go:179-193`) and `state show`:
   when the marker is absent and the signature holds, the read path emits the
   LEGACY NOTICE in place of the stored-vs-derived mismatch warning —
   "legacy phase semantics detected (pre-v2 stored implement); recovery:
   mindspec repair phase <spec-id>" — and that recovery command is made
   correct by adding `repair phase` (and `release`) to
   `EnsurePhaseSemanticsV2`'s invocation sites: repair reconciles
   (legacy → `plan` + marker) BEFORE its guard evaluates, then no-ops as
   already-consistent — the hint's execution improves state (R3.2-compliant),
   with no read path ever taking the write lock.
   **Six fixtures**: upgrade-before-first-claim (legacy → `plan` + marker);
   upgrade-then-first-claim (claim advances to `implement`); non-lifecycle
   claim (no advance, marker written); release (returns to `plan`, marker
   present, no re-reconcile); already-progressed epics (each of
   some-closed / in_progress / review / done — phase untouched, marker
   written); and the READ-path fixture — on a legacy epic, `state show`
   emits the legacy notice (never the obsolete mismatch warning), then
   `repair phase` reconciles, then `state show` is clean.
   **Fixture-state pinning (round-3 G2)**: because `EnsurePhaseSemanticsV2`
   at repair's entry CONSUMES the marker-absent legacy condition before the
   guard runs, every guard/warning fixture — AC-5(ii), (iv), (v), and
   (viii) — is constructed with the v2 marker PRESENT (their backward states
   built by post-marker means: seam writes or the sanctioned sync), so the
   reconciliation is a no-op and the condition under test survives to the
   guard; the marker-ABSENT stored-implement/all-open state is exclusively
   the LEGACY fixture above, exercising notice + self-heal. Classification:
   the legacy notice is a REPLACEMENT notice, not a retained form of the
   phase-mismatch warning — AC-5(iv)'s test targets the retained warning
   only.

**Verification**
- [ ] `go build ./... && go test ./... -short` green
- [ ] Both fixture routes assert zero warning emissions on stderr

**Acceptance Criteria**
- AC-5(i) (both routes, warning-absent arm), AC-5(iv) (the RETAINED general
  mismatch warning, where still reachable outside the dissolved windows,
  carries the direction-conditional recovery per the recorded disposition —
  forward keeps `repair phase`, backward emits the `bd show` inspection line;
  the legacy notice is a replacement, outside this AC's target — round-3
  G7/G2), AC-5(viii)

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
   `SnapshotWorktreeTree(wtPath, excludePathspecs)` (temp index **seeded via
   `read-tree <tip>` FIRST** — round-2 G1's synthetic-deletion fix — then
   `add -A` with the named-artifact exclusion, then `write-tree`;
   worktree/real-index/refs untouched, asserted by test; the exclusion means
   unchanged-at-tip, never absent-from-`T`) and the staged materialization
   machinery (round-4 G1, write-ahead form round-5 G1): `commit-tree`
   builders for `C` (from `T`) and `D` (C's tree + refreshed artifact blob),
   the write-ahead marker publish helpers with per-ref CAS (CREATE
   semantics for absent→`C`, expected-old for `C→D` — round-6 G2) and the
   expected-old single-ref advance (plain `update-ref` — following the
   spec-127 `CommitTreeMerge`/`UpdateRef`/`recordMergeSourceMarker`
   precedents; no multi-ref transaction dependency, no git-version floor
   beyond plain plumbing), the `Mindspec-Stage` trailer builders + reader
   (round-6 G1; exact trailer key pinned in this bead, read via
   `CommitMessageBody`), the re-entry discriminator over {tip == marker
   (kind from trailer), tip == marker-parent, foreign}, the index-only
   `reset --quiet --mixed`, and the failure legs of protocol step 3
   (expected-old race → refusal + re-measure; failed marker publication →
   refusal naming the marker, no overwrite; reset failure → forward-only
   refusal, the resume machinery converging the re-run).
2. `internal/executor`: **ADR-0030 seam conformance (round-7 G2)** — every
   new git read/mutation the state machine performs is a NAMED method on the
   `Executor` interface, implemented by `MindspecExecutor` as a thin wrap
   over the bead-6 gitutil primitives and stubbed in `MockExecutor`:
   `SnapshotWorktreeTree(wtPath, exclude)`,
   `MaterializeStageCommit(wtPath, tree, parent, stage)` (builds `C`/`D`
   with the pinned trailer), `PublishStageMarker(beadID, oid, expectedOld)`
   (CAS: empty expectedOld = CREATE), `ReadStageMarker(beadID)`,
   `ClearStageMarker(beadID)`, `AdvanceBranchExpectedOld(branch, new, old)`,
   and `ResetIndexToHead(wtPath)` — `internal/complete` calls ONLY these
   seams and gains no new `internal/gitutil` import (no unplanned boundary
   crossing). The interface + `MindspecExecutor` + mock also gain
   `ChangedFilesTree(workdir, base, treeOID)` and tree-grounded input reads;
   the gate-input surfaces (OWNERSHIP manifests, domain enumeration, doc/ADR
   contents) read from `T:path`, never from `g.Root`'s checkout (closes the
   `ChangedFiles` `g.Root`-rooting and the cwd-rooted `DiffNameOnlyRef("",…)`
   fallback holes, `mindspec_executor.go:1210/:1221`).
3. `internal/validate`: `ValidateDocsRange`/`CheckADRDivergence` snapshot legs
   over `base → T` (BOTH gates, same protocol); committed-range behavior
   preserved for reconcile mode and the impl-approve whole-branch backstop.
4. `internal/complete/complete.go`: the EARLY sequence before the panel gate
   in protocol-step-3 order — landed-merge reconciliation detection FIRST
   (clearing any orphaned marker), then marker/tip evaluation (resume vs
   foreign-delete); snapshot taken once after the panel gate; both gates
   measure it; on pass, the staged state machine runs in write-ahead order
   (round-4 G1.2's commit-tree-first, round-5 G1's marker-before-ref): build
   `C` from `T` → marker → `C` → bead-ref `A→C` expected-old → index-only
   `reset --quiet --mixed` → build `D` by plumbing → marker → `D` →
   bead-ref `C→D` expected-old → S-stage exit contract (index to `D`; dirt
   classification; genuine interval-edit dirt refuses BEFORE the merge —
   round-5 G2) → merge (resume routes into `--resolve-merge` on a preserved
   conflict) → terminal marker clear on both completion paths. This replaces
   BOTH the `CommitAll` porcelain path and the porcelain artifact-sync
   commit for this flow; reconcile path untouched. Marker + advance helpers
   live in `internal/gitutil` beside the 127 `recordMergeSourceMarker`
   precedent.
5. `.mindspec/adr/ADR-0041-gate-before-mutate.md`: the §1 amendment REPLACES
   BOTH load-bearing statements (round-1 G1.4), describing the CORRECTED
   protocol (round-2 G2): (i) panel-before-`CommitAll` is RETAINED with its
   reviewed-SHA rationale; (ii) gates-after-materialization is REPLACED by
   prospective-snapshot-before-any-complete-authored-commit — the tip-seeded
   snapshot (exclusion = unchanged-at-tip), the artifact-only normalization
   treatment (the artifact is unchanged in `T` and measured by no gate; only
   the sync commit updates it), why the gates cannot vacuously miss content
   (untracked files are staged into `T`; the commit IS `T`), and the
   materialization op's index-refresh + expected-old-tip semantics, PLUS
   (round-3 G1.2, extended round-4 G1.5) the §3 mutation-point
   classification gains the FULL M→R→S→G→T staged boundary with the
   WRITE-AHEAD marker invariant (marker published race-safely — CREATE /
   expected-old CAS — before every tool-authored bead-ref advance, its OID
   encoding intended tip + expected predecessor as the commit's first
   parent, stage kind encoded in the pinned `Mindspec-Stage` commit trailer
   — round-6 G1: read from the encoding, never the diff, total under empty
   stages, which are elided under the round-7 AT-LEAST-ONCE idempotent-
   replay contract (per-operation proofs stated; first-publication CREATE
   covers absent→`C` and absent→`D`; the both-elided no-marker route
   re-enters as an ordinary run); the total re-entry discriminator over
   {tip == marker: stage complete, kind from the trailer; tip ==
   marker-parent: finish the advance; foreign: delete + stale-SHA as at
   HEAD}; every `D`-resume passing through the single authoritative S-exit
   route — index to `D`, dirt classifier, then merge; reconcile-detection
   before any stale-marker decision; terminal clear on both completion
   paths), and
   (round-3 G6, confirmed correct at round 4) the refusal
   guarantee is stated precisely rather than as repository-byte-identical: a
   refused content gate leaves the bead ref/tip, tracker state, real index,
   and worktree unchanged, while UNREACHABLE LOOSE OBJECTS from the
   temp-index staging may exist (gc-collectable — the same accepted class as
   the existing non-mutating merge preview, `neteffect.go`'s
   "unreferenced loose objects only" precedent). No
   measured-equals-committed prose claim survives — the equality is the
   commit-tree construction. Extend `cmd/mindspec/adr0041_amendment_test.go`.
6. AC-6 + AC-7(i) fixtures: panel-pass at SHA A + dirty tracker +
   doc-sync-failing change → run 1 fails doc-sync with tip STILL at A; run 2
   with `--allow-doc-skew` is decided by doc-sync, never stale-SHA; and
   red-on-revert, a post-review non-tracker commit still trips stale-SHA
   (gate untouched).
7. Failure-injection + red-on-revert fixture legs (round-1 G1.5, extended
   rounds 2–7; all legs run on the DEFAULT FILES REF BACKEND explicitly —
   round-5 G1), all twenty: (i) an UNTRACKED source/doc/ADR file in the
   worktree — the gate measures it and refuses before any commit; (ii) a
   pre-commit hook that adds/changes a source file — the committed tree is
   still ≡ `T` (plumbing runs no hooks); (iii) the tracker export changing
   content post-measurement — the artifact is unchanged-at-tip in `T` and
   the export lands only in the sync commit; (iv) a worktree mutation in the
   gate→commit interval — absent from the commit, and after the mixed reset
   it is ORDINARY UNSTAGED dirt, never an index/HEAD split; (v) the
   never-touches-artifact leg (round-2 G1): the user commit's diff vs tip
   NEVER touches the named artifact — red against the empty-index synthetic
   deletion; (vi) an unchanged-after-snapshot worktree is status-CLEAN after
   materialization — red against the phantom-staged-deletion state; (vii)
   the excluded artifact is byte-identical at tip, in `T`, and in the user
   commit, changing only in the sync commit; (viii) the resume KILL fixture, split per round-6 G3
   into two runs: (viii-a) advance the ref to `C` with the marker written,
   interrupt before the reset, add an interval edit, invoke the documented
   recovery (plain re-run) — it REFUSES pre-merge at the S-exit contract
   naming the edited path, with NO rollback, NO re-panel, no phantom staged
   deletions; the file's content survives and no terminal cleanup removes
   the worktree while it is uncommitted; (viii-b) after the operator clears
   the named dirt, a clean follow-up run CONVERGES to the landed merge; a
   stale-marker variant (foreign tip) is cleaned and falls through to a
   normal run; (ix) kill AFTER the
   S-stage ref advance (tip `D`, marker `D` — round-4 G1.4): plain re-run
   resumes at the S-exit contract then the merge and converges, no
   re-panel, and an interval edit present at that point triggers the same
   named refusal, never silent loss; (x) kill DURING a preserved merge
   conflict (marker == tip == `D`): re-run / `--resolve-merge` converges
   through the 127 machinery without re-paneling or laundering; (xi) a
   genuinely foreign commit landed after materialization (tip ∉ {marker,
   marker-parent}): marker deleted, panel stale-SHA blocks exactly as at
   HEAD; (xii) kill BETWEEN marker→`C` and the `A→C` ref advance (marker
   `C`, tip `A` == marker-parent — round-5 G1's one-ref-installed
   ordering): plain re-run finishes the advance and converges, no re-panel;
   (xiii) the same window at the S stage (marker `D`, tip `C` ==
   marker-parent): plain re-run finishes `C→D` and converges; (xiv)
   concurrent-publisher leg (round-6 G2): a second invocation's marker
   publication fails on the CAS (create-exists / expected-old mismatch) with
   the refusal naming the marker, and the first invocation's marker OID is
   byte-identical before and after — recovery evidence never overwritten;
   (xv) empty-`C` elision (clean-tree completion — round-6 G1): no `C` is
   built, `D` (if any) parents directly on `A`, and re-entry after an
   interruption classifies every present commit correctly from its trailer,
   R and S replaying idempotently per the round-7 contract; (xvi)
   no-op-`D` elision (unchanged
   artifact): no `D` is built, and a kill after the `A→C` advance resumes
   through R, S (which elides), the S-exit contract, and the merge; (xvii)
   ordinary artifact-changing `D`: trailer classification identifies `D`
   after interruption and the resume passes through the S-exit contract,
   never straight to the merge; (xviii) kill after R with `C` elided,
   before any `D` publication (round-7 G1): nothing durable happened — tip
   still at the reviewed SHA, no marker — re-entry is an ordinary run that
   re-measures and converges; (xix) kill after a no-op `S` (`D` elided),
   pre-merge, marker `C` present: re-entry replays R and S idempotently
   (identical index; identical artifact bytes for unchanged Dolt state),
   passes the S-exit contract, and converges — no loop, no skip (the
   per-operation replay proofs asserted here); (xx) both stages elided,
   kill pre-merge: the no-marker route — an ordinary re-run converges to
   the landed merge.

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
