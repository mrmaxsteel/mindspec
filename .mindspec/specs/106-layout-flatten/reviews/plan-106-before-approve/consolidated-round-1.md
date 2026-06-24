# plan-106-before-approve — Round 1 consolidated changes

**Tally:** 2 APPROVE (R1 decomposition, R3 ADR/gate-safety) · 4 REQUEST_CHANGES (R2, R4, R5, R6) · 0 REJECT.
Threshold N−1 = 5/6 → **NOT passed**. Decision: **REQUEST_CHANGES** (revise plan → re-panel).
The decomposition SHAPE (5-bead DAG, honest edges, chain depth 3, adr_citations) is endorsed (R1+R3 verified). The asks are real seam-accuracy + sequencing defects that would break implementation — substantive, not cosmetic.

## BLOCKERS (hard_block)

1. **Bead 4 merge-guard names a function that does not exist.** (R4) Plan says add the layout-fingerprint hard-fail to `internal/complete MergeBranch` — no such function. Real seams: raw helpers `internal/gitutil.MergeBranch`/`MergeInto`; bead→spec flows `complete.Run → exec.CompleteBead`; spec→main finalization flows `internal/approve → executor.FinalizeEpic`. → Rewrite Bead 4 step + key_file_paths + tests to guard BOTH merge surfaces at their real seams (or an executor-level guard covering both).

2. **Bead 3 mover execution seam is unbacked.** (R4) Plan routes the git-mv driver through `internal/executor`, but the `Executor` interface has no git-mv / hard-reset / clean / ref-discovery primitives, and Bead 3 doesn't list `internal/executor`/`internal/gitutil` as changed surfaces. → Add the executor/gitutil API work (git mv, hard-reset, clean, branch/ref discovery, rollback) to Bead 3 scope + changed files + tests, OR redesign onto an existing repo-real seam.

3. **Bead 2 review classifier is a destructive REPLACE, mid-transition.** (R5) Replacing `HasPrefix(path,"review/")` with a spec-dir `/reviews/` exclusion in Phase 1 — before any review is co-located and before Bead 4 changes the panel scan — makes still-live root `review/**` artifacts read as source/unowned in the interim and trip gates. → Make it ADDITIVE: keep the root `review/**` exclusion as a PERMANENT historical-ref compatibility matcher (analogous to the permanent multi-prefix doc/ownership matchers) AND add the new spec-scoped `/reviews/` exclusion. Verification must assert BOTH root `review/<slug>/...` and `<spec-dir>/reviews/<slug>/...` classify non-source.

4. **Bead 5 is over-converged for one reviewable `complete`.** (R6) It bundles the irreversible tree move + dogfood eviction + vestigial drops + self-glob updates + ADR-0039/DOCS-LAYOUT/ADR-0037 governance + migrate.go rubric + harness/testdata fixture migration. → SPLIT: at minimum separate (a) the irreversible filesystem move + review co-location migration from (b) governance/rubric/static-text/fixture cleanup. (Accept 6 beads with the irreversibility/reviewability justification.)

5. **The existing root `review/**` artifacts are never migrated.** (R6) Req 8/goal require ELIMINATING repo-root `review/` by co-locating the (42) tracked artifacts under `<spec-dir>/reviews/`, but no bead step moves them — AC13/AC17 only prove scanner behavior. → Add an explicit step + verification migrating existing root `review/**` → `<spec-dir>/reviews/**` and removing the root tree.

6. **Bead 4 rewrites static skills/snapshots to flat paths BEFORE the tree is flat.** (R6) Leaves an intermediate canonical repo whose live skill instructions point at missing `.mindspec/{domains,adr}` paths. → Either make the skill/snapshot text dual-layout-safe, or move the flat-path rewrites to AFTER the flatten (the move bead), so no intermediate state is broken.

7. **AC22 Phase-1 "go test ./... green" vs deferred harness fixtures.** (R6) Resolve honestly: in Phase 1 nothing has moved (canonical tree still present, canonical resolver tier intact), so clarify WHY Phase-1 `go test ./...` is green (fixtures still reference the still-present canonical tree) and exactly when/whether harness scenario fixtures need migration (only if/when they must exercise the FLAT path). Make the fixture-migration timing explicit and the AC22 claim provably honest — don't leave a latent "full-suite green" claim that a later move falsifies.

## MAJORS

8. **AC5 verification path is incomplete.** (R2) AC5 names spec-list + domain list/show + doctor scans, but Bead 2's `go test` omits `internal/spec` and `internal/domain` (where `spec list` / `domain list|show` live, with `list_test.go`/`show_test.go`). → Add `./internal/spec/... ./internal/domain/...` to Bead 2's verification.

9. **AC10 doctor link-check lane unpinned.** (R2) The link-existence check is net-new; the shared-file note says it lives in `internal/doctor` but Bead 3's changed-files + `go test` (`./internal/layout/... ./cmd/mindspec/...`) omit `internal/doctor`. → Pin where the link-check lane lives and align the go-test path so the AC10 build-half is actually exercised.

10. **Dependency convergence integration risk** (R4/R6, non-block). The 2+3+4→5 convergence concentrates cross-branch conflict (migrate.go, doctor, moved-tree fixtures, link checks, ownership manifests) in the final bead — mitigated by splitting Bead 5 (item 4).

## MINORS

11. AC17 step list omits `ms-spec-create` by name (caught by the breadth grep, coverage holds — name it for clarity). (R2)
12. Extract a shared layout-fingerprint/signature helper between Bead 3 and Bead 4 to avoid duplication (code-quality). (R1)
13. ADR-0039 terminal-status governance hygiene; context-system resting solely on ADR-0018 (mechanically valid). (R3)

## NOT changing (panel-endorsed)
- The 5→(6) bead DAG shape, honest `depends_on` edges, Bead 3 independence, chain depth ≤3, and `adr_citations: ADR-0037/0018/0023/0025` (all Accepted, all domains covered; ADR-0039 lands Proposed-but-uncited → no `adr-divergence-proposed` block). (R1+R3 verified in-code.)
