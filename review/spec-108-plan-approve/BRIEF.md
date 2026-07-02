# spec-108-plan-approve — Round 1 Review Panel (plan-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-108-cleanup-frontmatter-perf-wave2
**Branch**: spec/108-cleanup-frontmatter-perf-wave2 | **Commit under review**: ae36203b2640b391479a905910ec90f3396caa34
**Target**: the PLAN (.mindspec/specs/108-cleanup-frontmatter-perf-wave2/plan.md) against the APPROVED spec (spec.md). Branch is merged-forward onto post-107 main (b02e236f) per the spec's R11 prerequisite — wave-1's changes are in this tree (findRoot deleted, domain docs appended, FetchChildren exists).

## DISCLOSURE — post-approval spec edit (review it)

Commit "mechanical Impacted-Domains formatting fix": the approved spec's Impacted Domains bullets had prose-style colons/dashes that made contextpack.ParseSpec extract garbage domain names ("workflow** — owns all four work areas' edits" + three attribution-note bullets as domains), which would hard-fail plan validation and every downstream gate. Fix: colon placement on the two domain bullets + attribution notes switched to '*' markers. ZERO prose changed — verify with git show of that commit. If you judge it more than mechanical, say so.

## What the plan does

Four independent beads (no dep edges): B1 ownership claims (internal/trace/** + .golangci.yml into workflow OWNERSHIP) + trace.Event.MarshalJSON deletion + NDJSON golden + all THREE stale unparam carve-out removals (findRoot now inert post-merge-forward) — doc target workflow/architecture.md; B2 frontmatter consolidation (approve mutate helper on frontmatter.Parse + 4 reader migrations + readSpecApprovalStatus→SpecStatus; golden byte-identical + fence-tightening + frontmatter-decides tests) — doc targets workflow/overview.md + context-system/architecture.md; B3 validate perf caching (per-run OWNERSHIP map in 3 call sites + ADR memoization; seam-count ×3 + counting-Store + golden-diagnostics tests) — doc target workflow/interfaces.md; B4 doctor single-walk (walkWorkspaceFn seam; walk-count + identical-Report tests) — doc target workflow/runbook.md. adr_citations: 0036/0032/0033. Validator: exit 0, one advisory (R=0.10 — intentional disjoint file sets).

## Environment notes

z4ps instruct-test failure + internal/harness TestLLM timeouts are pre-existing (use go test $(go list ./... | grep -v 'internal/harness')); GIT_TERMINAL_PROMPT=0 for git-touching tests in nested worktrees.

## Your job

Is this PLAN ready for `mindspec plan approve` (creates 4 beads)? Verdict: APPROVE / REQUEST_CHANGES / REJECT → JSON to `review/spec-108-plan-approve/<slot>-round-1.json` (relative to the worktree root) with keys: reviewer_id, verdict, confidence (0-1), rationale (<=200 words), concrete_changes_required (empty if APPROVE), findings; optional "hard_block": true.
