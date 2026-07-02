# spec-107-approve — Round 1 Review Panel (spec-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-107-cleanup-deadcode-dry-wave1
**Branch**: spec/107-cleanup-deadcode-dry-wave1
**Commit under review**: c7c70000 — docs(spec-107): draft spec for round-1 approval panel
**Target**: the SPEC DOCUMENT itself (pre-approval gate). No implementation exists yet.

## What the work does

Spec 107 proposes "cleanup wave 1" from the 2026-07-02 whole-repo review (full report: `review/spec-107-approve/source-report.md` in this worktree): (R1-R2) delete ~25 confirmed-dead functions/clusters plus 3 stale `.golangci.yml` unparam carve-outs; (R3-R5) unify the triplicated agent-setup managed-block installer through one helper routed via `safeio` — fixing a real gap where `internal/setup/codex.go:68,79,96` writes with plain `os.WriteFile`/`os.OpenFile` and lacks the symlink protection claude/copilot have — plus a new codex symlink-refusal test and a `chainBeads*` fold; (R6-R7) collapse `mindspec complete`'s per-status `bd list` children fan-out to one comma-joined query via a new exported `phase.FetchChildren`, and resolve the immutable spec→epic mapping once instead of 4×; (R8-R9) restore the missing `## Bead-loop guardrails (mindspec)` section in AGENTS.md that CLAUDE.md:43 and the ms-* skills dangle-reference, and dedup the spec-init alias's byte-identical 42-line RunE copy. Explicit wave-2 deferrals include `internal/trace` (unclaimed by all OWNERSHIP.yaml manifests — deleting there would trip `adr-divergence-unowned`).

The spec has been through one grill round: ADR touchpoints were rewritten so all four impacted domains have covering Accepted ADR citations (workflow←0034/0036/0037, execution←0030/0035/0037, core←0035, context-system←0033), and an In-Scope/Out-of-Scope contradiction on `trace` was fixed.

## Files in scope (final state at c7c70000)

- `.mindspec/docs/specs/107-cleanup-deadcode-dry-wave1/spec.md` (the document under review)
- `review/spec-107-approve/source-report.md` (the repo-review report the spec transcribes — evidence base)

## Your job

Evaluate whether this SPEC is ready for `mindspec spec approve` (which creates the lifecycle epic and moves to Plan Mode). This is a document review backed by live repo verification — NOT a code review of an implementation.

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `review/spec-107-approve/<your-slot>-round-1.json` (relative to the worktree root above) with keys:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (<=200 words), `concrete_changes_required` (array, empty if APPROVE), `findings` (array). An artifact-gate finding may set `"hard_block": true`.

---

# ROUND 2 ADDENDUM (commit under review: 18d7ea3fa5b2e6234f66708b993687ac458b0b6e)

**Prior round verdict**: 3 APPROVE (R2, R4, R6), 3 REQUEST_CHANGES (R1, R3, R5), 0 REJECT.

## Round-1 concrete_changes_required (consolidated — all claimed applied in 18d7ea3fa5b2e6234f66708b993687ac458b0b6e)

1. (R1) Report citation re-pointed to `review/spec-107-approve/source-report.md`.
2. (R1) Goal attribution reworded: three §4 items + guardrails (doc-integrity, beyond report) + spec-init dedup (DRY #10 slice).
3. (R3) False ".golangci.yml invisible to divergence" claim corrected (rootOperatorDocs allowlist = only AGENTS.md safe).
4. (R3) Stale-carve-out removal DEFERRED to wave 2 (option b, trace precedent); requirements renumbered 1-9; .golangci.yml scrubbed from In Scope.
5. (R5) Children-count AC now asserts on internal/phase's listJSONFn seam via phase.SetListJSONForTest; stubChildrenByStatus re-point noted.
6. (R5) "byte-identical" strengthened to per-agent full-equality comparator + Validation Proof.

## Your job (round 2)

Diff the spec at 18d7ea3fa5b2e6234f66708b993687ac458b0b6e against your round-1 verdict. R1/R3/R5: mark each of YOUR round-1 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE in findings. R2/R4/R6: re-verify your approval holds at the new SHA (the renumbering touched Requirements/ACs). Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output to `review/spec-107-approve/<slot>-round-2.json` (same key schema).
