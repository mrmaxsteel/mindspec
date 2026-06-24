# Pre-Release Remediation Plan — Review Panel (round 1)

**Repo**: `/Users/Max/replit/mindspec` · **main**: `fa3e7f40` (fully CI-green) · **Last release**: `v0.9.0` (2026-06-13).

You are reviewing a **plan**, not a code diff. Assess whether the plan is correct, complete, and safe to execute before tagging the next release. Inspect the live repo to verify the factual claims (do NOT take them on faith).

## Background (what just happened)
- **Spec 106 (flatten `.mindspec/` layout)** was merged via **PR #158** (merge commit `011f9340`) and finalized: `mindspec impl approve 106-layout-flatten` ran, closing epic `mindspec-3d3i` + all 6 beads, phase → idle, spec worktree/branch removed. Two more small PRs merged after: **#157** (test-harness turn-count tweak), **#138** (adds a `spec-orchestrator` agent file). main is now `fa3e7f40`, all CI green.
- The flatten promotes `.mindspec/docs/{specs,adr,domains,core}` + context-map to top-level `.mindspec/`, co-locates panel reviews under `<spec-dir>/reviews/`, evicts dogfood docs to top-level `project-docs/`, drops `glossary.md`+`policies.yml`, and ships a new `mindspec migrate layout` transactional mover. It is claimed **non-breaking** for existing projects via a multi-tier resolver (flat → canonical → legacy) + permanently multi-prefix gate matchers; flattening is **opt-in**.

## The problem the plan addresses
main's **committed** `.beads/issues.jsonl` records spec-106 epic `mindspec-3d3i` and bead `mindspec-3d3i.6` as **open / in_progress**, but **Dolt** (the bd source of truth) has them **closed**. The bd `post-merge` git hook (`bd hooks run post-merge`) re-imports the committed JSONL into Dolt on every merge/FF — so the closes **self-revert on every sync**. Verify yourself:
```
bd show mindspec-3d3i --json ; bd show mindspec-3d3i.6 --json          # Dolt: closed
git show HEAD:.beads/issues.jsonl | grep -E 'mindspec-3d3i["\.]'        # committed JSONL: open/in_progress
cat .git/hooks/post-merge                                               # the re-import shim
```
**Root cause:** `mindspec impl approve` on a *protected* main cannot commit the finalize JSONL to main; it pushed an epic-close commit to the spec branch, which was then deleted as post-merge debris, so the corrected JSONL never reached main.

## THE PLAN UNDER REVIEW
**(a)** Land a small **chore PR** committing the corrected `.beads/issues.jsonl` to main (Dolt-truth: 6 beads + epic closed; follow-ups `sc0w`/`3jq7` open), clearing the self-revert. CI-verify, merge.
**(b)** **File a bead** for the underlying lifecycle bug: `impl approve` on a protected-main repo orphans the finalize commit → bead state self-reverts on sync.
**(c)** *(optional, for cleanest release)* **Fix the two `migrate layout` P2 bugs** via a small PR — `mindspec-sc0w` (precondition false-positives on unrelated pre-flatten branches) and `mindspec-3jq7` (404 link-check scans too narrow a root-doc set) — and flip **ADR-0039** (`Status: Proposed` → `Accepted`) since its layout shipped.
**Then tag `v0.10.0`** (minor bump — flatten is non-breaking) with release notes headlining: new flat layout (non-breaking + opt-in), co-located reviews, `project-docs/` eviction, `migrate layout` mover.

## Questions to answer
1. Is the self-revert diagnosis correct, and does chore-PR (a) **durably** fix it? Any risk the post-merge hook fights the merge (e.g., re-imports at the wrong moment)?
2. Is filing the lifecycle bug (b) the right call, or does it need a real fix *before* release?
3. Should `sc0w`/`3jq7` (c) **block** the release, or is shipping `migrate layout` with a "known limitations / supervised" note acceptable?
4. Is **v0.10.0** right (vs v1.0.0)? Is the "non-breaking" claim justified, or is there hidden breakage for downstream projects on the old layout?
5. **What's MISSING** from the pre-release checklist? Consider: install-smoke against a *flat-layout* project, downstream migration testing, CHANGELOG/release-notes curation, the 2 known pre-existing sandbox-only test failures (`TestRun_IdleNoBeads`, `TestInstructPhaseDetection/{plan,implement}`), ADR governance, the untracked `review/106-*` artifacts.

## Your job
Verify the claims against the live repo, then judge the plan. **Verdict: APPROVE / REQUEST_CHANGES / REJECT** (REQUEST_CHANGES only for a genuine pre-release blocker the plan misses or gets wrong; REJECT if the plan is fundamentally unsound).

Output JSON to `/Users/Max/replit/mindspec/review/prerelease-plan/<your-slot>-round-1.json` with keys:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array — empty if APPROVE), `findings` (array of {severity, area, issue}).
**Do NOT run `go test ./internal/harness/...` (the LLM suite).**
