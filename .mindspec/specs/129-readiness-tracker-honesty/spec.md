---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 129-readiness-tracker-honesty: Readiness and tracker honesty — plan-bead selection, bd row caps, MF-2/MF-3 truth, scoped overrides (spec-128 follow-on, part 1 of 2)

*Citation scope: every code citation in this document names a function, symbol, file, or template, and was verified against `main` at `e2325e36` (v0.15.0) on 2026-10-18 in the spec-129 worktree. bd behaviour was verified against bd 1.1.0 (`8e4e59d39`) in a throwaway database. Round 1 of the spec-gate panel (9/9 REQUEST_CHANGES) corrected several draft claims; each correction is recorded in § Background under "Corrections".*

## Goal

After this spec, the verbs that decide *what work is ready* tell the truth about the tracker and about git:

- (a) `mindspec next --spec <id>` selects exactly the spec's **plan beads** (a structural tracker fact, never a title match), names every other bd-ready bead linked to the spec with the true reason it is not selectable, refuses a selector (positional ID or `--pick`) it cannot honour instead of ignoring it, and prints the spec's derived lifecycle state on an empty result, so no caller can mistake "nothing ready" for "done" (GH #255);
- (b) no bd `ready`/`list` call the binary makes a decision on can be truncated by a bd row cap, and a truncation bd announces is an error, never a silent short read;
- (c) `mindspec next` never validates one spec's plan and then dispatches the bead into another spec: the readiness gate refuses before any mutation when the bead's lineage spec differs from the spec it is being claimed for;
- (d) MF-3 holds a closed dependency to the landed-merge test only when the dependency belongs to the same spec by a tracker fact; it finds a correctly named landing that is reachable from the spec-branch tip but off its first-parent chain (GH #239); a stale or non-matching panel registration never vetoes a correct corroboration, in `FindLandedMerge` or in `reattest`; and it prints what is actually true for a contradiction or a revert. GH #232 is closed on its real mechanism (Background D.5), not by a remote-aware feature;
- (e) the MF-2 token resolver recognises the same claim forms on both sides, bead text and spec text;
- (f) `--allow-not-ready` is scoped to named failing signals, needs an evidence file on every claim, and `mindspec bead ready-check` reads a validated override marker back, so the override log can be audited against the currently failing set and the override rule has one home (the binary, not `ms-bead-impl`'s prose);
- (g) ADR-0045 ("The binary owns sequence and rules; skills are harness adapters") lands, with its conflicts with ADR-0040 and ADR-0044 stated, so specs 130–133 can cite it.

## Background

### Provenance and scope arithmetic

Spec 128 (`128-lifecycle-verb-state-honesty`, merged 2026-10-18, PR #257) deferred, by name, a "spec-129 follow-on": GH #255 (readiness-truth face), #239, #232, #234, #244, #250. A consolidated diagnosis was reproduced at `e2325e36`; this spec re-verified every load-bearing claim, and the round-1 panel corrected the rest.

The cluster is split as follows. Every inherited issue has one named home:

- **Spec 129 (this spec):** six code requirements, R1–R6 (#255, the row caps, the claim/dispatch spec-consistency guard, #239 and #232, MF-2 symmetry, the override), estimated at **7–10 beads** at plan time. R7 (ADR-0045) is a document that lands with this spec's commit. The ADR-0041 "Amendment (Spec 129)" is a document that lands with the R4 and R6 beads.
- **Spec 129b (part 2):** four deliverables. They are G (#234), H (#244), #250 (the same family as #244: a multi-step terminal mutation with no intermediate stop), and I (the dead `merge_strategy` key).
- **Spec 130:** the S0 rule table (ADR-0045 Implementation order step 1), and #233's review-to-landing binding (see D.5).

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

`readiness.EvaluateReadiness` additionally needs `beadIndex` (a trailing `.<N>`) to map MF-1 onto plan section `## Bead N`. So a bead can be ready-check-PASS (title-fallback lineage plus a dotted ID) and still be absent from the scoped set, which is #255's reported shape.

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
- `bd ready --json` rows carry `parent` and `metadata` as **omit-when-empty** fields. A row with a parent carries `parent`; a row with metadata carries `metadata`. `bd show --json` dependency entries carry each dependency's `issue_type`, `status`, `dependency_type` and `metadata`, but not its parent.

**D. MF-3 false negatives (#239, #232).**
1. *Over-broad dependency set.* `readiness.fetchBeadRecordReal` collects every `blocks` edge, and `evaluateMF3` holds every closed one to `findLandedMergeFn(root, specBranch, dep.ID)` with no membership check. A closed plain task or another spec's bead can never have a `bead/<id>` landing on THIS spec branch, so it fails by construction. REPRODUCED.
2. *First-parent-only candidate scan.* `lifecycle.FindLandedMerge` and `lifecycle.ReattestLandedMerge` both nominate from `firstParentMergesFn` = `gitutil.FirstParentMerges` (`git log --first-parent --merges`). #239's topology works like this: the bead lands on the spec branch, the spec branch's content then reaches `main` through a sibling branch's PR merge, and the spec branch is moved to that history. This leaves the landing reachable only through a second parent, and both functions find no candidate. REPRODUCED (real git, scratch).

   The round-1 panel showed the draft's fix, a full-history scan, to be unsound. It makes a refresh merge `Merge branch 'spec/S' into bead/X` a second owned candidate, and the FIX-2b ambiguity rule then refuses a landing that today's scan identifies. A scratch-clone probe (real git, with an emulated seam over `firstParentMergesFn`) reproduced this, and showed that the R4.2 design (first-parent primary, plus a source- and target-pinned fallback on zero candidates) identifies all four shapes in AC-11 (i)–(iv).
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
  - `plugins/mindspec/skills/ms-bead-impl/SKILL.md` and its installed `.claude/skills/` copy: the override-coverage prose is **removed** and replaced by "run `mindspec bead ready-check`; proceed on exit 0". This deletes a rule from a skill, as ADR-0045 §4 requires. No rule is added to any skill.
  - `plugins/mindspec/skills/ms-bead-cycle/SKILL.md`: the bare-form wording, if the plan's grep finds one (R6.5).
- **execution**:
  - `internal/bead/**`: R2's row-set waist (`ListJSON` and a new `ReadyJSON`); the shared plan-bead ID helper; R6's marker schema v2 constants and its validator in `readiness.go`.
  - `internal/gitutil/**`: R4.2's topo-ordered reachable-merge primitive beside `FirstParentMerges`.
- **core**: NOT impacted. `internal/phase` is consumed, not changed: `DerivePhaseDetail`, `Cache`'s child list and `FindEpicForBead` are read as they are. The rejected Option W would have changed `FindEpicForBead`; see OQ-9.

Documentation surfaces touched are not Impacted-Domains entries. They are claimed by no OWNERSHIP.yaml and are governed by the ADR-0031 doc-sync gate. `.mindspec/adr/ADR-0045-binary-owns-sequence-and-rules.md` is new and lands with this spec's commit, together with the "Amended by ADR-0045" back-pointers in ADR-0040 §1 and ADR-0044. ADR-0041's new § "Amendment (Spec 129)" lands with the R4 and R6 beads. R6.5's surfaces are `.mindspec/core/USAGE.md`, `.mindspec/core/WORKFLOW-STATE-MACHINE.md`, `.mindspec/domains/workflow/{architecture,interfaces,runbook}.md`, and `.mindspec/domains/execution/interfaces.md` (the marker schema row).

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — **AMENDED** by a new § "Amendment (Spec 129)". It is appended (not an in-place rewrite), follows the Spec-125 precedent, is cross-referenced from item (c), and is finalized by the bead that lands the citing code. AC-17 pins it. It covers three things:
  - **(i) Reachable-landing fallback (R4.2).** The Spec-125 text binds landed-ness to "a two-parent first-parent merge", and §2(ii) reads revert-awareness over "the first-parent chain SINCE the candidate merge". The amendment admits, for the MF-3 read and for `reattest` only, a fallback candidate: a two-parent merge reachable from the tip whose subject names `bead/<id>` as its SOURCE, with the spec branch as its target. Revert-awareness for such a candidate is the same net-effect test at the tip, of which the candidate is an ancestor. §2(ii)'s CLOSE decisions (`complete`'s merged-unclosed reconcile) keep the first-parent scan unchanged.
  - **(ii) Non-equal panel datum (R4.3).** Item (c) already reads "a registered panel's `reviewed_head_sha` equal to an exact match's second parent". The amendment makes the non-equal case explicit for `FindLandedMerge` and `reattest`: a registration whose SHA does not equal the second parent is NON-CORROBORATING. It neither confirms nor contradicts.
  - **(iii) Fourth-verb clause (R6).** `--allow-not-ready` becomes signal-scoped, needs an evidence file on every claim, and refuses when a non-named signal fails. An existing marker never authorizes a claim.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED. Every new refusal ends with a `recovery:` line:
  - R1.3's selector refusals use `recovery: mindspec next --spec <spec-id>`.
  - R1.2's near-misses use `bd update <id> --parent <epic-id>` only when the bead has no parent and has the plan-bead ID form (the one recovery shown to yield a claimable bead). Otherwise they get the inspection-first `bd show <id> --json` (Spec-127 amendment item 2; never a reparent over a parent the agent did not name).
  - R1.4 uses `bd blocked --parent <epic-id>`, which exists in bd 1.1.0.
  - R6's scoped recovery carries `--evidence <path-to-justification>`, a literal placeholder. It is marked non-mechanical, because the operator authors the evidence.

  New emitted commands are registered where the `internal/guard` recovery-convention lint requires. HC-4: an empty-set `next` with no selector and nothing excluded mutates nothing and exits 0. The one exception is R1.4's in-progress arm, which keeps today's worktree recovery.
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

1. **One structural predicate: the plan bead.** For spec S with epic E, a bd-ready row is a **member** (selectable) iff all four hold:
   - it is returned by `bd ready --parent <E> -n 0 --json`;
   - `issue_type` is not `epic` (stripped explicitly, as `filterReadyItems` does today);
   - its ID has the plan-bead form `<E>.<N>` (N ≥ 1, one segment);
   - its row's `parent` equals E.

   The ID-form test is one exported helper in `internal/bead` (working name `bead.PlanBeadIndex(epicID, id) (n int, ok bool)`), shared with R4.1. Members keep `bd ready --parent`'s returned order, so `--pick N` parity holds by construction. The title (`containsSpecID`) and `metadata.spec_id` are **never** membership inputs. A member is selectable whatever its title says, and a non-member is never selectable.
2. **Near-misses are named, on every scoped call.** `BuildReadySet` also reads the uncapped global `bd ready -n 0 --json`, so a scoped `next` makes exactly two bd ready calls and no per-bead `bd show`. A **near-miss** is a non-epic, non-member row linked to S by any of:
   - it is in the descendant set but fails the ID-form or parent test;
   - its title contains the spec ID;
   - its row `metadata.spec_id` equals S.

   Every near-miss is printed by ID with its true reason and one recovery line:
   - (N1) plan-bead ID form, `parent` absent: `bead <id> has the plan-bead form for epic <E> but its parent edge is missing (parent: none)` → `recovery: bd update <id> --parent <E>`. This is the only class whose recovery is shown to make the bead claimable. Verified on bd 1.1.0: after the update, `parent` is E and `bd show` carries the `parent-child` edge to the epic, which `FindEpicForBead` resolves.
   - (N2) plan-bead ID form, `parent` = another bead P: `… but its parent is <P>` → `recovery: bd show <id> --json`. A parent the agent did not name is never replaced.
   - (N3) any other linkage: `bead <id> is bd-ready and linked to spec <S> by <descendant tree | title | metadata.spec_id>, but it is not a plan bead (no <E>.<N> ID with parent <E>); mindspec next claims only plan beads` → `recovery: bd show <id> --json`. This recovery is non-mechanical. A non-plan bead cannot be made claimable by a bd edit, because MF-1 needs its `## Bead N` mapping. The honest paths are to fold the work into a plan bead or to work the bead outside `mindspec next`.

   Near-misses are printed when the member set is non-empty too. They never change selection.
3. **A selector `next` cannot honour is refused, not ignored.** On the claim path, with `--spec`:
   - Empty member set plus a positional ID: exit non-zero with `no ready plan bead for spec <S>; positional <id> cannot be claimed from an empty ready set`.
   - Empty member set plus `--pick N`: exit non-zero with `--pick <N> is out of range: 0 items`.
   - Both refusals end with `recovery: mindspec next --spec <S>`.
   - A positional that is a near-miss, whether the member set is empty or not, refuses with its R1.2 sentence. It never gets "not found or not ready", and needs no extra bd call.
   - A positional absent from the global ready set keeps today's "not found or not ready".

   `--emit-only`'s positional bypass (`FetchBeadByID`, pinned 128 R6.1) is unchanged and keeps its disclosure line.
4. **The empty-set result carries the spec's derived state.** When the member set is empty and no selector was supplied, the output reads E's child list, uncapped and all statuses, as `phase.Cache` lists it. That list is the one `DerivePhaseFromChildren` and `state show` use, and every member is in it. The output prints, in this order, before any other action:
   1. the lifecycle-children tally (`closed / in_progress / open`);
   2. `stored phase: <p>; child-derived phase: <q>` from `phase.DerivePhaseDetail`;
   3. the verbatim sentence "an empty ready set is not evidence the spec is complete; the derived phase above is";
   4. ONE line keyed on the **child-derived** phase:
      - all lifecycle children closed (`review`) → `recovery: mindspec impl approve <S>`;
      - an in-progress child → today's worktree-recovery branch, unchanged. This is the one empty-set arm that mutates (`EnsureWorktree`), and it runs after the lines above;
      - ≥1 open lifecycle child, none in progress → `<n> open lifecycle bead(s), none bd-ready` and `recovery: bd blocked --parent <E>`;
      - zero lifecycle children (`plan`) → `recovery: mindspec plan approve <S>`.

   When the child-derived phase is ahead of the stored phase, the existing forward-reconcile `recovery: mindspec repair phase <S>` is added. Exit codes:
   - nothing excluded: exit 0;
   - ≥1 near-miss: exit non-zero (the 128 R6.3(c) loud-empty class, settled fact 1), still printing the derived state;
   - failed child read: the indeterminate form (128 R2.3's class), no tally, and a non-zero exit, because the read evaluation did not complete (HC-4).

   The same sentence is stated once in `internal/instruct/templates/implement.md` (L3).
5. **Exclusion notices name IDs.** `ReadySet.ExclusionNotice` and the near-miss list render every bead ID via `idrender.Bead`, never a bare count.
6. **The ambiguity menu prints the form it accepts.** The multi-spec menu in `cmd/mindspec/next.go` prints `mindspec next --spec=<spec-id>` (a literal placeholder under the listed IDs), never `--spec=<number>` (#239 § Minor).
7. **Spec-128 tests rewritten, not dropped.** `TestBuildReadySet_ScopedFiltersByTitle_ExclusionAccounted` and `TestBuildReadySet_EmptyAfterFilter_LoudError` are rewritten over the plan-bead predicate: a non-plan descendant is excluded and named, and an all-near-miss set is loud. `TestNextSelection_{ClaimPath,EmitOnlyPath}_PartialExclusionNoticePrinted` gain the ID assertion. `exclusionReasonForName` is replaced by the near-miss classifier. `containsSpecID` survives only as R1.2's title-linkage test.
8. **Claim and dispatch use one spec.** `next.GateReadiness` receives the spec ID the claim will dispatch into: `--spec`, or the unscoped path's resolved spec. Before any mutation, it refuses when `readiness.EvaluateReadiness`'s lineage-resolved spec differs: `bead <id> resolves to spec <T> by lineage but is being claimed for spec <S>; nothing was claimed`, with `recovery: bd show <id> --json`. This holds by construction for R1.1 members, so it is a guard for the unscoped path and for beads with conflicting epic dependencies. `EvaluateReadiness` returns the resolved epic and spec, which it already computes.

### R2 — No silently truncated bd row set

1. **Every row-consuming bd `ready`/`list` call passes `-n 0`.** These are the uncapped sites in Background B, `adopt_lattice.go`'s `adoptListBDFn` (migrated to the waist), and every call R1 adds.
2. **Through one waist, with no cap escape.** `internal/bead` helpers `ListJSON` and a new `ReadyJSON` append `-n 0` unconditionally. A caller cannot pass `-n`/`--limit` through them; a caller that wants fewer rows slices client-side. Argument-recording seam tests pin the argv.
3. **Anti-drift is mechanised, by call site.** The lint `TestBDRowCapWaist` (`internal/lint`) walks non-test Go sources. It fails on a call whose callee is `bead.RunBD`, `bead.RunBDCombined`, `exec.Command` with first argument `"bd"`, or a package-level func var initialised to one of them, when its first bd argument is the literal `"ready"` or `"list"`, it lies outside `internal/bead`, and it carries no `// rowcap-allowlisted: <reason>` marker. Dispositions:
   - `internal/doctor/beads.go` probe: allowlisted (rows unread).
   - `internal/redact/redact.go`'s `"list"`: not a call, so not matched.
   - `internal/harness` `sandbox.runBD(...)`: a method outside the callee set; scratch databases only; not migrated.
   - `approve/adopt_lattice.go`: migrated (R2.1).

   **Defence in depth for `ready`:** on success, `ReadyJSON` captures stderr separately. Because it always appends `-n 0` itself, any `Showing N of M` notice means bd ignored the request. It returns `bd truncated the ready set (<escaped notice>) although mindspec requested no cap` with an ADR-0035 recovery line. bd prints no notice for a truncated `list` (measured), so for `list` the argv pin and the lint are the guard, and the spec claims no more.
4. **Version floor.** `doctor`'s `bdVersionFloor` stays `1.0.4` if planning verifies three things at that floor: `-n 0` is unlimited, `bd ready --json` rows carry `parent` when set, and they carry `metadata` when set (R1.1 and R1.2 depend on these). Otherwise the floor is raised in that bead with the evidence.

### R3 — Spec linkage: no new datum

1. **Selection reads the tree, not a linkage key.** R1.1's membership is the parent edge plus the plan-bead ID form. `metadata.spec_id`, already written on every lifecycle bead by `approve/plan.go`, is read only as a near-miss signal (R1.2) and as one disjunct of MF-3's dependency membership (R4.1). The epic gets **no** new `spec_id` key, because that would put the epic row into every metadata-field query (verified). No labels are added. bd's native spec-id field is not adopted: `bd list --spec` matches by prefix, `bd ready` has no spec filter, and it would be a second home for one fact.
2. **Pinned.** A test over `planRunBDFn`'s recorded `bd create` argv asserts that every bead carries `--metadata` JSON with `spec_id` equal to the spec ID, passing `idvalidate.SpecID`. This is existing behaviour, now load-bearing for R4.1.
3. **No migration, no backfill.** Every existing plan bead already has the plan-bead form and a parent edge.

### R4 — MF-3 tells the truth (#239, #232)

1. **Dependency membership is a status-independent tracker fact.** `evaluateMF3` holds a closed `blocks` dependency D to the landed-merge test iff D is a member of the bead's lineage spec S (epic E, which `EvaluateReadiness` already resolves). D is a member iff ANY of:
   - (i) `bead.PlanBeadIndex(E, D)` succeeds;
   - (ii) D's `metadata.spec_id` equals S, read from the dependency entry that the bead's own `bd show --json` already returns (`fetchBeadRecordReal` keeps it);
   - (iii) D's parent equals E, read by one `bd show <D> --json` through a new seam, and only when (i) and (ii) are both negative.

   The title is never used. Outcomes:
   - All three negative from successful reads → non-member: MF-3 PASSES on bd-closed alone and renders `dependency <D>: closed; not a bead of spec <S> — landed-merge test not applicable`.
   - Any read or parse error → MF-3 FAIL `could not determine whether <D> belongs to spec <S>: <escaped error>`, with `recovery: bd show <D> --json`. It is never the not-applicable PASS.
   - A metadata-only or non-plan member held to the test and failing it adds `<D> is linked to spec <S> by <disjunct> but is not a plan bead; if its work never landed as bead/<D>, override deliberately` and the scoped override line (R6).
   - A non-closed dependency of either kind fails as today.
2. **Reachable-landing fallback, scoped to the MF-3 and `reattest` callers.** Candidate generation keeps the first-parent scan as primary. Only when it nominates **zero** owned two-parent candidates does a fallback scan run, through a new `gitutil` primitive (`git log --merges --topo-order <spec-branch>`, newest-first in topological order). The fallback scans every merge reachable from the tip and admits only a two-parent merge whose subject is exactly `Merge bead/<id>` (`gitutil.MergeInto`'s form) or exactly `Merge branch 'bead/<id>' into <spec-branch>`. bead/<id> is the SOURCE in both. It excludes:
   - a merge INTO `bead/<id>` (a refresh from the spec branch);
   - a merge into another bead or spec branch;
   - an unrecognised format: the G2-2 any-token fallback in `parseMergeSubjectBeadBranch` does not apply on the fallback path.

   Same-second-parent grouping, the three corroboration legs, and the oldest-anchored revert/reapply test then run unchanged. `--topo-order` defines newest and oldest. The `SubsumptionConflict` rationale becomes "M is an ancestor of the tip" (true for both scans), and the `FirstParentMerges` doc contract is updated. Callers, and what changes for each:

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

   Decision rule when legs disagree: an exact (b) surviving-tip or (d) binding contradiction still contradicts, naming its datum, whatever the other legs say. The panel set never vetoes. The set alone never confirms an ancestor (settled fact 4). `TestFindLandedMerge_ReviewedHeadSHAContradicts`, `TestReattest_PanelShaContradictionRefuses` and the two `…HostilePanelSHAEscapedInRefusal` tests are rewritten to the non-corroborating outcome, keeping the escaping assertion where a SHA is still rendered.
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
   - `GateReadiness` proceeds iff the currently failing set is a non-empty SUBSET of the named set.
   - If any other signal fails, it refuses with the standard NOT-READY report plus `recovery: mindspec next <bead-id> --allow-not-ready=<all currently failing ids> --evidence <path-to-justification>`. The bead ID is idrender-gated, the IDs come from the closed set, and the placeholder is literal and non-mechanical.
   - A named signal that passes prints `MF-x named but passing — not recorded`.
   - The BARE flag, and the space-separated `--allow-not-ready MF-3` that cobra's `NoOptDefVal` parses as bare-plus-positional, always refuse before positional resolution. `MF-3` is never treated as a bead ID. The recovery is the scoped line above, or `recovery: mindspec next <bead-id>` when nothing fails.
   - `--allow-not-ready=all` is NOT admitted (OQ-5).
2. **Fresh evidence on every claim; an existing marker never authorizes one.** The scoped override REQUIRES `--evidence <path>`: a readable regular file of 1 byte to 64 KiB, read before any mutation. `RecordReadinessOverride` REPLACES the marker (never unions it) with schema v2:

   ```
   {schema: 2, signals: [failing ∩ named], details: {<id>: sha256(<the signal's rendered Detail>)}, overridden: <RFC-3339>, evidence: {path, sha256, bytes}}
   ```

   The evidence **text is not stored** (ADR-0025 touchpoint). The marker-before-claim, fail-closed, rollback-on-claim-failure contract (settled fact 2) is unchanged.
   - A re-claim after `mindspec release` needs the flag and evidence again. `release` does not clear the marker, and it does not need to, because no gate reads it for claim authority.
   - With an existing marker covering MF-2 and MF-3, a new request naming only MF-3 while both fail still refuses.
   - The marker is read in `cmd/mindspec` and passed into the report as data. `readiness.EvaluateReadiness` never reads it (spec 124 AC-12, `internal/complete/readiness_isolation_test.go`).
3. **One validity contract, fail-closed.** A shared validator in `internal/bead` (beside the marker constants) classifies a marker as VALID, LEGACY or MALFORMED:
   - VALID: schema 2; `signals` a non-empty, duplicate-free subset of the closed set; exactly one 64-hex `details` entry per signal; `overridden` RFC-3339; `evidence.sha256` 64-hex; `evidence.bytes` in [1, 65536]; `evidence.path` non-empty.
   - LEGACY: no `schema` key, `{signals, overridden}` only. This is today's writer, and such markers are already in the tracked JSONL.
   - MALFORMED: anything else.
   - A metadata read error is an error, never "no marker".

   Integrity is shape and self-consistency only. The marker is agent-writable bd metadata and is not authenticated (ADR-0037 §8).
4. **`ready-check` honours only VALID markers, as the dispatch-ingress answer.** `mindspec bead ready-check <id>` reads the marker. A failing signal X is COVERED iff the marker is VALID, X is in `signals`, and `details[X]` equals the sha256 of X's current rendered Detail. An override recorded for one failure therefore never covers a different failure under the same signal ID. Rendering:
   - covered → `MF-x: OVERRIDDEN (recorded <ts>; evidence <sha256-prefix> <escaped path>)`;
   - recorded but now passing → `MF-x: PASS (override recorded but no longer needed)`;
   - LEGACY → `override marker is legacy (no evidence) — not honoured; re-record with --allow-not-ready=<ids> --evidence <path-to-justification>`;
   - MALFORMED → `override marker is malformed — not honoured`.

   Exit codes: 0 iff every signal passes or every failing signal is covered; otherwise non-zero naming each UNCOVERED signal; a metadata read error refuses non-zero. This exit code replaces `ms-bead-impl`'s prose comparison (R6.5). It is never claim authority: only R6.2's fresh flag and evidence can authorize a claim.
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

- `internal/next/beads.go`, `internal/next/select.go`, `internal/next/ready_gate.go`, `cmd/mindspec/next.go` (R1, R6)
- `internal/bead/bdcli.go`, `internal/bead/hygiene.go`, `internal/bead/readiness.go`, plus a new plan-bead helper (R1.1, R2, R4.1, R6.3)
- `internal/lifecycle/landed.go`, `internal/lifecycle/reattest.go` (R4.2, R4.3); `internal/lifecycle/orphans.go`, `internal/lifecycle/stale_open.go` (R2 only); `internal/gitutil/gitops.go` (R4.2 primitive, `FirstParentMerges` doc contract)
- `internal/complete/complete.go` (R2, the post-close ready lookup only)
- `internal/validate/readiness/readiness.go`, `report.go`, `fixtures.go` (R1.8's returned lineage, R4.1, R4.4, R5, R6.4 rendering; `registerLandedPanel` gains a slug parameter)
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
- **Phase-lineage unification.** `phase.FindEpicForBeadWithCache`'s `[NNN]` title fallback is why `ready-check` can answer for a bead that `next` does not select (#255's "three answers"). 129 keeps that function unchanged (core domain). It makes the disagreement visible instead: the R1.2 near-miss sentence, plus R1.8's lineage-equality refusal. Removing the title fallback is 129b's.

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

- [ ] **AC-1 (R1.1, R1.2)**: Fixture (fake bd that honours `--parent`, `-n` and the default cap; ready rows carry `parent`/`metadata` when set). Epic E for spec S has the following beads:
  - A `<E>.1`, parent E, title neutral;
  - D `<E>.2`, parent none;
  - P `<E>.3`, parent another bead;
  - R, non-plan-form, parent E;
  - M, `metadata.spec_id=S`, no parent, title neutral;
  - T, titled `[S] …`, no linkage;
  - plus E's own epic row in the global set.

  Expected:
  - `next.BuildReadySet` returns members {A} only. The near-misses are D (N1), P (N2), and R, M, T (N3, each naming its linkage). The epic row is in neither list.
  - `mindspec next --spec S --emit-only --pick 1` emits A, and the claim path with `--pick 1` claims A.
  - The output contains every near-miss ID (`TestBuildReadySet_PlanBeadPredicate`, `TestNext_ScopedSelection_NamesNearMisses`).
  - Red-on-revert: today A is excluded by the title filter, and T is returned only through its title.
- [ ] **AC-2 (R1.2 N1 recovery is real)**: A hermetic test (fake bd, no `t.Skip`; spec 124's bd-less rule) uses the row and `bd show` shapes measured on bd 1.1.0. Background C covers the measured shapes, and the round-1 probe ran on a scratch database (`bd create --parent`, `bd dep remove`, `bd update --parent`). The test runs in two steps:
  - Before the update: a detached `<E>.3` row (no `parent`) is classed N1, with the `bd update <id> --parent <E>` line.
  - After it: the row carries `parent: E` and `bd show` carries the `parent-child` edge to the epic. The bead is then a member, `phase.FindEpicForBead` resolves E, and `readiness.EvaluateReadiness` reaches its signals, with no lineage or index error.

  (`TestNearMissN1_RecoveryYieldsClaimable`.) The same sequence against a real throwaway bd database is a Validation Proof.
- [ ] **AC-3 (R1.3)**: The claim path, with the selector-specific sentences above:
  - With an empty member set, `mindspec next <id> --spec S` and `mindspec next --spec S --pick 1` each exit non-zero, name the selector, and end with a line matching `^recovery: mindspec next --spec `.
  - A near-miss positional gets its R1.2 sentence on both an empty and a NON-empty member set, and never "not found or not ready".
  - `--emit-only <near-miss>` still prints the pinned bypass line and emits.

  (`TestNext_SelectorOnEmptySet_Refuses`, `TestNext_NearMissPositional_Refuses`.) Red-on-revert: today it exits 0 with "No ready work found.".
- [ ] **AC-4 (R1.4)**: Four fixtures with an empty member set, no selector and no near-miss, each with an explicit STORED phase:
  - (i) stored `implement`, all lifecycle children closed → `stored phase: implement; child-derived phase: review`, `recovery: mindspec impl approve S` and `recovery: mindspec repair phase S`;
  - (ii) stored `implement`, one child in_progress with a missing worktree → the tally and phase lines print BEFORE the worktree-recovery output;
  - (iii) stored `implement`, children open and blocked → `<n> open lifecycle bead(s), none bd-ready` and `recovery: bd blocked --parent <E>`;
  - (iv) stored `plan`, zero lifecycle children → `recovery: mindspec plan approve S`.

  Each output contains the tally and the verbatim sentence, and exits 0 except where the arm mutates. A fifth fixture, with one near-miss, exits non-zero with the same lines. A failing child read (seam) renders the indeterminate form, no tally, and exits non-zero (`TestNext_EmptyScoped_DerivedState`). `TestImplementTemplate_EmptinessRule` asserts that the template contains the sentence. Red-on-revert: fixture (i) today prints "No ready work found." and derives `implement`.
- [ ] **AC-5 (R1.5, R1.6, R1.7)**: Every rendered exclusion or near-miss notice contains each excluded ID via `idrender.Bead`, and a count-only notice is red (`TestExclusionNotice_NamesIDs`). The multi-spec menu prints `--spec=<spec-id>` and never `<number>` (`TestNextMenu_PrintsSpecIDForm`; red today). The rewritten spec-128 tests named in R1.7 pass.
- [ ] **AC-6 (R1.8)**: `GateReadiness` for a bead whose lineage resolves to spec T while it is being claimed for S refuses before any mutation (bd, branch and worktree byte-identical, spec 124's AC-4 audit shape), naming both specs (`TestGateReadiness_LineageSpecMismatchRefuses`). Red-on-revert: today it validates T and dispatches into S.
- [ ] **AC-7 (R2.1–R2.2)**: Argument-recording seams assert `-n 0` at every site in R2.1: `TestQueryReady_Uncapped`, `TestQueryReadyForEpic_Uncapped`, `TestResolveActiveBead_Uncapped`, `TestOrphans_ListUncapped`, `TestStaleOpen_ListUncapped`, `TestHygiene_ListUncapped`, `TestComplete_PostCloseReady_Uncapped`, `TestAdoptLattice_ListViaWaist`. The fake bd used end-to-end APPLIES the default cap (100 for `ready`) unless `-n 0`/`--limit 0` is in argv. With 101 ready rows, `next --emit-only` lists 101 items and `--pick 101` emits the last (`TestNext_101ReadyBeads_AllVisible`). Red-on-revert: today it lists 100.
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
- [ ] **AC-11 (R4.2)**: Real-git fixtures extending `initLandedRepo`/`mergeBead` (`internal/lifecycle/landed_test.go`):
  - (i) #239's topology: X lands on spec/S; spec/S's content reaches `main` through a sibling branch's PR merge; spec/S is fast-forwarded to `main`. X's landing is then reachable only through a second parent. `FindLandedMergeReachable` and `ReattestLandedMerge` both identify it (red today: `ErrLandedMergeNoCandidate` / `ReattestStateNoOwnedMerge`).
  - (ii) The same topology, with X first refreshed by `Merge branch 'spec/S' into bead/X`, is still identified and not refused as ambiguous.
  - (iii) X merged into sibling bead Y (`Merge branch 'bead/X' into bead/Y`) before landing: both X and Y are identified.
  - (iv) A refresh followed by a FIRST-parent landing is identified by the primary scan. This is the bead-4 shape, and it is unchanged.
  - (v) A correctly named merge on a branch NOT reachable from the tip is not a candidate.
  - (vi) `MergedUnclosed` for fixture (i) returns "not this state", so `complete`'s reconcile does not newly close a bead on an off-first-parent landing.

  Background D.2 records the round-1 evidence for (i)–(iv). Tests: `TestFindLandedMergeReachable_{OffFirstParent,RefreshedThenOff,SiblingBeadMerge,FirstParentRefresh,Unreachable}`, `TestReattest_NominatesSameCandidateAsFind`, `TestMergedUnclosed_IgnoresFallback`.
- [ ] **AC-12 (R4.3, R4.5)**: Registrations via `registerLandedPanel` (extended with a slug):
  - (i) dirs `<X>` with `reviewed_head_sha = A` (stale) and `<X>-r2` with `B` = the second parent → `FindLandedMerge` CONFIRMS (red today: the lexically first `A` contradicts).
  - (ii) registrations {A}, surviving tip = B → confirms via the tip leg; the panel leg is non-corroborating.
  - (iii) registrations {A}, surviving tip ≠ B → contradicts naming the tip datum (the mixed-leg rule).
  - (iv) #232/#233 shape: one registration {B}, the merge's second parent is a later tree-identical commit D, the binding records the merge → MF-3 PASS via (d) (red today: the zero-candidate sentence).
  - (v) the same with no binding and no branch → `*LandedMergeNoEvidence` naming the candidate. `ReattestLandedMerge` then writes the binding with a corroboration string recording the panel as non-corroborating, and MF-3 PASSES (red today: `ReattestStatePanelContradiction`).
  - (vi) the spec branch locally ahead of its remote-tracking ref → MF-3 PASS. This is a regression guard, green today.

  Tests: `TestFindLandedMerge_PanelSet*`, `TestReattest_PanelSetNonCorroborating`, `TestReadiness_MF3_CompleteWrapperShape`, `TestReadiness_MF3_LocalAheadOfRemotePasses`.
- [ ] **AC-13 (R4.4)**: `TestReadiness_MF3ErrorArmDiscrimination` gains four legs:
  - `ErrLandedMergeNoCandidate` → the "no two-parent merge reachable … lands bead/" sentence;
  - a binding contradiction → the "a merge landing bead/<id> exists (<sha>) but … contradicts" sentence, and NOT the zero-candidate one;
  - a revert shape → "was reverted or cleanly removed";
  - each ends with its own recovery line, and none recommends `mindspec complete` for the existing-merge arms.
- [ ] **AC-14 (R5)**: `TestMF2_FormSymmetry` passes over the full table. `TestReadiness_MF2AllClasses` gains the six R5.2 legs. Red-on-revert: legs 1, 4 and 6 fail today.
- [ ] **AC-15 (R6.1)**: Each leg below is a separate fixture or invocation:
  - MF-3 failing only: `--allow-not-ready=MF-3 --evidence <f>` claims and records `signals: [MF-3]`.
  - MF-2 and MF-3 failing: `--allow-not-ready=MF-3 --evidence <f>` refuses with nothing mutated (byte-identical audit), and its final `recovery:` line contains `--allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>` and no operator-supplied path.
  - The bare flag refuses with the same line.
  - `--allow-not-ready MF-3` (space-separated) refuses as bare, and `MF-3` is never looked up as a bead.
  - `--allow-not-ready=MF-9` refuses before any mutation.
  - A missing `--evidence` refuses before any mutation.

  (`TestGateReadiness_ScopedOverride`, `TestNext_BareAllowNotReady_Refuses`, `TestNext_AllowNotReadySpaceSeparated_Refuses`.)
- [ ] **AC-16 (R6.2, scope widening)**: Three scenarios:
  - Request `MF-2,MF-3` while only MF-3 fails: the stored marker holds only `MF-3`, with its `details` digest and `evidence.{path,sha256,bytes}`, and no `text` key. Then make MF-2 fail: `ready-check` exits non-zero naming MF-2 as uncovered, and an UNFLAGGED re-claim refuses.
  - An existing marker covers MF-2 and MF-3, both fail, and a new request names only MF-3: it refuses, and the marker is unchanged.
  - A 64 KiB + 1 evidence file refuses with zero metadata writes (seam).

  (`TestOverride_NoScopeWidening`, `TestOverride_ExplicitNarrowerRequestRefuses`, `TestAllowNotReady_EvidenceTooLargeRefuses`.)
- [ ] **AC-17 (R6.3, R6.4)**: `ready-check` with MF-3 failing:
  - plus a VALID marker for MF-3 with a matching digest → exit 0, `MF-3: OVERRIDDEN (…)`;
  - plus MF-2 also failing → non-zero, naming MF-2;
  - with the MF-3 Detail changed (a different failing dependency) → non-zero, MF-3 uncovered;
  - with MF-3 now passing → the "no longer needed" note;
  - with a LEGACY marker → non-zero, plus the legacy note;
  - with MALFORMED markers (unknown ID, duplicate ID, missing digest, bad timestamp, non-hex sha256, `bytes` = 0) → non-zero, plus the malformed note;
  - with a metadata read error → refuses non-zero.

  `EvaluateReadiness` does not read the marker (the isolation test stays green) (`TestReadyCheck_HonoursValidMarkerOnly`, `TestOverrideMarker_Validity`). Red-on-revert: the first case exits non-zero today.
- [ ] **AC-18 (R6.5, R7, ADR-0041)**:
  - `ms-bead-impl/SKILL.md` contains no `mindspec_readiness_override` subset comparison and contains the `ready-check` exit-code line; the rewritten pin tests pass.
  - `.mindspec/adr/ADR-0045-…md` exists with `Status: Accepted`, the Status fence, Amends ADR-0040 §1 and ADR-0044, and step 1 → spec 130.
  - ADR-0040 and ADR-0044 carry the back-pointer; `mindspec adr list` includes ADR-0045.
  - ADR-0041 contains § "Amendment (Spec 129)" naming the fallback-scoped callers, the non-corroborating panel datum, and the scoped fourth-verb override (a text-pin test in the R4 or R6 bead).
  - `mindspec validate spec 129-readiness-tracker-honesty` resolves every touchpoint link.
- [ ] **AC-19 (method)**: `go build ./... && go test ./... -short` is green. `internal/harness` and `internal/instruct` tests run under `-short` only (128 settled fact 3). `go test ./internal/lint/...` is green, including `TestBDRowCapWaist` and the ADR-0030 boundary lint (no new `os/exec` or `gitutil` import in `internal/validate/readiness`).

## Validation Proofs

- Fixture repo with beads A (plan bead), D (detached plan-form), M (`metadata.spec_id` only), T (title only): `mindspec next --spec <S> --emit-only` lists A only and names D, M and T with their reasons. `bd update <D> --parent <E>` followed by `mindspec next --spec <S> --emit-only` lists A and D. This runs against a real throwaway bd database, where D was made with `bd create --parent` and `bd dep remove` (AC-1, AC-2).
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
  - 129b = G (#234), H (#244), #250, I (`merge_strategy`), plus phase-lineage unification.
  - 130 = the S0 rule table (pure policy, a different review centre, an unowned path that first needs an OWNERSHIP claim) and #233's review-to-landing binding.

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
  - No selector and nothing excluded → exit 0 (HC-4: the read evaluation completed).
  - ≥1 near-miss → non-zero (128's loud-empty class).
  - A selector supplied → non-zero.
  - A failed child read → non-zero.
- [x] **OQ-8 (why not surface stderr on success in `tracedOutput` globally?) — RESOLVED: scope it to the waist.** Only in the row-set waist does a notice have a defined meaning (truncation) and a defined response (an error).
- [x] **OQ-9 (membership design: narrow vs widen) — RESOLVED: Option N (narrow), adopted; Option W rejected.**
  - *Option W*: a descendant-OR-`metadata.spec_id` membership shared by selection, readiness, MF-3 and dispatch. It would require `phase.FindEpicForBead` (core domain) to learn a metadata leg. It would also need MF-1's `beadIndex` to map non-dotted members onto plan sections, which no rule defines, and a precedence rule for conflicting linkage. That is new surface in three packages to make claimable a class of bead that no shipped flow creates: plan approve always writes the parent edge and the `<E>.<N>` ID.
  - *Option N*: selection is the plan bead, which is exactly the set readiness and `complete` can already evaluate. Every other linked bead is named with its true reason. The one repairable class (a detached plan-form bead, N1) gets a recovery that an integration test proves claimable (AC-2), as shown in a scratch bd 1.1.0 database. MF-3's dependency membership stays wider (plan-form OR spec_id OR parent), because holding more dependencies to the landed test is the fail-closed direction.

  #255's two reported defects are both fixed: a dropped bead is named, never reported as "no work"; and the three answers now disagree visibly, with reasons. Re-opens if planning finds a shipped flow that creates spec beads without the plan-bead form.

### Grill record (self-answered, headless — 2026-10-18; revised after round 1)

- [x] grill: does the membership predicate admit beads readiness cannot evaluate? → It did in the draft (metadata-only, reparented non-dotted). Narrowed to the plan bead (OQ-9). AC-2 proves the N1 recovery on a real bd.
- [x] grill: is bd's spec-id field a cheaper linkage? → It is filterable by `bd list --spec` (prefix) and shown by `bd show`, but `bd ready` cannot filter it, and it would be a second home. Not adopted (R3.1).
- [x] grill: is #232 the same bug as #239? → No. Its dependency is the same spec's bead, and its mechanism is #233's: the panel SHA is never the landing's second parent. R4.3 (both functions) plus R4.4 close it end-to-end (AC-12 (iv)/(v)). No remote ref is read.
- [x] grill: does a full-history scan fix #239 safely? → No. A refresh merge into `bead/X` becomes a second candidate, and the ambiguity rule refuses (reproduced). Redesigned as a first-parent-primary scan with a source/target-pinned fallback, scoped to MF-3 and reattest (R4.2, AC-11).
- [x] grill: does removing the panel veto reintroduce ancestor-tolerant confirmation? → No. The set confirms only on exact equality; the tip and binding contradictions remain.
- [x] grill: does R6 open a gate on a forged marker? → No gate reads the marker for claim authority (fresh flag and evidence on every claim). `ready-check`'s dispatch answer is no weaker than today's skill prose, and it is stricter: a VALID schema plus a per-signal detail digest are required.
- [x] grill: does the evidence leak into git? → Its path, sha256 and byte count do (the tracked JSONL); its text is never stored.
- [x] grill: "A re-run of impl approve tolerates the half-done state" — traced or run? → Traced. Recorded as 129b's obligation to run it.
- [x] grill: does ADR-0045 hold against the tree? → Its Context claims are verified by file. It conflicted with ADR-0044 and understated its ADR-0040 amendment; both are now stated. `Accepted` is fenced per ADR-0032.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
