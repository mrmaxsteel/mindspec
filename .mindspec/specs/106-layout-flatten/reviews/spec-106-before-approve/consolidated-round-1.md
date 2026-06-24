# spec-106-before-approve — Round 1 consolidated changes

**Tally:** 2 APPROVE (R3, R4) · 4 REQUEST_CHANGES (R1, R2, R5, R6) · 0 REJECT.
Threshold N−1 = 5/6 APPROVE → **NOT passed**. Decision: **REQUEST_CHANGES** (revise spec → re-panel before approve).
Design verdict is positive across the board (3-phase spine + flat-layout direction endorsed by all six); the asks are mechanism-precision and coverage, not redesign.

## BLOCKERS (must fix before approve)

1. **DetectLayout is under-modeled — distinguish canonical vs legacy AND add a mixed/error state.** (R5 hard_block, R6 hard_block — both codex, independent)
   Req 2's `{flat | legacy | greenfield}` collapses two distinct non-flat WRITE shapes (canonical `.mindspec/docs/` and legacy root `docs/`) into one "legacy", so it cannot drive the never-split write-default for canonical-only vs legacy-only projects; and it has no explicit ERROR when a flat lifecycle tree coexists with a canonical/legacy one (different IDs or artifact classes) — first-exists-wins reads mask the residue while writes go flat, reintroducing the half-old/half-flat split. → Make DetectLayout distinguish `flat | canonical | legacy | greenfield | mixed`, make `mixed` a hard error (except inside a recorded in-progress migration recovery), and add AC3 cases for canonical-only, legacy-only, flat, greenfield, mixed, and flat-ID-A+canonical-ID-B.

2. **Define the mover run-state machine + rollback semantics in git terms.** (R5 hard_block, R6 hard_block/major)
   Req 3/4/AC4 name "transactional / idempotent / two-commit / rollback" but never define crash-recovery boundaries or what rollback means after commits land. → Specify checkpoint order (before/after each `git mv`, pure-move commit, link-rewrite, rewrite commit, lineage/state write); required resume behavior after a crash at each boundary; and whether rollback is hard-reset-to-pre-run-ref vs compensating revert commits vs refuse-after-publish. Add AC4 coverage for an injected crash at each boundary.

3. **Specify a feasible, testable branch/PR discovery algorithm for the migrate precondition.** (R6 hard_block, R5 minor)
   Req 10/AC15 require "no unmerged pre-flatten branch/PR" but give no source-of-truth algorithm; AC15 only injects one branch. → Define which refs are scanned (local, remote-tracking, hosted PR metadata), offline/no-remote behavior, how the flatten base/fingerprint is computed, exactly what state blocks, and how locked agent worktrees + external forks are handled (guarded at merge per Req 9, not drained). Broaden AC15 accordingly.

4. **`internal/contextpack/budgeter.go` flat-awareness is ungoverned.** (R1 blocker; overlaps R5 major "root-enumerating DocsDir consumers")
   Background/Domains/Scope describe a silent-failure mode (drops spec/domain context on flat projects) but no Req and no AC govern it; Req 14's test matrix omits the context-pack builder. → Add a requirement that budgeter.go (budgeter.go:170/218) consumes the per-artifact accessors, an AC asserting identical pack bytes/sections flat vs canonical, and add context-pack to the Req 14 matrix.

## MAJORS

5. **Audit ALL direct `DocsDir`-root / enumerating consumers, not just the named accessors.** (R5 major) spec list, domain list/show, context-map enrichment, core-docs paths, doctor/orphan scans ride `DocsDir` directly → add layout-aware root accessors + command-level ACs on flat fixtures.

6. **Name `LoadOwnershipAtRef` (ownership.go:127) + `domainManifestRelPath` (ownership.go:123-125) in Req 5's multi-prefix set.** (R3 major) The divergence gate at `complete` loads ownership via the ref-anchored pair, which also hardcodes `.mindspec/docs/domains/`. If the plan updates only `LoadOwnership` (:79), every domain reads "missing" on a flat tree → every 106 bead hard-blocks `adr-divergence-unowned`.

7. **Tighten the link-rewriter/doctor 404 gate to scan EVERY markdown link in the post-migration moved tree + affected root docs**, not only the finite rewritten set. (R5 major) Add fixtures for review-co-location links and absolute `.mindspec/docs/core`/context-map refs.

8. **Fix AC8 — it is trivially-passable.** (R2 major) `grep ... --include='*.go'` is empty today and cannot reach the `.md` template line at `internal/instruct/templates/spec.md:35`; drop the `*.go` filter (or add `*.md`) and make the grep the load-bearing proof. `! test -e .mindspec/docs/glossary.md` is also trivially-true once Req 12 removes the wrapper.

9. **Resolve the `internal/redact` contradiction.** (R1 major) Impacted Domains claims redact "must become layout-aware," but it's absent from Scope/all reqs and the code has no functional docs-layout dependency (only a regex docstring at :320 + a cobra subcommand-set entry at :129). → Strike the claim (recommended) or add req+Scope+AC.

10. **Add a Req/AC for the DOCS-LAYOUT.md amendment.** (R1 major) It's an in-this-spec deliverable (Background l.47, Scope l.113) but ungoverned, unlike the ADR-0037 amendment (Req 7).

11. **Broaden skill/snapshot verification (AC13) to ALL path-bearing artifacts.** (R6 major, R2 major) AC13 covers only panel snapshots + `internal/setup` tests; it misses ms-bead-impl/prep/cycle/fix, ms-spec-final-review, ms-spec-create, plugin README/FINDINGS, `internal/setup/codex.go`, bootstrap text — and the CENTRAL ms-spec-grill flat-path read (domain-set + next-ADR-number). Add a direct assertion that ms-spec-grill reads flat `domains/`/`adr/`, or explicitly classify out-of-scope items.

## MINORS / COVERAGE

12. **Add direct ACs** for: Req 2 bootstrap-creates-flat (greenfield); Req 3 lineage-manifest write; Req 13 mover-package (`internal/layout/**`) added to workflow OWNERSHIP.yaml. (R2)
13. **AC wording**: AC7 — make the resolver assertion (+`test -d project-docs`) load-bearing, since `! test -d docs` is trivially true today; AC14 — use `git diff --name-status main...<phase1-tip>` (no rename under `.mindspec/docs/`), not `git status`; AC15 — also prove the tolerates-locked-worktrees/forks half. (R2)
14. **Govern the `cmd/mindspec/migrate.go` prompt-rubric reconciliation** (migrate.go:286-287 still routes to `.mindspec/docs/user/` & `…/agent/`) with the new `project-docs/` target. (R1 minor)
15. **Correct the occurrence-count statistic**: "~246 across 23 files" doesn't reproduce; actual is ~99 exact slash-form (21 files), ~110 incl. `filepath.Join` forms. Blast-radius point stands; fix the number. (R4 minor)
16. **Update OWNERSHIP self-glob entries** when domains flatten (each manifest self-claims `.mindspec/docs/domains/<d>/OWNERSHIP.yaml`); non-blocking (doc files) but a latent inconsistency. (R3 minor)
17. **Clarify Req 1 "FIRST" wording** vs the phased delivery (flat tier is resolution-order-first but lands in Phase 3, not Phase 1) so a reader doesn't infer flat dirs exist in Phase 1. (R1 minor)
