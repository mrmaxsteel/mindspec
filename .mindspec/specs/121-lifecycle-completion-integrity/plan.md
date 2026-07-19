---
adr_citations:
    - ADR-0041
    - ADR-0042
    - ADR-0037
    - ADR-0035
approved_at: "2026-07-19T21:09:55Z"
approved_by: user
bead_ids:
    - mindspec-clhv.1
    - mindspec-clhv.2
    - mindspec-clhv.3
    - mindspec-clhv.4
spec_id: 121-lifecycle-completion-integrity
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/gitutil/neteffect.go
        - internal/gitutil/neteffect_test.go
        - internal/executor/mindspec_executor.go
        - internal/executor/finalize_orphan_test.go
        - internal/lifecycle/finalize_orphans.go
        - internal/lifecycle/finalize_orphans_test.go
        - internal/lifecycle/scan.go
        - internal/lifecycle/scan_test.go
        - internal/doctor/lifecycle_integrity.go
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/lifecycle/landed.go
        - internal/lifecycle/landed_test.go
        - internal/executor/mindspec_executor.go
        - internal/executor/executor_test.go
        - internal/executor/merge_binding_test.go
        - internal/complete/complete.go
        - internal/complete/complete_test.go
        - internal/complete/reconcile_test.go
        - internal/bead/bdcli.go
        - internal/gitutil/neteffect.go
    - depends_on:
        - 1
      id: 3
      key_file_paths:
        - cmd/mindspec/impl.go
        - cmd/mindspec/finalize_pr.go
        - cmd/mindspec/finalize_pr_test.go
        - cmd/mindspec/config.go
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/config/config.go
        - internal/config/config_test.go
        - internal/harness/scenario_finalize_pr.go
        - internal/harness/asserts.go
        - internal/harness/recorder.go
        - internal/lifecycle/scan.go
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
    - depends_on:
        - 1
      id: 4
      key_file_paths:
        - internal/complete/complete.go
        - internal/complete/complete_test.go
        - internal/lifecycle/orphans.go
        - internal/lifecycle/orphans_test.go
        - internal/gitutil/gitops.go
        - internal/gitutil/gitops_test.go
---
# Plan: 121-lifecycle-completion-integrity

Four beads implement the completion-path convergence doctrine. The
decomposition follows the spec's four clusters with one deliberate
re-cut against the spec-approve sketch: the ADR-0041 **§2 amendment
lands in Bead 1** (the net-effect predicate bead), not the R5 bead —
because beads land one at a time in ready order and Bead 1's predicate
is the FIRST code that cites §2 (its clause (iii), net-effect
already-merged re-derivation), so R8's "same bead as the code that
first cites it" rule points there. Beads 2 and 4 then cite the
already-landed §2 from their own consuming sites; Bead 3 authors §4.

Only genuinely produced-then-consumed STATE is an edge:

- **Bead 2 depends on Bead 1**: R5(d) revert/reapply-awareness is
  defined as "net effect of the first-parent chain since M — the same
  doctrine as Requirement 4", and this plan resolves the mechanism as
  ONE shared content-subsumption primitive (`gitutil.ContentSubsumed`,
  Bead 1 Step 1) that Bead 2's since-M evaluation consumes. Two
  divergent implementations of the same net-effect doctrine would be
  exactly the drift R9 exists to prevent.
- **Bead 3 depends on Bead 1**: AC-4 asserts that after the auto-merge
  + refresh, `lifecycle.ScanIntegrityFindings` reports NO finding of
  EITHER kind. The LOAD-BEARING half of the edge is Bead 1's
  refreshed-`origin/main` stale-tracker classifier (R2(c)) — the
  `finalize_branch` half clears via the EXISTING SHA-ancestry
  suppression once the automation's TRUE merge commit makes the
  carrier an ancestor of the refreshed `origin/main`, but the
  `stale_tracker` half cannot clear against today's local-`main`
  classifier (Bead 3's fixture keeps local `main` lagging by
  design). The net-effect SUPPRESSION is load-bearing for AC-9/AC-19
  (Bead 1's own ACs), not for AC-4. Bead 3's end-to-end test
  consumes the produced classifier behavior; it cannot pass against
  the local-`main` baseline.
- **Bead 4 depends on Bead 1 — an R8 merge-order pin, not a
  code-state edge.** Bead 4's step-1.6 preflight cites ADR-0041
  §2(i) — clause-level numbering that exists ONLY after Bead 1's §2
  amendment lands (today's ADR-0041 has plain §1/§2/§3). Without the
  edge, Bead 4 could merge first and land code citing non-existent
  doctrine, violating R8's "amendment lands with the first citing
  code" (the divergence GATE itself is domain-coverage-based and
  would not fail — the hazard is purely merge order, so the edge is
  the pin that preserves the precise clause citation).

Waves: W1 = {1}, W2 = {2, 3, 4}. Longest dependency chain: 2
(1 → 2, 1 → 3, 1 → 4). Bead count 4, within the 3–5 target.

**Shared-file adjacencies WITHOUT edges** (shared source files are not
dependencies; flagged for the implementers):

- Beads 1 and 4 both touch `internal/gitutil` (Bead 1: the NEW
  `neteffect.go` only — it reuses `IsAncestor`/`gitArgs` WITHOUT
  editing `gitops.go`; Bead 4: the `WorktreeRemoveForce` body in
  `gitops.go`) — disjoint files, and the 4-on-1 ordering edge makes
  them sequential anyway.
- Beads 2 ∥ 4 both touch `internal/complete/complete.go` (Bead 2: the
  no-evidence reconcile refusal in the step-2.1 area; Bead 4: the
  step-1.6 orphan preflight at `:491-499`) — disjoint hunks; whichever
  merges second rebases trivially.
- Beads 1 → 2 both touch `FinalizeEpic` in
  `internal/executor/mindspec_executor.go` (Bead 1: the probe block at
  `:711-726`; Bead 2: the binding write after the auto-merge
  `MergeInto` at `:622`) — SAME function, different hunks ~100 lines
  apart; the 2-on-1 edge makes this sequential, so Bead 2 branches
  after Bead 1 merges and no concurrent rebase occurs.
- Beads 1, 2, 3 all touch `.mindspec/adr/ADR-0041-gate-before-mutate.md`
  (1: the §2 amendment text; 2: no text — code-side citations only;
  3: the new §4 section). 2 and 3 both follow 1, and §2/§4 are
  disjoint sections.

**Delivery housekeeping (kickoff, not a bead)**: per the spec's In
Scope list, the orchestrator closes the verified-shipped tracker
entries `mindspec-zty3`, `mindspec-blp6`, `mindspec-yqdf`,
`mindspec-h4n5` against 119's landed evidence at spec kickoff.

**Plan-level choices the spec delegates (Open Questions), resolved:**

- **gh seam home (ADR-0030 note): cmd-side.** A new
  `cmd/mindspec/finalize_pr.go` beside the result-consumption site
  (`implApproveTail`, `cmd/mindspec/impl.go:150`). Rationale: the
  automation is strictly post-finalize and consumes
  `approve.ImplResult` at the cmd layer; `gh` is not a git fact, so it
  does not belong behind `executor.Executor`; cmd is outside the
  enforcement-package pin (`internal/lint/boundary_test.go`), so a
  process-spawning seam there adds no enforcement-package legs; and
  the executor interface stays untouched. The seam is a package-var
  `*Fn` (the `complete.go` seam convention) whose default execs `gh`
  with a per-leg `context` timeout; the harness `gh` PATH shim
  (`internal/harness/recorder.go:122`) records the end-to-end argv.
  The post-merge refresh calls `gitutil.FetchRemoteBranch` from cmd
  (precedent: `cmd/mindspec/migrate.go` already imports gitutil).
  Bounded timeouts are fixed constants (no new config surface, per
  Non-Goals): 60s each for `pr create`/lookup/`pr merge`/the
  reconcile query; 15m for the checks watch.
- **Net-effect predicate home: `internal/gitutil`** (the spec's second
  named option). Forced by the import graph: `internal/executor`'s
  package contract is "must NOT import any enforcement package"
  (`internal/executor/executor.go:6-11` — the documented
  ADR-0030 boundary; the enforcement set, including
  `internal/phase`, is the list `boundary_test.go:112-117` pins),
  and `internal/lifecycle` imports `internal/phase`
  (`internal/lifecycle/scan.go:39`) — so an executor import of a
  lifecycle-homed predicate would pull `internal/phase` into the
  executor's import graph, violating that contract. Both consumers
  (executor probe, lifecycle doctor suppression) already import
  gitutil.
- **Net-effect mechanism**: a two-leg content evaluation in ONE
  exported symbol, `gitutil.NetEffectLanded(workdir, ref, target)`:
  **(a) tree subsumption** — three-way `git merge-tree --write-tree`
  (base = merge-base(ref, target), ours = target, theirs = ref);
  landed iff the merge result's tree OID equals target's tree OID
  with no conflicts (handles squash, partial-plus-novel, and
  reverted-content-reappears — a revert on target makes the merge
  re-introduce ref's content, so the trees differ and the answer is
  NOT landed); **(b) tracker-payload subsumption** — reached only
  when (a) says not-landed AND ref's entire diff against the
  merge-base is confined to `.beads/issues.jsonl` (the tracker-only
  carrier shape, derived from the diff itself, not from the caller):
  every id→status assertion ref's export changes relative to the
  merge-base is already satisfied by target's committed export
  (equal, or a later terminal status). Leg (b) is what makes the
  "LATER superseding export" clause of R4 true (a superseding export
  edits the same JSONL lines further, which leg (a) reads as a
  conflict). The JSONL parsing is stdlib-only (~20 lines, the
  `issueStatusesInJSONL` shape), so gitutil stays a leaf.
  `merge-tree --write-tree` needs git ≥ 2.38; an older git is an
  INFRA failure routed per each consumer's pinned posture (probe:
  warn + ancestry-only fail-open; suppression: skip + first-error),
  never a silent "landed"/"not landed". The mechanism may be refined
  at implementation ONLY within the observable contract AC-8/AC-9/
  AC-19 pin.
- **Landed-binding metadata shape** (R5(b)): three keys written
  through the existing `bead.MergeMetadata` surface
  (`internal/bead/bdcli.go:322` — no new bead-package API):
  `mindspec_landed_merge_sha`, `mindspec_landed_second_parent`,
  `mindspec_landed_at` (RFC3339). Read back in `FindLandedMerge` as
  the third admissible datum: it confirms a candidate M when its
  recorded merge SHA equals M's SHA, or its recorded second-parent
  equals/is an ancestor of M's second parent (the same
  equal-or-ancestor discipline as the two existing legs).
- **Revert/reapply detection** (R5(d)): net effect since M via the
  shared primitive — `ContentSubsumed(M^1, M, <specBranch tip>)`:
  M's work still subsumed at the CURRENT first-parent tip ⇔
  identified; a revert-then-reapply (either shape) leaves the content
  subsumed, so M is identified by construction and no
  "ever-reverted ⇒ reject" over-rejection is even expressible.
- **Reconcile-query shape** (R3): `gh pr list --head
  chore/finalize-<specID> --base main --state all --json
  number,state,url,headRefName,baseRefName` through the same seam,
  itself bounded (60s). OPEN ⇒ create-success; MERGED ⇒ merge-success
  (R2(c) post-merge path); empty ⇒ confirmed-unmerged NOTE; query
  failure ⇒ UNDETERMINED warning, NOTE without asserting either way.
  Checks are read via `gh pr checks <branch> --json name,state`
  polled until all completed or the 15m bound; an EMPTY checks array
  is NOT green (R2(b)).

## ADR Fitness

- **ADR-0041 (Gate-Before-Mutate) — AMENDED by this spec (R8), the
  only ADR change.** Two additions, each landing in the bead whose
  code first cites it so the ADR-divergence gate sees the touchpoint:
  - **§2 convergence-completeness — Bead 1** (its net-effect
    predicate + doctor-suppression rewire is the first citing code;
    the amendment text also carries the deadlock-free/genuine-forward-
    exit rule, the attested-restore exit, and the durable-corroboration
    + fail-closed-before-cleanup binding contract, which Beads 2 and 4
    subsequently cite from `landed.go`, the executor binding writes,
    and the step-1.6 preflight). Authored as part of Bead 1's single
    commit; asserted by Bead 1's AC-16 §2 anchor test.
  - **§4 machine-owned finalize carrier — Bead 3** (the automation is
    the first and only citing code). The amend-not-new rationale is
    the spec's, recorded in the amendment text: §4 grants no new
    review/merge authority (tracker-only carrier, opt-in default-off,
    checks-gated, head/base-pinned) and completes §2's forward
    reconcile of the merged-unclosed-on-protected-main state.
  Both additions refine the contract this plan's preflight/refusal
  code enacts; no rollback machinery anywhere (Non-Goals).
- **ADR-0042 (Render & Derivation Provenance) — unchanged, APPLIED
  twice** exactly as the spec's Touchpoints state: Bead 4 extends the
  check-at-use discipline to `WorktreeRemoveForce` (the one
  destructive wrapper the 120 sweep scoped out), mirroring the
  sibling wrappers' `containment.CheckContainment` + option-like
  guard; Bead 3 applies the full provenance discipline at the gh
  seam — every ID operand in `gh` argv passes `idvalidate` at each
  construction site (gate-all-ids), and every remote-influenced
  string (PR URL, echoed titles, `gh` stderr, check names) renders
  `termsafe`-escaped. Remains the best-fit contract; nothing it
  governs changes.
- **ADR-0037 (Panel Gate as Enforced Contract) — unchanged.** Bead 2
  strengthens the landed-merge EVIDENCE legs outside the panel
  decision (where 119 R4 placed the anti-bypass burden); the decision
  ladder — including decision (5) MissingRef → Allow + Warn and §6's
  registered-panels-only scope — is not touched. The fail-closed
  registered-unsatisfied-panel variant stays the 119-named follow-up,
  out of scope.
- **ADR-0035 (Agent Error Contract) — unchanged, applied.** Every new
  refusal and warning carries a single-lever recovery line: the
  binding-write refusal names the verb's own re-run (AC-22); the
  all-orphans refusal names the full finite `mindspec complete`
  sequence (AC-14); the attested-restore refusal names the exact
  `git branch bead/<id> <second-parent SHA>` command PLUS the R5(c)
  human-verification marker — the one deliberately NON-mechanical
  recovery line in the tree, and the marker is what keeps ADR-0035's
  "orchestrators follow recovery lines mechanically" assumption
  honest there.
- **ADR-0023 / Dolt as state authority — unchanged, extended in
  spirit**: the R5 landed-binding is recorded IN Dolt through
  `bead.MergeMetadata`, extending single-authority to landed-work
  evidence rather than adding a git-side shadow.
- **ADR-0030 (Executor Boundary) — unchanged.** The gh seam lives
  cmd-side (see the plan choice above), outside the enforcement
  packages pinned by `internal/lint/boundary_test.go`; enforcement
  packages gain no process-spawning legs. The net-effect predicate's
  gitutil home respects the executor's no-enforcement-imports pin.
  Deliberately not in `adr_citations` (the 120 precedent: its
  domain-intersection is carried by the cited ADRs; nothing it
  governs changes).

## Testing Strategy

- **Unit tests via package seams.** The gh automation is pinned at
  the cmd-side `*Fn` seam with scripted per-leg responses (create /
  lookup / checks / merge / reconcile-query), so AC-1..AC-7, AC-20,
  AC-21 assert exact argv, adoption decisions, degrade paths, and
  exit-0 without a network. The net-effect predicate and landed-merge
  identity are pinned with real-git throwaway fixtures (the
  `internal/lifecycle`/`internal/gitutil` fixture helpers).
- **Real-origin fixtures, never faked ancestry.** AC-4, AC-8, AC-9,
  AC-19 fixtures create a REAL bare `origin` remote and materialize
  squash merges, true merges, reverts, and superseding exports on
  `origin/main` by pushing to the bare repo, so the probe/suppression
  observe the landing through `git fetch` exactly as a live run would
  (AC-4's stated fixture discipline). Bead 3's "merge succeeded" gh
  stub performs the real merge push on the bare origin.
- **Harness gh-shim end-to-end scenario** (Bead 3): a new
  `internal/harness` scenario drives `impl approve` on a
  protected-main fixture with the recording `gh` shim installed
  (`recorder.go:122`), asserting the R1 `pr create` argv (head/base/
  title with epicID), idempotent re-run adoption, and degrade-to-NOTE.
  `asserts.go:168` is the precedent that harness recordings are
  QUERIED for `gh pr create`/`pr merge` argv (there as the
  pre-approve prohibition check); the same recording surface supports
  this scenario's positive argv assertions. This is AC-1's
  end-to-end leg and exercises AC-2/AC-3/AC-21 shapes in one recorded
  flow.
- **Fault-injection discipline, classified honestly (R9 / ADR-0041
  §3)**: the R1–R3 automation legs are DOCUMENTED-FORWARD-SAFE — each
  leg's error is absorbed into warning + NOTE + exit 0 by design, and
  AC-6's per-leg fault matrix records the classification with the
  code cites proving the absorb (no fabricated kill tests). The R5
  landed-binding write is KILL-TESTED, classified by the spec itself:
  AC-22 injects a `bead.MergeMetadata` failure through the executor
  binding seam immediately after a REAL `MergeInto` landed in a
  real-git fixture, at BOTH producer legs, asserting
  cleanup-suppressed + recoverable refusal + locate-by-identity
  convergence on re-run. It is never recorded forward-safe.
- **Anti-drift pins (AC-17, the 119 AC-12 pattern)**: both R4
  consumers route the SAME exported `gitutil.NetEffectLanded` through
  package seam variables; a function-identity test asserts both seam
  defaults ARE that symbol, so a private reimplementation at either
  site fails compilation-or-test.
- **RED-on-revert, with the spec's honest tag deviations**: every AC
  test reproduces its original trigger and fails on today's `main`,
  EXCEPT the deviations the spec pins inline — AC-10(ii) and
  AC-19(i)/(ii) PASS today (they are anti-overreach/anti-false-
  positive guards against non-conforming implementations), and AC-5
  is the negative half of the AC-4/AC-5 differential pair. Bead
  panels spot-check revert-RED where cheap and record it in review
  evidence.
- **Validation proofs**: each bead's verification runs its package
  subset of the spec's Validation Proofs commands plus
  `golangci-lint run ./...`; the spec-end review evidence maps every
  AC-1..AC-22 to exact `go test <pkg> -run <test>` commands. Known
  pre-existing flakiness (`mindspec-z4ps`) is the only tolerated red,
  byte-identical to the spec-init SHA.

## Bead 1: Net-effect already-merged predicate — one symbol, both consumers, refreshed stale-tracker classifier

R4 in full plus R2(c)'s classifier leg, and the ADR-0041 §2 amendment
(the first citing code — see the preamble re-cut rationale). Makes
"already landed on `origin/main`" a single exported content-aware
predicate consumed by the `FinalizeEpic` probe and the doctor
merged-carrier suppression; retires the known-blind-spot comment;
moves the stale-tracker classifier off possibly-stale local `main`.

**Steps**

1. Add `internal/gitutil/neteffect.go`:
   `ContentSubsumed(workdir, base, ref, target string) (bool, error)`
   — the shared three-way primitive (merge-tree --write-tree; result
   tree == target tree, no conflicts ⇒ subsumed) — and the ONE
   exported predicate `NetEffectLanded(workdir, ref, target string)
   (bool, error)` implementing the two-leg mechanism from the plan
   choice above (tree subsumption; tracker-payload subsumption for a
   diff confined to `.beads/issues.jsonl`). The `merge-tree`
   EXIT-CODE TRICHOTOMY is part of the contract (panel O1): exit 0 ⇒
   compare the result tree OID against target's tree OID; exit 1 ⇒
   CONFLICT — a leg-(a) NOT-landed answer (or, for a JSONL-confined
   diff, the fall-through into leg (b)), NEVER an infra error; exit
   ≥ 2 ⇒ infra error, propagated. Conflating exit 1 with infra would
   silently break the "later superseding export ⇒ suppressed" clause
   and R2(c)'s self-loop avoidance (AC-19's superseding leg pins
   it). Leg (b) subsumes by the status TOTAL ORDER
   `open < in_progress < closed`: a changed id→status assertion is
   satisfied only by an equal-or-LATER status in target's export — a
   non-terminal status on `main` can never subsume a carrier's
   close. Doc comment cites ADR-0041 §2(iii) and states the pinned
   consequences verbatim: squash-then-revert ⇒ NOT landed;
   partially-landed-plus-novel ⇒ NOT landed;
   squash-then-unrelated-later-changes ⇒ landed. Infra failures
   (including git < 2.38, where `--write-tree` is unsupported)
   return errors, never a guessed boolean.
2. Amend `.mindspec/adr/ADR-0041-gate-before-mutate.md` with the §2
   convergence-completeness additions exactly as the spec's ADR
   Touchpoints state them: (i) deadlock-free recovery graph / genuine
   forward exits (naming the tpjn all-orphans sequence and the q9ea
   attested-restore exit as the two instances); (ii) durably
   corroborated, never-subject-only, revert/reapply-aware-by-net-
   effect landed evidence, with the merge-time binding recorded
   fail-closed-before-cleanup; (iii) content-aware already-merged
   re-derivation where the hosting workflow can discard SHAs. Cite §2
   from the predicate's doc comment (this bead's citing code).
3. Rewire the PROBE (`internal/executor/mindspec_executor.go:711-726`)
   through a package seam var (`netEffectLandedFn =
   gitutil.NetEffectLanded`): ancestry remains sufficient as-is (the
   spent-carrier routing rationale, restated in the comment); when
   ancestry is false, fall back to the net-effect predicate over the
   spec branch's full diff. On a content-fallback INFRA failure: warn
   naming the undetermined probe and proceed on the ancestry answer
   alone — the DELIBERATE fail-open the spec pins (the stranded
   outcome stays fully detected by the shipped doctor finding).
   Retire the known-blind-spot comment at `:700-704`, replacing it
   with the per-consumer contract note.
4. Rewire the DOCTOR suppression
   (`internal/lifecycle/finalize_orphans.go:126-139`) through its own
   seam var (`finalizeOrphanNetEffectFn = gitutil.NetEffectLanded`):
   the suppression decision becomes the net-effect test EVEN WHEN
   ancestry holds (a true-merged-then-reverted carrier is flagged
   again); on an infra failure the branch is SKIPPED asserting
   nothing, surfaced through the existing mixed-list `firstErr`
   contract (`:130-135` discipline unchanged).
5. R2(c) classifier leg: `StaleTrackerOnMain` and
   `ScanIntegrityFindings`' committed-export read
   (`finalize_orphans.go:186`, `scan.go`) consult the refreshed
   `origin/main:.beads/issues.jsonl` for the "export never reached
   main" determination (falling back to local `main` only when no
   `origin/main` ref exists — the no-remote direct workflow); a
   residual local-`main` lag behind an export that HAS reached
   `origin/main` surfaces only as a pull advisory whose recovery is a
   pull/update of local `main` — never `mindspec impl approve`
   (the self-loop the spec's §2(i) forbids). The
   `internal/doctor/lifecycle_integrity.go` wiring is UNCHANGED
   (interop pins only, per Impacted Domains).
6. AC-17 anti-drift pin: a function-identity test asserting BOTH seam
   defaults (executor probe, lifecycle suppression) are
   `gitutil.NetEffectLanded`.
7. Fixtures (real bare-origin, per Testing Strategy): AC-8 (squash-
   merged spec branch → predicate landed; probe routes to the
   `chore/finalize-` carrier); AC-9 both consumers both polarities
   (genuinely-unmerged novel branch NOT landed / normal push path
   unchanged; squash-merged carrier no longer flagged; truly-unmerged
   carrier still flagged); AC-19 all four halves incl. (iv)
   true-merge-then-revert flagged at the doctor while the same shape
   stays ancestry-routed at the probe (asserted unchanged); the
   superseding-export suppression leg; an OLD-GIT subtest (a stubbed
   primitive seam returning the unsupported-`--write-tree` error:
   the predicate propagates an ERROR, never a boolean — panel O1);
   INFRA-POSTURE sentinel-stub subtests PER CONSUMER (seam stubbed
   to a sentinel error: the probe WARNS naming the undetermined
   probe and proceeds on the ancestry answer alone; the suppression
   SKIPS the branch asserting nothing, sentinel surfaced as the
   mixed-list `firstErr` — panel F2); the AC-16 §2 anchor test
   (amendment text covers the §2 anchors; predicate doc cites §2).

**Verification**

- [ ] `go test ./internal/gitutil/... ./internal/executor/... ./internal/lifecycle/... ./internal/doctor/...` passes; `golangci-lint run ./...` clean
- [ ] AC-8 subtest RED on today's `main` (the documented blind spot); probe routes squash-merged spec branch to the carrier
- [ ] AC-9 subtests: both consumers, both polarities, normal push path byte-identical for a novel branch
- [ ] AC-19 subtests: (i)/(ii) pass as anti-false-positive guards (tag deviation stated in-test), (iii)/(iv) RED on revert; probe ancestry-split asserted
- [ ] AC-17 identity pin fails when either consumer is rewired to a private copy
- [ ] Old-git subtest: unsupported `--write-tree` propagates as an error, never a boolean; per-consumer infra-posture subtests green (probe warn+ancestry-only; suppression skip+`firstErr`)
- [ ] Stale-tracker classifier reads `origin/main`'s export; pull-advisory recovery line asserted (never a re-finalize); no-remote fallback covered
- [ ] AC-16 §2 half: amendment text anchors present (`rg -n 'convergence' .mindspec/adr/ADR-0041-gate-before-mutate.md` non-empty); predicate cites §2
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-8 — squash fixture: predicate reports landed; probe routes to the `chore/finalize-` carrier
- [ ] AC-9 — negative squash-detection at both consumers, both polarities
- [ ] AC-17 — one exported predicate symbol consumed by both sites, identity-pinned
- [ ] AC-19 — net-effect polarity at both consumers incl. the per-consumer ancestry split
- [ ] AC-16 (§2 half) — §2 amendment text anchors + citation from the predicate
- [ ] R2(c) classifier leg — refreshed-`origin/main` stale-tracker determination + pull advisory (consumed end-to-end by Bead 3's AC-4)

**Domain:** execution (primary — `internal/gitutil` predicate home,
`internal/executor` probe) + workflow (`internal/lifecycle`
suppression + classifier, `internal/doctor` interop pins) — per the
spec's Impacted Domains assignments.

**Depends on**
None (foundational; the sole W1 root — Beads 2, 3, and 4 all
follow it). (Human-readable
documentation only — bd edges are wired exclusively from the
`work_chunks[].depends_on` frontmatter.)

## Bead 2: Landed-merge identity — durable corroboration, merge-time binding, revert/reapply, attested restore

R5 in full — the crux. `FindLandedMerge` stops accepting subject-only
matches; the merge-time landed-binding is recorded fail-closed before
cleanup at BOTH in-binary producer legs; the no-datum state refuses
with the attested-restore forward exit; revert/reapply-awareness is
net effect since M via Bead 1's shared primitive.

**Steps**

1. Harden `internal/lifecycle/landed.go` (R5(a)): a candidate is
   positively identified ONLY when at least one admissible datum
   CONFIRMS it (equals or is an ancestor of the candidate's second
   parent) and no available datum contradicts it. Admissible data:
   the registered panel's `reviewed_head_sha` (existing,
   `:136-146`), a surviving `bead/<id>` tip (existing, `:147-153`),
   and the NEW landed-binding read from bd metadata (the three keys
   from the plan choice, via a lifecycle seam over
   `bead.GetMetadata`). The current fall-through acceptance at
   `:155` (subject text alone) is DELETED; no content heuristic over
   the second parent's commits substitutes for identity (Non-Goals).
   Cite ADR-0041 §2(ii) from the function doc.
2. R5(d) revert/reapply-awareness: after corroboration, evaluate
   `gitutil.ContentSubsumed(M^1, M, <specBranch tip>)` (Bead 1's
   primitive — the produced-consumed edge): not subsumed ⇒ the
   refusal names the revert evidence and M is NOT identified;
   subsumed (including revert-then-reapply in EITHER shape) ⇒
   identified. "Ever-reverted ⇒ reject" is structurally
   inexpressible under this mechanism (the AC-10(ii) anti-overreach
   guard).
3. R5(c) attested-restore exit: when a subject-scan candidate exists
   but NO datum confirms it, `FindLandedMerge` returns a typed
   no-evidence error CARRYING the candidate's merge + second-parent
   SHAs; `internal/complete`'s reconcile (119's existing no-evidence
   path) renders the refusal naming those SHAs, the exact command
   `git branch bead/<id> <second-parent SHA>`, and the R5(c)
   human-verification marker ("running this command attests that
   merge <SHA> carried THIS bead's work — verify before running; do
   not execute blindly"). Executing the command converts the state
   into the surviving-branch corroboration leg; re-running converges.
4. R5(b) merge-time binding at BOTH producer legs
   (`internal/executor/mindspec_executor.go`): after
   `gitutil.MergeInto` succeeds at `CompleteBead`'s merge leg
   (`:410`) and at `FinalizeEpic`'s auto-merge leg (`:622`), and
   BEFORE any branch/worktree cleanup for that bead, locate the
   landed merge BY IDENTITY — an executor-local gitutil-only scan
   (`gitutil.FirstParentMerges` subject match corroborated by the
   surviving branch tip; NEVER rev-parse of the current tip/HEAD,
   also on the fresh path, so one code path serves first-run and
   re-run alike) — and write the binding through a new executor seam
   `mergeBindingFn = bead.MergeMetadata`. FAILURE POSTURE (the
   contract, not a choice): a failed write SUPPRESSES that bead's
   cleanup and refuses recoverably with an ADR-0035 recovery line
   naming the verb's own re-run (`mindspec complete <id>` /
   `mindspec impl approve <spec>`) — the `complete.go:1032-1055`
   refuse-recoverably shape, NOT the adjacent warn-and-continue
   cleanup style (`:437-450`). The branch survives as the
   corroborating datum; a re-attempted `MergeInto` of an
   already-ancestor branch is a no-op, and the re-run's
   locate-by-identity records M's SHAs. RE-RUN CONVERGENCE AT THE
   FINALIZE LEG (panel G2): `FinalizeEpic`'s auto-merge loop skips a
   bead branch that is already an ancestor of the spec branch — so
   an already-merged-but-UNBOUND lifecycle bead (merged, but a prior
   run's binding write failed and suppressed its cleanup) must NOT
   be skipped past the binding: for every allow-set candidate whose
   branch is already an ancestor, the loop checks the landed-binding
   metadata and, when ABSENT, runs the same locate-by-identity +
   binding write for it BEFORE the cleanup leg — so the fail-closed
   refusal converges on re-run at BOTH producer legs, not just
   `CompleteBead`'s. Cite ADR-0041 §2(ii) at both write sites.
5. Regression floor (R5(e)/AC-12): 119's merged-unclosed fixtures
   (genuine landed merge; panel-free AND panel-registered variants;
   already-closed obligation variant) still reconcile and close with
   assertions unchanged — the panel-registered variant stays
   binding-ABSENT (the real pre-121 state); add honest multi-commit
   and conflict-resolution merge fixtures that reconcile and close;
   pin each datum INDEPENDENTLY sufficient: (i) panel-only,
   (ii) surviving-branch-only, (iii) binding-only — so a
   binding-REQUIRED implementation fails (i)/(ii) and a
   binding-ignoring one fails (iii).
6. Fixtures: AC-10 both polarities with the binding PLANTED in every
   variant (revert ⇒ refuse naming revert evidence; revert-then-
   reapply in BOTH shapes ⇒ identify + close, passing today by
   design); AC-11 spoof (hand-crafted `Merge bead/<id>` subject over
   a NON-EMPTY unrelated second parent, no datum ⇒ refuse via the
   no-evidence path — identity, not non-emptiness); AC-18 honest
   out-of-band merge ⇒ refusal with SHAs + command + marker asserted
   present, then EXECUTING the command + re-running converges to
   close; AC-22 kill test at BOTH producer legs (real-git fixture,
   injected `mergeBindingFn` failure after the real merge):
   cleanup suppressed, branch survives, recoverable refusal, no
   warn-and-continue; re-invocation locates M by identity, records
   M's SHAs (asserted ≠ current tip), completes cleanup, converges —
   the `FinalizeEpic` leg's re-run subtest constructs the
   already-merged-but-UNBOUND state (branch already an ancestor,
   binding absent) and asserts the re-run BINDS it rather than
   skipping it (the G2 convergence hole); recorded KILL-TESTED in
   the fault-injection suite (R9).

**Verification**

- [ ] `go test ./internal/lifecycle/... ./internal/executor/... ./internal/complete/... ./internal/bead/...` passes; `golangci-lint run ./...` clean
- [ ] AC-11 spoof subtest RED on today's `main` (subject-only close); refuses via 119's no-evidence path
- [ ] AC-10(i) RED on revert; AC-10(ii) passes today AND against this implementation (tag deviation stated in-test); cherry-pick reapply shape covered
- [ ] AC-12: all 119 fixtures green with assertions unchanged; per-datum sufficiency triple (i)/(ii)/(iii) green; multi-commit + conflict-resolution fixtures close
- [ ] AC-18: refusal names candidate SHAs + exact restore command + human-verification marker (string-asserted); executing + re-running closes the bead; RED on today's `main`
- [ ] AC-22 at BOTH legs: cleanup suppressed, ADR-0035 recovery line, locate-by-identity records M's SHAs not HEAD, convergence — incl. the FinalizeEpic already-merged-unbound re-run subtest (re-run binds, never skips); suite records KILL-TESTED
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-10 — revert/reapply net-effect, both polarities, binding planted
- [ ] AC-11 — never-subject-only; identity not non-emptiness
- [ ] AC-12 — regression floor with each admissible datum independently sufficient
- [ ] AC-18 — no-durable-datum refusal + attested-restore genuine forward exit + marker
- [ ] AC-22 — binding-write kill test at both producer legs, fail-closed-before-cleanup, locate-by-identity
- [ ] AC-16 (citation leg) — `landed.go` and both binding write sites cite ADR-0041 §2

**Domain:** workflow (primary — `internal/lifecycle/landed.go`,
`internal/complete` refusal rendering) + execution (the two
`MergeInto` producer legs in `internal/executor`, `bead.MergeMetadata`
consumption — no new bead-package API) — per the spec's Impacted
Domains assignments.

**Depends on**
Bead 1 (consumes `gitutil.ContentSubsumed` for the R5(d) since-M net
effect, and the §2 amendment text its citations reference). (bd edges
wired from `work_chunks[].depends_on`.)

## Bead 3: Finalize-PR automation — gh seam, config keys, reconcile-by-query, harness scenario

R1 + R2 + R3 — the manual-dance automation, carrying the ADR-0041 §4
amendment. Auto-opens the templated finalize PR when `gh` is
available; opt-in auto-merges on affirmative green checks with a true
merge commit; every failure degrades to today's NOTE with exit 0 and
reconciles by query; end-to-end recordable through the harness gh
shim.

**Steps**

1. Config surface (R1/R2, AC-7): add flat keys
   `auto_open_finalize_pr` (bool, default **true**) and
   `auto_merge_finalize_pr` (bool, default **false**) beside
   `auto_finalize` (`internal/config/config.go:21`); defaults set in
   `DefaultConfig()` (the `loadUncached` unmarshal-over-defaults
   pattern makes absent-key ⇒ true work for the first key); render
   both in `mindspec config` beside `auto_finalize`
   (`cmd/mindspec/config.go:139` area). The inert combination
   (`auto_open_finalize_pr: false` + `auto_merge_finalize_pr: true`)
   opens/adopts nothing, merges nothing, and warns naming the inert
   key. Result surface (R1): add `EpicID` to `approve.ImplResult`
   (`internal/approve/impl.go:141`), populated wherever
   `FinalizeBranch` is set (`impl.go:562` area) — the bounded
   addition the templated title needs.
2. New `cmd/mindspec/finalize_pr.go`: the injectable gh seam
   (package-var `*Fn` execing `gh` with per-leg context timeouts —
   60s create/lookup/merge/query, 15m checks watch; `exec.LookPath`
   detects gh absence) and the automation entry called from
   `implApproveTail`'s success path when `result.FinalizeBranch` is
   non-empty (`cmd/mindspec/impl.go:186-198` area), strictly AFTER
   the finalize mutation chain completed (R3). Every ID operand
   (specID via `idvalidate.SpecID`, epicID via `idvalidate.BeadID`)
   gates at each argv construction site; a malformed ID degrades
   WITHOUT constructing any gh argv (AC-20(ii)). Every
   remote-influenced string (PR URL, echoed title, gh stderr, check
   names) renders `termsafe`-escaped (AC-20(i)). Cite ADR-0041 §4
   and ADR-0042 from the seam's doc comment.
3. R1 open/adopt: `pr create` with head `chore/finalize-<specID>`,
   base `main`, title `chore(beads): finalize epic <epicID> for spec
   <specID>` (the `mindspec_executor.go:937` commit shape — the
   NOTE's epicID omission corrected), body naming spec/epic/staleness
   consequence. Idempotency + adoption pin: an already-open PR is
   adopted as success ONLY when base == `main` AND head ==
   `chore/finalize-<specID>` (the `pr list --head --base` query); a
   same-head/other-base PR is NOT adopted — warning + NOTE, and never
   auto-merged.
4. R2 opt-in merge: only on a PR THIS run opened or adopted
   (boundary (a)); checks via `gh pr checks --json` polled to the
   bound — an empty/none-reported result is NOT green (boundary
   (b)); merge with `gh pr merge --merge` (true merge commit, never
   squash/rebase). Post-merge — for BOTH finding kinds (boundary
   (c)): best-effort `gitutil.FetchRemoteBranch("origin", "main")`
   (the probe's own fetch discipline; a fetch failure degrades to a
   warning), after which `lifecycle.ScanIntegrityFindings` evaluated
   against the refreshed refs reports NO `FinalizeOrphan` of either
   kind — the `finalize_branch` half clears via the EXISTING
   SHA-ancestry suppression (the true merge commit makes the carrier
   an ancestor of refreshed `origin/main`); the `stale_tracker` half
   is the consumed Bead-1 edge (the R2(c) refreshed-`origin/main`
   classifier, while local `main` still lags). With the key false:
   no merge, PR left open, doctor keeps flagging (the 3xqm item-2
   interop preserved).
5. R3 degrade + reconcile-by-query: after ANY failed leg or timeout
   (create, lookup, checks, merge, watch), run the bounded reconcile
   query — OPEN ⇒ create-success, MERGED ⇒ merge-success (step 5's
   post-merge path), confirmed-absent ⇒ the stranded-carrier NOTE
   asserts unmerged, query-failure ⇒ UNDETERMINED warning + NOTE
   asserting nothing. Every failure path: warning naming the leg
   (and the open PR when one exists), today's NOTE
   (`cmd/mindspec/impl.go:194-197` shape), **exit 0**, tracker/git
   state byte-identical to a no-gh run. The fault-injection suite
   records all legs DOCUMENTED-FORWARD-SAFE with code cites (never
   kill tests) — R9/AC-6.
6. Amend ADR-0041 with the new §4 (machine-owned finalize carrier)
   per the spec's Touchpoints text: tracker-only carrier;
   machine-open always safe; machine-merge opt-in default-false +
   affirmative green checks + head/base adoption pin + true merge
   commit; every failure degrades to NOTE + doctor surfacing, never
   fails or un-finalizes the verb (legs documented-forward-safe per
   §3). AC-16 §4 anchor test beside it.
7. Tests — end-to-end AND unit. Harness scenario
   `internal/harness/scenario_finalize_pr.go`: the protected-main
   finalize fixture with the recording gh shim. The recorder shims
   record-and-DELEGATE (`recorder.go:128-152`), so the scenario
   installs a SCRIPTED FAKE `gh` later in PATH as the delegate
   target — the `writeFakeBD` PATH-shim precedent
   (`internal/harness/r7_hostile_test.go:127-160`) — scripting each
   leg's stdout/exit per subtest. Asserts the AC-1 argv
   (head/base/title-with-epicID), AC-2 re-run adoption (no duplicate
   create), AC-3 no-gh byte-identical degrade, and the AC-21
   landed-then-error reconcile shape. Unit fixtures at the
   seam: AC-1 (argv + escaped URL in output),
   AC-2 (adoption + base pin, no merge against other-base even with
   the key true + green), AC-3 (gh absent ⇒ exactly today's NOTE,
   warning, exit 0, state byte-identical), AC-4 (real bare-origin
   merge ⇒ both suppressions clear against refreshed refs while
   local `main` still lags), AC-5 (default-false ⇒ no merge, PR
   open, doctor still flags — the AC-4/AC-5 differential pair),
   AC-6 (the full per-leg fault matrix incl. zero-checks-not-green,
   per-leg timeouts, AND the post-merge `git fetch` leg — R9
   classifies every new automation point, and a fetch failure defers
   both doctor-finding clears with a warning, detection delayed
   never lost: DOCUMENTED-FORWARD-SAFE — each case exit 0 + NOTE +
   intact finalize state + reconcile query polarity), AC-7
   (round-trip + inert-combination
   warning), AC-20 (hostile output escaped; malformed ID ⇒ zero gh
   argv, pinned via seam recording), AC-21 (created-then-errored and
   merged-then-errored ⇒ success surfaced, no stranded NOTE, re-run
   converges to the correct doctor result).

**Verification**

- [ ] `go test ./internal/config/... ./internal/approve/... ./cmd/mindspec/... ./internal/harness/...` passes; `golangci-lint run ./...` clean
- [ ] AC-1 argv pinned at the seam AND in the harness recording (title carries the epicID from `ImplResult.EpicID`)
- [ ] AC-2 adoption + base pin: other-base PR never adopted, never merged; AC-3 byte-identical no-gh degrade
- [ ] AC-4/AC-5 differential pair green (AC-4 against REAL refreshed refs — both finding kinds clear; AC-5 boundary holds)
- [ ] AC-6 matrix: every leg exit 0 + NOTE + warning naming the leg; zero-checks NOT green; reconcile polarity incl. UNDETERMINED; post-merge fetch leg included (failure defers both finding clears); legs recorded DOCUMENTED-FORWARD-SAFE with cites
- [ ] AC-7 round-trip + inert warning; `rg -n 'auto_open_finalize_pr|auto_merge_finalize_pr' internal/config/ cmd/mindspec/` non-empty
- [ ] AC-20 both directions (termsafe outputs; zero gh argv on malformed ID); AC-21 both landed-then-error polarities converge
- [ ] AC-16 §4 half: amendment anchors present (`rg -n 'finalize carrier' .mindspec/adr/ADR-0041-gate-before-mutate.md` non-empty); seam cites §4
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-1 — templated auto-open with epicID-bearing title, escaped URL, seam + harness pinned
- [ ] AC-2 — idempotent adoption with the head/base pin
- [ ] AC-3 — gh-absent degrade byte-identical to today
- [ ] AC-4 — opt-in merge-commit auto-merge; both suppressions clear against refreshed refs
- [ ] AC-5 — default-false boundary (differential pair with AC-4)
- [ ] AC-6 — per-leg fault matrix, exit 0, reconcile-by-query, documented-forward-safe
- [ ] AC-7 — config round-trip + inert-combination warning
- [ ] AC-20 — hostile input both directions (termsafe out; idvalidate-gated argv in)
- [ ] AC-21 — landed-then-error reconcile treats server-side success as success
- [ ] AC-16 (§4 half) — §4 amendment anchors + citation from the automation

**Domain:** workflow (primary — `cmd/mindspec` automation home +
config rendering, `internal/approve` result surface) + core
(`internal/config` keys) + execution (`internal/harness` scenario) —
per the spec's Impacted Domains assignments.

**Depends on**
Bead 1 (AC-4's load-bearing consumption is the R2(c)
refreshed-`origin/main` stale-tracker classifier — produced state,
not shared files; the `finalize_branch` half clears via existing
SHA-ancestry once the true merge commit lands). (bd edges wired from
`work_chunks[].depends_on`.)

## Bead 4: Orphan-preflight convergence + WorktreeRemoveForce containment gate

R6 + R7 — the two remaining, mutually independent fixes, folded into
one bead by the trivial-work test (R7 alone is a two-step change).
Makes the step-1.6 orphan preflight converge for ANY set of
simultaneously orphaned closed siblings, and closes the one
destructive wrapper outside spec 120's containment discipline.

**Steps**

1. R6(a) self-orphan determination: in `complete`'s step-1.6
   preflight (`internal/complete/complete.go:491-499`), when the
   other-orphan scan finds orphans, additionally determine whether
   the INVOKED bead is ITSELF orphaned-closed (closed in the
   tracker, `bead/<id>` exists and is NOT an ancestor of the spec
   branch — a new exported single-bead helper in
   `internal/lifecycle/orphans.go` reusing the
   `ScanOrphanedClosedBeads` trigger+confirmation discipline, so the
   logic cannot drift). Self-orphaned ⇒ the refusal DEMOTES to a
   WARN naming EVERY orphaned sibling and the invoked bead's
   recovery proceeds. An infra/ancestry error in the self-orphan
   determination ⇒ treated as NOT self-orphaned, refusal RETAINED
   (a retryable preflight refusal with nothing mutated, ADR-0041 §1
   — never a demotion on unproven evidence). Cite ADR-0041 §2(i)
   at the preflight.
2. R6(b) all-orphans refusal: when the invoked bead is NOT
   self-orphaned, the refusal names ALL orphaned siblings (the full
   `findOrphanedClosedBeadsFn` list, not `orphans[0]`) with the
   single recovery sequence (`mindspec complete <A>`,
   `mindspec complete <B>`, …, then re-run) — the full non-manual
   exit knowable from one refusal. IDs render via
   `idrender`/`termsafe` per the shipped conventions at this site
   (`:493-499`).
3. R6(c) is safety-by-construction, stated in the code comment: each
   WARN-demoted step strictly shrinks the orphan set (monotone,
   finite), and the shipped fail-closed impl-approve orphan gate
   (`runOrphanObligationGate`, `internal/approve/impl.go:403`/`:711`,
   consuming `lifecycle.ScanOrphanedClosedBeads` via
   `implScanOrphansFn`, `impl.go:65` — spec 115 Bead 2) backstops
   the intermediate state.
   No relaxation for normal beads (Non-Goals): a non-orphaned bead
   still refuses past orphaned siblings.
4. R7: `gitutil.WorktreeRemoveForce`
   (`internal/gitutil/gitops.go:935`) gains BOTH halves before the
   git invocation: `containment.CheckContainment(workdir, wtPath)`
   (the `WorktreeAddDetach`/`WorktreeAdd` sibling discipline,
   `:895`/`:920` — refuse with the containment `guard.NewFailure`,
   remove nothing) and the option-like operand guard
   (`rejectOptionLike(wtPath)`, the sibling convention). In-tree,
   plainly-named removals stay byte-identical. Note verified against
   the callers: the finalize self-heal's `os.RemoveAll` fallback
   (`mindspec_executor.go:901-905`) is already protected by the
   caller-level S1 gate at `:891` (defense-in-depth retained), so
   the wrapper gate cannot be defeated by that fallback; assert this
   in a test comment beside the AC-15 fixtures. Cite ADR-0042
   check-at-use from the wrapper doc.
5. Fixtures: AC-13 (N=3 mutually-orphaned siblings — `complete A`
   proceeds WARNing B and C, then B WARNing C, then C; all three
   merges landed, all three closed, zero manual git); AC-14
   (non-orphaned C refused naming BOTH A and B + full sequence;
   after completing A and B, C proceeds); the infra-error retention
   subtest (self-orphan determination error ⇒ refusal retained,
   nothing mutated); AC-15 both halves (symlink-escaped composed
   path ⇒ containment refusal, out-of-tree target SURVIVES
   untouched; option-like path ⇒ refused before any git spawn,
   pinned beside the existing `rejectOptionLike` tests; in-tree
   removal behavior-identical) beside the existing `WorktreeAdd`
   containment tests.

**Verification**

- [ ] `go test ./internal/complete/... ./internal/lifecycle/... ./internal/gitutil/...` passes; `golangci-lint run ./...` clean
- [ ] AC-13 RED on today's `main` (mutual refusal); full three-bead sequence converges with no manual git commands
- [ ] AC-14 RED on today's `main` (only `orphans[0]` named); refusal carries ALL siblings + the full recovery sequence
- [ ] Infra-error subtest: determination failure retains the refusal with zero mutation
- [ ] AC-15(i): out-of-tree symlink target untouched after the refusal; AC-15(ii): option-like path never reaches git; in-tree removal byte-identical
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-13 — multi-orphan convergence (N=3), WARN-demotion, monotone sequence
- [ ] AC-14 — all-orphans refusal with the full recovery sequence
- [ ] AC-15 — WorktreeRemoveForce containment + option-like guard, both halves
- [ ] AC-16 (citation leg) — the step-1.6 preflight cites ADR-0041 §2(i)

**Domain:** workflow (primary — `internal/complete` preflight,
`internal/lifecycle/orphans.go`) + execution (`internal/gitutil`
wrapper gate) — per the spec's Impacted Domains assignments.

**Depends on**
Bead 1 — an R8 merge-order pin, not a code-state edge: this bead's
step-1.6 preflight cites ADR-0041 §2(i), clause numbering that
exists only after Bead 1's §2 amendment lands (see the preamble
edge rationale). Shared-file adjacency with Bead 2 (`complete.go`)
stays disjoint hunks. (bd edges wired from
`work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly one owning bead; AC-16 is the one
conscious composite (the R8 amendment spans two beads by design —
§2 text+anchors owned by Bead 1, §4 text+anchors landing in Bead 3,
with per-bead citation legs in Beads 2 and 4 — mirroring spec 120's
AC-22 composite precedent).

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (auto-open argv + templated title + escaped URL) | Bead 3 Steps 1–3, 7 | Bead 3 verification: seam argv pin + harness gh-shim recording |
| AC-2 (idempotent adoption, head/base pin) | Bead 3 Step 3 | Bead 3 verification: adoption subtests incl. other-base never merged |
| AC-3 (gh-absent byte-identical degrade) | Bead 3 Steps 2, 5 | Bead 3 verification: no-gh fixture, exit 0, state byte-identical |
| AC-4 (opt-in merge-commit auto-merge; both finding kinds clear on refreshed refs) | Bead 3 Step 4 (consuming Bead 1 Step 5 — the R2(c) classifier; `finalize_branch` clears via existing SHA-ancestry) | Bead 3 verification: real bare-origin fixture, `ScanIntegrityFindings` empty of both kinds |
| AC-5 (default-false boundary, differential pair) | Bead 3 Step 4 | Bead 3 verification: AC-4/AC-5 pair |
| AC-6 (per-leg fault matrix, exit 0, reconcile, forward-safe records) | Bead 3 Steps 5, 7 | Bead 3 verification: AC-6 matrix |
| AC-7 (config round-trip + inert warning) | Bead 3 Step 1 | Bead 3 verification: round-trip + rg proof |
| AC-8 (squash probe routing) | Bead 1 Steps 1, 3 | Bead 1 verification: AC-8 fixture (RED today) |
| AC-9 (negative squash-detection, both consumers/polarities) | Bead 1 Steps 1, 3–4 | Bead 1 verification: AC-9 subtests |
| AC-10 (revert/reapply net-effect, both polarities) | Bead 2 Steps 2, 6 | Bead 2 verification: AC-10 fixtures (ii passes today — stated deviation) |
| AC-11 (never-subject-only spoof rejection) | Bead 2 Steps 1, 6 | Bead 2 verification: AC-11 spoof fixture (RED today) |
| AC-12 (regression floor, per-datum sufficiency) | Bead 2 Step 5 | Bead 2 verification: 119 fixtures unchanged + datum triple |
| AC-13 (multi-orphan convergence N=3) | Bead 4 Steps 1, 3, 5 | Bead 4 verification: three-bead sequence (RED today) |
| AC-14 (all-orphans refusal + full sequence) | Bead 4 Steps 2, 5 | Bead 4 verification: AC-14 fixture (RED today) |
| AC-15 (WorktreeRemoveForce both halves) | Bead 4 Step 4–5 | Bead 4 verification: AC-15 fixtures |
| AC-16 (ADR-0041 amendment anchors + citations + divergence touchpoint) | COMPOSITE — Bead 1 Step 2 (§2 text + predicate citation), Bead 3 Step 6 (§4 text + seam citation), Beads 2/4 citation legs (landed.go + binding sites; step-1.6 preflight) | Bead 1 + Bead 3 anchor tests; Beads 2/4 citation checks; each bead's own zero-override `mindspec complete` exercising the divergence gate |
| AC-17 (one exported predicate, anti-drift identity pin) | Bead 1 Steps 3–4, 6 | Bead 1 verification: seam-identity test |
| AC-18 (no-datum refusal + attested-restore forward exit + marker) | Bead 2 Steps 3, 6 | Bead 2 verification: AC-18 execute-and-converge fixture (RED today) |
| AC-19 (net-effect polarity + per-consumer ancestry split) | Bead 1 Steps 1, 3–4, 7 | Bead 1 verification: AC-19 four halves (i/ii pass today — stated deviation) |
| AC-20 (hostile input both directions at the gh seam) | Bead 3 Steps 2, 7 | Bead 3 verification: termsafe outputs + zero-argv-on-malformed-ID |
| AC-21 (landed-then-error reconcile) | Bead 3 Steps 5, 7 | Bead 3 verification: both polarities + harness shape |
| AC-22 (binding-write kill test, both producer legs) | Bead 2 Steps 4, 6 | Bead 2 verification: AC-22 kill tests, KILL-TESTED recorded |

R8's "amendment lands with the first citing code" is carried by the
Bead 1/Bead 3 split above; R9's classifications are carried per-bead
(AC-6's documented-forward-safe records in Bead 3, AC-22's
kill-tested record in Bead 2, AC-17's anti-drift pin in Bead 1).
R2(c)'s classifier leg is produced by Bead 1 Step 5 and consumed by
Bead 3's AC-4; the spec's delivery-housekeeping item (closing
zty3/blp6/yqdf/h4n5) is orchestrator kickoff work, not a bead.
