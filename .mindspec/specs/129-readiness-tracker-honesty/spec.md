---
approved_at: "2026-10-19T07:48:35Z"
approved_by: user
status: Approved
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
- (d) MF-3 holds a closed dependency to the landed-merge test only when the dependency belongs to the same spec by a tracker fact; a correctly named landing that is reachable from the spec-branch tip but off its first-parent chain (GH #239) is found and NAMED, and MF-3 fails loudly with the one audited command (`mindspec reattest`) that can accept it. The automatic path never passes it, so #239 is fixed as loud, named and recoverable, not as an automatic pass; a stale or non-matching panel registration never vetoes a correct corroboration, in `FindLandedMerge` or in `reattest`; and it prints what is actually true for a contradiction or a revert. GH #232 is closed on its real mechanism (Background D.5), not by a remote-aware feature;
- (e) the MF-2 token resolver recognises the same claim forms on both sides, bead text and spec text;
- (f) `--allow-not-ready` is scoped to named failing signals, needs an evidence file on every claim, and `mindspec bead ready-check` reads a validated override marker back, so the override log can be audited against the currently failing set and the override rule has one home (the binary, not `ms-bead-impl`'s prose);
- (g) ADR-0045 ("The binary owns sequence and rules; skills are harness adapters") lands, with its conflicts with ADR-0040 and ADR-0044 stated, so specs 130–133 can cite it;
- (h) panel review artifacts (`reviews/`) are an optional local helper, never a hard blocker: no review registration refuses `reattest` or the MF-3 read-back (R4.2), `mindspec init` gitignores the review directories, and no auto-committing verb commits them by default (R8).

## Background

### Provenance and scope arithmetic

Spec 128 (`128-lifecycle-verb-state-honesty`, merged 2026-10-18, PR #257) deferred, by name, a "spec-129 follow-on": GH #255 (readiness-truth face), #239, #232, #234, #244, #250. A consolidated diagnosis was reproduced at `e2325e36`; this spec re-verified every load-bearing claim, and the round-1 panel corrected the rest.

The cluster is split as follows. Every inherited issue has one named home:

- **Spec 129 (this spec):** six code requirements, R1–R6 (#255, the row caps, the claim/dispatch spec-consistency guard, #239 and #232, MF-2 symmetry, the override), plus R8 (review artifacts out of git by default, operator decision 2026-10-19). #239 is fixed as loud, named, and recoverable by one audited `mindspec reattest`, not as an automatic pass (R4.2), estimated at **9–12 beads** at plan time; the R6 relief-valve split is expected. R7 (ADR-0045) is a document that lands with this spec's commit. The ADR-0041 "Amendment (Spec 129)" is a document that lands with the R4 and R6 beads.
- **The follow-on spec (part 2; the next free plain number, never a lettered ID — Background K):** five deliverables:
  - G (#234);
  - H (#244);
  - #250 (the same family as #244: a multi-step terminal mutation with no intermediate stop);
  - I (the dead `merge_strategy` key);
  - phase-lineage unification (removing `FindEpicForBead`'s title fallback and tightening `beadIndex`).
- **Spec 130:** the S0 rule table (ADR-0045 Implementation order step 1), #233's review-to-landing binding (see D.5), and the rest of #251. #251 is the work that reached the spec tip inside another bead's branch with no merge naming it. R4.2 covers only the sub-shape described there.

Relief valve: if the plan gate judges 129 over-compressed, R6 (override UX) splits out as a further plain-numbered spec. R6 is coupled to R1–R5 only through the readiness report. R4 does not split cleanly, because R4.1 shares the plan-bead helper with R1.1.

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

The fix therefore must not rest on `--parent` or on the title substring (R1.1). On bd 1.1.0 at `e2325e36`, the reporter shape with a FULL-spec-ID title is already selected today: prefix `--parent`, plus a title that passes `containsSpecID`. The real-bd Validation Proof is therefore green today, and R1.1 hardens against the two variants that fail:
- an edge-only `--parent`;
- a short `[NNN]` title tag, which `FindEpicForBead`'s `Sscanf("%d")` resolves and `containsSpecID` drops.

AC-1 pins both variants red-on-revert.

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

Under the round-1 text, the fallback admitted that merge and the panel set confirmed it on `A`, which gave a false MF-3 PASS.

Round 3 found two more false PASSes, each with the panel excluded:
- the surviving `bead/X` branch reset to the stale tip `A`, which corroborates as (b);
- an agent-written binding naming the X→Y merge, which corroborates as (d).

A third reviewer built a fixture where same-second-parent grouping across independent sibling merges made an empty merge the content anchor, masking the revert of a real landing.

Three rounds, three new holes. The undirected form is a set that the mechanism cannot enumerate. So R4.2 no longer confirms automatically on the fallback path at all: it finds and names the candidate, and `reattest`, the audited path with its own git-derived refusals, is the only way to accept one.
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

**K. Spec numbering: no lettered IDs.** `idvalidate`'s spec-ID pattern allows one trailing letter (`129b-…`), but lineage breaks on such an ID:
- `internal/approve/spec.go`'s `parseSpecID` runs `strconv.Atoi` on the text before the first hyphen, so the epic gets `spec_num: 0`.
- `phase.FindEpicForBeadWithCache` then skips that parent epic (`num > 0` is required). Its title fallback `fmt.Sscanf("129b-…", "%d")` yields 129 and matches this spec's epic.

So a lettered successor's beads would resolve by lineage to spec 129. The panel proved this in round 3, and it is filed as **mindspec-wtjr**, a known defect that this spec does not fix. Successor specs therefore take the next free plain number.

Related, as measured by two panel slots: when two epics carry the same spec number, the epic chosen is nondeterministic across lookups, and the newest epic wins. That direction is fail-safe for selection, because members must resolve to the epic `--spec` resolved, and anything else is named N2. It is recorded with mindspec-wtjr for the follow-on spec.

**L. Review artifacts in git (verified 2026-10-19 on `spec/129-readiness-tracker-honesty`).**
- `git ls-files | grep -c '/reviews/'` → 1538. This spec's own `reviews/` (72 files) was committed by `mindspec spec approve`'s auto-commit at `7bae2664`.
- At `7bae2664`, `.gitignore` ignores only `**/reviews/*/dispositions.lock`. `bootstrap.starterGitignore` and `gitutil.RuntimeIgnoreEntries` name no review path.
- Every auto-commit stages with `git add -A` (`gitutil.CommitAll`; `gitutil.SnapshotWorktreeTree`'s `add -A -- .`) or an explicit pathspec (`gitutil.CommitPaths`). In a scratch repo, `git add -A` skips an ignored untracked `reviews/` file, `git status --porcelain` stays empty, and `git worktree remove` deletes ignored files with the worktree.
- `panelDirFor` (`cmd/mindspec/panel.go`) writes `<spec-dir>/reviews/<slug>` (flat) or `.mindspec/reviews/<slug>` (ad hoc), and `workspace.SpecDir` resolves the spec WORKTREE first.
- The operator's lane kit (an untracked local kit, not in this repository) already states "`reviews/` is never committed".

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
  - `internal/bootstrap/**`: R8.1, where `mindspec init` writes the review ignore entries.
  - `internal/instruct/**`: `templates/implement.md`, R1.4's L3 rule.
  - `internal/setup/**`: rewrite of `readiness_skill_pin_test.go`'s override-coverage pins, under the skill-refresh protocol.
  - `cmd/mindspec/**`: `next.go` flag shape, emptiness output and menu text; `bead_ready.go` marker read-back; `bead_clarify.go` text.
  - `plugins/mindspec/skills/ms-bead-impl/SKILL.md`: the override-coverage prose (a rule) is **removed**, and its logic moves into `bead ready-check`. The one remaining line, "run `mindspec bead ready-check`; proceed on exit 0", is marked as a staged rule under ADR-0045 Decision 5's transition clause, with target `next-action` (spec 133) (R6.5).
  - `.claude/skills/ms-bead-impl/SKILL.md` (installed copy): it is a stale July copy with no override prose (0 matches), so it is **refreshed** from the plugin source, not edited.
  - `internal/setup/historical_skills/**` (fenced): no existing snapshot is edited. Only the protocol's new snapshot of the replaced plugin text is added.
  - `plugins/mindspec/skills/ms-bead-cycle/SKILL.md`: the bare-form wording, if the plan's grep finds one (R6.5).
  - `internal/bead/clarify.go` and `internal/bead/bdcli.go` doc comments (not edited): the operator-reset analogy and `RemoveMetadataKeys`'s claim-failure note name the flag, not its bare form, and stay true under R6.
- **execution**:
  - `internal/bead/**`: R2's row-set waist (`ListJSON` and a new `ReadyJSON`); the shared plan-bead ID helper; R6's marker schema v2 constants and its validator in `readiness.go`.
  - `internal/gitutil/**`: R4.2's topo-ordered reachable-merge primitive beside `FirstParentMerges`; R8's review ignore-entry list.
- **core**: `.gitignore` only, for R8's two entries. `internal/phase` is consumed, not changed: `DerivePhaseDetail`, `Cache`'s child list, `IsLifecycleIssueType` and `FindEpicForBead` are read as they are. `phase.Cache`'s three `bead.ListJSON(…, "-n", "0")` calls keep working unchanged, because the R2.2 waist tolerates an explicit `-n 0`. The rejected Option W would have changed `FindEpicForBead`; see OQ-9.

Documentation surfaces touched are not Impacted-Domains entries. They are claimed by no OWNERSHIP.yaml and are governed by the ADR-0031 doc-sync gate. `.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md` is new and lands with this spec's commit, together with the "Amended by ADR-0045" back-pointers in ADR-0040 §1 and ADR-0044. ADR-0041's new § "Amendment (Spec 129)" lands with the R4 and R6 beads. R6.5's surfaces are `.mindspec/core/USAGE.md`, `.mindspec/core/WORKFLOW-STATE-MACHINE.md`, `.mindspec/domains/workflow/{architecture,interfaces,runbook}.md`, and `.mindspec/domains/execution/interfaces.md` (the marker schema row).

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — **AMENDED** by a new § "Amendment (Spec 129)". It is appended (not an in-place rewrite), follows the Spec-125 precedent, is cross-referenced from item (c), and is finalized by the bead that lands the citing code. AC-18 pins it. It covers three things:
  - **(i) Reachable-landing fallback (R4.2): a finder, not an identifier.** The Spec-125 text binds landed-ness to "a two-parent first-parent merge", and §2(ii) reads revert-awareness over "the first-parent chain SINCE the candidate merge".
    - **What it may do.** The amendment lets a fallback scan FIND and NAME a reachable, source-named merge when the first-parent scan finds none.
    - **What it never does.** No automatic path treats such a candidate as landed. It returns a typed off-first-parent result, and MF-3 fails with `mindspec reattest <id>`.
    - **Target.** It can be checked only for the explicit `… into <spec-branch>` forms. `gitutil.MergeInto`'s `Merge bead/<id>` encodes none.
    - **Acceptance.** Only the operator-invoked, audited `reattest` may accept a fallback candidate, after the git-derived fallback safety checks: (F1) exactly one admitted candidate; (F3) the surviving-tip contradiction; (F4) the net-effect/revert test at the current tip; (F5) read errors fail closed. F2 (a refusal on a visible unlanded review registration) is retired by operator decision: review registrations are advisory output and never refuse. reattest records an audited tuple: scanned branch, accepted merge SHA, accepted second parent.
    - **Read-back.** A later automatic read accepts the candidate only when a VALID audit's tuple equals the current binding pair and the scan-derived candidate, AND F1, F3, F4 and F5 pass again at read time. A stale audit left in place by a later `MergeMetadata` binding rewrite therefore grants nothing.
    - **Residual.** A metadata-only forge reaches only a candidate that reattest itself would accept, including one with a later unlanded review round. That reach (an undirected off-first-parent merge) is new and bounded, and AC-11 (ix-f) pins it.
    - **Exits.** The fallback's exit is the dependent's scoped override. q9ea is not a fallback exit.
    - **Earlier rules superseded.** The round-2 corroboration and tie-break rules for the fallback path are withdrawn. They departed from FIX-2b and from Spec 125's "fail-closed on every ambiguity", and they are moot here.
    - **CLOSE decisions.** `complete`'s merged-unclosed reconcile keeps the first-parent scan only.
  - **(ii) Non-equal panel datum (R4.3).** Item (c) already reads "a registered panel's `reviewed_head_sha` equal to an exact match's second parent". The amendment makes the non-equal case explicit for `FindLandedMerge` and `reattest`: a registration whose SHA does not equal the second parent is NON-CORROBORATING. It neither confirms nor contradicts.
    - This reaches the CLOSE paths, and it is safe. A stale SHA used to contradict. Now an exact tip, binding or panel match confirms, which is the evidence class Spec 125 already admits; otherwise the result is still a refusal (`*LandedMergeNoEvidence`), never a close.
    - A contradictory landed binding still vetoes automatic identification. Only an explicit `reattest` may replace it, with the scan-derived identity, after its own checks pass, and with the prior pair audited.
  - **(iii) Fourth-verb clause (R6).** `--allow-not-ready` becomes signal-scoped and needs an evidence file on every claim that records an override. It refuses when a non-named signal fails, and coverage is bound to per-record digests of the signal's complete failure set (growth uncovers; shrinkage stays covered). A named signal carrying an `indeterminate` record cannot be overridden. An existing marker never authorizes a claim.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED. Every new refusal ends with a `recovery:` line:
  - R1.3's selector refusals use `recovery: mindspec next --spec <spec-id>`.
  - R1.2's near-misses use `bd update <id> --parent <epic-id>` only for N1: a plan-form bead with no parent whose lineage does not resolve. That is the one recovery shown to yield a claimable bead. Otherwise they get the inspection-first `bd show <id> --json` (Spec-127 amendment item 2; never a reparent over a parent the agent did not name).
  - R1.4 uses `bd blocked --parent <epic-id>`, which exists in bd 1.1.0.
  - R6's scoped recovery carries `--evidence <path-to-justification>`, a literal placeholder. It is marked non-mechanical, because the operator authors the evidence.

  New emitted commands are registered where the `internal/guard` recovery-convention lint requires. HC-4: an empty-set `next` with no selector and no BLOCKING near-miss mutates nothing and exits 0. The one exception is R1.4's in-progress arm, which keeps today's worktree recovery.
- [ADR-0042](../../adr/ADR-0042-render-derivation-provenance.md) — APPLIED. IDs render via `idrender.Bead`/`idrender.Spec`; bd stderr (R2.3) and the evidence path (R6) via `termsafe.Escape`. **No dynamic operand appears in command-shaped text.** Recovery lines carry only idrender-gated IDs, closed-set signal IDs, and literal placeholders, following the `internal/next/select.go` final-review R5/G4 rule. The operator's evidence path is never interpolated into a recovery line.
- [ADR-0025](../../adr/ADR-0025-jsonl-as-build-artifact.md) — RESPECTED, with the consequence stated: the R6 marker is bd metadata, so it IS exported into the git-tracked `.beads/issues.jsonl` and pushed. For that reason the marker stores the evidence file's path, sha256 and byte count, **never its text**. The rendered path is the only operator-supplied string that reaches git, so R6.5's guidance recommends a repository-relative path.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED. `internal/validate/readiness` stays inside the boundary: R4.1's membership reads go through `internal/bead`, and its landed-merge read through `internal/lifecycle`. R4.2's primitive lands in `internal/gitutil`. The R2 waist stays in `internal/bead`.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — RESPECTED. `internal/panel` is untouched. Agent-writable data (panel.json, bd metadata) stays inside §8's anti-footgun boundary. No step here claims to authenticate it; R6 validates its shape only. R8 changes only whether review files are tracked: registration, rounds, threshold, staleness, dirty-tree and fail-open/closed semantics are untouched, and every gate reads registrations from the filesystem.
- [ADR-0039](../../adr/ADR-0039-flat-layout-v2.md) — RESPECTED. §4's co-located location (`.mindspec/specs/<id>/reviews/<panel-slug>/`) and its layout-aware gate scan are unchanged. R8 ignores that location; it moves nothing.
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

1. **One claim predicate: the plan bead.** A bead is a **plan bead of spec S** (epic E) iff all three of the following hold:
   - (a) its ID has the plan-bead form `<E>.<N>` (N ≥ 1, one segment). This is checked by one exported helper in `internal/bead` (working name `bead.PlanBeadIndex(epicID, id) (n int, ok bool)`), shared with R1.8 and R4.1;
   - (b) `phase.FindEpicForBead(id)`, which is unchanged and is the resolver `readiness.EvaluateReadiness` and `complete` use, returns E. This is checked for every plan-form row. There is no parent-E shortcut, so a bead with parent E and a conflicting epic dependency that `FindEpicForBead` resolves first is excluded and named (N2c), never selected and then refused;
   - (c) `plan.md` maps N. "Mapped" is MF-1's exact structural precondition, through ONE shared helper, `readiness.PlanMapsBead(planContent, n) (mapped bool, why string)`, which both selection and `evaluateMF1` call. N is mapped iff BOTH of these hold:
     - the plan has at least N `## Bead ` headings. The count is POSITIONAL: `extractBeadSectionRaw` takes the Nth heading by count and ignores the number written in it, so a plan whose headings are numbered non-contiguously maps by count. `PlanMapsBead` counts headings EXACTLY as `extractBeadSectionRaw` does today: every line beginning `## Bead `, including one inside a fenced code block. Selection and MF-1 therefore cannot disagree even on that edge;
     - `work_chunks` carries an entry with id N.

     A half-mapped N (a section but no chunk, or a chunk but no section) is unmapped, which is class N4. MF-1's remaining checks (a concrete AC entry, non-empty `key_file_paths`) stay in `evaluateMF1`. They are content, not mapping, so a mapped bead can still fail MF-1 as NOT-READY. That is a named readiness failure, not a selection lie.
   - **Where `plan.md` is read from, and when.**
     - **Source.** The committed `spec/<S>:<path>` blob, read through `gitutil` from `internal/next` (outside the ADR-0030 enforcement packages), whenever `spec/<S>` exists AND carries the blob. Otherwise the invoking checkout's `workspace.SpecDir(root, S)/plan.md` is used. That includes the case where the branch exists but `plan.md` is not committed on it yet.
       - **Telling absent from failed.** `gitutil.FileAtRef` wraps every `git show` failure the same way, so it cannot tell "absent" from "failed". The read therefore uses a typed probe:
         - first `git rev-parse --verify --quiet refs/heads/spec/<S>` (ref present or absent);
         - then `git ls-tree spec/<S> -- <path>` (empty output means the path is absent; a non-zero exit is a failure);
         - then the blob read.

         This is a new `gitutil` helper returning a typed `ErrPathNotAtRef` (working name). Only "ref absent" or "path absent" falls through to the checkout. Any other git error refuses fail-closed.
       - When both exist and differ, one line is printed: `plan.md in this checkout differs from spec/<S>; mapping used: spec/<S>`. Without it, a bead worktree whose `plan.md` predates an amendment would silently classify differently from the spec worktree, and `--emit-only` has no gate to catch that.
       - `EvaluateReadiness` still reads the invoking checkout's file. Selection equals readiness only when the two read the same content, and the divergence notice says when they do not. Aligning readiness's source is a plan-stage decision (OQ-10).
     - **Laziness.** The plan is read when the classification of any plan-form `<E>.<N>` row depends on the mapping. That is every plan-form row not already classified N2a or N2c, which includes rows with no lineage and rows whose title-fallback lineage is another epic (N1, N2b and N4 all need it). No read happens when there are no plan-form rows, or when every plan-form row is N2a or N2c. A plan-phase spec with no children reads nothing.
     - **Absence.** If `plan.md` is absent in both places, no N is mapped. Every plan-form row is then N4 (`no plan.md exists yet for <S>`); this is informational, not a refusal.
     - **Other errors.** Any other read error (permissions, a git failure, unparseable frontmatter) refuses fail-closed.
     - **Claim path from a checkout without, or with a different, `plan.md`.** Selection can then name a member that `GateReadiness` refuses with readiness's existing "reading plan.md" error, or with an MF-1 failure. The divergence notice above names the cause. That readiness behaviour is unchanged from today, and its recovery is to run from the spec worktree.

   bd mints `<E>.<N>` for every `bd create --parent E` whatever its type, so ID form plus lineage alone cannot tell a plan-approve bead from a follow-up.
   - **Example of a follow-up:** `mindspec-cter.8`, a bug under spec 093's epic, whose plan has 7 `## Bead` sections.
   - **Counter-example:** `mindspec-jfgj.4` is typed `bug`, yet it IS spec 073's mapped Bead 4. This is why type is not a membership input.

   The scoped set:
   - It comes from ONE uncapped global `bd ready -n 0 --json`. It does not use `bd ready --parent`, whose edge-or-prefix semantics differ between bd versions (Background A), and it does not use the title substring.
   - A row is a **member** iff it is not an epic, is open, and is a plan bead of S. Members keep bd's returned order, so `--pick N` parity holds by construction.
   - Extra cost: one `bd show` per plan-form row for (b), with the epic list cached in `phase.Cache`, plus at most one lazy `plan.md` read (R1.1 (c)).
   - A real lineage read error, or a `plan.md` read error other than absence (R1.1 (c)), refuses the whole command fail-closed (`could not resolve the lineage of <id>` / `could not read plan.md for <S>`). It never silently drops a row.

   `issue_type` is not a membership input: #255's own bead was a `bug`, and a mapped plan bead of any type is selectable. Neither the title nor `metadata.spec_id` is a membership input. *Rejected alternative:* keeping unmapped `<E>.<N>` children as members and default-picking lifecycle members first. MF-1 refuses every unmapped bead, so such a member could never be claimed without an override. Listing it as selectable would be the dishonesty R1 removes.

   The honest limit: `readiness.beadIndex` accepts any trailing `.<N>`, so readiness also evaluates a `<other>.<N>` bead whose title-fallback lineage is S. `next` does not select that bead (N3). Tightening `beadIndex` is the follow-on spec's phase-lineage unification.
2. **Near-misses are named, on every scoped call.** A **near-miss** is a non-epic, non-member row of the same global set with any of these links to S:
   - a plan-bead-form ID for E;
   - row `parent` E;
   - a title containing the spec ID;
   - row `metadata.spec_id` equal to S.

   Each near-miss is printed by ID with its true reason and one recovery line:
   - **(N1)** Plan-bead form, N mapped, no `parent`, and `FindEpicForBead` returns `ErrNoEpicLineage`:
     - message: `bead <id> has the plan-bead form for epic <E> and plan.md maps Bead <N>, but it has no lineage (no parent edge and no spec tag in its title)`;
     - recovery: `bd update <id> --parent <E>`, shown to make the bead claimable (AC-2).
   - **(N2a)** Plan-bead form, the row's `parent` is a NON-EPIC bead P the agent did not name. A parent that is an epic E2 is N2c, through its parent-child dependency:
     - message: `… but its parent is <P>`;
     - recovery: `bd show <id> --json`. A named parent is never overwritten.
   - **(N2b)** Plan-bead form, N mapped, no `parent`, and `FindEpicForBead` resolved another epic E2 through the TITLE fallback. The message is `… but its lineage resolves to <E2> by title number <NNN>`. Two sub-cases follow.
     - **E2 carries S's own spec number** (a duplicate epic for the same spec, Background K):
       - recovery: `bd update <id> --parent <E>`;
       - this is safe because no parent is overwritten, and an epic dependency wins over the title in `FindEpicForBeadWithCache`, so the edit also corrects readiness and `complete`.
     - **E2's number differs from S's** (the title names another spec; the bead may have been minted under E by mistake):
       - first line: `recovery: bd show <id> --json`;
       - second line: `recovery (only if <id> belongs to <S>): bd update <id> --parent <E>`.
   - **(N2c)** Plan-bead form, `FindEpicForBead` resolved another epic E2 through an epic DEPENDENCY (including a parent-E row whose conflicting epic dependency is returned first):
     - message: `… but its lineage resolves to <E2> through its dependency on epic <E2>`;
     - recovery: `bd show <id> --json`.
   - **Classification order** (first match wins): N2a, then N2c, then N4 for any unmapped N whatever the title lineage, then N2b, then N1, then N3.
   - **(N4)** Plan-bead form, but N is NOT mapped by `plan.md`:
     - message: `bead <id> is a follow-up child of epic <E> that plan.md does not map (<why>: e.g. "plan.md declares 7 ## Bead sections" or "no work_chunks id 8"); readiness would fail MF-1, so mindspec next does not select it`;
     - when the lineage is none, it adds: `; it also has no lineage, so readiness cannot evaluate it at all`;
     - recovery: `bd show <id> --json`.
     - No "makes it claimable" recovery is offered. The deliberate route is the scoped override (`--allow-not-ready=MF-1 --evidence …`, R6) by positional, or working the bead outside `mindspec next`.
   - **(N3)** Any other link:
     - message: `bead <id> is bd-ready and linked to spec <S> by <parent edge | title | metadata.spec_id>, but it is not a plan bead (no <E>.<N> ID with lineage <E>); mindspec next claims only plan beads`;
     - recovery: `bd show <id> --json`. This is non-mechanical.
     - The sentence holds on every `next` path, because R1.8 applies the same predicate to every claim.

   Near-misses never change selection, are never default-picked, and are printed whether the member set is empty or not. A near-miss is **blocking** if it is N1, N2a, N2b, N2c, or an N3 whose row `parent` is E and whose type is lifecycle (`phase.IsLifecycleIssueType`). It is **informational** if it is N4 (a follow-up child), a title-only N3, a metadata-only N3, or a reparented non-lifecycle child. Informational near-misses print the same lines and never change the exit code.
3. **A selector `next` cannot honour is refused, not ignored.** On the claim path, with `--spec`:
   - **Empty member set, positional ID.** Exit non-zero with `no ready plan bead for spec <S>; positional <id> cannot be claimed from an empty ready set`.
   - **Empty member set, `--pick N`.** Exit non-zero with `--pick <N> is out of range: 0 items`.
   - Both of these end with `recovery: mindspec next --spec <S>`.
   - **Near-miss positional.** Whether the member set is empty or not, this refuses with its R1.2 sentence. The exception is an N4 whose lineage resolves to E: that positional is passed to `GateReadiness`, which refuses it NOT-READY on MF-1. A scoped, evidenced `--allow-not-ready=MF-1` (R6) can then claim it deliberately.
   - **N4 with no lineage.** This positional refuses at selection, before any mutation: `bead <id> has no lineage, so readiness cannot evaluate it; no readiness override applies`, with `recovery: bd show <id> --json`. No reparent is offered, because the bead is unmapped. Such a bead is worked outside `mindspec next`.
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
      - ≥1 open lifecycle child and none in progress → `<n> open lifecycle bead(s), none selectable (any that are bd-ready but unmapped or unlinked are named above)` and `recovery: bd blocked --parent <E>`. The child-derived phase here is `implement` if any child is closed and `plan` if none is;
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
   - (d) Before any mutation, `spec/<T>` must resolve (`gitutil.RevParseRef`, through `internal/lifecycle` or `cmd`, never from `internal/validate`). Otherwise it refuses: `spec branch spec/<T> for bead <id> does not exist (finalized and deleted, or never created); nothing was claimed`, with `recovery: mindspec state show`. This prevents the claimed-but-homeless state that a later `CreateBranch` failure would leave (ADR-0041).

   **Behaviour change, deliberate.** Some beads that today reach `GateReadiness` are now refused by (a): a lineage-resolved bead that is not of the form `<E_T>.<N>`, such as a reparented non-dotted bug, or a `<other>.<N>` with a `[NNN]`-tagged title. A bug bead with no epic lineage at all is already refused today: `EvaluateReadiness` returns `ErrNoEpicLineage` as an error (`internal/validate/readiness/readiness.go`), whatever `advancePhaseOnClaim`'s comment in `cmd/mindspec/next.go` suggests. That comment is corrected under R6.5's doc surfaces. For such beads the alternatives are to work them outside mindspec (`bd update <id> --claim` and a manual branch), or to fold the work into a plan bead.

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
   - A dependency held to the test only by (ii) or (iii), and not by plan-FORM (i), adds this hint to its failure, together with the scoped override line (R6): `<D> is linked to spec <S> by <metadata.spec_id | parent edge> but does not have the plan-bead form; if its work never landed as bead/<D>, override deliberately`.
   - So does a plan-form dependency that `plan.md` does not map (an N4 follow-up, closed after being worked outside `mindspec next`): `<D> has the plan-bead form but plan.md does not map it (a follow-up); if it was worked outside mindspec next and never landed as bead/<D>, override deliberately`. Such a follow-up is now the likeliest benign MF-3 failure.
   - A non-closed dependency of either kind fails as today.
2. **Reachable-landing fallback: it finds and names a candidate, and never auto-confirms.** Candidate generation keeps the first-parent scan as primary, unchanged. A fallback scan runs only when the primary scan nominates **zero** owned two-parent candidates. It uses a new `gitutil` primitive: `git log --merges --topo-order <spec-branch>`, newest-first in topological order, over every merge reachable from the tip.

   **Admitted subjects.** The merge must have two parents, and its subject must name `bead/<id>` as the merged SOURCE, in one of these forms:
   - `Merge bead/<id>`: `gitutil.MergeInto`'s form, which is **undirected** (it names no target);
   - `Merge branch 'bead/<id>' into <spec-branch>`;
   - `Merge bead/<id> into <spec-branch>`.

   The last two are admitted only when `<target>` is this spec branch. mindspec writes `Merge <source> into <target>` only through `gitutil.MergeBranch`, for the spec→main finalize, so the last form is defensive.

   **Excluded:** a merge INTO `bead/<id>` (a refresh), an explicit `… into <other branch>`, and an unrecognised format. The G2-2 any-token fallback in `parseMergeSubjectBeadBranch` does not apply.

   **The automatic paths never accept a fallback candidate.** `lifecycle.FindLandedMergeReachable`, the MF-3 read, never returns an identified merge for one, whatever the panel set, the surviving tip or the binding says. It returns a typed `*LandedMergeOffFirstParent`, which:
   - wraps `ErrLandedMergeNotFound`, and never `ErrLandedMergeNoCandidate`;
   - names the bead, the spec branch, every admitted candidate's merge SHA and second parent, and the exact recovery `mindspec reattest <id>`.

   MF-3 FAILS with that result. Three consecutive panel rounds each found a new automatic false PASS here:
   - (round 2) the panel set alone;
   - (round 3) a surviving branch reset to a stale round tip, or an agent-written binding naming a merge into a sibling bead branch;
   - (round 3) same-second-parent grouping that let an empty sibling merge's anchor mask the revert of a real landing.

   The undirected subject cannot be made safe by enumerating corroborations. #239 is therefore fixed as **loud, named, and recoverable by one audited command**, not as an automatic pass.

   **Fallback safety checks (F).** Both `reattest` (acceptance) and every automatic read-back (below) run F1, F3, F4 and F5. All four are derived from git; none reads bd metadata or a panel registration. F2 (a refusal on a visible unlanded review registration) is retired by operator decision (grill record, 2026-10-19), and the remaining labels are not renumbered. Every check refuses fail-closed:
   - **(F1) Exactly one admitted fallback candidate.** Two or more refuse, even if they share a second parent, because independent sibling merges are not a re-merge chain. There is no grouping and no tie-break, so an empty sibling merge can never become the content anchor that masks a revert. The refusal state is a NEW `ReattestStateMultipleFallbackCandidates` (`"multiple-fallback-candidates"`). `ReattestStateAmbiguous` (`"ambiguous-owned-second-parents"`) keeps its primary-path meaning, so no existing value changes.
   - **(F3) Spec-125 surviving-tip contradiction.** A surviving bead branch whose tip is not the candidate's second parent refuses.
   - **(F4) Revert/net-effect at the CURRENT tip.** This is the Spec-125 R5(d) test anchored on the single candidate, run at read time. A landing accepted earlier and reverted since therefore fails again with the reverted sentence.
   - **(F5)** Read errors in any of the above are errors, never a pass.

   **Registrations are advisory.** On a fallback candidate, `reattest` scans `panel.ForBead` in the invoking checkout's roots (`workspace.FindRoot(cwd)` and the spec dir, as R4.3 does). For each class-(c) `reviewed_head_sha` (R4.3) that is not an ancestor of the tip it prints `advisory: registration <dir> (round <r>) reviewed <sha>, which is outside <spec-branch>'s tip history`, then once `advisory: panel roots scanned: <root>, <spec-dir>`. An absent object or a failed ancestry probe prints `(not determinable locally)` in place of the verdict. A malformed value prints `advisory: registration <dir> carries a malformed reviewed_head_sha; ignored` and never reaches git. Every rendered value is termsafe-escaped. No advisory changes the exit code or what is written. So the O3 shape (AC-11 (vii)) is ACCEPTED, with the advisory.

   **Exits when a check refuses.** The one audited forward exit that can actually change the outcome (ADR-0041 §2(i)) is for the dependent bead: `recovery: mindspec next <dependent> --allow-not-ready=MF-3 --evidence <path-to-justification>` (R6). The q9ea attested-restore is NOT listed here. Restoring `bead/<id>` at the second parent satisfies only F3, and the fallback never identifies from a tip, so it changes no fallback refusal. It stays on the primary-path `*LandedMergeNoEvidence` arm, where it works. MF-3's off-first-parent message (R4.4 arm 1b) lists the override as the second exit, after `mindspec reattest`.

   **The audited tuple.** On acceptance of a fallback candidate, `reattest` writes three new keys, alongside the existing audit keys:
   - `mindspec_landed_reattest_merge_sha`;
   - `mindspec_landed_reattest_second_parent`;
   - the existing `mindspec_landed_reattest_scanned_branch`.

   It writes them in the same `MergeMetadata` call as the binding. On a fallback acceptance, `mindspec_landed_reattest_corroboration` records the provenance, so an inspector of the JSONL history can tell a fallback acceptance from a primary-path one without re-running git:

   ```
   off-first-parent fallback; F1, F3, F4, F5 passed; advisory registrations outside tip: <n>; panel roots scanned: <roots>
   ```

   **Convergent no-op.** The re-run is a no-op ONLY when the stored audit is VALID (the validity contract below) AND the audit tuple equals both the binding and the candidate. If the audit is absent, MALFORMED (for example a missing `op`, `at` or `actor`), or its tuple differs, reattest re-runs F1, F3, F4 and F5 and writes a valid record, even when the binding values are unchanged.

   **Read-back.** An automatic read identifies an off-first-parent candidate iff ALL of the following hold:
   - the audit record is VALID: `op` equals reattest's op constant (`"mindspec reattest"`); `at` parses as RFC-3339; `actor` is non-empty; `scanned_branch` equals this spec branch; and both SHA keys are well-formed git object IDs. Anything else is MALFORMED and grants nothing;
   - audit tuple == current binding pair == the single scan-derived candidate (merge SHA and second parent);
   - F1, F3, F4 and F5 all pass at read time; no registration is read.

   **Why a stale audit can never confirm.** The executor's merge-time write and reattest's write both go through `bead.MergeMetadata` (read-merge-write). A later executor rewrite of the binding therefore leaves an earlier audit in place. That stale audit names a different pair than the binding, so it confirms nothing. No executor change is needed.

   **Residual, stated honestly.**
   - **What a forge needs.** The audit and binding are agent-writable bd metadata, exported into the tracked JSONL, and one `bd update --metadata` call can write all of them.
   - **What a forge gains.** F1, F3 and F4 are re-derived from git on every read, so a forge of the binding and audit can make MF-3 accept only a candidate that `reattest` itself would accept: a real, reachable, source-named, single, non-reverted fallback merge with no contradicting surviving tip. That includes AC-11 (vii)'s O3 shape. The forge is visible in the tracked JSONL history, and panel files play no part in it.
   - **NEW REACH, stated as such.** The reach is an undirected off-first-parent merge, for example one made into a sibling bead branch. Spec 125 fails such a merge today. The reach is new and bounded, and AC-11 (ix-f) pins it as documented.

   **`reattest` on a fallback candidate** runs every Spec-125 check, then F1, F3, F4 and F5, then writes the binding and the audited tuple. Before the write, its output states `candidate <sha> reached <spec-branch>'s tip off the first-parent chain; its subject names no target`, followed by the advisory lines. A refusal writes nothing. These checks are scoped to FALLBACK candidates; the primary path's reattest is unchanged.

   **#251 boundary.** A prerequisite that reached the tip inside another bead's branch through an admitted short-form `Merge bead/<id>` is nominated and becomes recoverable by `reattest`, subject to F1, F3, F4 and F5. The long form `Merge branch 'bead/X' into bead/Y` names a non-spec target, so it is excluded. Branch-from, cherry-pick or rebase arrivals (#251's literal report) stay with spec 130.

   **Doc contracts.** The `SubsumptionConflict` rationale becomes "M is an ancestor of the tip". The `FirstParentMerges` and `ErrLandedMergeNoCandidate` doc contracts are updated to match.

   Callers, and what changes for each. This table covers R4.2. R4.3's set semantics live inside `FindLandedMerge`, so they reach every caller below (see R4.3).

   | Caller | Change |
   |---|---|
   | MF-3 (`readiness.findLandedMergeFn`) | gains the fallback as a FINDER only (`lifecycle.FindLandedMergeReachable` → `*LandedMergeOffFirstParent`, or identified only via a reattest-audited binding) |
   | `lifecycle.ReattestLandedMerge` | nominates through the same fallback; accepts only after F1, F3, F4 and F5, and writes the audited tuple |
   | `lifecycle.MergedUnclosed` → `complete`'s merged-unclosed reconcile (CLOSES beads, cleans branches) and `stale_open.go` | **unchanged**, first-parent only (an ADR-0041 §2(ii) CLOSE decision) |
   | `orphan_hints.go` `findLandedMergeFn` | unchanged |
   | `approve/plan.go` `planFindLandedMergeFn` | unchanged |
   | `approve/adopt_lattice.go` `adoptFindLandedMergeFn` | unchanged |
   | `executor` binding write (`gitutil.ExactSecondParentMerges`) | unchanged: the merge was just made by `gitutil.MergeInto` and is the first-parent tip by construction |
   | `executor/merge_resumption.go` (`FirstParentMerges(HEAD)`) | unchanged |
3. **Panel-leg set semantics, in both functions.** `reviewedHeadSHAForBead` becomes a SET: every class-(c) `reviewed_head_sha` across all registrations `panel.ForBead` returns. The value classes are: (a) empty, skipped; (b) placeholder (for example `"design-review-no-code"` in `.mindspec/reviews/6ou2-fix-zfc/panel.json`), skipped; (c) a `wellFormedGitObjectID`, a set member; (d) option-like or a revision expression (`^`, `~`, `:`, `@{`, `..`, whitespace, a control byte), malformed. Classes (a), (b) and (d) are non-corroborating, never git operands, and refuse nothing. In `FindLandedMerge` and in `ReattestLandedMerge`'s datum (c):
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
   1b. `*LandedMergeOffFirstParent` (R4.2) → `a merge <sha> naming bead/<D> is reachable from <spec-branch>'s tip but off its first-parent chain; automatic identification never accepts it without an audited reattest`. Its recovery lines, in order:
       - `recovery: mindspec reattest <D>`;
       - `recovery (if reattest refuses): mindspec next <this bead> --allow-not-ready=MF-3 --evidence <path-to-justification>`.

       The q9ea attested-restore is not offered on this arm (R4.2 Exits).

       Because of spec 124 R1's one-line rule, the extra exits are rendered as Detail lines; `Recovery` carries the first.
   2. `errors.Is(err, lifecycle.ErrLandedMergeNoCandidate)` → the ONLY arm allowed to say no landing exists. It reads `no two-parent merge reachable from <spec-branch>'s tip lands bead/<D> (first-parent scan, then the reachable-landing fallback)`, followed by today's two-possibility wording.
   3. A corroboration contradiction → `a merge landing bead/<D> exists (<sha>) but <datum> contradicts its second parent <sha>`.
   4. A revert shape → `merge <sha> landing bead/<D> was reverted or cleanly removed after landing`.

   Each arm ends with its own recovery: inspection first, and never `mindspec complete` for a bead whose merge exists. The `ErrLandedMergeNoCandidate` doc comment is updated to the same predicate.

   **Aggregated failures (R6.2) keep spec 124 R1's one `recovery:` line per failing signal.** `Signal.Detail` lists EVERY failing dependency with its class and its own recovery text. `Signal.Recovery`, the one line `RecoveryCommands` consumes, is that of the highest-precedence record, in this order:

   `indeterminate` > `not-closed` > `no-candidate` > `off-first-parent` > `no-evidence` > `contradiction` > `reverted`

   An infrastructure failure is fixed first, unfinished work before uncorroborated work, and inspection before any repair. Within one class the order is by dependency ID.
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
   - **Indeterminate failures.** If a NAMED failing signal's failure set contains an `indeterminate` record (R6.2), the scoped override refuses before any mutation with `<MF-x> cannot be overridden: <record> could not be evaluated; fix the read first`. Claiming anyway would leave a bead whose `ready-check` can never pass.
   - The BARE flag, and the space-separated `--allow-not-ready MF-3` that cobra's `NoOptDefVal` parses as bare-plus-positional, always refuse before positional resolution. `MF-3` is never treated as a bead ID. The recovery is the scoped line above, or `recovery: mindspec next <bead-id>` when nothing fails.
   - `--allow-not-ready=all` is NOT admitted (OQ-5).
2. **Fresh evidence on every claim; an existing marker never authorizes one.** A scoped override that records ≥1 signal REQUIRES `--evidence <path>`: a readable regular file of 1 byte to 64 KiB, read before any mutation. A missing `--evidence` then refuses before any mutation. `RecordReadinessOverride` REPLACES the marker (never unions it) with schema v2:

   ```
   {schema: 2, signals: [failing ∩ named], fingerprints: {<id>: [sha256(record), …]}, overridden: <RFC-3339>, evidence: {path, sha256, bytes}}
   ```

   **The fingerprint covers a signal's complete failure set** (this prices a `readiness` API change):
   - `readiness.Signal` gains `Failures []string`: canonical, raw (pre-render) failure records, one per failing item. Each `evaluateMF*` aggregates **all** failures instead of returning on the first.
     - MF-1: one record per missing structural element.
     - MF-2: one per dangling token.
     - MF-3: one per failing dependency, `<dep-id>:<class>[:<candidate-sha>]`. MF-3 therefore evaluates every closed member dependency, at one landed-merge evaluation each, where it used to stop at the first failure.
     - MF-4: one per blocking OCCURRENCE, `<kind>:<source>:<section>:<k>:<sha256 of the normalised line>`, where:
       - `<kind>` is `token` or `checklist`;
       - `<source>` is `description` or `plan`;
       - `<section>` is the plan heading path, or `-` for the description;
       - `<k>` is the 1-based occurrence index among identical `(kind, source, section, hash)` tuples.

       So two standalone `TBD`s in different places, or a repeated checklist item, stay distinct records. Multiplicity is retained and nothing is collapsed.
   - Records carry no free-form error text, timestamps or bd stderr. They are sorted, and the identity rules above keep them unique.
   - `readiness.FailureRecordDigests(Signal) []string` returns the sorted sha256 of each record. It is ONE function used by both the writer (`RecordReadinessOverride`'s caller) and the reader (`ready-check`).
   - An MF-3 record of class `indeterminate` (a membership or landed-merge read error) is **never coverable**: an override never covers an infrastructure failure.
   - `Detail` stays the human text, and may list every failure.

   The evidence **text is not stored** (ADR-0025 touchpoint). The marker-before-claim, fail-closed, rollback-on-claim-failure contract (settled fact 2) is unchanged.
   - A re-claim after `mindspec release` needs the flag and evidence again. `release` does not clear the marker, and it does not need to, because no gate reads it for claim authority.
   - With an existing marker covering MF-2 and MF-3, a new request naming only MF-3 while both fail still refuses.
   - The marker is read in `cmd/mindspec` and passed into the report as data. `readiness.EvaluateReadiness` never reads it (spec 124 AC-12, `internal/complete/readiness_isolation_test.go`).
3. **One validity contract, fail-closed.** A shared validator in `internal/bead` (beside the marker constants) classifies a marker as VALID, LEGACY or MALFORMED:
   - VALID: schema 2; `signals` a non-empty, duplicate-free subset of the closed set; for each signal, one `fingerprints` entry that is a non-empty, sorted, duplicate-free list of 64-hex digests; `overridden` RFC-3339; `evidence.sha256` 64-hex; `evidence.bytes` in [1, 65536]; `evidence.path` non-empty.
   - LEGACY: no `schema` key, `{signals, overridden}` only. This is today's writer, and such markers are already in the tracked JSONL.
   - MALFORMED: anything else.
   - A metadata read error is an error, never "no marker".

   Integrity is shape and self-consistency only. The marker is agent-writable bd metadata and is not authenticated (ADR-0037 §8).
4. **`ready-check` honours only VALID markers, as the dispatch-ingress answer.** `mindspec bead ready-check <id>` reads the marker. A failing signal X is COVERED iff three things hold:
   - the marker is VALID;
   - X is in `signals`;
   - every digest in `FailureRecordDigests` of X's CURRENT failure set is in `fingerprints[X]`.

   **Growth uncovers; shrinkage stays covered.** An override recorded while dependency A failed does not cover A+B, and an MF-4 override for one blocker does not cover a second blocker. When the set shrinks from A+B to A (B landed), the override still covers it, because every remaining failure was vouched for. Rendering:
   - covered → `MF-x: OVERRIDDEN (recorded <ts>; evidence <sha256-prefix> <escaped path>)`;
   - recorded but now passing → `MF-x: PASS (override recorded but no longer needed)`;
   - LEGACY → `override marker is legacy (no evidence) — not honoured`;
   - MALFORMED → `override marker is malformed — not honoured`.

   Exit codes: 0 iff every signal passes or every failing signal is covered; otherwise non-zero naming each UNCOVERED signal; a metadata read error refuses non-zero.

   **Re-recording for a claimed bead.** `mindspec next` cannot re-select an in-progress bead, so the LEGACY, MALFORMED and uncovered arms end with two recovery lines:
   - `recovery: mindspec release <bead-id>` (removes the worktree, keeps the bead branch, returns the bead to open);
   - `recovery: mindspec next <bead-id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>`.

   Two related lines also appear:
   - The diagnostic line before them names the UNCOVERED signals (`uncovered: MF-2`).
   - The re-claim names ALL currently failing signals, covered or not, because the old marker grants nothing and is replaced, never unioned (R6.1/R6.2). An explicit re-claim naming only the uncovered signals refuses under R6.1.

   The re-claim reuses the surviving bead branch. This exit code replaces `ms-bead-impl`'s prose comparison (R6.5). It is never claim authority: only R6.2's fresh flag and evidence can authorize a claim.
5. **One home: guidance follows, the skill copy goes.** `plugins/mindspec/skills/ms-bead-impl/SKILL.md`'s override-coverage paragraph is replaced by one line: run `mindspec bead ready-check <id>`, proceed on exit 0, otherwise stop and surface its report. That line is a conditional sequence added after ADR-0045 is accepted. It is therefore NEW staged content: the transition clause grandfathers only content that already existed, so Decision 5's marking duty applies now. The line carries an inline staged-rule marker in `SKILL.md` itself:

   ```
   <!-- staged-rule: target=next-action (ADR-0045 step 4, spec 133); remove-by=2027-06-30 (provisional; the spec-130 rule table may revise it) -->
   ```

   The marker names the target and a removal date. Only the `doctor` date check waits for the rule table (the transition clause).

   The rule itself (coverage) has moved into the binary, and only the call remains staged. The installed `.claude/skills/` copy is refreshed. `internal/setup/readiness_skill_pin_test.go`'s `TestMsBeadImplSkill_OverrideCoverage{CheckShipped,MutationProbe}` are rewritten to pin the new line, under the skill-refresh protocol (the replaced text is snapshotted for existing installs). The bare/all-or-nothing wording is replaced by the scoped form in:
   - `cmd/mindspec/bead_clarify.go`;
   - `internal/next/ready_gate.go`'s NOT-READY recovery string;
   - `cmd/mindspec/next.go`'s record-failure recovery and flag help, and `advancePhaseOnClaim`'s comment about non-lifecycle claims (R1.8);
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

   Re-pointed 2026-10-19: the rule table moves to spec 131; #233's reviewed-tree check and #251 move to the spec-129 follow-on

### R8 — Review artifacts stay out of git by default

Panel reviews are an optional local helper (operator decision, 2026-10-19). The operator's lane rules (an untracked local kit) already state "`reviews/` is never committed"; R8 makes the binary agree.

1. **`mindspec init` ignores the review directories.** A new list `gitutil.ReviewIgnoreEntries` holds exactly two patterns, `.mindspec/specs/*/reviews/**` and `.mindspec/reviews/**`. The `/**` form, unlike a directory form, lets R8.4 option (B) re-include a file by appending negation lines. `bootstrap.Run` passes it to `gitutil.EnsureGitignoreEntries` after the runtime entries. The patterns are anchored to the paths `panelDirFor` writes (flat layout, which is what init creates). A bare `**/reviews/` is rejected because it would hide a consumer project's own `src/reviews/`. The list is SEPARATE from `gitutil.RuntimeIgnoreEntries`: doctor's runtime-file check reports every tracked runtime entry, and this repo tracks 1538 review files. A second `mindspec init` leaves `.gitignore` byte-identical (exact-line presence, then `git check-ignore`). This repo's `.gitignore` gains the same two lines.
2. **No auto-committing verb commits reviews by default.** The verbs that commit on the operator's behalf, enumerated by `grep -rn -e 'CommitAll' -e 'commitWithExport' -e 'CommitPaths' -e '"add", "-A"' --include='*.go' internal cmd | grep -v _test.go`, are: `spec create`, `spec approve`, `plan approve`, the executor's `commitWithExport` (bead and finalize commits), `complete`'s materialization (`gitutil.SnapshotWorktreeTree`), `adopt` (explicit `.beads/issues.jsonl`) and the legacy layout mover (an already-staged `git mv`). Each stages with `git add -A` or an explicit path, so the ignore entry is the whole mechanism and no commit path changes. A review file that is already tracked stays tracked, and its edits are still committed; untracking is out of scope.
3. **Every gate that reads reviews works on untracked files.** Every consumer reads the filesystem, and none reads a review path from a ref or blob: `complete`'s `panelGate` (via `panelGateRoots`), `impl approve`'s advisory slot line, `lifecycle.reviewedHeadSHAForBead`, `instruct`'s panel state, `config show`, the disposition store and `panel disposition query`. `validate`'s `isCoLocatedReview` matches diff paths, and an untracked file is never in a diff.
4. **Retention after finalize: needs operator decision.** An ignored review lives only in the spec worktree, and finalize's worktree removal deletes it. Afterwards the panel datum for a later `reattest`, the spec-117 `dispositions.jsonl` telemetry and `panel disposition query` from `main` see nothing. The options are:
   Decided 2026-10-19: option (B), shipped by spec 130 R6

   - (A) accept the loss, since reviews are advisory;
   - (B) keep `dispositions.jsonl` tracked, with the patterns `.mindspec/specs/*/reviews/**`, `!.mindspec/specs/*/reviews/*/` and `!.mindspec/specs/*/reviews/*/dispositions.jsonl`, reached by appending the two negation lines after R8.1's entries (git cannot re-include a file under an excluded directory);
   - (C) finalize copies `reviews/` to an untracked location in the main checkout before removing the worktree.

   R8.1–R8.3 hold under any option: (B) appends its two negation lines after R8.1's `/**` entries, and AC-20 is unchanged. Until the operator decides, the plan implements R8.1–R8.3 only.

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
- `internal/gitutil/gitignore.go` (`ReviewIgnoreEntries`), `internal/bootstrap/bootstrap.go`, and this repo's `.gitignore` (two lines) (R8)

### Out of Scope

- Everything under Non-Goals.
- `internal/panel` (gate and registry untouched).
- `internal/phase` (consumed, not changed).
- `internal/approve/impl.go` and `internal/executor` behaviour (the follow-on spec).
- Untracking the 1538 already-tracked review files, and a `doctor`/upgrade nudge to do so: a named follow-on, filed at plan time.
- `mindspec setup`'s ignore list, and the legacy root `review/` pattern (R8 covers what init creates).
- R8.4's retention mechanism, pending the operator decision.

## Non-Goals

Deferred to **spec 132-readiness-followon**. Status note (2026-10-21): spec 132 owns G, H, #250, I, and phase-lineage unification. The diagnosis is already reproduced and recorded here, so the follow-on spec need not re-derive it:

- **G — OWNERSHIP.yaml read from the wrong ref (#234).** `validate.CheckADRDivergence` is called from `complete.runCommittedRangeGate` with `ownerRef = beadHead`, and `CheckADRDivergenceSnapshot` forces `ownerRef` to the bead worktree's candidate tree `T`. The impl-approve backstop passes the spec-branch tip. A claim committed on the spec branch after the bead branched is invisible on both per-bead legs. Status note (2026-10-21): spec 132 uses one landing-tree ownership ref. The bead/spec union is not the rule.
- **H — `impl approve` closes the epic before finalizing (#244).** `approve.ApproveImpl` runs `bd close <epic>` and `mindspec_phase=done` BEFORE `exec.FinalizeEpic`. Any FinalizeEpic failure leaves a CLOSED epic and an unmerged spec: push (`gitutil.PushBranch`), `guardMergeLayout`, `preflightMergeDestruction`, or a direct-path merge conflict. A plain reorder is not available: `FinalizeEpic`'s `commitWithExport(specWtPath, …)` must export the JSONL with the epic already closed (the wu7t carrier). Fix shape:
  - hoist the refusal checks and a `git push --dry-run` auth probe ahead of the close;
  - turn any post-close failure into a guard failure naming "epic closed, spec unmerged", with the resume command.

  The re-run tolerance was traced, not run; the follow-on spec must run it.
- **#250 — no verification window between merging a bead and closing it** (`mindspec complete`). This is #244's family: a multi-step terminal mutation with no intermediate stop, at the bead level. The issue records that the reconcile path already converges an out-of-band merge. the follow-on spec decides between a merge-only/verify seam and documenting the out-of-band route.
- **I — dead config key.** `merge_strategy` (`internal/config.Config.MergeStrategy`) is read only by `config show`. `MindspecExecutor.FinalizeEpic` decides PR versus direct merge by `gitutil.HasRemote()`. the follow-on spec either honours the key or removes it.
- **Phase-lineage unification.** 129 adopts `phase.FindEpicForBead`'s lineage, including its `[NNN]` title fallback, unchanged, as the selection lineage (R1.1). So #255's reported shape is selected, and `next` and `ready-check` agree for every `<E>.<N>` bead. They still differ for a `<other>.<N>` bead with title-fallback lineage: readiness evaluates it, while `next` names it (N3) and R1.8 refuses to claim it. the follow-on spec removes the title fallback, tightens `beadIndex`, and migrates any bead that depended on them.

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

- [ ] **AC-1 (R1.1, R1.2)**: Status note (2026-10-21): the emitted N2c diagnostic is "bead <id> has the plan-bead form for epic <epic>, but its lineage resolves to <lineage> through its dependency on epic <lineage>". Fixture: a fake bd that honours `-n` and the default cap.
  - Its `--parent` is **edge-only**, which is the reporter's apparent bd.
  - Ready rows carry `parent`/`metadata` when set.
  - `bd show` and the epic list are served for `FindEpicForBead`.

  Epic E for spec S has `spec_num` NNN.
  - **plan.md.** It has 8 `## Bead` headings, numbered 1–7 and 12 (non-contiguous, so membership is pinned to the positional count), and `work_chunks` ids 1–8 and 10.
  - **Beads.** Every title below that "names S" contains the full spec ID, the follow-up convention:
    - A: `<E>.1`, parent E, title neutral.
    - G: `<E>.2`, no parent, title `[S] …` (#255's reporter shape).
    - G′: `<E>.3`, no parent, title `[NNN] …` (numeric tag only).
    - D: `<E>.4`, no parent, title neutral.
    - P: `<E>.5`, parent another bead.
    - Q: `<E>.6`, no parent, title `[S] …`; lineage resolves through the title fallback to a duplicate epic E′ with the same spec number NNN. The fixture is pinned: E′ is created after E, and the fake epic list is ordered newest-first, so `FindEpicForBead`'s title fallback deterministically picks E′ (Background K).
    - Q2: `<E>.7`, no parent, title `[MMM] …` naming another spec's number.
    - X: `<E>.8`, parent E, title `[S] …`, with a conflicting dependency on epic E2 that `bd show` returns first. Planning must confirm on real bd that such an edge can exist; a round-1 scratch `bd dep add <bead> <epic>` produced none. If it cannot, X uses `parent: E2` instead.
    - O: `<E>.9`, parent E, type bug, title `[S] …`; plan.md has no 9th section and no chunk 9.
    - H: `<E>.10`, parent E, title `[S] …`; chunk 10 exists, but there is no 10th section (half-mapped).
    - L: `<E>.11`, no parent, title neutral, unmapped.
    - R: non-plan-form, parent E, type task.
    - F: non-plan-form, parent E, type bug.
    - M: `metadata.spec_id=S`, no parent, title neutral.
    - T: titled `[S] …`, no other link.
    - The global set also includes E's own epic row.

  Expected:
  - **Members:** {A, G, G′}, in bd order. Bead 8's mapping is positional: it maps to the heading labelled 12, by count. `TestPlanMapsBead_AndPositional` pins this, and so does a variant fixture in which X is replaced by a plain `<E>.8` with parent E, which is selected as a member.
  - **Blocking near-misses:**
    - D (N1);
    - P (N2a);
    - Q (N2b, same-number: `bd update --parent` is the first line);
    - Q2 (N2b, other-number: `bd show` is first and the conditional reparent second);
    - X (N2c);
    - R (N3).
  - **Informational near-misses:**
    - O (N4);
    - H (N4, half-mapped: "no ## Bead section 10");
    - L (N4, no lineage);
    - F, M and T (N3).
  - The epic row is in neither list.
  - `--emit-only --pick 2` emits G. The claim path claims G and G′ with no override.
  - No `bd ready --parent` call is issued (argv seam).
  - **Second fixture:** the same beads and no `plan.md` in the checkout, but on `spec/S`. `--emit-only` reads the plan from `spec/S` and gives the same classification.
  - **Third fixture:** no `plan.md` anywhere. Every plan-form row with lineage E is N4 (`no plan.md exists yet`), exit 0.
  - **Fifth fixture (blob absent):** `spec/S` exists without `plan.md` committed, and the checkout has `plan.md`. The checkout file is used, with no divergence notice. A git failure on the probe (seam) refuses, so failure is distinguished from absence (`TestNext_PlanBlobAbsentUsesCheckout`, `TestNext_PlanProbeFailureRefuses`).
  - **Fourth fixture (stale checkout):** a bead-worktree `plan.md` with one fewer `## Bead` section than `spec/S`'s. Classification uses `spec/S`, and the divergence notice is printed.
  - **Standalone fixtures (lazy trigger):** the whole ready set is ONE neutral-title, parentless `<E>.1`, so zero rows have lineage E.
    - With section 1 AND chunk 1, it is N1 (blocking) with the reparent recovery.
    - With section 1 but no chunk 1, it is N4 (informational) with no reparent recovery.
    - The same pair runs with a `[MMM]`-titled row whose title-fallback lineage is E2: N2b versus N4.
  - **Fenced heading pin:** `TestPlanMapsBead_AndPositional` includes a plan whose fenced code block contains a `## Bead ` line, and it asserts the same count `extractBeadSectionRaw` gives.

  Tests:
  - `TestBuildReadySet_PlanBeadPredicate`;
  - `TestPlanMapsBead_AndPositional` (shared with `evaluateMF1`);
  - `TestNext_ScopedSelection_ReporterShapesClaimable`;
  - `TestNext_ScopedSelection_NamesNearMisses`;
  - `TestNext_EmitOnlyReadsPlanFromSpecBranch`;
  - `TestNext_PlanDivergenceNotice`;
  - `TestNext_LazyPlanReadForNoLineageRow`;
  - `TestNext_NoPlanYetAllN4`.

  Red-on-revert, today, against the edge-only fake:
  - G and G′ are excluded (no edge), and G′ is also excluded by the full-ID title filter;
  - A is excluded by the title filter;
  - O, H and X pass the title filter and the `--parent` edge, so they are selected and then refused at the gate (O and H by MF-1, X on lineage E2).
- [ ] **AC-2 (R1.2 recoveries are real)**: A hermetic test (fake bd, no `t.Skip`; spec 124's bd-less rule) uses the row and `bd show` shapes measured on bd 1.1.0 (Background A, C). The rows:
  - **D (N1).** Before the update, D is classed N1 with the `bd update <id> --parent <E>` line. After it, D's row carries `parent: E` and `bd show` carries the `parent-child` edge, so D is a member, `phase.FindEpicForBead` resolves E, and `readiness.EvaluateReadiness` reaches its signals.
  - **Q (N2b, same number).** The same update makes the epic dependency win over the title number, so Q becomes a member resolving E.
  - **Q2 (N2b, other number).** The `bd show` line comes first, and the reparent line is printed as conditional.
  - **O and L (N4).** No `bd update --parent` line is offered.

  Tests: `TestNearMissN1_RecoveryYieldsClaimable`, `TestNearMissN2b_SameNumberReparentCorrectsLineage`, `TestNearMissN2b_OtherNumberInspectsFirst`, `TestNearMissN4_NoClaimableRecovery`. The real-bd run of the reporter shape and of the N1 sequence is a Validation Proof.
- [ ] **AC-3 (R1.3)**: The claim path, with the selector-specific sentences above:
  - With an empty member set, `mindspec next <id> --spec S` and `mindspec next --spec S --pick 1` each exit non-zero, name the selector, and end with a line matching `^recovery: mindspec next --spec `.
  - A near-miss positional gets its R1.2 sentence on both an empty and a NON-empty member set.
  - A positional that is bd-ready but unlinked (another spec's plan bead) gets `is bd-ready but is not linked to spec S (parent: <P>)`, never "not found or not ready".
  - `--emit-only <near-miss>` still prints the pinned bypass line and emits.
  - **N4 positional with lineage E** (`mindspec next <O> --spec S`): it reaches `GateReadiness` and refuses NOT-READY on MF-1. The scoped recovery line contains `--allow-not-ready=MF-1 --evidence <path-to-justification>`. Then `mindspec next <O> --spec S --allow-not-ready=MF-1 --evidence <f>` claims O and records `signals: [MF-1]`.
    - Red-on-revert: today's flag is a Bool, so `--allow-not-ready=MF-1` is not accepted, and the bare form records every failing signal.
  - **N4 positional with no lineage** (`mindspec next <L> --spec S`, with or without `--allow-not-ready=MF-1 --evidence <f>`): it refuses at selection with the "no lineage … no readiness override applies" sentence. `GateReadiness` is never called (seam), and the audit is byte-identical.
    - Red-on-revert: today the title filter drops L, and the user sees "not found or not ready".

  (`TestNext_SelectorOnEmptySet_Refuses`, `TestNext_NearMissPositional_Refuses`, `TestNext_UnlinkedReadyPositional_Refuses`, `TestNext_N4PositionalOverridesMF1`, `TestNext_N4NoLineagePositionalRefuses`.) Red-on-revert: today it exits 0 with "No ready work found.".
- [ ] **AC-4 (R1.4)**: Four fixtures with an empty member set, no selector and no near-miss, each with an explicit STORED phase:
  - (i) Stored `implement`, all lifecycle children closed: `stored phase: implement; child-derived phase: review`, `recovery: mindspec impl approve S` and `recovery: mindspec repair phase S`.
  - (ii) Stored `implement`, one child in_progress with a missing worktree: the tally and phase lines print BEFORE the worktree-recovery output.
  - (iii-a) Stored `implement`, one child closed and the rest open and blocked: `child-derived phase: implement`, `<n> open lifecycle bead(s), none selectable` and `recovery: bd blocked --parent <E>`.
  - (iii-b) Stored `implement`, every child open and blocked, none closed: `child-derived phase: plan`, with the same blocked line, never `plan approve`.
  - (iv) Stored `plan`, zero lifecycle children, and NO `plan.md` in the checkout or on `spec/S`: `recovery: mindspec plan approve S`. No plan read is attempted (lazy read; the seam records zero reads), and there is no refusal.

  Each output contains the tally and the verbatim sentence, and exits 0 except where the arm mutates. Further fixtures:
  - (v) Stored `review`, all children closed. Three informational near-misses are present:
    - an N4 follow-up bug `<E>.<K+1>` with parent E and no plan mapping (it is non-lifecycle, so it is not in the tally);
    - a reparented non-plan-form bug child;
    - a global bead whose title contains S.

    It exits **0** and still names all three.
  - (vi) The same, plus one blocking near-miss (a detached plan-form bead): it exits non-zero with the same lines.
  - A failing child read (seam) renders the indeterminate form with no tally and exits non-zero.

  (`TestNext_EmptyScoped_DerivedState`, `TestNext_EmptyScoped_InformationalNearMissExitsZero`.) `TestImplementTemplate_EmptinessRule` asserts that the template contains the sentence. Red-on-revert: fixture (i) today prints "No ready work found." and derives `implement`.
- [ ] **AC-5 (R1.5, R1.6, R1.7)**: Every rendered exclusion or near-miss notice contains each excluded ID via `idrender.Bead`, and a count-only notice is red (`TestExclusionNotice_NamesIDs`). The multi-spec menu prints `--spec=<spec-id>` and never `<number>` (`TestNextMenu_PrintsSpecIDForm`; red today). The rewritten spec-128 tests named in R1.7 pass.
- [ ] **AC-6 (R1.8)**: Before any mutation, with bd, branch and worktree byte-identical afterwards (spec 124's AC-4 audit shape):
  - (a) A bead whose lineage resolves to spec T while it is claimed with `--spec S` refuses, naming both specs.
  - (b) On the unscoped path, a resolved target spec ≠ T refuses likewise.
  - (c) On the unscoped path with NO resolved spec, the claim dispatches into T. The argv seam on `EnsureWorktree`/`DispatchBead` records spec ID T, and the bead branch's base is `spec/<T>`, never `HEAD`.
  - (d) A bead whose lineage resolves to T but whose ID is not `<E_T>.<N>` refuses with the "not a plan bead" sentence on both scoped and unscoped paths. Examples: a reparented non-dotted `bug`, and a `<other>.<N>` with a `[NNN]` title.
  - (e) When `spec/<T>` does not resolve, the claim refuses naming T, and `ClaimBead` is never called (seam).
  - (f) A bug bead with no epic lineage refuses through `EvaluateReadiness`'s lineage error, which is today's behaviour, pinned so that it stays deliberate.

  Tests: `TestGateReadiness_LineageSpecMismatchRefuses`, `TestNext_UnscopedDispatchesIntoLineageSpec`, `TestGateReadiness_NonPlanBeadRefuses`, `TestGateReadiness_MissingSpecBranchRefuses`, `TestNext_NoLineageBugRefuses`. Red-on-revert: today (a) validates T and dispatches into S, (c) cuts from `HEAD`, and (e) claims and then fails at `CreateBranch`.
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
- [ ] **AC-11 (R4.2)**: Real-git fixtures extending `initLandedRepo`/`mergeBead` (`internal/lifecycle/landed_test.go`).
  - **The "#239 topology":** spec/S's content reaches `main` through a sibling branch's PR merge, and then spec/S is fast-forwarded to `main`, so earlier landings become reachable only through a second parent.
  - **"Refuses":** writes nothing; MF-3 names the candidate; the recovery lists `reattest`, then the dependent's `--allow-not-ready=MF-3 --evidence <path-to-justification>`, and never q9ea.
  - **Panel registrations** are written as separate registration dirs, each with an explicit `round`. In every leg they are advisory: they change printed lines, never the exit code or the writes. Status note (2026-10-21): advisories change printed lines only. They never change the exit code, the binding, or the audited tuple.

  *Acceptance and its exits*
  - (i) **#239 recoverable, not auto-passed.** X lands on spec/S by its own `Merge bead/X`, then the #239 topology follows. Registration: `{round 1: B}`, where B is the second parent.
    - `FindLandedMergeReachable` returns `*LandedMergeOffFirstParent` naming the merge, even with a surviving tip that corroborates.
    - MF-3 FAILS with arm 1b. It is never the zero-candidate sentence (red today).
    - `ReattestLandedMerge` PASSES and writes the binding plus the audited tuple (red today: `NoOwnedMerge`).
    - MF-3 then PASSES via the read-back.
  - (i-r) **Same-tuple re-run.** A re-run of reattest is a no-op (`Wrote=false`), and MF-3 stays PASS.
  - (i-m) **Malformed audit, same tuple.** The binding, the candidate and the stored audit tuple all agree, but `op`/`at`/`actor` is missing. MF-3 FAILS (the audit is MALFORMED). An explicit reattest re-runs F1, F3, F4 and F5 and writes a VALID record. MF-3 then PASSES, and a further reattest is `Wrote=false`.
  - (i-a) **Single-directory rebased round, the repo convention.** As (i), but the one registration dir holds `{round 2: B}`: round 1's SHA A was overwritten in place, and B is the second parent. reattest PASSES, and MF-3 then PASSES.
  - (i-c) **#233 completion descendant.** One registration `{B}`, where the second parent D is a descendant of B through `complete`'s later commits (materialization or JSONL sync, no tree identity assumed). reattest PASSES.
  - (ii) **Refresh.** As (i), with X first refreshed by `Merge branch 'spec/S' into bead/X`. The refresh is excluded, so the outcome is the same as (i).
  - (iii) **Long-form sibling merge.** `Merge branch 'bead/X' into bead/Y` before X's own landing. It is excluded, and the outcome is the same as (i).
  - (iii-neg) **Only via Y.** X's ONLY path to the tip is that long-form merge. The result is `ErrLandedMergeNoCandidate`, and reattest returns `NoOwnedMerge` (the #251 boundary).
  - (iv) **Primary path unchanged.** A refresh followed by a FIRST-parent landing is identified by the primary scan.
  - (v) **Unreachable merge.** A correctly named merge on a branch NOT reachable from the tip is not a candidate.
  - (vi) **No closing.** `MergedUnclosed` for fixture (i), before reattest, returns "not this state".

  *Refusals at acceptance*
  - (ix) **Forged binding without an audit.** As (vii), plus a binding without audit keys that names the X→Y merge. MF-3 FAILS, and an explicit reattest is required before it passes.
  - (x) **Empty-sibling anchor masking a reverted landing.** The topology:
    - X commits content C.
    - Branch bead/Y, cut from spec/S, independently commits identical content, then merges bead/X as `Merge bead/X`. That merge is empty, and its second parent is X's tip.
    - bead/Y then deletes C.
    - spec/S lands X by its own `Merge bead/X`, which has the same second parent and is the real landing, and then reverts it.
    - bead/Y merges spec/S, spec/S fast-forwards to bead/Y, and spec/S is then carried through `main` and fast-forwarded again.
    - So two admitted fallback candidates share one second parent, and grouping would anchor the content check on the empty sibling merge.
    - Expected: MF-3 FAILS naming both. reattest REFUSES `MultipleFallbackCandidates` and writes nothing.

  *Registrations are advisory* (each leg: same exit code and same writes as the fixture without the registration)
  - (vii) **O3 shape: a later unlanded round.** X's round-1 tip A is merged into bead/Y as `Merge bead/X`; Y lands; X's round B never lands; bead/X is deleted, or survives reset to A. Registrations: `{round 1: A, round 2: B}`. reattest ACCEPTS, writes the binding and audited tuple, and prints the advisory line naming the directory, round 2 and B, then the scanned-roots line. With the registrations removed, the binding and tuple are byte-identical.
  - (vii-a) **Amended after the panel.** The reviewed commit B is amended to B′ with no re-panel, and B′ lands. Registration `{B}`, with B outside the tip. reattest ACCEPTS with the advisory line naming B.
  - (ix-f) **Full forged audit, the stated residual.** As (vii) before any reattest, plus a binding AND a complete, well-formed forged audited tuple, both written by one `bd update --metadata`, naming the X→Y merge. MF-3 PASSES. This pins R4.2's bounded NEW REACH as documented.
  - (x-m) **Malformed reviewed SHA.** A registration whose `reviewed_head_sha` is option-like (`--output=/tmp/x`) or a revision expression (`HEAD~1`, `main^`, `a..b`) prints the malformed advisory. No git process receives it (argv seam), and the outcome is unchanged.
  - (x-e) **Empty SHA.** A registration with an empty `reviewed_head_sha` is skipped, with no advisory line.
  - (x-p) **Placeholder.** A registration with `reviewed_head_sha: "design-review-no-code"` (the 6ou2 shape) is skipped. No git process receives it.
  - (x-g) **Garbage-collected SHA.** A well-formed SHA whose object is absent locally prints its advisory with `(not determinable locally)`, and is not an F5 error.
  - (x-w) **Per-checkout visibility.** The registration exists only in the spec worktree's untracked `reviews/`. Run from the spec worktree and from a bead worktree without `reviews/`, reattest writes the same binding and tuple; only the advisory lines differ, and both name their scanned roots.

  *Read-back stays bound*
  - (xi) **Merge-time binding, no audit.** A merge-time executor binding with no audit names the (i) candidate. MF-3 FAILS with the reattest recovery. reattest writes the audited tuple even though the binding values are unchanged. MF-3 then PASSES.
  - (xii) **Stale audit after an executor rewrite.** reattest genuinely accepts M1. Then the executor's ordinary `MergeMetadata` binding write replaces the binding with M2 and leaves M1's audit in place, and M2 is moved off the first-parent chain.
    - MF-3 FAILS: the audit tuple M1 ≠ binding M2.
    - With both M1 and M2 admitted, a fresh reattest REFUSES `MultipleFallbackCandidates` and writes nothing.
  - (xiii) **Malformed audits:** wrong `op`, non-RFC-3339 `at`, empty actor, a `scanned_branch` naming another spec, or a missing or non-hex SHA key. Each grants nothing, and MF-3 FAILS.
  - (xiv) **Accepted, then reverted.** Accept (i) through reattest, then revert X's landing on spec/S. MF-3 FAILS with the reverted sentence (F4 at read time).
  - (xv) **Accepted, then a later review.** Accept (i), then add a new registration dir at its default `{round 1: B′}`, outside the tip's history. MF-3 stays PASS, because the read-back reads no registration.
  - (xvi) **Accepted, then a second candidate.** Accept (i), then a second admitted `Merge bead/X` becomes reachable. MF-3 FAILS (F1 at read time).

  *Primary path*
  - `TestReattest_HappyPathFleetState`'s re-run no-op leg (`Wrote=false`) and the primary-path reattest tests stay green and unchanged.

  Tests:
  - `TestFindLandedMergeReachable_{OffFirstParentNamesOnly,RefreshedThenOff,LongFormSiblingExcluded,OnlyViaSibling,FirstParentRefresh,Unreachable}`;
  - `TestReadiness_MF3_OffFirstParentFailsWithExits`;
  - `TestReattest_Fallback{Accepts239Shape,AcceptsSingleDirRebasedRound,Accepts233Descendant,SameTupleNoOp,MalformedSameTupleRewrites,AcceptsLaterUnlandedRoundWithAdvisory,AcceptsAmendedAfterPanelWithAdvisory,MalformedReviewedSHANeverGitOperand,SkipsEmptyAndPlaceholderSHA,GarbageCollectedSHAAdvisory,VisibilityChangesAdvisoryOnly,RefusesMultipleCandidates,RecordsAuditOnNoOp}`;
  - `TestReadBack_{ForgedBindingNoAudit,FullForgedAuditResidual,StaleAuditAfterExecutorRewrite,MalformedAudit,AcceptedThenReverted,AcceptedThenLaterReviewStaysPass,AcceptedThenSecondCandidate}`;
  - `TestMergedUnclosed_IgnoresFallback`.
- [ ] **AC-12 (R4.3, R4.5)**: Registrations via `registerLandedPanel`, extended with a slug parameter:
  - (i) Dirs `<X>` with `reviewed_head_sha = A` (stale) and `<X>-r2` with `B` = the second parent: `FindLandedMerge` CONFIRMS (primary path). Red today: the lexically first `A` contradicts.
  - (ii) Registrations {A} and surviving tip = B: confirms via the tip leg, and the panel leg is non-corroborating.
  - (iii) Registrations {A} and surviving tip ≠ B: contradicts, naming the tip datum (the mixed-leg rule).
  - (iv) The #232/#233 shape. One registration {B}; the merge's second parent D is a later commit made by `complete` (a wrapper on an older binary; materialization or JSONL sync on the current one), with B an ancestor of D; the binding records the merge. MF-3 PASSES via (d). Red today: the zero-candidate sentence.
  - (v) The same with no binding and no branch: `*LandedMergeNoEvidence` naming the candidate. `ReattestLandedMerge` then writes the binding, with a corroboration string recording the panel as non-corroborating, and MF-3 PASSES. Red today: `ReattestStatePanelContradiction`.
  - (vi) The spec branch locally ahead of its remote-tracking ref: MF-3 PASS. This is a regression guard, green today.
  - (vii) **CLOSE paths.** `MergedUnclosed` with {stale A} and surviving tip = second parent identifies the landing. With {A} alone and no tip or binding, it returns the `*LandedMergeNoEvidence` error and never "merged-unclosed", so `complete` closes nothing.
  - (viii) **Binding veto and audited repair.** A primary-path candidate has a contradictory binding and a non-corroborating panel set {A}. `FindLandedMerge` refuses (binding contradiction). `ReattestLandedMerge` writes the scan-derived identity and records the prior pair in the audit. On a fallback candidate, the same repair happens only when R4.2's reattest checks (F1, F3, F4, F5) pass. A reattest that refuses (reverted, ambiguous, multiple candidates) writes nothing. `TestReattest_ContradictoryBindingOverwrittenAudited` and `TestReattest_StaleOlderRemergeBindingRewrittenToNewest` stay green.

  Tests: `TestFindLandedMerge_PanelSet*`, `TestReattest_PanelSetNonCorroborating`, `TestReadiness_MF3_CompleteWrapperShape`, `TestReadiness_MF3_LocalAheadOfRemotePasses`, `TestMergedUnclosed_PanelSet`, `TestReattest_RepairsContradictoryBindingOnFallback`.
- [ ] **AC-13 (R4.4)**: `TestReadiness_MF3ErrorArmDiscrimination` gains four legs:
  - `ErrLandedMergeNoCandidate` → the "no two-parent merge reachable … lands bead/" sentence;
  - a binding contradiction → the "a merge landing bead/<id> exists (<sha>) but … contradicts" sentence, and NOT the zero-candidate one;
  - a revert shape → "was reverted or cleanly removed";
  - each ends with its own recovery line, and none recommends `mindspec complete` for the existing-merge arms.
  - **Precedence.** A two-class MF-3 failure (dependency A not closed, dependency B `no-candidate`) renders both records in `Detail`, and exactly ONE `recovery:` line, A's `mindspec complete A`. `indeterminate` plus anything else gives the indeterminate recovery (`TestReadiness_MF3AggregatedPrecedence`).
- [ ] **AC-14 (R5)**: `TestMF2_FormSymmetry` passes over the full table. `TestReadiness_MF2AllClasses` gains the six R5.2 legs. Red-on-revert: legs 1, 4 and 6 fail today.
- [ ] **AC-15 (R6.1)**: Each leg below is a separate fixture or invocation:
  - MF-3 failing only: `--allow-not-ready=MF-3 --evidence <f>` claims and records `signals: [MF-3]`.
  - MF-2 and MF-3 failing: `--allow-not-ready=MF-3 --evidence <f>` refuses with nothing mutated (byte-identical audit). Its final `recovery:` line contains `--allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>` and no operator-supplied path.
  - The bare flag refuses with the same line.
  - `--allow-not-ready MF-3` (space-separated) refuses as bare, and `MF-3` is never looked up as a bead.
  - `--allow-not-ready=MF-9` refuses before any mutation.
  - A missing `--evidence` refuses before any mutation when a signal would be recorded.
  - Nothing failing with `--allow-not-ready=MF-3` and no `--evidence`: a normal claim, the "named but passing" note, and no marker written.
  - MF-3 failing with an `indeterminate` record (a membership read error) and `--allow-not-ready=MF-3 --evidence <f>`: refuses before any mutation with "cannot be overridden … fix the read first".

  (`TestGateReadiness_ScopedOverride`, `TestNext_BareAllowNotReady_Refuses`, `TestNext_AllowNotReadySpaceSeparated_Refuses`, `TestNext_ScopedFlagNothingFailing_ClaimsWithoutMarker`, `TestGateReadiness_IndeterminateNotOverridable`.)
- [ ] **AC-16 (R6.2, record digests and scope widening)**: Ten legs:
  - **Named-but-passing never widens.** Request `MF-2,MF-3` while only MF-3 fails.
    - The stored marker holds only `MF-3`, with its digest list and `evidence.{path,sha256,bytes}`, and no `text` key.
    - Then make MF-2 fail: `ready-check` exits non-zero naming MF-2, and an UNFLAGGED re-claim refuses.
  - **Growth (MF-3).** Record an override while dependency A fails. Keep A and add a newly failing B: MF-3 is uncovered until a fresh override is recorded.
  - **Shrinkage (MF-3).** Record an override for A+B, then land B: MF-3 stays covered (A's digest was recorded).
  - **MF-4 growth.** Keep the first blocker and add a second: uncovered.
  - **MF-4 multiplicity.** Override a standalone `TBD` in the bd description, then add a second standalone `TBD` in a different plan design section: uncovered, because the source and section differ.
  - **Checklist boundary.** A repeated identical checklist item in the same section gets occurrence index k=2: uncovered.
  - **Explicit narrower request.** An existing marker covers MF-2 and MF-3, both fail, and a new request names only MF-3: it refuses, and the marker is unchanged.
  - **Size limit.** A 64 KiB + 1 evidence file refuses with zero metadata writes (seam).
  - **Determinism.** Two consecutive `EvaluateReadiness` runs on an unchanged fixture give byte-identical digest lists for every signal.
  - **Writer/reader agreement.** Over an MF-4 failure whose line contains a control byte, the writer's digests equal the reader's (raw, pre-render records).

  Tests: `TestOverride_NoScopeWidening`, `TestOverride_GrowthUncovered`, `TestOverride_ShrinkageStaysCovered`, `TestOverride_MF4SecondBlockerUncovered`, `TestOverride_MF4OccurrenceIdentity`, `TestOverride_ExplicitNarrowerRequestRefuses`, `TestAllowNotReady_EvidenceTooLargeRefuses`, `TestFailureRecordDigests_Deterministic`, `TestFailureRecordDigests_WriterReaderAgree`. Red-on-revert: the growth and MF-4-multiplicity legs, against the round-2 single-fingerprint design.
- [ ] **AC-17 (R6.3, R6.4)**: `ready-check` with MF-3 failing:
  - with a VALID marker for MF-3 and a matching fingerprint: exit 0, `MF-3: OVERRIDDEN (…)`;
  - with MF-2 also failing: non-zero, naming MF-2;
  - with MF-3's failing dependency changed: non-zero, MF-3 uncovered;
  - with MF-3 now passing: the "no longer needed" note;
  - with an `indeterminate` MF-3 record: never covered;
  - with a LEGACY marker: non-zero, the legacy note, and the two-line `mindspec release <id>` / `mindspec next <id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>` recovery;
  - **mixed coverage:**
    - MF-2 fails and is validly covered, and MF-3 newly fails uncovered. `ready-check` names `uncovered: MF-3`, and its re-claim line names `--allow-not-ready=MF-2,MF-3`.
    - `release`, then the fresh re-claim with MF-2,MF-3 and new evidence, succeeds and REPLACES the marker.
    - An explicit MF-3-only re-claim refuses without mutation.
    - The same holds where MF-3 stays covered and MF-4 becomes uncovered (re-claim MF-3,MF-4).
    - The same holds where a signal's coverage was lost through a grown digest list while another signal stays covered;
  - with MALFORMED markers (unknown ID, duplicate ID, missing fingerprint, bad timestamp, non-hex sha256, `bytes` = 0): non-zero, the malformed note;
  - with a metadata read error: refuses non-zero.

  After `release` and the re-claim, the bead's surviving branch is reused. `EvaluateReadiness` does not read the marker, and the isolation test stays green. Tests: `TestReadyCheck_HonoursValidMarkerOnly`, `TestOverrideMarker_Validity`, `TestReadyCheck_LegacyRecoveryReleasesThenReclaims`, `TestReadyCheck_MixedCoverageRecoveryNamesAllFailing`. Red-on-revert: the first case exits non-zero today.
- [ ] **AC-18 (R6.5, R7, ADR-0041)**:
  - `ms-bead-impl/SKILL.md` contains no `mindspec_readiness_override` subset comparison. It contains the `ready-check` exit-code line immediately preceded by the staged-rule marker naming `target=next-action` (spec 133) and a `remove-by=` date. The rewritten pin tests pass, and they assert the marker.
  - `.mindspec/adr/ADR-0045-…md` exists with `Status: Accepted`, the Status fence, Amends ADR-0040 §1 and ADR-0044, step 1 → spec 130, and Decision 5's transition clause (pre-existing skill content grandfathered until step 7).
  - ADR-0040 and ADR-0044 carry the back-pointer; `mindspec adr list` includes ADR-0045.
  - ADR-0041 contains § "Amendment (Spec 129)" naming the fallback-scoped callers, the finder-only rule (no automatic acceptance of a fallback candidate), the fallback safety checks F1, F3, F4 and F5 (all git-derived), F2's retirement with registrations as advisory output only, the audited tuple, its validity contract and the validity-qualified no-op, the fallback-provenance corroboration string, q9ea's exclusion from the fallback exits, the metadata-forge residual with its bounded new reach, the read-back conditions, the undirected-subject target gap, the non-corroborating panel datum and its reach into the CLOSE paths, the binding veto versus audited repair, and the scoped, fingerprint-bound fourth-verb override (a text-pin test in the R4 or R6 bead).
  - `mindspec validate spec 129-readiness-tracker-honesty` resolves every touchpoint link.
- [ ] **AC-19 (method)**: `go build ./... && go test ./... -short` is green. `internal/harness` and `internal/instruct` tests run under `-short` only (128 settled fact 3). `go test ./internal/lint/...` is green, including `TestBDRowCapWaist` and the ADR-0030 boundary lint (no new `os/exec` or `gitutil` import in `internal/validate/readiness`).
- [ ] **AC-20 (R8.1)**: In a temp repo, a fresh `bootstrap.Run` and a run over an existing `.gitignore` both leave `.mindspec/specs/*/reviews/**` and `.mindspec/reviews/**` in it, and a second run leaves it byte-identical. `git check-ignore` reports `.mindspec/specs/<id>/reviews/<slug>/panel.json` and `.mindspec/reviews/<slug>/panel.json` ignored, and `src/reviews/x.go` not ignored. `ReviewIgnoreEntries` shares no entry with `RuntimeIgnoreEntries`, so doctor reports no tracked review file. Tests: `TestBootstrap_ReviewsIgnored{Greenfield,ExistingGitignore,Idempotent}`, `TestReviewIgnoreEntries_NotRuntimeEntries`. This repo's `.gitignore` contains both lines.
- [ ] **AC-21 (R8.2, R8.3)**: With the entries present and an untracked `panel.json` under a review dir, `gitutil.CommitAll` and `gitutil.SnapshotWorktreeTree` leave it out of the commit or tree, and `git status --porcelain` is empty (`TestCommitAll_SkipsIgnoredReviews`, `TestSnapshotWorktreeTree_SkipsIgnoredReviews`). `complete`'s panel gate decides the same with the registration untracked and ignored as with it tracked (`TestPanelGate_UntrackedIgnoredRegistration`), and `reviewedHeadSHAForBead` reads it (`TestReviewedHeadSHASet_UntrackedIgnored`).

## Validation Proofs

- A real throwaway bd database. The epic carries `spec_num`, and the beads are:
  - A: a plan bead;
  - G: `[S] …`, created with `--parent` and then detached with `bd dep remove` (#255's reporter shape: `bd dep tree` shows no dependencies);
  - D: neutral title, detached the same way;
  - M: `metadata.spec_id` only;
  - T: title only.

  `mindspec next --spec <S> --emit-only` lists A and G, and names D, M and T with their reasons. Then `bd update <D> --parent <E>` followed by the same command lists A, G and D (AC-1, AC-2). On bd 1.1.0 the G leg of this proof is green today too (prefix `--parent` and a full-ID title). The red-on-revert evidence for G is AC-1's edge-only fake and its numeric-tag G′.
- `mindspec next <T> --spec <S>`: exits non-zero with T's N3 sentence (AC-3).
- All-closed fixture with stored `implement`: `mindspec next --spec <S>` prints `child-derived phase: review` and `recovery: mindspec impl approve <S>` (AC-4).
- This repo: `bd ready --json 2>&1 >/dev/null` prints the truncation notice. After R2, `mindspec next --emit-only` lists every ready bead (AC-7).
- `go test ./internal/lifecycle/ -run 'TestFindLandedMergeReachable|TestReattest_Fallback|TestMergedUnclosed_IgnoresFallback|TestFindLandedMerge_PanelSet|TestReattest_PanelSet'` is green (AC-11, AC-12).
- On a #239-shaped fixture, `mindspec bead ready-check <dependent>` prints MF-3 FAIL naming the off-first-parent merge with `recovery: mindspec reattest <dep>`. After that command, it passes (AC-11 (i)).
- `go test ./internal/validate/readiness/ -run 'TestReadiness_MF3|TestMF2_FormSymmetry'` is green (AC-10, AC-12, AC-13, AC-14).
- With MF-3 failing: `mindspec next <bead> --allow-not-ready=MF-3 --evidence ./why.md` claims, and `mindspec bead ready-check <bead>` then exits 0 with `MF-3: OVERRIDDEN`. Changing the failing dependency makes `ready-check` exit non-zero. The bare `--allow-not-ready` refuses with the scoped line (AC-15–AC-17).
- `mindspec adr list | grep ADR-0045` and `mindspec validate spec 129-readiness-tracker-honesty` → 0 errors (AC-18).
- `go build ./... && go test ./... -short` is green (AC-19).
- `mindspec init` in a scratch repo under `/tmp`, then `git check-ignore -v .mindspec/specs/001-x/reviews/a/panel.json` names the new entry, and `git status --porcelain` stays empty after a review dir is written (AC-20, AC-21).

## Open Questions

Every question below is either resolved with its rationale or EXPLICITLY DEFERRED to the plan stage with its re-open condition.

- [x] **OQ-1 (scope split) — RESOLVED.**
  - 129 = R1–R6 (estimate: **9–12 beads** — R1 2–3, R4 3, R6 2–3, R2/R3/R5 2–3 together; the plan gate should EXPECT the R6 relief-valve split to fire) + R7 (doc, this commit) + the ADR-0041 amendment (with the R4/R6 beads) + R8 (reviews out of git, added 2026-10-19).
  - the follow-on spec (next plain number) = five deliverables: G (#234), H (#244), #250, I (`merge_strategy`), and phase-lineage unification.
  - 130 = the S0 rule table (pure policy, a different review centre, an unowned path that first needs an OWNERSHIP claim), #233's review-to-landing binding, and the rest of #251.

  Re-opens if the plan gate judges 129 over-compressed: R6 then splits out as a further plain-numbered spec.
- [x] **OQ-2 (title filter) — RESOLVED.** The title is never a membership input. `containsSpecID` survives only as R1.2's near-miss linkage test, and R1.7 lists the spec-128 tests that are rewritten.
- [x] **OQ-3 (can the panel leg ever confirm after spec 128 Bead 6?) — EXPLICITLY DEFERRED to the plan stage.** The plan must run the fixture: the panel reviews `B`, `complete` materialises its landing commit, then the merge. It records whether the second parent equals `B` on a clean worktree.
  - If it never does, R4.3's non-corroborating semantics are exactly right, AC-12 (iv)/(v) is the normal path, and the ADR-0041 amendment says so.
  - If it does, AC-12 (i) is the leg's live proof.

  Either way #233's binding question stays spec 130's. This re-opens R4.3's wording, never its rule.
- [x] **OQ-4 (one discovery for identify AND nominate) — RESOLVED.** `ReattestLandedMerge` nominates through the same first-parent-then-reachable-fallback generator as MF-3. On the fallback path MF-3 only names the candidate until reattest accepts it under F1, F3, F4 and F5. Later reads accept it only through the matching audited tuple, with those checks re-run (R4.2, AC-11). The executor's write-time `gitutil.ExactSecondParentMerges` stays first-parent by requirement (R4.2's table), because the merge it binds was just made and is the first-parent tip.
- [x] **OQ-5 (bare `--allow-not-ready`) — RESOLVED: refuse.** `--allow-not-ready=all` is NOT admitted. It would restore the all-or-nothing form R6 removes, and the refusal's recovery line already names the complete failing set. The cobra shape (a `String` flag with `NoOptDefVal`) is a plan decision, bound by AC-15's space-separated leg.
- [x] **OQ-6 (index dotted sub-numbers `R6.3`?) — RESOLVED: no.** Base resolution is symmetric today. Sub-number indexing would be a new claim form that turns existing dotted citations into potential dangling claims. Re-opens if a later spec wants sub-number-exact MF-2.
- [x] **OQ-7 (exit code of an empty-set result) — RESOLVED.**
  - No selector and no BLOCKING near-miss → exit 0 (HC-4: the read evaluation completed). Informational near-misses are printed and do not change the exit code. They are: N4 follow-up children of E that `plan.md` does not map (bd mints `<E>.<N>` for them whatever their type), title-only or metadata-only N3, and reparented non-lifecycle children. Such beads are by design never selectable, so failing on them would make every scoped `next` fail for as long as one follow-up stays open.
  - ≥1 blocking near-miss (N1, N2, or a lifecycle-type N3 under E) → non-zero (128's loud-empty class).
  - A selector supplied → non-zero.
  - A failed child read → non-zero.
- [x] **OQ-8 (why not surface stderr on success in `tracedOutput` globally?) — RESOLVED: scope it to the waist.** Only in the row-set waist does a notice have a defined meaning (truncation) and a defined response (an error).
- [x] **OQ-9 (membership design) — RESOLVED: a plan bead = plan-form ID + lineage E + a plan mapping (round 3). Option W is still rejected.**
  - **Round 1 (Option N)** selected `<E>.<N>` rows with parent E. That left #255's edge-less, title-lineage bead unclaimable.
  - **Round 2** selected by lineage through the UNCHANGED `phase.FindEpicForBead`. Evidence (scratch bd 1.1.0, Background A): `pr-1rs.4` resolves `epic=pr-1rs`; a neutral-title sibling gives `ErrNoEpicLineage`.
  - **Round 3** added MF-1's own mapping, because bd mints `<E>.<N>` for EVERY child created under the epic. The example is `mindspec-cter.8`, a bug outside spec 093's 7-bead plan. `mindspec-jfgj.4`, also typed bug, is spec 073's mapped Bead 4. Round 3 also removed the parent-E shortcut, so selection agrees with the resolver on conflicting epic dependencies.
  - **What this gives.** The member set equals the beads that `GateReadiness` can pass on MF-1 and lineage, for `<E>.<N>` IDs, when selection and readiness read the same `plan.md` content. Selection reads `spec/<S>` when that branch exists, and readiness reads the invoking checkout. Where they differ, the R1.1 divergence notice is printed (OQ-10).
  - **#255.**
    - Defect 1 (zero instead of naming) is fixed for every shape.
    - Defect 2 (the exclusion) is fixed for the reporter's shape, including on an edge-only bd and with a numeric-only tag. On bd 1.1.0 with a full-ID title that shape was already selected at `e2325e36`, so R1.1 is a hardening there.
  - **Rejected alternative.** Keeping unmapped children as members and default-picking lifecycle members first. MF-1 refuses unmapped beads, so they could never be claimed without an override.
  - **Honest limit.** `readiness.beadIndex` also evaluates `<other>.<N>` beads by suffix. `next` names them (N3), and R1.8 refuses to claim them. The follow-on spec tightens it.
  - **Option W** (descendant-OR-`metadata.spec_id` everywhere) stays rejected. It needs core changes and an undefined non-dotted mapping.
  - **MF-3's dependency membership stays wider** (the fail-closed direction).

  Re-opens if planning finds a shipped flow that creates plan beads without the plan-bead form or a plan mapping.

- [x] **OQ-11 (review retention after finalize) — EXPLICITLY DEFERRED to the operator: needs operator decision.** R8.4's options (A), (B) and (C). The plan implements R8.1–R8.3, which hold under every option. It re-opens when the operator picks an option.
- [x] **OQ-10 (which plan.md does readiness read?) — EXPLICITLY DEFERRED to the plan stage.** Selection prefers the committed `spec/<S>` blob and prints a divergence notice. `EvaluateReadiness` still reads the invoking checkout. The plan decides whether readiness also reads `spec/<S>`, which would make selection and readiness equal by construction, or keeps the notice as the honest boundary. It re-opens if the plan finds a shipped flow that edits `plan.md` in a bead worktree before committing it.

### Grill record (self-answered, headless — 2026-10-18; revised after rounds 1–5 and the 2026-10-19 operator decision)

- [x] grill (operator, 2026-10-19): should a review registration refuse `reattest` or the read-back (F2)? → No: "yes drop F2". Reviews are an optional helper, not a hard blocker. Registrations are advisory output, and AC-11 (vii)'s O3 shape is accepted with an advisory. The F2 entries below are kept as history.
- [x] grill (operator, 2026-10-19): should `reviews/` be committed? → No by default: `mindspec init` gitignores it and no auto-commit verb stages it (R8). Untracking existing files is a follow-on.

- [x] grill (round 5): can panel `round` order reviews across directories? → No (orchestrator ruling 5): it is a per-directory counter. F2 now refuses on ANY visible outside-tip registration and never compares rounds, because one directory holds one record under the in-place bump.
- [x] grill (round 5): is F2 git-derived? → No: it reads untracked, agent-writable panel files. The residual names the panel-edit reach, and AC-11 (ix-g) pins it.
- [x] grill (round 5): is q9ea a fallback exit? → No: restoring the branch satisfies only F3. It is dropped from the fallback exits.

- [x] grill (round 4): does the reattest audit bind to what it vouched for? → It did not: no merge-SHA key existed, and `MergeMetadata` kept stale audits alive. There is now an audited tuple equal to the binding and the candidate, and every read-back re-runs the fallback checks (R4.2).
- [x] grill (round 4): does the unreached-review refusal dead-end a genuine landing with a rebased-away round? → It did. F2 now refuses only a LATER-round non-ancestor, using panel.json `round`, fails closed on a tie or a missing round, and names the remaining exits. (Superseded in round 5: rounds are never compared, and see the round-5 entry.)
- [x] grill (round 4): is "mapped" MF-1's precondition? → Now yes: `PlanMapsBead` = positional section AND chunk, shared with `evaluateMF1`.

- [x] grill (round 3): can the fallback ever pass MF-3 automatically? → No (orchestrator ruling). Three rounds found three different false PASSes. It finds and names; `reattest` accepts after refusing unreached reviews, multiple candidates and missing audits (R4.2).
- [x] grill (round 3): is every `<E>.<N>` with lineage E a plan bead? → No: bd mints that ID for follow-up bugs too. A plan mapping is now required, and unmapped children are N4, informational (R1.1, R1.2).
- [x] grill (round 3): may successors use lettered IDs? → No: `parseSpecID` gives `spec_num 0` and the title fallback resolves 129 (mindspec-wtjr). Plain numbers only (Background K).
- [x] grill (round 3): does an override fingerprint over the whole set churn when failures shrink? → It did. Per-record digests now let shrinkage stay covered while growth uncovers (R6.4).

- [x] grill (round 2): is #255's reported bead claimable? → In round 1, no: it was an N1/N3 near-miss. Now yes: R1.1 selects by lineage, and the reporter shape was resolved on a real scratch bd (OQ-9).
- [x] grill (round 2): does the fallback's `Merge bead/<id>` form pin a target? → No: it names none, and a reviewer probe built a false PASS. Fixed: on the fallback path the panel never confirms on its own, (b) or (d) is required, and the residual is stated (R4.2, AC-11 (vii)). (Superseded in round 3: the fallback no longer confirms automatically at all.)
- [x] grill (round 2): does an override fingerprint over the first Detail widen coverage? → Yes: A then A+B stayed covered. Fixed: the fingerprint is over the complete sorted failure records (R6.2, AC-16).
- [x] grill (round 2): can an unscoped claim be cut from `HEAD`? → Yes, today. Fixed: the lineage spec is the dispatch spec (R1.8 (c), AC-6 (c)).

- [x] grill: does the membership predicate admit beads readiness cannot evaluate? → It did in the draft (metadata-only, reparented non-dotted). Narrowed to the plan bead (OQ-9). AC-2 proves the N1 recovery on a real bd.
- [x] grill: is bd's spec-id field a cheaper linkage? → It is filterable by `bd list --spec` (prefix) and shown by `bd show`, but `bd ready` cannot filter it, and it would be a second home. Not adopted (R3.1).
- [x] grill: is #232 the same bug as #239? → No. Its dependency is the same spec's bead, and its mechanism is #233's: the panel SHA is never the landing's second parent. R4.3 (both functions) plus R4.4 close it end-to-end (AC-12 (iv)/(v)). No remote ref is read.
- [x] grill: does a full-history scan fix #239 safely? → No. A refresh merge into `bead/X` becomes a second candidate, and the ambiguity rule refuses (reproduced). Redesigned as a first-parent-primary scan with a source-named fallback, scoped to MF-3 and reattest (R4.2, AC-11). The fallback is target-pinned only for the `into <spec-branch>` forms; round 2 added its non-panel confirmation rule.
- [x] grill: does removing the panel veto reintroduce ancestor-tolerant confirmation? → No. The set confirms only on exact equality; the tip and binding contradictions remain.
- [x] grill: does R6 open a gate on a forged marker? → No gate reads the marker for claim authority (fresh flag and evidence on every claim). `ready-check`'s dispatch answer is no weaker than today's skill prose, and it is stricter: a VALID schema plus a per-signal fingerprint of the complete failure set are required.
- [x] grill: does the evidence leak into git? → Its path, sha256 and byte count do (the tracked JSONL); its text is never stored.
- [x] grill: "A re-run of impl approve tolerates the half-done state" — traced or run? → Traced. Recorded as the follow-on spec's obligation to run it.
- [x] grill: does ADR-0045 hold against the tree? → Its Context claims are verified by file. It conflicted with ADR-0044 and understated its ADR-0040 amendment; both are now stated. `Accepted` is fenced per ADR-0032.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-10-19
- **Notes**: Approved via mindspec approve spec