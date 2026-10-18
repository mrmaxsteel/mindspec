# Bead 6 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.6
- Branch: bead/mindspec-ta30.6
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.6
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §"OQ-1 — #186
  mechanism" (the FULL protocol, reproduced verbatim below) and §Bead 6
  (reproduced verbatim below). This bead's machinery survived an 8-round
  plan gate; your job is FAITHFUL TRANSMISSION of the designed protocol,
  not invention. Where this prompt and plan.md disagree, plan.md wins —
  but the sections below were extracted mechanically from plan.md, so they
  should not disagree.
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R4, AC-6,
  AC-7 (quoted below with live-leg tags), settled facts 3/4.
- `.mindspec/adr/ADR-0041-gate-before-mutate.md` — the §1 text this bead
  amends as a designed REVERSAL, and §3 which gains the staged-boundary
  classification.
- The MERGED bead-1..5 code in THIS worktree — landed API is the authority
  (see "Landed-surface interactions" below).

## THE PROTOCOL — plan §OQ-1, verbatim and complete
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


## THE BEAD — plan §Bead 6, verbatim and complete
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


## Spec ACs this bead claims (verbatim, with live-leg tags)

- [ ] **AC-6 (R4)**: Fixture: bead with passing panel at SHA `A`, dirty tracker file, doc-sync-failing change. Run 1 fails at doc-sync; run 2 with `--allow-doc-skew "<r>"` **succeeds** (or fails at doc-sync again if the skew persists) — it does not fail at the panel stale-SHA gate — this retry-convergence assertion holds under every OQ-1 arm. The diff-pathspec assertion (that the delta the gate ignored was confined to the named artifact set, today `.beads/issues.jsonl`) applies **under OQ-1(c) only** (round-2 G7) — under (a)/(b) no delta is ignored, because the branch is never left advanced past the reviewed SHA in the first place.
- [ ] **AC-7 (R4)**: Negative cases matching the paths-only pin (round-1 G3.3; OQ-1-arm scoping per round-2 G7): (i) a commit touching a non-tracker path landing after review still trips the stale-SHA gate unchanged — red-on-revert against gate.go's block, and this leg holds under every OQ-1 arm; legs (ii)–(iv) exist **under OQ-1(c) only** (under (a)/(b) there is no exemption to bound): (ii) a MIXED commit touching the named artifact file AND any non-artifact path still trips it (the exemption applies only to deltas *confined* to the named set); (iii) a delta confined to the named artifact file is exempt regardless of who authored it — the test asserts this stated residual so the pin's shape cannot silently drift toward an unprovable provenance claim; (iv) a `.beads/` path *outside* the named list (e.g. `.beads/config.yaml`) is NOT exempt.

**Which legs are LIVE under the delivered mechanism**: OQ-1 was resolved
to (a) — the reorder — so AC-6's retry-convergence assertion is live and
its diff-pathspec sentence is VACATED ("under (a)/(b) no delta is
ignored"); AC-7(i) is live (red-on-revert: a post-review non-tracker
commit still trips the stale-SHA gate unchanged); AC-7(ii)-(iv) are
VACATED as OQ-1(c)-only. Do not implement vacated legs; do not delete
their spec text.

## Landed-surface interactions (verified constraints from beads 1-5)
- Bead 5's `completeFetchChildrenFn` seam + `Result.NextModeIndeterminate`
  + the step-6.5 skip-on-indeterminate are LANDED in internal/complete:
  the S stage's export/sync must respect the indeterminate skip — an
  indeterminate advanceState never feeds a phase write, and your staging
  changes must keep bead 5's AC-4/AC-4b tests green unmodified.
- Bead 1's phase lock is NOT the marker CAS instrument — plan round-6 G2's
  DELIBERATE decoupling: marker race-safety is per-ref CAS (CREATE for the
  first publication, expected-old for C→D); no lock-ordering edge between
  the phase-metadata lock and the materialization machinery may be
  introduced.
- Bead 1's step-6.5 write is lock-wrapped (`WithPhaseWriteLock`); the
  reorder moves gates, not that wrap.
- ADR-0030: `internal/complete` gains NO `internal/gitutil` import — every
  new git read/mutation routes through the seven named Executor seam
  methods (the §Bead 6 step-2 list above); `MindspecExecutor` thin-wraps
  the gitutil primitives; `MockExecutor` stubbed.
- The reconcile path (landed-merge detection) and the 127
  preserved-merge/`--resolve-merge` machinery are consumed, not modified;
  the G-stage resume routes INTO them.

## Hazards (each hard-blocked an earlier bead — do not repeat)
- Error-discarding: every read propagates; no fabricated modes/values;
  a failed probe is a refusal or an error, never a default.
- Hermeticity: fixture repo roots AND lock/marker paths in `t.TempDir()`;
  the suite must be green run FROM the bead worktree; no dependence on
  this repo's own lifecycle state; all twenty fixture legs run on the
  DEFAULT FILES ref backend explicitly.
- The `cmd/mindspec` suite green is part of the gate NOW: run BOTH
  `go test ./... -short` (full tree) and the bead-scope packages
  (`internal/complete`, `internal/gitutil`, `internal/executor`,
  `internal/validate`, `cmd/mindspec`) and report both result lines.
  `-short` is mandatory (spec settled fact 3: an LLM-gated harness test
  otherwise shells out to a real claude CLI and hangs).
- Long test runs: emit interim status (which fixture leg group is
  running) rather than going silent; if a leg needs >600s something is
  wrong — stop and diagnose, do not raise timeouts.
- Scratch cleanup: everything you create outside the worktree lives ONLY
  under
  /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  and is removed when you finish.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge` (your MERGE-stage changes go through the
existing executor merge producers — never a hand-run git merge in tests
either: fixtures drive the product code), no scope creep beyond the
§Bead 6 file list + the ADR-0041 amendment, no reimplementing landed
helpers, exactly ONE commit whose message ends with
`Deviations: <list or "none">`, and tests must PASS. Absolute paths
everywhere. Work exclusively inside the bead worktree listed under
Identity.

## Expected commit shape
```
impl(mindspec-ta30.6): prospective-snapshot gate reorder + ADR-0041 §1

- gitutil: tip-seeded SnapshotWorktreeTree, stage commit builders w/
  Mindspec-Stage trailer, write-ahead marker CAS, expected-old advance,
  index reset
- executor: seven seam methods (MindspecExecutor wraps, mock stubbed)
- validate: tree-measured ValidateDocsRange/CheckADRDivergence legs
- complete: M/R/S/G/T staged flow, early reconcile+discriminator,
  S-exit contract, at-least-once resume; no gitutil import
- ADR-0041: §1 replacement amendment + §3 boundary classification;
  amendment test extended
- all twenty fixture legs (i)-(xx)

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.6`.
- BOTH suite result lines: full `go test ./... -short` AND the bead-scope
  package list above, with passed/failed/skipped counts.
- The fixture-leg inventory: confirm each of (i)-(xx) exists by test name.
- Flagged deviations (one line each + reason) — explicitly including any
  interaction with bead 5's indeterminate machinery and any place the
  landed code forced a departure from the plan text above.
