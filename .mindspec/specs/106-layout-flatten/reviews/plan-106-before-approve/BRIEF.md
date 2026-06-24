# plan-106-before-approve — Round 2 Review Panel

**Target**: `spec/106-layout-flatten` (plan-before-approve; `bead_id` null)
**Plan under review (REVISED)**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.mindspec/docs/specs/106-layout-flatten/plan.md`
**Approved spec**: `…/106-layout-flatten/spec.md` (17 reqs, 25 ACs)
**Repo root**: `/Users/Max/replit/mindspec`

## Round 2 context

Round 1 returned **REQUEST_CHANGES** (2 APPROVE / 4 REQUEST_CHANGES) with 7 hard_blocks — real seam/sequencing defects. The decomposition SHAPE (DAG, honest edges, adr_citations) was endorsed (R1+R3). The plan has been REVISED — now **6 beads** — addressing every consolidated round-1 item. Read the consolidated list: `/Users/Max/replit/mindspec/review/plan-106-before-approve/consolidated-round-1.md`.

Key round-1 blockers and how the revision claims to fix them:
1. **Bead 4 merge guard** → now at the REAL seams in `internal/executor/mindspec_executor.go` (guard before `gitutil.MergeInto` in `CompleteBead` + `FinalizeEpic`; before `gitutil.MergeBranch` in `FinalizeEpic`), tested via `./internal/executor/...`.
2. **Bead 3 mover seam** → Bead 3 now ADDS `GitMv`/`ResetHard`/`CleanForce`/`LocalBranchRefs`/`RemoteTrackingRefs` to `internal/gitutil/gitops.go` + the `Executor` interface (+ mock), in scope/changed-files/tests.
3. **Bead 2 review classifier** → now ADDITIVE: keeps the root `review/**` exclusion (divergence.go:280) as a permanent matcher AND adds a `/reviews/` segment exclusion; test asserts both non-source.
4. **Bead 5 over-converged** → split: Bead 5 = the isolated irreversible move; Bead 6 = post-move governance/rubric/static-text/fixtures.
5. **Root `review/**` migration missing** → Bead 5 step 2 migrates the 42 tracked root artifacts → `<spec-dir>/reviews/**` and removes the root tree (`! test -d review`).
6. **Static-skill rewrites before flat** → moved to post-move Bead 6 (Bead 4 is now code-only).
7. **AC22 honesty** → reconciled: Phase-1 green because nothing moved (canonical tree + resolver tier intact); fixtures migrated only in Bead 6.
Plus AC5 (Bead 2 adds `./internal/spec/... ./internal/domain/...`), AC10 (link-check pinned to `internal/doctor/links.go`, Bead 3 go-test includes `./internal/doctor/...`).

**New structure:** Beads 1-6; roots {1,3}; 2,4 fan from 1; 5 converges 2+3+4; 6 depends on 5. Longest chain = **4** (justified override: irreversible move isolated for reviewability; 5→6 dependency is intrinsic). `adr_citations` unchanged (ADR-0037/0018/0023/0025, all Accepted).

## Your job (round 2)

Re-read the REVISED plan.md in full. For YOUR lens (same as round 1 — your round-1 verdict is at `<your-slot>-round-1.json`), evaluate each concrete_changes_required item you raised as **ADDRESSED / PARTIAL / MISSED**, verifying the claimed fixes against the REAL code (especially the newly-named seams: executor merge points, gitutil primitives, `internal/doctor/links.go`, `internal/spec`/`internal/domain`). Surface any NEW issue the revision introduced (e.g. the depth-4 chain, the Bead 5/6 split boundary, a mis-mapped AC). Bar for APPROVE: your round-1 asks addressed (or any remainder minor enough to defer to impl), no new blocker.

**Verdict**: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/plan-106-before-approve/<your-slot>-round-2.json` with keys:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array; empty if APPROVE), `findings` (array of {severity, area, issue, status?, hard_block?}) where `status` is ADDRESSED/PARTIAL/MISSED/NEW per round-1 item.
