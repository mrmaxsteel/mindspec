---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 129-readiness-tracker-honesty: Readiness and tracker honesty — plan-bead selection, bd row caps, MF-2/MF-3 truth, scoped overrides (spec-128 follow-on, part 1 of 2)

*Citation scope: every code citation in this document names a function, symbol, file, or template, and was verified against `main` at `e2325e36` (v0.15.0) on 2026-10-18 in the spec-129 worktree. bd behaviour was verified against bd 1.1.0 (`8e4e59d39`) in a throwaway database. Round 1 of the spec-gate panel (9/9 REQUEST_CHANGES) corrected several draft claims; each correction is recorded in § Background under "Corrections".*

## Goal

After this spec, the verbs that decide *what work is ready* tell the truth about the tracker and about git:

- (a) `mindspec next --spec <id>` makes these guarantees (GH #255, both reported defects):
  - It selects every bd-ready **plan bead** of the spec: a bead with ID `<epic>.<N>` whose lineage, as `readiness` resolves it today, is the spec's epic. A missing parent edge does not matter, and the spec does not depend on how a bd version interprets `--parent`.
  - It names every other bd-ready bead linked to the spec, with the true reason it is not selectable.
  - It refuses a selector (positional ID or `--pick`) that it cannot honour, instead of ignoring it.
  - On an empty result it prints the spec's derived lifecycle state, so no caller can mistake "nothing ready" for "done".
- (b) no bd `ready`/`list` call the binary makes a decision on can be truncated by a bd row cap, and a truncation bd announces is an error, never a silent short read;
- (c) `mindspec next` claims a bead only if it is a plan bead of its lineage spec. It dispatches every claim into that lineage spec, and never cuts from `HEAD`. It refuses before any mutation when a scoped or resolved spec disagrees with the lineage spec;
- (d) MF-3 holds a closed dependency to the landed-merge test only when the dependency belongs to the same spec by a tracker fact; it finds a correctly named landing that is reachable from the spec-branch tip but off its first-parent chain (GH #239); a stale or non-matching panel registration never vetoes a correct corroboration, in `FindLandedMerge` or in `reattest`; and it prints what is actually true for a contradiction or a revert. GH #232 is closed on its real mechanism (Background D.5), not by a remote-aware feature;
- (e) the MF-2 token resolver recognises the same claim forms on both sides, bead text and spec text;
- (f) `--allow-not-ready` is scoped to named failing signals, needs an evidence file on every claim, and `mindspec bead ready-check` reads a validated override marker back, so the override log can be audited against the currently failing set and the override rule has one home (the binary, not `ms-bead-impl`'s prose);
- (g) ADR-0045 ("The binary owns sequence and rules; skills are harness adapters") lands, with its conflicts with ADR-0040 and ADR-0044 stated, so specs 130–133 can cite it.

## Background

### Provenance and scope arithmetic

Spec 128 (`128-lifecycle-verb-state-honesty`, merged 2026-10-18, PR #257) deferred, by name, a "spec-129 follow-on": GH #255 (readiness-truth face), #239, #232, #234, #244, #250. A consolidated diagnosis was reproduced at `e2325e36`; this spec re-verified every load-bearing claim, and the round-1 panel corrected the rest.

The cluster is split as follows. Every inherited issue has one named home:

- **Spec 129 (this spec):** six code requirements, R1–R6 (#255, the row caps, the claim/dispatch spec-consistency guard, #239 and #232, MF-2 symmetry, the override), estimated at **7–10 beads** at plan time. R7 (ADR-0045) is a document that lands with this spec's commit. The ADR-0041 "Amendment (Spec 129)" is a document that lands with the R4 and R6 beads.
- **Spec 129b (part 2):** five deliverables:
  - G (#234);
  - H (#244);
  - #250 (the same family as #244: a multi-step terminal mutation with no intermediate stop);
  - I (the dead `merge_strategy` key);
  - phase-lineage unification (removing `FindEpicForBead`'s title fallback and tightening `beadIndex`).
- **Spec 130:** the S0 rule table (ADR-0045 Implementation order step 1), #233's review-to-landing binding (see D.5), and the rest of #251. #251 is the work that reached the spec tip inside another bead's branch with no merge naming it. R4.2 covers only the sub-shape described there.

Relief valve: if the plan gate judges 129 over-compressed, R6 (override UX) splits out as 129c. R6 is coupled to R1–R5 only through the readiness report. R4 does not split cleanly, because R4.1 shares the plan-bead helper with R1.1.

### Settled facts inherited from spec 128 (binding)

1. **The ready set is ONE construction on both paths.** `next.BuildReadySet` (`internal/next/beads.go`) is shared by the claim path (`cmd/mindspec/next.go` `RunE`) and `runEmitOnly`; `--pick N` names the same bead on both; an explicit `--spec` whose epic cannot be resolved REFUSES via `scopeResolutionRefusal` and never widens to global `bd ready` (128 R6.7/AC-15). A non-empty scoped set that selection reduces to empty is a loud error naming every excluded ID (`emptyAfterFilterError`, 128 R6.3(c)/AC-10b). **129 keeps both mechanisms and changes the predicate they report on**, from "title references the spec" to "is a plan bead" (R1.1). The spec-128 tests that pin the title predicate are rewritten, not deleted (R1.7).
2. **`mindspec next` adopts ADR-0041's preflight leg for readiness** (the fourth-verb clause): `next.GateReadiness` runs after selection and before any mutation; `--allow-not-ready` records a durable marker via `next.RecordReadinessOverride` BEFORE `ClaimBead`, fail-closed; `next.RollbackReadinessOverride` deletes it on claim failure. R6 amends that clause (ADR Touchpoints).
3. **`internal/validate` is an ADR-0030 enforcement package** (`internal/lint/boundary_test.go` `enforcementPkgs`): `internal/validate/readiness` may not import `os/exec` or `internal/gitutil`. It reaches git through `internal/lifecycle` and bd through `internal/bead`. It cannot import `internal/next`, because `internal/next/ready_gate.go` imports it.
4. **Spec 125's exact-equality corroboration rule** (ADR-0041 § Amendment (Spec 125)): a landed merge is a subject-nominated candidate whose second parent is corroborated exactly. Ancestor-tolerant confirmation is the removed misattribution vector and stays removed.

### The live evidence, verified 2026-10-18

**A. Spec-scoped `next` drops a ready bead (#255).** `BuildReadySet` with a spec flag runs `QueryReadyForEpic` → `bd ready --parent <epic> --json`, then keeps only rows whose title contains the spec ID (`containsSpecID`). bd's `--parent` means "descendants". In bd 1.1.0, a hierarchical ID `<epic>.<N>` counts as a descendant **even when its parent edge has been removed**. This was verified: a child created with `--parent`, then detached with `bd dep remove`, still appears in `bd ready --parent <epic>` with `parent` absent. Three different answers about one bead therefore come from three different predicates:
- `bd ready` uses no spec predicate.
- `next` uses descendant AND title.
- `ready-check` uses `phase.FindEpicForBead` lineage. That function reads a direct dependency of issue_type epic, then falls back to a `[NNN]` number parsed from the **title**, and never reads `metadata.spec_id`.

`readiness.EvaluateReadiness` additionally needs `beadIndex` (a trailing `.<N>`) to map MF-1 onto plan section `## Bead N`. So a bead can be ready-check-PASS (title-fallback lineage plus a dotted ID) and still be absent from the scoped set. That is #255's reported shape: "`bd dep tree` reports it with zero dependencies", so there is no parent edge.

Round 2 reproduced the shape against a scratch bd 1.1.0 database:
- **Setup.** An epic `pr-1rs` carries `spec_num: 129`. Child `pr-1rs.4`, titled `[129-probe] …`, was created with `--parent`. Its edge was then removed with `bd dep remove`, after which `bd dep tree` shows no dependencies.
- **Lineage.** The real `phase.FindEpicForBead("pr-1rs.4")` returns `epic=pr-1rs spec=129-probe`, through the title fallback. A sibling `pr-1rs.5` with a neutral title returns `ErrNoEpicLineage`.
- **Edge-based exclusion.** bd 1.1.0 still lists both under `bd ready --parent pr-1rs` because of the ID prefix. A bd that reads `--parent` from edges only, or a title that names `[129]` but not the full spec ID (so `containsSpecID` misses it), drops the reporter's bead.

The fix therefore must not rest on `--parent` or on the title substring (R1.1).

On the claim path, the `len(items) == 0` branch in `RunE` runs BEFORE the positional/`--pick` selector is consulted: `mindspec next <id> --spec X` and `--pick 3` on an empty scoped set print "No ready work found." and exit 0, with the selector ignored. On `--emit-only` the same emptiness exits 1. A positional there goes to `next.FetchBeadByID` (pinned 128 R6.1 preview semantics). `ReadySet.ExclusionNotice` renders a count without IDs. The empty-set text is the same sentence whether every bead is closed or every open bead is blocked. The ambiguity menu at `cmd/mindspec/next.go` still prints `mindspec next --spec=<number>` after listing spec IDs, and `--spec=1` is rejected (#239 § Minor; still live).

**B. bd row caps (bd 1.1.0, measured).**
- `bd ready --json` caps at 100 rows. It prints "Showing 100 of 106 ready issues. Use --limit 0 for all, or --limit N to raise the cap." on **stderr** at exit 0. With an explicit `-n N` it prints the same notice.
- `bd list --json` is **not** capped by the documented default of 50. A 61-bead scratch database returned 61 rows. An explicit `--limit 50` truncates to 50 rows **with no stderr notice**.
- `bead.tracedOutput` (`internal/bead/bdcli.go`) runs `cmd.Output()`, so a successful truncated call discards the notice.

The `ready` sites are a live defect today: this repository had 106 ready beads, so unscoped `next` was truncated. The `list` sites are defence in depth, guarding a default that bd's `--json` path does not apply today.

The call sites, found by grep of every `"ready"`/`"list"` bd argv:
- Uncapped: `next.QueryReady`, `next.QueryReadyForEpic`, `next.ResolveActiveBead`, `lifecycle`'s `listClosedBeadsFn` (`orphans.go`) and `listOpenBeadsFn` (`stale_open.go`), `bead/hygiene.go`'s two `ListJSON("--status=open")` calls, and `complete`'s post-close `runBDFn("ready", "--parent", …)`.
- Already passing `-n 0`: `phase.Cache`'s three `listJSONFn` calls, `approve/adopt_lattice.go`'s `adoptListBDFn("list", …, "-n", "0", …)` (outside any waist), and `approve/plan.go`'s `queryExistingChildren`.
- Not a decision read: `internal/doctor/beads.go`'s schema-drift probe (`bd list --json`, rows unread), `internal/redact/redact.go`'s `"list"` map key (not a bd call), and the harness sandbox calls `sandbox.runBD("list", …)` in `internal/harness/asserts.go` and `internal/harness/scenario_spec_lifecycle.go`. Those read scratch databases far below any cap.

**C. Spec linkage data in the tracker.**
- `approve/plan.go`'s bead `bd create` passes `--parent <epic>` and `--metadata {"spec_id":…,"file_paths":…}`. `approve/spec.go`'s epic `bd create` writes `spec_num`, `spec_title` and `mindspec_phase`, with no `spec_id`.
- Verified in bd 1.1.0: `bd ready`/`bd list --metadata-field spec_id=<S>` filter on the nested metadata. The global `bd ready` returns the open **epic row itself**, and so would a metadata-field query once an epic carries `spec_id`.
- bd's native `--spec-id` IS filterable by `bd list --spec` (a **prefix** match: `--spec 12` returned a bead whose spec_id is `129-probe`) and IS exposed by `bd show --json`. `bd ready` has no spec filter.
- `bd ready --json` rows carry `parent` and `metadata` as **omit-when-empty** fields. A row with a parent carries `parent`; a row with metadata carries `metadata`.
- `bd show --json` dependency entries carry each dependency's `issue_type`, `status`, `dependency_type` and `metadata`. Their `parent` key is **null even when the dependency has a parent**: `pr-1rs.1`, parent `pr-1rs`, shows `parent: None` in its dependent's entry. That null is not evidence of "no parent".

**D. MF-3 false negatives (#239, #232).**
1. *Over-broad dependency set.* `readiness.fetchBeadRecordReal` collects every `blocks` edge, and `evaluateMF3` holds every closed one to `findLandedMergeFn(root, specBranch, dep.ID)` with no membership check. A closed plain task or another spec's bead can never have a `bead/<id>` landing on THIS spec branch, so it fails by construction. REPRODUCED.
2. *First-parent-only candidate scan.* `lifecycle.FindLandedMerge` and `lifecycle.ReattestLandedMerge` both nominate from `firstParentMergesFn` = `gitutil.FirstParentMerges` (`git log --first-parent --merges`). #239's topology works like this: the bead lands on the spec branch, the spec branch's content then reaches `main` through a sibling branch's PR merge, and the spec branch is moved to that history. This leaves the landing reachable only through a second parent, and both functions find no candidate. REPRODUCED (real git, scratch).

   The round-1 panel showed the draft's fix, a full-history scan, to be unsound. It makes a refresh merge `Merge branch 'spec/S' into bead/X` a second owned candidate, and the FIX-2b ambiguity rule then refuses a landing that today's scan identifies. A scratch-clone probe (real git, with an emulated seam over `firstParentMergesFn`) reproduced this. It showed that a first-parent-primary scan with a zero-candidate fallback identifies the refresh, #239 and refresh-then-first-parent shapes.

Round 2 found the limit of that fallback: `gitutil.MergeInto` writes `Merge bead/<id>`, which **names no target**, and git cannot tell where such a merge was made. A reviewer probe built this shape:
- X's round-1 tip `A` is merged into `bead/Y` with the subject `Merge bead/X`.
- Y lands, and X's later round never lands.
- Registrations are `{A, B}`.

Under the round-1 text, the fallback admitted that merge and the panel set confirmed it on `A`, which gave a false MF-3 PASS. R4.2 now requires the fallback path to be confirmed by a datum other than the panel.
3. *Stale panel corroboration.* `lifecycle.reviewedHeadSHAForBead` returns the FIRST non-empty `reviewed_head_sha` in `panel.ForBead(panel.Scan(roots…))` order (sorted by directory). `FindLandedMerge` returns an exact-equality contradiction for that SHA BEFORE the surviving-tip and landed-binding legs run. `ReattestLandedMerge` refuses with `ReattestStatePanelContradiction` on the same value. REPRODUCED.
4. *Misleading message.* `ErrLandedMergeNoCandidate` wraps `ErrLandedMergeNotFound`, and so do the contradiction and revert returns. `evaluateMF3` tests only `*LandedMergeNoEvidence` and the broad sentinel, so a contradiction or a revert prints the zero-candidate sentence "no landed merge into … names bead/X".
5. *#232's real mechanism.* #232's failing dependency is bead 1 of the **same** spec, merged by `mindspec complete` after one panel round, so cause 1 cannot apply, and cause 3 (two rounds) is not its shape. Its companion #233 shows the mechanism: `mindspec complete` lands a commit that no panel can have reviewed, so the merge's second parent `D` differs from the single registration's `reviewed_head_sha = B`. From there:
   - `FindLandedMerge` returns the panel contradiction before the tip and binding legs run.
   - `evaluateMF3` prints it as the zero-candidate sentence (cause 4).
   - `reattest`, MF-3's own recovery for this state, refuses on the same panel datum.

   No `origin/` or remote-tracking ref is read anywhere on this path (not in `internal/validate/readiness`, `lifecycle.FindLandedMerge`/`reviewedHeadSHAForBead`, `gitutil.FirstParentMerges`, or `panel.Scan`). The "remote" reading in #232 is a misattribution.

**E. MF-2 token parser.** `readiness.claimTokenRe` is `\b(R|AC-)([0-9]+)([a-z])?\b`.
- `extractLineClaims` (bead side) also recognises a directly following parenthetical (`R9(a)` → exact key `R9a`). `extractSpecTokens` (spec side) runs only `claimTokenRe`.
- Probed at the reviewed commit: bead `R9(a)` vs spec `R9(a)` FAILS; bead `AC-9` vs spec `AC9` FAILS. Bead `AC9` vs spec `AC-9` PASSES only vacuously, because `AC9` is never harvested.
- Dotted sub-numbers (`R6.3`) resolve at base granularity on both sides.

**F. `--allow-not-ready` is all-or-nothing in the binary, and the coverage rule lives in a skill.** `next.GateReadiness(root, beadID, allowNotReady bool)` passes every failing signal when the flag is set. `RecordReadinessOverride` writes `mindspec_readiness_override: {signals, overridden}`. `cmd/mindspec/bead_ready.go` never reads that marker.

The comparison of recorded signals against the currently failing set already exists, but **at L4**: `plugins/mindspec/skills/ms-bead-impl/SKILL.md`'s dispatch-ingress step tells the agent to `bd show` the bead, read `signals`, and dispatch only if every failing signal is recorded. `internal/setup/readiness_skill_pin_test.go` (`TestMsBeadImplSkill_OverrideCoverageCheckShipped`, `…MutationProbe`) pins that text. This is a rule with no gate: the "rules in prose are advice" case that ADR-0045 names.

The tracked `.beads/issues.jsonl` already holds legacy `{signals, overridden}` markers with no evidence.

**J. Governance.** `.mindspec/adr/` ends at `ADR-0044`, so `0045` is free. `.mindspec/policy/` is claimed by no `OWNERSHIP.yaml`. The claimed roots are:
- core, workflow, execution and context-system: their `internal/**` packages;
- workflow also: `cmd/**`, `plugins/mindspec/**`, `.claude/{skills,workflows,agents}/**`, `.agents/skills/**`, `scripts/bd-jsonl-merge-driver.sh`, `.golangci.yml` and `.github/workflows/ci.yml`;
- core: `.gitignore`;
- each manifest that lists itself.

A rule table there needs an ownership claim first.

### Corrections (draft claims the round-1 panel or re-verification proved wrong)

- *bd's spec-id field "not filterable, not in `bd show --json`"* — wrong: `bd list --spec` filters it by prefix, and `bd show --json` exposes it. It is still not adopted (R3).
- *"bd `list` defaults to 50; an epic with more than 50 closed children can miss orphans"* — wrong for `--json`, which returns every row. An explicit `--limit` truncates silently.
- *"`bd ready` rows carry no parent or metadata"* (a reviewer claim) — imprecise: both are omit-when-empty fields and are present when set. The near-miss scan below relies on this. R2.4 pins it at the version floor.
- *"`DerivePhaseWithStatus` gives the phase R1.4 prints"* — wrong: it returns the stored `mindspec_phase` whenever one exists and only warns on disagreement. On the normal path nothing writes `review` (only `phase.SanctionedDerivedSync`, via `release`, and `mindspec repair phase` can write a child-derived value). R1.4 uses `phase.DerivePhaseDetail`, which gives the stored and child-derived phases separately.
- *"#232 fits cause 1 or cause 3"* — wrong; see D.5.
- *"`ExactSecondParentMerges` is reattest's nominator"* — wrong: it is used only by the executor's merge-time binding write (`internal/executor/mindspec_executor.go`). `ReattestLandedMerge` scans with `firstParentMergesFn`.
- *ADR-0041 item (c) "the first registered `reviewed_head_sha`"* — misquote: item (c) reads "a registered panel's `reviewed_head_sha` equal to an exact match's second parent". The first-registration veto is code behaviour, not ADR text.
- *"nothing compares the recorded signals with the failing set"* — wrong: `ms-bead-impl` does it in prose (F).
- *"the evidence is stored in bd metadata, never committed"* — wrong: bd metadata is exported into the git-tracked `.beads/issues.jsonl` (ADR-0025 §3; existing override markers are in the tracked file at `7a490064`).
- *"`registerLandedPanel` does not exist"* (a reviewer claim) — it exists in `internal/validate/readiness/fixtures.go`. It writes one registration under a fixed slug, so AC-11 extends it with a slug parameter.
- *(round 1's own text) "Option N's set is exactly the set readiness can evaluate", "both #255 defects fixed"* — wrong: #255's dotted, edge-less, title-lineage bead stayed unclaimable. R1.1 now selects by lineage (OQ-9).
- *(round 1's own text) "the fallback is source- and target-pinned"* — wrong for `Merge bead/<id>`, which names no target. R4.2 now states the real defence: no panel-only confirmation on the fallback path.
- *(round 1's own text) "an override for one failure never covers a different failure under the same signal"* — only true for the first failure. The fingerprint now covers the whole failure set (R6.2).
- *ADR-0045 Context, "content-sha provenance"* — `previouslyShippedSkills` (`internal/setup/skills.go`) matches byte-exact snapshots. Corrected in the ADR.

## Impacted Domains

- **workflow**:
  - `internal/next/**`: R1 plan-bead selection, near-misses, selector refusal, derived-state emptiness, the menu fix; R1.8's lineage-equality guard; R6's scoped flag and fresh-evidence rule.
  - `internal/lifecycle/**`: R4.2's reachable-landing fallback for the MF-3 and `reattest` callers only; R4.3's set semantics in `FindLandedMerge` and `ReattestLandedMerge`; R2's `-n 0` at the orphan and stale-open seams.
  - `internal/validate/**` (`readiness`): R4.1 dependency membership; R4.4 message arms; R5 form symmetry.
  - `internal/approve/**`: R3.2 pin test only, with no production change.
  - `internal/complete/**`: R2's `-n 0` on the post-close ready lookup.
  - `internal/lint/**`: R2.3's call-site row-cap lint.
  - `internal/instruct/**`: `templates/implement.md`, R1.4's L3 rule.
  - `internal/setup/**`: rewrite of `readiness_skill_pin_test.go`'s override-coverage pins, under the skill-refresh protocol.
  - `cmd/mindspec/**`: `next.go` flag shape, emptiness output and menu text; `bead_ready.go` marker read-back; `bead_clarify.go` text.
  - `plugins/mindspec/skills/ms-bead-impl/SKILL.md`: the override-coverage prose is **removed** and replaced by "run `mindspec bead ready-check`; proceed on exit 0". This deletes a rule from a skill, as ADR-0045 §4 requires. No rule is added to any skill.
  - `.claude/skills/ms-bead-impl/SKILL.md` (installed copy): it is a stale July copy with no override prose (0 matches), so it is **refreshed** from the plugin source, not edited.
  - `internal/setup/historical_skills/**` (fenced): no existing snapshot is edited. Only the protocol's new snapshot of the replaced plugin text is added.
  - `plugins/mindspec/skills/ms-bead-cycle/SKILL.md`: the bare-form wording, if the plan's grep finds one (R6.5).
  - `internal/bead/clarify.go` and `internal/bead/bdcli.go` doc comments (not edited): the operator-reset analogy and `RemoveMetadataKeys`'s claim-failure note name the flag, not its bare form, and stay true under R6.
- **execution**:
  - `internal/bead/**`: R2's row-set waist (`ListJSON` and a new `ReadyJSON`); the shared plan-bead ID helper; R6's marker schema v2 constants and its validator in `readiness.go`.
  - `internal/gitutil/**`: R4.2's topo-ordered reachable-merge primitive beside `FirstParentMerges`.
- **core**: NOT impacted. `internal/phase` is consumed, not changed: `DerivePhaseDetail`, `Cache`'s child list, `IsLifecycleIssueType` and `FindEpicForBead` are read as they are. `phase.Cache`'s three `bead.ListJSON(…, "-n", "0")` calls keep working unchanged, because the R2.2 waist tolerates an explicit `-n 0`. The rejected Option W would have changed `FindEpicForBead`; see OQ-9.

Documentation surfaces touched are not Impacted-Domains entries. They are claimed by no OWNERSHIP.yaml and are governed by the ADR-0031 doc-sync gate. `.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md` is new and lands with this spec's commit, together with the "Amended by ADR-0045" back-pointers in ADR-0040 §1 and ADR-0044. ADR-0041's new § "Amendment (Spec 129)" lands with the R4 and R6 beads. R6.5's surfaces are `.mindspec/core/USAGE.md`, `.mindspec/core/WORKFLOW-STATE-MACHINE.md`, `.mindspec/domains/workflow/{architecture,interfaces,runbook}.md`, and `.mindspec/domains/execution/interfaces.md` (the marker schema row).

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — **AMENDED** by a new § "Amendment (Spec 129)". It is appended (not an in-place rewrite), follows the Spec-125 precedent, is cross-referenced from item (c), and is finalized by the bead that lands the citing code. AC-18 pins it. It covers three things:
  - **(i) Reachable-landing fallback (R4.2).** The Spec-125 text binds landed-ness to "a two-parent first-parent merge", and §2(ii) reads revert-awareness over "the first-parent chain SINCE the candidate merge". The amendment admits a fallback candidate for the MF-3 read and for `reattest` only. A fallback candidate is a two-parent merge reachable from the tip whose subject names `bead/<id>` as the merged SOURCE.
    - **Target.** The target is checkable only for the explicit `… into <target>` forms; those are admitted only when `<target>` is the spec branch. `gitutil.MergeInto`'s `Merge bead/<id>` encodes no target.
    - **Defence for the undirected form.** It is not subject pinning. It is reachability from the tip, plus an exact (b) surviving-tip or (d) landed-binding corroboration, plus the net-effect test at the tip. On the fallback path **a panel registration alone never confirms**.
    - **Residual, stated.** A `Merge bead/<id>` made into another branch whose history later reaches this tip is identified when (b) or (d) corroborates it, because its content is then present at the tip.
    - **Revert-awareness.** It is the same net-effect test at the tip, of which the candidate is an ancestor.
    - **CLOSE decisions.** `complete`'s merged-unclosed reconcile keeps the first-parent scan.
  - **(ii) Non-equal panel datum (R4.3).** Item (c) already reads "a registered panel's `reviewed_head_sha` equal to an exact match's second parent". The amendment makes the non-equal case explicit for `FindLandedMerge` and `reattest`: a registration whose SHA does not equal the second parent is NON-CORROBORATING. It neither confirms nor contradicts.
    - This reaches the CLOSE paths, and it is safe. A stale SHA used to contradict. Now an exact tip, binding or panel match confirms, which is the evidence class Spec 125 already admits; otherwise the result is still a refusal (`*LandedMergeNoEvidence`), never a close.
    - A contradictory landed binding still vetoes automatic identification. Only an explicit `reattest` may replace it, with the scan-derived identity, after its own checks pass, and with the prior pair audited.
  - **(iii) Fourth-verb clause (R6).** `--allow-not-ready` becomes signal-scoped and needs an evidence file on every claim that records an override. It refuses when a non-named signal fails, and coverage is bound to a fingerprint of the signal's complete failure set. An existing marker never authorizes a claim.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED. Every new refusal ends with a `recovery:` line:
  - R1.3's selector refusals use `recovery: mindspec next --spec <spec-id>`.
  - R1.2's near-misses use `bd update <id> --parent <epic-id>` only for N1: a plan-form bead with no parent whose lineage does not resolve. That is the one recovery shown to yield a claimable bead. Otherwise they get the inspection-first `bd show <id> --json` (Spec-127 amendment item 2; never a reparent over a parent the agent did not name).
  - R1.4 uses `bd blocked --parent <epic-id>`, which exists in bd 1.1.0.
  - R6's scoped recovery carries `--evidence <path-to-justification>`, a literal placeholder. It is marked non-mechanical, because the operator authors the evidence.

  New emitted commands are registered where the `internal/guard` recovery-convention lint requires. HC-4: an empty-set `next` with no selector and no BLOCKING near-miss mutates nothing and exits 0. The one exception is R1.4's in-progress arm, which keeps today's worktree recovery.
- [ADR-0042](../../adr/ADR-0042-render-derivation-provenance.md) — APPLIED. IDs render via `idrender.Bead`/`idrender.Spec`; bd stderr (R2.3) and the evidence path (R6) via `termsafe.Escape`. **No dynamic operand appears in command-shaped text.** Recovery lines carry only idrender-gated IDs, closed-set signal IDs, and literal placeholders, following the `internal/next/select.go` final-review R5/G4 rule. The operator's evidence path is never interpolated into a recovery line.
- [ADR-0025](../../adr/ADR-0025-jsonl-as-build-artifact.md) — RESPECTED, with the consequence stated: the R6 marker is bd metadata, so it IS exported into the git-tracked `.beads/issues.jsonl` and pushed. For that reason the marker stores the evidence file's path, sha256 and byte count, **never its text**. The rendered path is the only operator-supplied string that reaches git, so R6.5's guidance recommends a repository-relative path.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED. `internal/validate/readiness` stays inside the boundary: R4.1's membership reads go through `internal/bead`, and its landed-merge read through `internal/lifecycle`. R4.2's primitive lands in `internal/gitutil`. The R2 waist stays in `internal/bead`.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — RESPECTED. `internal/panel` is untouched. Agent-writable data (panel.json, bd metadata) stays inside §8's anti-footgun boundary. No step here claims to authenticate it; R6 validates its shape only.
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — APPLIED, and amended by ADR-0045:
  - R1.4's rule sits at L3 (`templates/implement.md`).
  - R6 ratchets `ms-bead-impl`'s override-coverage rule from L4 into L1 (`bead ready-check`) and deletes the L4 copy.
  - ADR-0040 §1 receives an "Amended by ADR-0045" back-pointer in this commit.
- [ADR-0044](../../adr/ADR-0044-panel-review-conduct.md) — AMENDED by ADR-0045 (placement of conduct text only: the role-local skill fragments move into the binary-owned `panel brief` template when it ships). It receives a back-pointer in this commit. Nothing in 129 moves conduct text.
- [ADR-0032](../../adr/ADR-0032-adr-semantic-gates.md) — APPLIED. ADR-0045 lands `Accepted` as the operator's decided direction, with a fence: a bead may cite it as coverage only for a rule that the bead itself moves out of a skill (R6.4). See R7.
- [ADR-0023](../../adr/ADR-0023.md) — APPLIED: R1.4's completion signal comes from bead statuses (the child-derived phase), never from the emptiness of a ready query.
- [ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md) — NEW, lands with this spec: R7.

## Requirements

Diagnostic strings are first-class outputs. ACs pin what a message says and what state it claims, and are red-on-revert; an AC never only checks that a string is non-empty. No AC executes an emitted recovery string. A hinted command's outcome is asserted by invoking that command's own code path on the fixture.

### R1 — Spec-scoped selection honesty (#255)

1. **One claim predicate: the plan bead.** A bead is a **plan bead of spec S** (epic E) iff two things hold:
   - (a) its ID has the plan-bead form `<E>.<N>` (N ≥ 1, one segment). This is one exported helper in `internal/bead` (working name `bead.PlanBeadIndex(epicID, id) (n int, ok bool)`), shared with R1.8 and R4.1.
   - (b) its lineage resolves to E. If the row's `parent` is E, lineage holds by the parent edge, with no extra call. Otherwise `phase.FindEpicForBead(id)` must return E. This is the same function, unchanged, that `readiness.EvaluateReadiness` and `complete` use, so a missing parent edge with a `[NNN]`-tagged title qualifies.

   For `next --spec S`:
   - **One bd call.** The scoped set comes from ONE uncapped global `bd ready -n 0 --json` call. It does not use `bd ready --parent`, whose edge-or-prefix semantics differ by bd version (Background A), and it does not use the title substring.
   - **Members.** A row is a member iff it is not an epic, is open, and is a plan bead of S. Members keep bd's returned order, so `--pick N` parity holds by construction.
   - **Extra cost.** One `bd show` per plan-form row whose `parent` is not E. These are rare by construction.
   - **Read errors.** A real lineage read error refuses the whole command, fail-closed (`could not resolve the lineage of <id>`). It never silently drops the row.

   `issue_type` is not a membership input, because #255's own bead was a `bug`. Neither the title nor `metadata.spec_id` is a membership input. The honest limit: `readiness.beadIndex` accepts any trailing `.<N>`, so readiness also evaluates a `<other>.<N>` bead whose title-fallback lineage is S. `next` does not select it (N3). Tightening `beadIndex` is 129b's phase-lineage unification.
2. **Near-misses are named, on every scoped call.** A **near-miss** is a non-epic, non-member row of the same global set that has any of these links to S:
   - a plan-bead-form ID for E;
   - row `parent` E;
   - a title containing the spec ID;
   - row `metadata.spec_id` equal to S.

   Each is printed by ID with its true reason and one recovery line:
   - **(N1)** Plan-bead form, no `parent`, and `FindEpicForBead` returns `ErrNoEpicLineage`. Message: `bead <id> has the plan-bead form for epic <E> but no lineage (no parent edge and no spec tag in its title)`. Recovery: `recovery: bd update <id> --parent <E>`. This is the one class whose recovery is shown to make the bead claimable (AC-2).
   - **(N2)** Plan-bead form, but lineage resolves to another epic E2, or the row's parent is a bead P that the agent did not name. Message: `… but its lineage resolves to <E2>` or `… but its parent is <P>`. Recovery: `recovery: bd show <id> --json`.
   - **(N3)** Any other link. Message: `bead <id> is bd-ready and linked to spec <S> by <parent edge | title | metadata.spec_id>, but it is not a plan bead (no <E>.<N> ID with lineage <E>); mindspec next claims only plan beads`. Recovery: `recovery: bd show <id> --json`. This is non-mechanical: the honest paths are to fold the work into a plan bead or to work the bead outside `mindspec next`. The sentence is true on every `next` path, because R1.8 applies the same predicate to every claim.

   Near-misses never change selection, and they are printed whether the member set is empty or not. A near-miss is **blocking** if it is N1, N2, or an N3 whose row `parent` is E and whose type is lifecycle (`phase.IsLifecycleIssueType`). Every other near-miss is **informational**: title-only or metadata-only N3, and non-lifecycle children such as a follow-up `bug` under E. Informational near-misses print the same lines and never change the exit code.
3. **A selector `next` cannot honour is refused, not ignored.** On the claim path, with `--spec`:
   - **Empty member set, positional ID.** Exit non-zero with `no ready plan bead for spec <S>; positional <id> cannot be claimed from an empty ready set`.
   - **Empty member set, `--pick N`.** Exit non-zero with `--pick <N> is out of range: 0 items`.
   - Both of these end with `recovery: mindspec next --spec <S>`.
   - **Near-miss positional.** Whether the member set is empty or not, this refuses with its R1.2 sentence.
   - **Positional in the global ready set, but neither member nor near-miss.** This is a plan bead of another spec or an unrelated task. Refuse with `bead <id> is bd-ready but is not linked to spec <S>`, adding `(parent: <P>)` when its row carries a parent, and `recovery: mindspec next --spec <S>`.
   - **Positional absent from the global ready set.** Keep today's "not found or not ready".

   `--emit-only`'s positional bypass (`FetchBeadByID`, pinned 128 R6.1) is unchanged and keeps its disclosure line.
4. **The empty-set result carries the spec's derived state.** When the member set is empty and no selector was supplied, the output reads E's child list (uncapped, all statuses, as `phase.Cache` lists it: the list `DerivePhaseFromChildren` and `state show` use). It prints, in this order, before any other action:
   1. the lifecycle-children tally (`closed / in_progress / open`);
   2. `stored phase: <p>; child-derived phase: <q>` from `phase.DerivePhaseDetail`;
   3. the verbatim sentence "an empty ready set is not evidence the spec is complete; the derived phase above is";
   4. ONE line keyed on the **tally**. The tally is used because `DerivePhaseFromChildren` maps two different tallies to `plan`. The arms:
      - all lifecycle children closed → `recovery: mindspec impl approve <S>`;
      - an in-progress child → today's worktree-recovery branch, unchanged. This is the one empty-set arm that mutates (`EnsureWorktree`), and it runs after the lines above;
      - ≥1 open lifecycle child and none in progress → `<n> open lifecycle bead(s), none bd-ready` and `recovery: bd blocked --parent <E>`. The child-derived phase here is `implement` if any child is closed and `plan` if none is;
      - zero lifecycle children → `recovery: mindspec plan approve <S>`.

   When the child-derived phase is ahead of the stored phase, the existing forward-reconcile `recovery: mindspec repair phase <S>` is added. Exit codes:
   - **0** when there is no blocking near-miss. Informational near-misses do not count (HC-4: the read evaluation completed).
   - **Non-zero** when there is ≥1 blocking near-miss. This is the 128 R6.3(c) loud-empty class (settled fact 1), and the derived state is still printed.
   - **Non-zero** when the child read fails. The indeterminate form is printed (128 R2.3's class) with no tally.

   The same sentence is stated once in `internal/instruct/templates/implement.md` (L3).
5. **Exclusion notices name IDs.** `ReadySet.ExclusionNotice` and the near-miss list render every bead ID via `idrender.Bead`, never a bare count.
6. **The ambiguity menu prints the form it accepts.** The multi-spec menu in `cmd/mindspec/next.go` prints `mindspec next --spec=<spec-id>` (a literal placeholder under the listed IDs), never `--spec=<number>` (#239 § Minor).
7. **Spec-128 tests rewritten, not dropped.**
   - `TestBuildReadySet_ScopedFiltersByTitle_ExclusionAccounted` and `TestBuildReadySet_EmptyAfterFilter_LoudError` are rewritten over the plan-bead predicate. A non-plan descendant is excluded and named; a set with only blocking near-misses is loud.
   - `TestNextSelection_{ClaimPath,EmitOnlyPath}_PartialExclusionNoticePrinted` gain the ID assertion.
   - `exclusionReasonForName` is replaced by the near-miss classifier.
   - `containsSpecID` survives only as R1.2's title-link test.
8. **One claim predicate and one dispatch spec, on every claim path.** `readiness.EvaluateReadiness` returns the lineage epic and spec it already resolves (T, E_T). Before any mutation, `next.GateReadiness` checks three things in order:
   - (a) It refuses unless `bead.PlanBeadIndex(E_T, id)` holds: `bead <id> is not a plan bead of spec <T> (no <E_T>.<N> ID); mindspec next claims only plan beads`, with `recovery: bd show <id> --json`.
   - (b) If `--spec S` was given, or the unscoped path resolved a target spec S (`resolve.ResolveTarget`), it refuses when S ≠ T: `bead <id> resolves to spec <T> by lineage but is being claimed for spec <S>; nothing was claimed`, with `recovery: bd show <id> --json`.
   - (c) **The dispatch spec is T**, passed to `EnsureWorktree`, `DispatchBead` and recording. When the unscoped path resolved no spec, `specFlag` is empty today and `DispatchBead` cuts the bead branch from `HEAD`. Under this rule the bead branch is instead cut from `spec/<T>`, never from `HEAD`.

   For R1.1 members, (a) and (b) hold by construction, so these checks are the guard for the unscoped path and for beads with conflicting epic dependencies.

### R2 — No silently truncated bd row set

1. **Every row-consuming bd `ready`/`list` call passes `-n 0`.** These are the uncapped sites in Background B, `adopt_lattice.go`'s `adoptListBDFn` (migrated to the waist), and every call R1 adds.
2. **Through one waist, with no cap escape.** `internal/bead` helpers `ListJSON` and a new `ReadyJSON` ensure `-n 0` on every call.
   - An explicit `"-n", "0"` or `"--limit", "0"` pair from a caller is TOLERATED and not duplicated. This covers `phase.Cache`'s three calls and `approve/plan.go`'s `queryExistingChildren`, so core stays untouched.
   - Any other `-n`/`--limit` value returns an error from the waist without spawning bd. A caller that wants fewer rows slices client-side.
   - Argument-recording seam tests pin the argv.
3. **Anti-drift is mechanised, by call site.** The lint `TestBDRowCapWaist` (`internal/lint`) walks non-test Go sources. It fails on a call whose callee is `bead.RunBD`, `bead.RunBDCombined`, `exec.Command` with first argument `"bd"`, or a package-level func var initialised to one of them, when its first bd argument is the literal `"ready"` or `"list"`, it lies outside `internal/bead`, and it carries no `// rowcap-allowlisted: <reason>` marker. Dispositions:
   - `internal/doctor/beads.go` probe: allowlisted (rows unread).
   - `internal/redact/redact.go`'s `"list"`: not a call, so not matched.
   - `internal/harness` `sandbox.runBD(...)`: a method outside the callee set; scratch databases only; not migrated.
   - `approve/adopt_lattice.go`: migrated (R2.1).

   **Defence in depth for `ready`:** on success, `ReadyJSON` captures stderr separately. Because it always appends `-n 0` itself, any `Showing N of M` notice means bd ignored the request. It returns `bd truncated the ready set (<escaped notice>) although mindspec requested no cap` with an ADR-0035 recovery line. bd prints no notice for a truncated `list` (measured), so for `list` the argv pin and the lint are the guard, and the spec claims no more.
4. **Version floor.** `doctor`'s `bdVersionFloor` stays `1.0.4` if planning verifies three things at that floor: `-n 0` is unlimited, `bd ready --json` rows carry `parent` when set, and they carry `metadata` when set (R1.1 and R1.2 depend on these). Otherwise the floor is raised in that bead with the evidence.

### R3 — Spec linkage: no new datum

1. **Selection reads lineage, not a linkage key.** R1.1's membership is the plan-bead ID form plus lineage (the parent edge, or `FindEpicForBead` unchanged). `metadata.spec_id`, already written on every lifecycle bead by `approve/plan.go`, is read only as a near-miss signal (R1.2) and as one disjunct of MF-3's dependency membership (R4.1). The epic gets **no** new `spec_id` key, because that would put the epic row into every metadata-field query (verified). No labels are added. bd's native spec-id field is not adopted: `bd list --spec` matches by prefix, `bd ready` has no spec filter, and it would be a second home for one fact.
2. **Pinned.** A test over `planRunBDFn`'s recorded `bd create` argv asserts that every bead carries `--metadata` JSON with `spec_id` equal to the spec ID, passing `idvalidate.SpecID`. This is existing behaviour, now load-bearing for R4.1.
3. **No migration, no backfill.** Every bead that plan approve mints already has the plan-bead form and a parent edge.

### R4 — MF-3 tells the truth (#239, #232)

1. **Dependency membership is a status-independent tracker fact.** `evaluateMF3` holds a closed `blocks` dependency D to the landed-merge test iff D is a member of the bead's lineage spec S (epic E, which `EvaluateReadiness` already resolves). D is a member iff ANY of:
   - (i) `bead.PlanBeadIndex(E, D)` succeeds;
   - (ii) D's `metadata.spec_id` equals S, read from the dependency entry that the bead's own `bd show --json` already returns (`fetchBeadRecordReal` keeps it);
   - (iii) D's parent equals E, read by one `bd show <D> --json` through a new seam, and only when (i) and (ii) are both negative. The dependency entry's own `parent` key is null even when D has a parent (Background C). It is never read as "no parent", and (iii) never short-circuits on it.

   The title is never used. Outcomes:
   - All three negative from successful reads → non-member: MF-3 PASSES on bd-closed alone and renders `dependency <D>: closed; not a bead of spec <S> — landed-merge test not applicable`.
   - Any read or parse error → MF-3 FAIL `could not determine whether <D> belongs to spec <S>: <escaped error>`, with `recovery: bd show <D> --json`. It is never the not-applicable PASS.
   - A metadata-only or non-plan member held to the test and failing it adds `<D> is linked to spec <S> by <disjunct> but is not a plan bead; if its work never landed as bead/<D>, override deliberately` and the scoped override line (R6).
   - A non-closed dependency of either kind fails as today.
2. **Reachable-landing fallback, scoped to the MF-3 and `reattest` callers.** Candidate generation keeps the first-parent scan as primary, unchanged. A fallback scan runs only when the primary scan nominates **zero** owned two-parent candidates. It uses a new `gitutil` primitive (`git log --merges --topo-order <spec-branch>`, newest-first in topological order; `--topo-order` defines newest and oldest) and considers every merge reachable from the tip.
   - **Admitted subjects.** The fallback admits a two-parent merge only when its subject names `bead/<id>` as the merged SOURCE, in exactly one of three forms:
     - `Merge bead/<id>` — `gitutil.MergeInto`'s form, which is **undirected**: the subject names no target;
     - `Merge branch 'bead/<id>' into <spec-branch>`;
     - `Merge bead/<id> into <spec-branch>`.

     The two `into` forms are admitted only with this spec branch as `<target>`. mindspec writes `Merge <source> into <target>` only through `gitutil.MergeBranch`, for spec→main finalize, never for a bead, so the third form is defensive.
   - **Excluded:**
     - a merge INTO `bead/<id>` (a refresh from the spec branch);
     - an explicit `… into <other branch>` (positive evidence it was not a landing here);
     - any unrecognised format (the G2-2 any-token fallback in `parseMergeSubjectBeadBranch` does not apply on the fallback path).
   - **No target guarantee for the undirected form.** `Merge bead/<id>` proves nothing about where it was made. The fallback's defence for that form is reachability from the tip, plus the confirmation rule below, plus the net-effect test at the tip. The residual is stated: a `Merge bead/<id>` made into another branch whose history later reaches this tip is identified when (b) or (d) corroborates it, because its content is then at the tip.
   - **Confirmation rule on the fallback path.** A fallback candidate is confirmed ONLY by an exact (b) surviving-tip or (d) landed-binding datum. The panel set (R4.3) never confirms a fallback candidate on its own; a panel-only match returns `*LandedMergeNoEvidence`, so the operator-vouched `mindspec reattest` stays the exit. The binding is the strongest datum here, because the executor writes it only at its own spec-branch merge.
   - **Ambiguity on the fallback path.** Owned candidates whose second parents differ are resolved iff exactly one second-parent group is exactly corroborated by (b), or by a (d) binding that names a member of that group, and no (b)/(d) datum contradicts that group. Otherwise the FIX-2b `*LandedMergeNoEvidence` ambiguity is returned, as today. `reattest` applies the same rule. The primary path's FIX-2b rule is unchanged.
   - **#251 boundary.** The fallback covers only the #251 sub-shape where a prerequisite reached the tip inside another bead's branch through an admitted, short-form `Merge bead/<id>`, confirmed by (b) or (d). The long form `Merge branch 'bead/X' into bead/Y` is excluded, because it positively names a non-spec target. Work that reached the tip by branch-from, cherry-pick or rebase, with no merge naming it (#251's literal report), is still not nominable and stays with spec 130.
   - **Unchanged machinery.** Same-second-parent grouping, the corroboration legs and the oldest-anchored revert/reapply test run unchanged. The `SubsumptionConflict` rationale becomes "M is an ancestor of the tip", which is true for both scans. The `FirstParentMerges` and `ErrLandedMergeNoCandidate` doc contracts are updated with the same wording.
   - **`reattest` on a fallback candidate** writes the binding only after its own checks. Its output adds `candidate <sha> reached <spec-branch>'s tip off the first-parent chain; its subject names no target — confirm it landed this bead's work before vouching`. It also lists any registered `reviewed_head_sha` that is not an ancestor of the tip (`later reviewed work for <id> is not in <spec-branch>'s history: <sha>`) as an operator warning. The warning does not refuse.

   Callers, and what changes for each. This table covers R4.2. R4.3's set semantics live inside `FindLandedMerge` and so reach every caller below (see R4.3):

   | Caller | Change |
   |---|---|
   | MF-3 (`readiness.findLandedMergeFn`) | gains the fallback (working name `lifecycle.FindLandedMergeReachable`) |
   | `lifecycle.ReattestLandedMerge` | nominates through the same fallback, so `ready-check`'s `mindspec reattest` recovery can write the binding MF-3 asks for |
   | `lifecycle.MergedUnclosed` → `complete`'s merged-unclosed reconcile (CLOSES beads, cleans branches) and `stale_open.go` | **unchanged**, first-parent only (an ADR-0041 §2(ii) CLOSE decision) |
   | `orphan_hints.go` `findLandedMergeFn` | unchanged |
   | `approve/plan.go` `planFindLandedMergeFn` | unchanged |
   | `approve/adopt_lattice.go` `adoptFindLandedMergeFn` | unchanged |
   | `executor` binding write (`gitutil.ExactSecondParentMerges`) | unchanged: the merge was just made by `gitutil.MergeInto` and is the first-parent tip by construction |
   | `executor/merge_resumption.go` (`FirstParentMerges(HEAD)`) | unchanged |
3. **Panel-leg set semantics, in both functions.** `reviewedHeadSHAForBead` becomes a SET: every non-empty `reviewed_head_sha` across all registrations `panel.ForBead` returns. In `FindLandedMerge` and in `ReattestLandedMerge`'s datum (c):
   - the set CONFIRMS iff it contains the candidate's exact second parent;
   - a non-empty set that does not contain it is NON-CORROBORATING: it neither confirms, nor contradicts, nor pre-empts another leg;
   - in `reattest` it is recorded in the audit `corroboration` string as non-corroborating, and reattest proceeds on its other data (at minimum (a)) instead of refusing.

   Decision rule when legs disagree: an exact (b) surviving-tip or (d) binding contradiction still contradicts, naming its datum, whatever the other legs say. The panel set never vetoes. The set alone never confirms an ancestor (settled fact 4), and it never confirms a fallback candidate (R4.2).

   **The binding veto and audited repair are kept distinct.** A contradictory landed binding vetoes automatic identification in `FindLandedMerge` and `FindLandedMergeReachable`. Only an explicit `ReattestLandedMerge` may replace it, with the independently scan-derived exact identity, and only after its ownership, ambiguity, surviving-tip and revert checks pass. The prior pair is recorded in the audit. A reattest refusal writes nothing. `TestReattest_ContradictoryBindingOverwrittenAudited` and `TestReattest_StaleOlderRemergeBindingRewrittenToNewest` are preserved.

   **Reach into the CLOSE paths, and why it is safe.** The set semantics apply on the primary path too, so they reach `MergedUnclosed`: `complete`'s merged-unclosed reconcile, which CLOSES beads, and `stale_open`. They also reach `planFindLandedMergeFn`, `adoptFindLandedMergeFn` and `orphan_hints`.
   - **Before:** a stale SHA contradicted, and the result was a refusal.
   - **After:** an exact tip, binding or panel match confirms, and nothing else does.
     - An exact panel match is item (c) of Spec 125's admitted evidence. A registration set `{A, B}` with B equal to the second parent now confirms exactly as `{B}` alone did before.
     - A non-matching set alone still yields `*LandedMergeNoEvidence`. `MergedUnclosed` propagates that as an error, so the result is a refusal and never a close.

   No CLOSE decision is ever taken on evidence Spec 125 did not admit. `TestFindLandedMerge_ReviewedHeadSHAContradicts`, `TestReattest_PanelShaContradictionRefuses` and the two `…HostilePanelSHAEscapedInRefusal` tests are rewritten to the non-corroborating outcome, keeping the escaping assertion where a SHA is still rendered.
4. **Messages say what is true.** `evaluateMF3` discriminates, in this order:
   1. `*LandedMergeNoEvidence` → unchanged; recovery `mindspec reattest <D>`.
   2. `errors.Is(err, lifecycle.ErrLandedMergeNoCandidate)` → the ONLY arm allowed to say no landing exists. It reads `no two-parent merge reachable from <spec-branch>'s tip lands bead/<D> (first-parent scan, then the reachable-landing fallback)`, followed by today's two-possibility wording.
   3. A corroboration contradiction → `a merge landing bead/<D> exists (<sha>) but <datum> contradicts its second parent <sha>`.
   4. A revert shape → `merge <sha> landing bead/<D> was reverted or cleanly removed after landing`.

   Each arm ends with its own recovery: inspection first, and never `mindspec complete` for a bead whose merge exists. The `ErrLandedMergeNoCandidate` doc comment is updated to the same predicate.
5. **#232, closed on its mechanism.** For the D.5 shape (one registration `{B}`, merge second parent `D ≠ B`):
   - with a merge-time binding present, MF-3 PASSES via (d);
   - with no binding and no surviving branch, MF-3 reports `*LandedMergeNoEvidence` naming the candidate (never the zero-candidate sentence), and `mindspec reattest` then writes the binding on datum (a), after which MF-3 PASSES.

   This also clears #233's reproduced refusal (the same shape). #233 itself stays open for spec 130: the panel leg can never CONFIRM on the normal path because `complete` lands a commit no panel reviewed (OQ-3). Binding review to landing is the panel substrate's job. No `origin/`-aware mode is built. A local-ahead-of-remote fixture stays as a regression guard; it is labelled green today and is not the proof.

### R5 — MF-2 form symmetry

1. **One form table, both sides.** The bead-side harvester and the spec-side indexer recognise the SAME claim forms, pinned by one shared table:
   - appended sub-letter (`R9a`, `AC-2b`);
   - parenthetical sub-letter (`R9(a)`, `AC-2(b)`);
   - Roman-enumerator parenthetical, degrading to base (`R5(ii)`);
   - bare base (`R9`, `AC-9`);
   - hyphenless `AC<n>`, NORMALISED to `AC-<n>` on both sides, and harvested as a claim.

   Dotted sub-numbers (`R6.3`) continue to resolve at base granularity on both sides, and the table says so (OQ-6).
2. **Symmetry is tested.** `TestMF2_FormSymmetry` runs every form through both the harvester and the indexer and asserts each harvested key is producible by the indexer from the same text. `TestReadiness_MF2AllClasses` gains five legs:
   - bead `R9(a)` vs spec `R9(a)` → resolves (red today);
   - bead `R3(c)` vs spec `R3c` → resolves;
   - bead `AC9` vs spec `AC-9` → resolves;
   - bead `AC-9` vs spec `AC9` → resolves (red today);
   - bead `R9(b)` vs spec `R9(a)` only → dangling (exactness preserved).

   A sixth leg makes the third red-on-revert: bead `AC9` against a spec with no AC-9 reports dangling `AC9`. Today `AC9` is never harvested, so that leg passes vacuously.

### R6 — Signal-scoped `--allow-not-ready` with evidence; `ready-check` reads a validated marker

1. **Scoped form.** `--allow-not-ready=<ids>` takes a comma-separated list from the closed set `MF-1`…`MF-4`. An ID outside the set refuses before any mutation.
   - When ≥1 signal fails, `GateReadiness` proceeds iff the currently failing set is a SUBSET of the named set.
   - When nothing fails and a scoped flag is given, the claim proceeds as a normal claim. It prints the per-signal "named but passing" note, writes no marker, and does not require `--evidence`.
   - If any other signal fails, it refuses with the standard NOT-READY report plus `recovery: mindspec next <bead-id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>`. The bead ID is idrender-gated, the IDs come from the closed set, and the placeholder is literal and non-mechanical.
   - A named signal that passes prints `MF-x named but passing — not recorded`.
   - The BARE flag, and the space-separated `--allow-not-ready MF-3` that cobra's `NoOptDefVal` parses as bare-plus-positional, always refuse before positional resolution. `MF-3` is never treated as a bead ID. The recovery is the scoped line above, or `recovery: mindspec next <bead-id>` when nothing fails.
   - `--allow-not-ready=all` is NOT admitted (OQ-5).
2. **Fresh evidence on every claim; an existing marker never authorizes one.** A scoped override that records ≥1 signal REQUIRES `--evidence <path>`: a readable regular file of 1 byte to 64 KiB, read before any mutation. A missing `--evidence` then refuses before any mutation. `RecordReadinessOverride` REPLACES the marker (never unions it) with schema v2:

   ```
   {schema: 2, signals: [failing ∩ named], fingerprints: {<id>: <FailureFingerprint(signal)>}, overridden: <RFC-3339>, evidence: {path, sha256, bytes}}
   ```

   **The fingerprint covers a signal's complete failure set** (this prices a `readiness` API change):
   - `readiness.Signal` gains `Failures []string`: canonical, raw (pre-render) failure records, one per failing item. Each `evaluateMF*` aggregates **all** failures instead of returning on the first.
     - MF-1: one record per missing structural element.
     - MF-2: one per dangling token.
     - MF-3: one per failing dependency, `<dep-id>:<class>[:<candidate-sha>]`. MF-3 therefore evaluates every closed member dependency, at one landed-merge evaluation each, where it used to stop at the first failure.
     - MF-4: one per blocking token or checklist item, `<kind>:<sha256 of the normalised line>`.
   - Records carry no free-form error text, timestamps or bd stderr. They are deduplicated and sorted.
   - `readiness.FailureFingerprint(Signal)` = sha256 of the sorted records joined by `\n`. It is ONE function used by both the writer (`RecordReadinessOverride`'s caller) and the reader (`ready-check`).
   - An MF-3 record of class `indeterminate` (a membership or landed-merge read error) is **never coverable**: an override never covers an infrastructure failure.
   - `Detail` stays the human text, and may list every failure.

   The evidence **text is not stored** (ADR-0025 touchpoint). The marker-before-claim, fail-closed, rollback-on-claim-failure contract (settled fact 2) is unchanged.
   - A re-claim after `mindspec release` needs the flag and evidence again. `release` does not clear the marker, and it does not need to, because no gate reads it for claim authority.
   - With an existing marker covering MF-2 and MF-3, a new request naming only MF-3 while both fail still refuses.
   - The marker is read in `cmd/mindspec` and passed into the report as data. `readiness.EvaluateReadiness` never reads it (spec 124 AC-12, `internal/complete/readiness_isolation_test.go`).
3. **One validity contract, fail-closed.** A shared validator in `internal/bead` (beside the marker constants) classifies a marker as VALID, LEGACY or MALFORMED:
   - VALID: schema 2; `signals` a non-empty, duplicate-free subset of the closed set; exactly one 64-hex `fingerprints` entry per signal; `overridden` RFC-3339; `evidence.sha256` 64-hex; `evidence.bytes` in [1, 65536]; `evidence.path` non-empty.
   - LEGACY: no `schema` key, `{signals, overridden}` only. This is today's writer, and such markers are already in the tracked JSONL.
   - MALFORMED: anything else.
   - A metadata read error is an error, never "no marker".

   Integrity is shape and self-consistency only. The marker is agent-writable bd metadata and is not authenticated (ADR-0037 §8).
4. **`ready-check` honours only VALID markers, as the dispatch-ingress answer.** `mindspec bead ready-check <id>` reads the marker. A failing signal X is COVERED iff three things hold:
   - the marker is VALID;
   - X is in `signals`;
   - `fingerprints[X]` equals `FailureFingerprint` of X's current failure set.

   So an override recorded while dependency A failed does not cover A+B, and an MF-4 override for one blocker does not cover a second blocker. Rendering:
   - covered → `MF-x: OVERRIDDEN (recorded <ts>; evidence <sha256-prefix> <escaped path>)`;
   - recorded but now passing → `MF-x: PASS (override recorded but no longer needed)`;
   - LEGACY → `override marker is legacy (no evidence) — not honoured`;
   - MALFORMED → `override marker is malformed — not honoured`.

   Exit codes: 0 iff every signal passes or every failing signal is covered; otherwise non-zero naming each UNCOVERED signal; a metadata read error refuses non-zero.

   **Re-recording for a claimed bead.** `mindspec next` cannot re-select an in-progress bead, so the LEGACY, MALFORMED and uncovered arms end with two recovery lines:
   - `recovery: mindspec release <bead-id>` (removes the worktree, keeps the bead branch, returns the bead to open);
   - `recovery: mindspec next <bead-id> --allow-not-ready=<uncovered ids> --evidence <path-to-justification>`.

   The re-claim reuses the surviving bead branch. This exit code replaces `ms-bead-impl`'s prose comparison (R6.5). It is never claim authority: only R6.2's fresh flag and evidence can authorize a claim.
5. **One home: guidance follows, the skill copy goes.** `plugins/mindspec/skills/ms-bead-impl/SKILL.md`'s override-coverage paragraph is replaced by one harness line: run `mindspec bead ready-check <id>`, proceed on exit 0, otherwise stop and surface its report. The installed `.claude/skills/` copy is refreshed. `internal/setup/readiness_skill_pin_test.go`'s `TestMsBeadImplSkill_OverrideCoverage{CheckShipped,MutationProbe}` are rewritten to pin the new line, under the skill-refresh protocol (the replaced text is snapshotted for existing installs). The bare/all-or-nothing wording is replaced by the scoped form in:
   - `cmd/mindspec/bead_clarify.go`;
   - `internal/next/ready_gate.go`'s NOT-READY recovery string;
   - `cmd/mindspec/next.go`'s record-failure recovery and flag help;
   - `.mindspec/core/USAGE.md` (two places);
   - `.mindspec/core/WORKFLOW-STATE-MACHINE.md`;
   - `.mindspec/domains/workflow/{architecture,interfaces,runbook}.md`;
   - `.mindspec/domains/execution/interfaces.md` (the schema-v2 marker row);
   - `ms-bead-cycle/SKILL.md`, where the plan's grep finds the bare form.

   `project-docs/user/guides/autonomy.md` does not mention the flag and is not touched.

### R7 — Governance rider: ADR-0045

1. `ADR-0045-binary-owns-sequence-and-rules.md` lands in `.mindspec/adr/` with this spec's commit. The operator-supplied text has these round-1 corrections, each recorded in the ADR:
   - "content-sha" → "byte-exact snapshot";
   - the Amends line widened to the parts of ADR-0040 §1 it really changes (the L4 definition, the "where does this rule live?" default, and the open-ended staging ratchet), with the §3 reconciliation stated;
   - ADR-0044 added as amended, for the placement of conduct text, with the reconciliation stated;
   - Implementation order step 1 assigned to spec 130, and steps 5–7 marked unassigned;
   - a Status note: `Accepted` records the operator's decided direction (Deciders: Max). Nothing in its Implementation order has shipped, so under ADR-0032 a bead may cite ADR-0045 as `adr_citations` coverage only for a rule it moves out of a skill. In spec 129 that is R6.5.

   The dated Verification note cites the files that ground each Context claim. "Amended by ADR-0045" back-pointer blockquotes are added to ADR-0040 §1 and to ADR-0044 in the same commit.
2. The S0 rule table (`.mindspec/policy/rules.yaml`) is not delivered here. Its home is spec 130 (OQ-1).

## Scope

### In Scope

- `internal/next/beads.go`, `internal/next/select.go`, `internal/next/ready_gate.go`, `cmd/mindspec/next.go` (R1, including R1.8's dispatch spec passed to the unchanged `EnsureWorktree`/`DispatchBead`; R6)
- `internal/bead/bdcli.go`, `internal/bead/hygiene.go`, `internal/bead/readiness.go`, plus a new plan-bead helper (R1.1, R2, R4.1, R6.3)
- `internal/lifecycle/landed.go`, `internal/lifecycle/reattest.go` (R4.2, R4.3); `internal/lifecycle/orphans.go`, `internal/lifecycle/stale_open.go` (R2 only); `internal/gitutil/gitops.go` (R4.2 primitive, `FirstParentMerges` doc contract)
- `internal/complete/complete.go` (R2, the post-close ready lookup only)
- `internal/validate/readiness/readiness.go`, `report.go`, `fixtures.go`:
  - R1.8's returned lineage;
  - R4.1 and R4.4;
  - R5;
  - R6.2's `Signal.Failures`, `FailureFingerprint` and the all-failure aggregation in `evaluateMF1`–`evaluateMF4`;
  - R6.4 rendering;
  - `registerLandedPanel` gains a slug parameter.
- `internal/approve/plan_test.go`-class pin (R3.2), `internal/approve/adopt_lattice.go` (R2.1 waist migration)
- `internal/lint/` (R2.3), `cmd/mindspec/bead_ready.go`, `cmd/mindspec/bead_clarify.go` (R6)
- `internal/instruct/templates/implement.md` (R1.4)
- `plugins/mindspec/skills/ms-bead-impl/SKILL.md`, its `.claude/skills/` copy, `internal/setup/readiness_skill_pin_test.go` and the skill-refresh snapshot; `ms-bead-cycle/SKILL.md` if needed (R6.5)
- R6.5's `.mindspec/core/**` and `.mindspec/domains/**` doc surfaces
- `.mindspec/adr/ADR-0045-…md` (new), the ADR-0040 §1 and ADR-0044 back-pointers (this commit), `.mindspec/adr/ADR-0041-gate-before-mutate.md` (Amendment (Spec 129), with the R4 and R6 beads)

### Out of Scope

- Everything under Non-Goals.
- `internal/panel` (gate and registry untouched).
- `internal/phase` (consumed, not changed).
- `internal/approve/impl.go` and `internal/executor` behaviour (129b).

## Non-Goals

Deferred to **spec 129b (readiness-tracker-honesty, part 2)**. The diagnosis is already reproduced and recorded here, so 129b need not re-derive it:

- **G — OWNERSHIP.yaml read from the wrong ref (#234).** `validate.CheckADRDivergence` is called from `complete.runCommittedRangeGate` with `ownerRef = beadHead`, and `CheckADRDivergenceSnapshot` forces `ownerRef` to the bead worktree's candidate tree `T`. The impl-approve backstop passes the spec-branch tip. A claim committed on the spec branch after the bead branched is invisible on both per-bead legs. Fix shape: a path is owned if a manifest claims it at the bead tree OR at the spec-branch tip.
- **H — `impl approve` closes the epic before finalizing (#244).** `approve.ApproveImpl` runs `bd close <epic>` and `mindspec_phase=done` BEFORE `exec.FinalizeEpic`. Any FinalizeEpic failure leaves a CLOSED epic and an unmerged spec: push (`gitutil.PushBranch`), `guardMergeLayout`, `preflightMergeDestruction`, or a direct-path merge conflict. A plain reorder is not available: `FinalizeEpic`'s `commitWithExport(specWtPath, …)` must export the JSONL with the epic already closed (the wu7t carrier). Fix shape:
  - hoist the refusal checks and a `git push --dry-run` auth probe ahead of the close;
  - turn any post-close failure into a guard failure naming "epic closed, spec unmerged", with the resume command.

  The re-run tolerance was traced, not run; 129b must run it.
- **#250 — no verification window between merging a bead and closing it** (`mindspec complete`). This is #244's family: a multi-step terminal mutation with no intermediate stop, at the bead level. The issue records that the reconcile path already converges an out-of-band merge. 129b decides between a merge-only/verify seam and documenting the out-of-band route.
- **I — dead config key.** `merge_strategy` (`internal/config.Config.MergeStrategy`) is read only by `config show`. `MindspecExecutor.FinalizeEpic` decides PR versus direct merge by `gitutil.HasRemote()`. 129b either honours the key or removes it.
- **Phase-lineage unification.** 129 adopts `phase.FindEpicForBead`'s lineage, including its `[NNN]` title fallback, unchanged, as the selection lineage (R1.1). So #255's reported shape is selected, and `next` and `ready-check` agree for every `<E>.<N>` bead. They still differ for a `<other>.<N>` bead with title-fallback lineage: readiness evaluates it, while `next` names it (N3) and R1.8 refuses to claim it. 129b removes the title fallback, tightens `beadIndex`, and migrates any bead that depended on them.

Deferred to **spec 130**:
- the S0 rule table (ADR-0045 step 1);
- #233's review-to-landing binding (R4.5);
- the panel substrate.

Also not goals:
- lane machinery;
- any `origin/`-aware MF-3 mode;
- bd upstream changes;
- a `spec_id` on epics, labels, or bd's native spec-id (R3);
- a `doctor` check for linkage-less beads;
- authenticating agent-writable metadata (ADR-0037 §8);
- re-verifying the evidence file's content at read time;
- changing `next/mode.go`'s title-based `parseSpecID` (display only);
- tightening `readiness.beadIndex` (MF-1's mapping is unchanged);
- a universal "no verb ever misreports emptiness" claim — R1 is scoped to `next`'s spec-scoped set.

## Acceptance Criteria

- [ ] **AC-1 (R1.1, R1.2)**: Fixture: a fake bd that honours `-n` and the default cap. Ready rows carry `parent`/`metadata` when set, and `bd show` and the epic list are served for `FindEpicForBead`. Epic E for spec S (`spec_num` set) has the following beads:
  - A: `<E>.1`, parent E, title neutral.
  - G: `<E>.2`, no parent, title `[S] …`. This is #255's reporter shape.
  - D: `<E>.3`, no parent, title neutral.
  - P: `<E>.4`, lineage resolving to another epic E2.
  - R: non-plan-form, parent E, type task.
  - F: non-plan-form, parent E, type bug.
  - M: `metadata.spec_id=S`, no parent, title neutral.
  - T: titled `[S] …`, no other link.
  - The global set also includes E's own epic row.

  Expected:
  - `next.BuildReadySet` returns members {A, G} in bd order.
  - Near-misses are D (N1), P (N2), R (N3, blocking), and F, M, T (N3, informational). The epic row is in neither list.
  - `mindspec next --spec S --emit-only --pick 2` emits G, and the claim path with `--pick 2` claims G with no override. Readiness resolves G by title-fallback lineage, unchanged.
  - The output contains every near-miss ID.
  - No `bd ready --parent` call is issued (argv seam).

  Tests: `TestBuildReadySet_PlanBeadPredicate`, `TestNext_ScopedSelection_ReporterShapeClaimable`, `TestNext_ScopedSelection_NamesNearMisses`. Red-on-revert: today G is excluded on an edge-only `--parent` bd, or when its title lacks the full spec ID; A is excluded by the title filter; and T is returned through its title.
- [ ] **AC-2 (R1.2 N1 recovery is real; #255 shape on real bd)**: A hermetic test (fake bd, no `t.Skip`; spec 124's bd-less rule) uses the row and `bd show` shapes measured on bd 1.1.0 (Background A, C). It runs in two steps:
  - Before the update: D is classed N1, with the `bd update <id> --parent <E>` line.
  - After it: D's row carries `parent: E`, and `bd show` carries the `parent-child` edge to the epic. D is then a member, `phase.FindEpicForBead` resolves E, and `readiness.EvaluateReadiness` reaches its signals with no lineage or index error.

  (`TestNearMissN1_RecoveryYieldsClaimable`.) The real-bd run of both the reporter shape and the N1 sequence is a Validation Proof.
- [ ] **AC-3 (R1.3)**: The claim path, with the selector-specific sentences above:
  - With an empty member set, `mindspec next <id> --spec S` and `mindspec next --spec S --pick 1` each exit non-zero, name the selector, and end with a line matching `^recovery: mindspec next --spec `.
  - A near-miss positional gets its R1.2 sentence on both an empty and a NON-empty member set.
  - A positional that is bd-ready but unlinked (another spec's plan bead) gets `is bd-ready but is not linked to spec S (parent: <P>)`, never "not found or not ready".
  - `--emit-only <near-miss>` still prints the pinned bypass line and emits.

  (`TestNext_SelectorOnEmptySet_Refuses`, `TestNext_NearMissPositional_Refuses`, `TestNext_UnlinkedReadyPositional_Refuses`.) Red-on-revert: today it exits 0 with "No ready work found.".
- [ ] **AC-4 (R1.4)**: Four fixtures with an empty member set, no selector and no near-miss, each with an explicit STORED phase:
  - (i) Stored `implement`, all lifecycle children closed: `stored phase: implement; child-derived phase: review`, `recovery: mindspec impl approve S` and `recovery: mindspec repair phase S`.
  - (ii) Stored `implement`, one child in_progress with a missing worktree: the tally and phase lines print BEFORE the worktree-recovery output.
  - (iii-a) Stored `implement`, one child closed and the rest open and blocked: `child-derived phase: implement`, `<n> open lifecycle bead(s), none bd-ready` and `recovery: bd blocked --parent <E>`.
  - (iii-b) Stored `implement`, every child open and blocked, none closed: `child-derived phase: plan`, with the same blocked line, never `plan approve`.
  - (iv) Stored `plan`, zero lifecycle children: `recovery: mindspec plan approve S`.

  Each output contains the tally and the verbatim sentence, and exits 0 except where the arm mutates. Further fixtures:
  - (v) Stored `review`, all children closed, plus one informational near-miss: a follow-up `bug` child of E, and a global bead whose title contains S. It exits **0** and still names both.
  - (vi) The same, plus one blocking near-miss (a detached plan-form bead): it exits non-zero with the same lines.
  - A failing child read (seam) renders the indeterminate form with no tally and exits non-zero.

  (`TestNext_EmptyScoped_DerivedState`, `TestNext_EmptyScoped_InformationalNearMissExitsZero`.) `TestImplementTemplate_EmptinessRule` asserts that the template contains the sentence. Red-on-revert: fixture (i) today prints "No ready work found." and derives `implement`.
- [ ] **AC-5 (R1.5, R1.6, R1.7)**: Every rendered exclusion or near-miss notice contains each excluded ID via `idrender.Bead`, and a count-only notice is red (`TestExclusionNotice_NamesIDs`). The multi-spec menu prints `--spec=<spec-id>` and never `<number>` (`TestNextMenu_PrintsSpecIDForm`; red today). The rewritten spec-128 tests named in R1.7 pass.
- [ ] **AC-6 (R1.8)**: Before any mutation, with bd, branch and worktree byte-identical afterwards (spec 124's AC-4 audit shape):
  - (a) A bead whose lineage resolves to spec T while it is claimed with `--spec S` refuses, naming both specs.
  - (b) On the unscoped path, a resolved target spec ≠ T refuses likewise.
  - (c) On the unscoped path with NO resolved spec, the claim dispatches into T: the argv seam on `EnsureWorktree`/`DispatchBead` records spec ID T, and the bead branch's base is `spec/<T>`, never `HEAD`.
  - (d) A bead whose lineage resolves to T but whose ID is not `<E_T>.<N>` refuses with the "not a plan bead" sentence on both scoped and unscoped paths.

  Tests: `TestGateReadiness_LineageSpecMismatchRefuses`, `TestNext_UnscopedDispatchesIntoLineageSpec`, `TestGateReadiness_NonPlanBeadRefuses`. Red-on-revert: today (a) validates T and dispatches into S, and (c) cuts from `HEAD`.
- [ ] **AC-7 (R2.1–R2.2)**: Argument-recording seams assert `-n 0` at every site in R2.1:
  - `TestQueryReady_Uncapped`, `TestResolveActiveBead_Uncapped`;
  - `TestOrphans_ListUncapped`, `TestStaleOpen_ListUncapped`, `TestHygiene_ListUncapped`;
  - `TestComplete_PostCloseReady_Uncapped`, `TestAdoptLattice_ListViaWaist`.

  The waist tests are `TestListJSON_ToleratesExplicitNZero` (phase.Cache's argv is passed through with exactly one `-n 0`) and `TestListJSON_RejectsCap` (`-n 5` returns an error, and no bd is spawned). The fake bd used end-to-end APPLIES the default cap (100 for `ready`) unless `-n 0`/`--limit 0` is in argv. With 101 ready rows, `next --emit-only` lists 101 items and `--pick 101` emits the last (`TestNext_101ReadyBeads_AllVisible`). Red-on-revert: today it lists 100.
- [ ] **AC-8 (R2.3)**: `TestBDRowCapWaist` is red when an injected non-test file outside `internal/bead` adds a `bead.RunBD("list", …)` without the marker (self-test). It stays green for the doctor probe's marker, the `redact.go` map key and the harness `sandbox.runBD` calls. A fake bd that exits 0 and prints `Showing 100 of 106 ready issues…` on stderr makes `ReadyJSON` return the truncation error, with the notice `termsafe`-escaped and a `recovery:` line (`TestReadyJSON_TruncationNoticeIsError`).
- [ ] **AC-9 (R3.2)**: `planRunBDFn`'s recorded `bd create` argv for each bead carries `--metadata` JSON with `spec_id` equal to the spec ID (`TestPlanApprove_BeadsCarrySpecID`). `specRunBDFn`'s epic argv carries no `spec_id` (`TestSpecApprove_EpicHasNoSpecID`).
- [ ] **AC-10 (R4.1)**: `TestReadiness_MF3Variants` gains these closed-dependency legs:
  - (a) a plain task: non-plan-form, no `spec_id`, parent not E → PASS with the "not applicable" detail;
  - (b) another spec's plan bead (`<E2>.1`, `spec_id=S2`) → PASS likewise;
  - (c) a same-spec plan bead `<E>.1` with no landed merge → FAIL, unchanged;
  - (d) a metadata-only dependency (`spec_id=S`, no parent, neutral title) with no merge → FAIL, naming the metadata disjunct;
  - (e) a reparented non-plan-form dependency with parent E, no metadata and no merge → FAIL;
  - (f) a title-only `[S]` plain task → PASS;
  - (g) the parent read errors through the seam → FAIL "could not determine whether…", never PASS.

  Red-on-revert: today (a), (b) and (f) FAIL.
- [ ] **AC-11 (R4.2)**: Real-git fixtures extending `initLandedRepo`/`mergeBead` (`internal/lifecycle/landed_test.go`). Every leg asserts both `FindLandedMergeReachable` and `ReattestLandedMerge` unless it says otherwise. "#239 topology" means: spec/S's content reaches `main` through a sibling branch's PR merge, and spec/S is fast-forwarded to `main`, so earlier landings become reachable only through a second parent.
  - (i) X lands on spec/S by its own `Merge bead/X` (first-parent at that time), then the #239 topology. Surviving tip = second parent. Identified (red today: `ErrLandedMergeNoCandidate` / `ReattestStateNoOwnedMerge`).
  - (ii) As (i), with X first refreshed by `Merge branch 'spec/S' into bead/X`. Still identified, not refused as ambiguous: the refresh is excluded as a merge INTO `bead/X`.
  - (iii) X ALSO merged into sibling bead Y by `Merge branch 'bead/X' into bead/Y` before X's own `Merge bead/X` landing on spec/S, then Y's own landing, then the #239 topology. X is identified from its own landing and Y from Y's. The long-form X→Y merge is excluded (explicit non-spec target), so it neither identifies X nor creates a FIX-2b ambiguity.
  - (iii-neg) X's ONLY path to the tip is that long-form X→Y merge inside Y's landing: X is NOT identified (`ErrLandedMergeNoCandidate`) and is not credited through Y. This is the #251 boundary.
  - (iv) A refresh followed by a FIRST-parent landing is identified by the primary scan, unchanged.
  - (v) A correctly named merge on a branch NOT reachable from the tip is not a candidate.
  - (vi) `MergedUnclosed` for fixture (i) returns "not this state", so `complete`'s reconcile ignores the fallback.
  - (vii) **O3's false PASS.** X's round-1 tip A is merged into `bead/Y` with the exact short subject `Merge bead/X`; Y lands; X's later round B never lands; `bead/X` is deleted; there is no binding for X; registrations are {A, B}. `FindLandedMergeReachable` returns `*LandedMergeNoEvidence`, and MF-3 does NOT pass. Red against the round-1 text, where the panel confirmed on A. `reattest` on the same fixture nominates the merge and renders both the target-unknown caveat and the `later reviewed work … not in <spec-branch>'s history: B` warning.
  - (viii) **Short form X→Y with X's branch surviving.** X's surviving tip equals the second parent of a `Merge bead/X` made into Y, and Y alone reaches spec/S. X is identified via (b), the bounded reachability residual stated in R4.2.
  - (ix) **Short form X(old tip A)→Y, then X(new tip B)→spec/S, then the #239 topology.** Two owned candidates with second parents A and B. With `bead/X` surviving at B, or a binding naming the X→spec/S merge, the B group is identified and not refused as ambiguous. With neither, the result is FIX-2b `*LandedMergeNoEvidence`.

  Background D.2 records the round-1 and round-2 evidence. Tests: `TestFindLandedMergeReachable_{OffFirstParent,RefreshedThenOff,LongFormSiblingExcluded,OnlyViaSibling,FirstParentRefresh,Unreachable,PanelOnlyNeverConfirms,ShortFormSurvivingTip,AmbiguityResolvedByTip}`, `TestReattest_NominatesSameCandidateAsFind`, `TestReattest_FallbackCaveatRendered`, `TestMergedUnclosed_IgnoresFallback`.
- [ ] **AC-12 (R4.3, R4.5)**: Registrations via `registerLandedPanel`, extended with a slug parameter:
  - (i) Dirs `<X>` with `reviewed_head_sha = A` (stale) and `<X>-r2` with `B` = the second parent: `FindLandedMerge` CONFIRMS (primary path). Red today: the lexically first `A` contradicts.
  - (ii) Registrations {A} and surviving tip = B: confirms via the tip leg, and the panel leg is non-corroborating.
  - (iii) Registrations {A} and surviving tip ≠ B: contradicts, naming the tip datum (the mixed-leg rule).
  - (iv) The #232/#233 shape. One registration {B}; the merge's second parent is a later tree-identical commit D; the binding records the merge. MF-3 PASSES via (d). Red today: the zero-candidate sentence.
  - (v) The same with no binding and no branch: `*LandedMergeNoEvidence` naming the candidate. `ReattestLandedMerge` then writes the binding, with a corroboration string recording the panel as non-corroborating, and MF-3 PASSES. Red today: `ReattestStatePanelContradiction`.
  - (vi) The spec branch locally ahead of its remote-tracking ref: MF-3 PASS. This is a regression guard, green today.
  - (vii) **CLOSE paths.** `MergedUnclosed` with {stale A} and surviving tip = second parent identifies the landing. With {A} alone and no tip or binding, it returns the `*LandedMergeNoEvidence` error and never "merged-unclosed", so `complete` closes nothing.
  - (viii) **Binding veto and audited repair.** A contradictory binding plus a non-corroborating panel set {A}, on a fallback candidate: `FindLandedMergeReachable` refuses (binding contradiction). `ReattestLandedMerge` writes the scan-derived identity and records the prior pair in the audit. A reattest that refuses (e.g. reverted) writes nothing. `TestReattest_ContradictoryBindingOverwrittenAudited` and `TestReattest_StaleOlderRemergeBindingRewrittenToNewest` stay green.

  Tests: `TestFindLandedMerge_PanelSet*`, `TestReattest_PanelSetNonCorroborating`, `TestReadiness_MF3_CompleteWrapperShape`, `TestReadiness_MF3_LocalAheadOfRemotePasses`, `TestMergedUnclosed_PanelSet`, `TestReattest_RepairsContradictoryBindingOnFallback`.
- [ ] **AC-13 (R4.4)**: `TestReadiness_MF3ErrorArmDiscrimination` gains four legs:
  - `ErrLandedMergeNoCandidate` → the "no two-parent merge reachable … lands bead/" sentence;
  - a binding contradiction → the "a merge landing bead/<id> exists (<sha>) but … contradicts" sentence, and NOT the zero-candidate one;
  - a revert shape → "was reverted or cleanly removed";
  - each ends with its own recovery line, and none recommends `mindspec complete` for the existing-merge arms.
- [ ] **AC-14 (R5)**: `TestMF2_FormSymmetry` passes over the full table. `TestReadiness_MF2AllClasses` gains the six R5.2 legs. Red-on-revert: legs 1, 4 and 6 fail today.
- [ ] **AC-15 (R6.1)**: Each leg below is a separate fixture or invocation:
  - MF-3 failing only: `--allow-not-ready=MF-3 --evidence <f>` claims and records `signals: [MF-3]`.
  - MF-2 and MF-3 failing: `--allow-not-ready=MF-3 --evidence <f>` refuses with nothing mutated (byte-identical audit). Its final `recovery:` line contains `--allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>` and no operator-supplied path.
  - The bare flag refuses with the same line.
  - `--allow-not-ready MF-3` (space-separated) refuses as bare, and `MF-3` is never looked up as a bead.
  - `--allow-not-ready=MF-9` refuses before any mutation.
  - A missing `--evidence` refuses before any mutation when a signal would be recorded.
  - Nothing failing with `--allow-not-ready=MF-3` and no `--evidence`: a normal claim, the "named but passing" note, and no marker written.

  (`TestGateReadiness_ScopedOverride`, `TestNext_BareAllowNotReady_Refuses`, `TestNext_AllowNotReadySpaceSeparated_Refuses`, `TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker`.)
- [ ] **AC-16 (R6.2, fingerprint and scope widening)**: Seven legs:
  - **Named-but-passing never widens.** Request `MF-2,MF-3` while only MF-3 fails. The stored marker holds only `MF-3`, with its `fingerprints` entry and `evidence.{path,sha256,bytes}`, and no `text` key. Then make MF-2 fail: `ready-check` exits non-zero naming MF-2, and an UNFLAGGED re-claim refuses.
  - **A then A+B (MF-3).** Record an override while dependency A fails MF-3. Keep A unchanged and add a newly failing dependency B: `ready-check` refuses with MF-3 uncovered until a fresh override with evidence is recorded.
  - **The MF-4 analogue.** Keep the first blocker and add a second blocking line: MF-4 uncovered.
  - **Explicit narrower request.** An existing marker covers MF-2 and MF-3, both fail, and a new request names only MF-3: it refuses, and the marker is unchanged.
  - **Size limit.** A 64 KiB + 1 evidence file refuses with zero metadata writes (seam).
  - **Determinism.** Two consecutive `EvaluateReadiness` runs on an unchanged fixture give byte-identical `FailureFingerprint`s for every signal.
  - **Writer/reader agreement.** Over an MF-4 failure whose line contains a control byte, the writer's fingerprint equals the reader's (the raw, pre-render records are hashed).

  Tests: `TestOverride_NoScopeWidening`, `TestOverride_NewFailureInSameSignalUncovered`, `TestOverride_MF4SecondBlockerUncovered`, `TestOverride_ExplicitNarrowerRequestRefuses`, `TestAllowNotReady_EvidenceTooLargeRefuses`, `TestFailureFingerprint_Deterministic`, `TestFailureFingerprint_WriterReaderAgree`. Red-on-revert: the A-then-A+B leg, against the round-1 first-Detail digest.
- [ ] **AC-17 (R6.3, R6.4)**: `ready-check` with MF-3 failing:
  - with a VALID marker for MF-3 and a matching fingerprint: exit 0, `MF-3: OVERRIDDEN (…)`;
  - with MF-2 also failing: non-zero, naming MF-2;
  - with MF-3's failing dependency changed: non-zero, MF-3 uncovered;
  - with MF-3 now passing: the "no longer needed" note;
  - with an `indeterminate` MF-3 record: never covered;
  - with a LEGACY marker: non-zero, the legacy note, and the two-line `mindspec release <id>` / `mindspec next <id> --allow-not-ready=… --evidence <path-to-justification>` recovery;
  - with MALFORMED markers (unknown ID, duplicate ID, missing fingerprint, bad timestamp, non-hex sha256, `bytes` = 0): non-zero, the malformed note;
  - with a metadata read error: refuses non-zero.

  After `release` and the re-claim, the bead's surviving branch is reused. `EvaluateReadiness` does not read the marker, and the isolation test stays green. Tests: `TestReadyCheck_HonoursValidMarkerOnly`, `TestOverrideMarker_Validity`, `TestReadyCheck_LegacyRecoveryReleasesThenReclaims`. Red-on-revert: the first case exits non-zero today.
- [ ] **AC-18 (R6.5, R7, ADR-0041)**:
  - `ms-bead-impl/SKILL.md` contains no `mindspec_readiness_override` subset comparison and contains the `ready-check` exit-code line; the rewritten pin tests pass.
  - `.mindspec/adr/ADR-0045-…md` exists with `Status: Accepted`, the Status fence, Amends ADR-0040 §1 and ADR-0044, step 1 → spec 130, and Decision 5's transition clause (pre-existing skill content grandfathered until step 7).
  - ADR-0040 and ADR-0044 carry the back-pointer; `mindspec adr list` includes ADR-0045.
  - ADR-0041 contains § "Amendment (Spec 129)" naming the fallback-scoped callers, the undirected-subject residual, the no-panel-only confirmation rule on the fallback path, the non-corroborating panel datum and its reach into the CLOSE paths, the binding veto versus audited repair, and the scoped, fingerprint-bound fourth-verb override (a text-pin test in the R4 or R6 bead).
  - `mindspec validate spec 129-readiness-tracker-honesty` resolves every touchpoint link.
- [ ] **AC-19 (method)**: `go build ./... && go test ./... -short` is green. `internal/harness` and `internal/instruct` tests run under `-short` only (128 settled fact 3). `go test ./internal/lint/...` is green, including `TestBDRowCapWaist` and the ADR-0030 boundary lint (no new `os/exec` or `gitutil` import in `internal/validate/readiness`).

## Validation Proofs

- A real throwaway bd database. The epic carries `spec_num`, and the beads are:
  - A: a plan bead;
  - G: `[S] …`, created with `--parent` and then detached with `bd dep remove` (#255's reporter shape: `bd dep tree` shows no dependencies);
  - D: neutral title, detached the same way;
  - M: `metadata.spec_id` only;
  - T: title only.

  `mindspec next --spec <S> --emit-only` lists A and G, and names D, M and T with their reasons. Then `bd update <D> --parent <E>` followed by the same command lists A, G and D (AC-1, AC-2).
- `mindspec next <T> --spec <S>`: exits non-zero with T's N3 sentence (AC-3).
- All-closed fixture with stored `implement`: `mindspec next --spec <S>` prints `child-derived phase: review` and `recovery: mindspec impl approve <S>` (AC-4).
- This repo: `bd ready --json 2>&1 >/dev/null` prints the truncation notice. After R2, `mindspec next --emit-only` lists every ready bead (AC-7).
- `go test ./internal/lifecycle/ -run 'TestFindLandedMergeReachable|TestReattest_NominatesSameCandidateAsFind|TestMergedUnclosed_IgnoresFallback|TestFindLandedMerge_PanelSet|TestReattest_PanelSet'` is green (AC-11, AC-12).
- `go test ./internal/validate/readiness/ -run 'TestReadiness_MF3|TestMF2_FormSymmetry'` is green (AC-10, AC-12, AC-13, AC-14).
- With MF-3 failing: `mindspec next <bead> --allow-not-ready=MF-3 --evidence ./why.md` claims, and `mindspec bead ready-check <bead>` then exits 0 with `MF-3: OVERRIDDEN`. Changing the failing dependency makes `ready-check` exit non-zero. The bare `--allow-not-ready` refuses with the scoped line (AC-15–AC-17).
- `mindspec adr list | grep ADR-0045` and `mindspec validate spec 129-readiness-tracker-honesty` → 0 errors (AC-18).
- `go build ./... && go test ./... -short` is green (AC-19).

## Open Questions

Every question below is either resolved with its rationale or EXPLICITLY DEFERRED to the plan stage with its re-open condition.

- [x] **OQ-1 (scope split) — RESOLVED.**
  - 129 = R1–R6 (estimate: 7–10 beads) + R7 (doc, this commit) + the ADR-0041 amendment (with the R4/R6 beads).
  - 129b = five deliverables: G (#234), H (#244), #250, I (`merge_strategy`), and phase-lineage unification.
  - 130 = the S0 rule table (pure policy, a different review centre, an unowned path that first needs an OWNERSHIP claim), #233's review-to-landing binding, and the rest of #251.

  Re-opens if the plan gate judges 129 over-compressed: R6 then splits out as 129c.
- [x] **OQ-2 (title filter) — RESOLVED.** The title is never a membership input. `containsSpecID` survives only as R1.2's near-miss linkage test, and R1.7 lists the spec-128 tests that are rewritten.
- [x] **OQ-3 (can the panel leg ever confirm after spec 128 Bead 6?) — EXPLICITLY DEFERRED to the plan stage.** The plan must run the fixture: the panel reviews `B`, `complete` materialises its landing commit, then the merge. It records whether the second parent equals `B` on a clean worktree.
  - If it never does, R4.3's non-corroborating semantics are exactly right, AC-12 (iv)/(v) is the normal path, and the ADR-0041 amendment says so.
  - If it does, AC-12 (i) is the leg's live proof.

  Either way #233's binding question stays spec 130's. This re-opens R4.3's wording, never its rule.
- [x] **OQ-4 (one discovery for identify AND nominate) — RESOLVED.** `ReattestLandedMerge` nominates through the same first-parent-then-reachable-fallback generator as MF-3 (AC-11 (i)). The executor's write-time `gitutil.ExactSecondParentMerges` stays first-parent by requirement (R4.2's table), because the merge it binds was just made and is the first-parent tip.
- [x] **OQ-5 (bare `--allow-not-ready`) — RESOLVED: refuse.** `--allow-not-ready=all` is NOT admitted. It would restore the all-or-nothing form R6 removes, and the refusal's recovery line already names the complete failing set. The cobra shape (a `String` flag with `NoOptDefVal`) is a plan decision, bound by AC-15's space-separated leg.
- [x] **OQ-6 (index dotted sub-numbers `R6.3`?) — RESOLVED: no.** Base resolution is symmetric today. Sub-number indexing would be a new claim form that turns existing dotted citations into potential dangling claims. Re-opens if a later spec wants sub-number-exact MF-2.
- [x] **OQ-7 (exit code of an empty-set result) — RESOLVED.**
  - No selector and no BLOCKING near-miss → exit 0 (HC-4: the read evaluation completed). Informational near-misses (title-only, metadata-only, non-lifecycle children) are printed and do not change the exit code. Otherwise one permanently ready follow-up bead, whose title names the spec, would fail every scoped `next` forever.
  - ≥1 blocking near-miss (N1, N2, or a lifecycle-type N3 under E) → non-zero (128's loud-empty class).
  - A selector supplied → non-zero.
  - A failed child read → non-zero.
- [x] **OQ-8 (why not surface stderr on success in `tracedOutput` globally?) — RESOLVED: scope it to the waist.** Only in the row-set waist does a notice have a defined meaning (truncation) and a defined response (an error).
- [x] **OQ-9 (membership design) — RESOLVED: plan bead by lineage (round 2), with Option W still rejected.**
  - **Round 1 (Option N)** selected `<E>.<N>` rows whose `parent` was E. Round 2 showed that this left #255's own bead unclaimable: it has a dotted ID, no parent edge, and title-fallback lineage, and it passes `ready-check`. That made round 1's claim "exactly the set readiness can evaluate" false.
  - **R1.1 now selects** a `<E>.<N>` bead whose lineage, through the UNCHANGED `phase.FindEpicForBead`, resolves to E. This is the same resolution `readiness` and `complete` already use, so no core change is needed.
  - **Evidence** (scratch bd 1.1.0, Background A): the reporter shape `pr-1rs.4` resolves `epic=pr-1rs spec=129-probe`, and the neutral-title sibling returns `ErrNoEpicLineage` (class N1, repairable by the AC-2 recovery).
  - **Both #255 defects are fixed for that shape.** The bead is selected and claimable without an override, and a bead that is still excluded is named, never reported as "no work".
  - **The remaining honest limit.** `readiness.beadIndex` accepts any trailing `.<N>`, so readiness also evaluates `<other>.<N>` beads whose title-fallback lineage is S. `next` names them as N3, and R1.8 refuses to claim them. Tightening `beadIndex` and removing the title fallback is 129b's phase-lineage unification.
  - **Option W** (descendant-OR-`metadata.spec_id` membership everywhere) stays rejected. It needs a metadata leg in `phase.FindEpicForBead` (core), a non-dotted MF-1 mapping that no rule defines, and a conflict-precedence rule, all for beads no shipped flow creates.
  - **MF-3's dependency membership stays wider** (plan-form OR spec_id OR parent), because holding more dependencies to the landed test is the fail-closed direction.

  Re-opens if planning finds a shipped flow that creates spec beads without the plan-bead form.

### Grill record (self-answered, headless — 2026-10-18; revised after rounds 1 and 2)

- [x] grill (round 2): is #255's reported bead claimable? → In round 1, no: it was an N1/N3 near-miss. Now yes: R1.1 selects by lineage, and the reporter shape was resolved on a real scratch bd (OQ-9).
- [x] grill (round 2): does the fallback's `Merge bead/<id>` form pin a target? → No: it names none, and a reviewer probe built a false PASS. Fixed: on the fallback path the panel never confirms on its own, (b) or (d) is required, and the residual is stated (R4.2, AC-11 (vii)).
- [x] grill (round 2): does an override fingerprint over the first Detail widen coverage? → Yes: A then A+B stayed covered. Fixed: the fingerprint is over the complete sorted failure records (R6.2, AC-16).
- [x] grill (round 2): can an unscoped claim be cut from `HEAD`? → Yes, today. Fixed: the lineage spec is the dispatch spec (R1.8 (c), AC-6 (c)).

- [x] grill: does the membership predicate admit beads readiness cannot evaluate? → It did in the draft (metadata-only, reparented non-dotted). Narrowed to the plan bead (OQ-9). AC-2 proves the N1 recovery on a real bd.
- [x] grill: is bd's spec-id field a cheaper linkage? → It is filterable by `bd list --spec` (prefix) and shown by `bd show`, but `bd ready` cannot filter it, and it would be a second home. Not adopted (R3.1).
- [x] grill: is #232 the same bug as #239? → No. Its dependency is the same spec's bead, and its mechanism is #233's: the panel SHA is never the landing's second parent. R4.3 (both functions) plus R4.4 close it end-to-end (AC-12 (iv)/(v)). No remote ref is read.
- [x] grill: does a full-history scan fix #239 safely? → No. A refresh merge into `bead/X` becomes a second candidate, and the ambiguity rule refuses (reproduced). Redesigned as a first-parent-primary scan with a source-named fallback, scoped to MF-3 and reattest (R4.2, AC-11). The fallback is target-pinned only for the `into <spec-branch>` forms; round 2 added its non-panel confirmation rule.
- [x] grill: does removing the panel veto reintroduce ancestor-tolerant confirmation? → No. The set confirms only on exact equality; the tip and binding contradictions remain.
- [x] grill: does R6 open a gate on a forged marker? → No gate reads the marker for claim authority (fresh flag and evidence on every claim). `ready-check`'s dispatch answer is no weaker than today's skill prose, and it is stricter: a VALID schema plus a per-signal fingerprint of the complete failure set are required.
- [x] grill: does the evidence leak into git? → Its path, sha256 and byte count do (the tracked JSONL); its text is never stored.
- [x] grill: "A re-run of impl approve tolerates the half-done state" — traced or run? → Traced. Recorded as 129b's obligation to run it.
- [x] grill: does ADR-0045 hold against the tree? → Its Context claims are verified by file. It conflicted with ADR-0044 and understated its ADR-0040 amendment; both are now stated. `Accepted` is fenced per ADR-0032.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
