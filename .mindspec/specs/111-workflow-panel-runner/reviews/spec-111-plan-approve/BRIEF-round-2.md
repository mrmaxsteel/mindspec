# spec-111-plan-approve — Round 2 (fix re-verification, 9 reviewers, three families)

**Under review**: `.mindspec/specs/111-workflow-panel-runner/plan.md` @ **bf01a21a** (fix on round-1's 3ca2cd2c; 686 → 848 lines) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner`.
**Fix delta**: `git diff 3ca2cd2c..bf01a21a -- .mindspec/specs/111-workflow-panel-runner/plan.md`
**Round-1 tally**: 4 APPROVE (F2, O2, O3, G3) / 5 RC (F1, F3, O1, G1, G2). Consolidated: `consolidated-round-1.md` in this dir (10 items).
**Pass = >=8 APPROVE, no REJECT.** READ-ONLY rule unchanged.

## What the fix did (orchestrator-verified present at text level; judge sufficiency)

1. (G1.1+F1.1) The ALLOWED_CLI snippet no longer contains the literal merge-terminal verb string; the array is the exact four (create/verify/tally + `codex exec --sandbox read-only --skip-git-repo-check`).
2. (F1.2) Positive enumeration: a static test extracts EVERY `mindspec`-bearing construct in the file and asserts each is exactly one of the four allowlisted forms — subsuming the old blocklist greps.
3. (G2.1) `buildCommand(verb, ...args)` specified as the single command-construction chokepoint; agent prompts embed only its return value; structural test pins all construction through it.
4. (F1.3) Codex allowlist entry pinned to `--sandbox read-only`.
5. (G2.2) Input hardening: slug/spec/target/round/etc. validated against the traversal/control-byte/shell-metacharacter contract at workflow entry (mirrors 110's slug table); e2e scenario 7 proves the abort happens before any registration call.
6. (G2.3) Deterministic stdout parsing: verdict accepted only if stdout contains EXACTLY ONE JSON object matching the schema; multi-object/ambiguous → fail-closed reserialize-then-MISSING; value fidelity compared on canonical decode.
7. (G1.2) Manual e2e pins the branch build: `go build -o /tmp/ms111/mindspec` + the session launched with `PATH=/tmp/ms111:$PATH`.
8. (G1.3+F3.2) Codex PATH-shim test double (`/tmp/ms111-shim/codex`, scenario via `MS111_CODEX_SCENARIO` env) with canned payloads per failure branch; exact `/ms-panel` invocation args specified per scenario.
9. (F3.1) The `/ms-panel` handoff grep is word-boundary-anchored (`grep -Eq '/ms-panel([^-a-z]|$)'`) in both skill-copy checks.
10. (O1.1–3) Platform claims corrected against a fresh doc fetch: the docs DO document the `schema` option; the plan now states the real reason `panel verify` remains required (schema governs the agent's return value, not the on-disk verdict files), reviewer steps carry `schema` over their transcribed-verdict return, and fan-out is via the documented `pipeline(list, itemFn)`.

`mindspec validate plan` passes (WARN R=0.04 advisory, unchanged).

## Round-2 jobs

- **F1, F3, O1, G1, G2 (round-1 RC voters)**: disposition EACH of your round-1 asks ADDRESSED/PARTIAL/MISSED/NEW_ISSUE against bf01a21a. Judge sufficiency: F1 — can any indirection still slip the positive enumeration + buildCommand pin? is read-only sandbox correctly specified? F3 — is the shim mechanism a real induction path for every claimed branch; is the word-boundary grep now falsifiable? O1 — are the corrected platform claims accurate to the docs (re-check them), and is pipeline()/schema usage implementable as written? G1 — are the e2e commands now runnable end-to-end as written? G2 — do the chokepoint + hardening + parsing specs close your injection/determinism classes?
- **F2, O2, O3, G3 (round-1 approvers)**: confirm the fix delta introduces no regression in your lens (F2: DAG/edges untouched? O2: beads still ≤7 steps, provenance consistent? O3: carry-forwards intact, no new scope creep — the hardening additions stay within R2–R5's remit? G3: runner result/contract surface still stable for the loop + skills?).

Verdict → `<slot>-round-2.json` in this dir. Keys as round 1; RC voters include per-item dispositions.
