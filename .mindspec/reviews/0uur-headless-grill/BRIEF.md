# 0uur-headless-grill — Round 1 Review Panel

**Worktree**: /Users/Max/replit/mindspec/.claude/worktrees/agent-a0f12e237206a4a4c (branch checked out; run `go test` here)
**Repo (for diffs)**: /Users/Max/replit/mindspec
**Branch**: fix/0uur-headless-grill-guard (base d1b4bebf)
**Commit under review**: 24af2fdbe3af4a070bf12c7b59bde69647674e36 — fix(skills): headless guard for the ms-spec-grill auto-chain — defer with audit marker instead of stalling (mindspec-0uur)
**Panel type**: ad-hoc standalone-bug panel (bead mindspec-0uur, P2, autopilot-blocking) — bead-review mix: 3 Opus + 3 Sonnet.

## What the work does

Spec 105's `ms-spec-create` step 5 auto-invokes the interactive `ms-spec-grill` skill (cardinal rule: one question at a time, refuse to advance until answered). In headless runs (orchestrator, ms-spec-autopilot, LLM-test harness) no human can answer, so the agent stalls (observed live: TestLLM_SpecToIdle stalled at turn ~4; the harness got a prompt-clause workaround in PR #155 but the product path was unguarded — blocking all unattended loop work, WS-C prerequisite).

The fix (skill-text + embed plumbing, no behavioral Go changes): (1) `plugins/mindspec/skills/ms-spec-grill/SKILL.md` gains a `## Headless guard — check before asking anything` section before the Cardinal rule: in a headless/non-interactive session, append `- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval.` to the spec's Open Questions and return immediately. (2) The `ms-spec-create` canonical content in `internal/setup/claude.go` `lifecycleSkillFiles()` gains a matching **Headless guard** clause on step 5. (3) Mirrors kept byte-identical: `.claude/skills/` AND `.agents/skills/` (the latter not named in the bead but kept in lockstep per repo convention — deviation B). (4) Refresh mechanism: pre-guard bodies captured as NEW historical snapshots `internal/setup/historical_skills/{ms-spec-create,ms-spec-grill}.pre0uur.md` (spec-105 pattern) so existing installs auto-refresh on next `mindspec setup`; no existing snapshot modified (HC-6: user-modified installs left alone). (5) Two new tests pin the refresh: `TestInstallSkills_RefreshesPreHeadlessGuardSnapshot` / `...GrillSnapshot`.

## Files in scope (9 files, +256/-0)

plugins/mindspec/skills/{ms-spec-create? (no — create lives in claude.go), ms-spec-grill}/SKILL.md · .claude/skills/{ms-spec-create,ms-spec-grill}/SKILL.md · .agents/skills/{ms-spec-create,ms-spec-grill}/SKILL.md · internal/setup/claude.go · internal/setup/historical_skills/{ms-spec-create,ms-spec-grill}.pre0uur.md · internal/setup/skills_test.go — verify the exact set with `git diff --stat d1b4bebf...fix/0uur-headless-grill-guard`.

## Fix-author deviations (assess explicitly)

A. Snapshot tag `.pre0uur.md` (bead-id) rather than `.preNNN.md` (spec number) — bead isn't spec-tied; claims consistency with the "segment before first dot" convention in `previouslyShippedSkills()`.
B. `.agents/skills/` mirrors updated though the bead named only `.claude/skills/` — repo convention keeps them in lockstep (commits 0024150f, aa98e1b3); same install path.

## Your job

Evaluate cold (round 1). Scrutinize:
1. **Guard decidability**: can an agent reading the skill text actually DECIDE "headless" reliably? Is the wording operational (what signals does it name?) or vague enough to over-trigger (suppressing the grill in interactive sessions — destroying spec 105's value) or under-trigger (still stalling autopilot)?
2. **Marker auditability**: is the deferral marker durable and findable (Open Questions section semantics; will `/ms-spec-approve` or the spec template surface it)? Could a spec reach approval with the grill silently skipped and nobody noticing?
3. **Snapshot byte-exactness**: are the .pre0uur.md snapshots byte-identical to the pre-edit canonical bodies (compare against `git show d1b4bebf:...` for the grill SKILL.md and the pre-edit `lifecycleSkillFiles` string for ms-spec-create)? A near-miss snapshot silently breaks the auto-refresh.
4. **HC-6 respected**: user-modified installs must NOT be overwritten; only exact historical matches refresh. Do the tests pin this?
5. **Sync**: plugin / .claude / .agents copies byte-identical post-change? Does anything else embed or assert this skill text (grep bench/grill/, internal/harness, TestSkillInventory)?

Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/.mindspec/reviews/0uur-headless-grill/<your-slot>-round-1.json` with keys: `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
