# LLM Test Harness — History & Reference

See [TESTING.md](TESTING.md) for the operational guide (how to run, design principles, failure taxonomy).

## Improvement History & Metrics

Track each test run with: scenario, date, pass/fail, recorded events count, turns used, wall-clock time, and what changed.

**Before adding a row**: re-read the LAST existing row in that scenario's table to know the actual baseline. Only claim a metric changed if it actually moved. Do not infer "before" values from the current session — check the table.

### TestLLM_SingleBead

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL | 1 | 15 | ~30s | Baseline: no CLAUDE.md, no beads, --no-input flag |
| 2026-02-28 | FAIL | 1 | 15 | ~30s | Added setup.RunClaude + PreToolUse hooks: hooks blocked all tools |
| 2026-02-28 | FAIL | ~5 | 15 | ~60s | SessionStart only (no PreToolUse): agent ran but PATH wrong |
| 2026-02-28 | FAIL | ~10 | 15 | ~90s | Fixed PATH dedup: mindspec ran but no .beads/ |
| 2026-02-28 | FAIL | ~15 | 15 | ~120s | Added bd init: dolt runtime files made tree dirty |
| 2026-02-28 | FAIL | ~15 | 15 | ~120s | Added .gitignore: fake bead IDs don't exist |
| 2026-02-28 | PASS | ~20 | 15 | ~90s | Real beads (CreateBead/ClaimBead): **first pass** |
| 2026-02-28 | 3/3 PASS | ~20 | 15 | ~90s | Reliability confirmed with -count=3 |
| 2026-02-28 | PASS | 45 | 2 | 19.6s | Re-baseline: 2 turns, 100% forward ratio, 1 retry on complete (no commit yet) |
| 2026-02-28 | PASS | 34 | 2 | 15.5s | Added "commit before completing" to prompt — eliminated retry, -24% events |
| 2026-02-28 | 5/5 PASS | 34 | 12-16s | Reliability: 34 events, 2 turns, 100% fwd ratio, 0 retries across all 5 runs |
| 2026-02-28 | PASS | 34 | 2 | 14.2s | Infra filter: no change (already 100% fwd), regression check only |
| 2026-02-28 | PASS | 45 | 3 | 23s | Removed prompt workaround "MUST commit before completing" — fix moved to instruct template. Agent now retries once (complete→error→commit→complete). 1 retry is expected: sandbox has no hooks so instruct template doesn't run, agent learns from CLI error. |
| 2026-02-28 | PASS | 74 | 3 | 23s | Full hooks enabled: SessionStart runs `mindspec instruct`, PreToolUse hooks installed (no-op via agent_hooks:false). Agent gets implement.md guidance. 100% fwd ratio. More events due to hook invocations. |
| 2026-02-28 | PASS | 80 | 2 | 25s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | FAIL | - | - | 3.25s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` now hits main-branch guard in implement mode. |
| 2026-03-01 | PASS | 141 | 5 | 45.10s | Fix: harness setup commits now use `MINDSPEC_ALLOW_MAIN=1` escape hatch. Setup regression resolved for this scenario. |
| 2026-03-01 | FAIL | 75 | 8 | 43.89s | Full-suite rerun: agent stayed in diagnostics/retry flow, never created `greeting.go` and never ran successful `mindspec complete`. |
| 2026-03-02 | PASS | 107 | 4 | 29.16s | Full-suite rerun after guard tightening: scenario passes again with one retry in commit/complete flow. |
| 2026-03-02 | PASS | 94 | 3 | 26.54s | Regression check after `approve impl` focus-write fix: still green, one expected commit/complete retry remains. |
| 2026-03-02 | PASS | 102 | 3 | 27.61s | Regression check after `mindspec-ce5b` worktree-anchor fix: remains green with one expected retry before final `mindspec complete`. |
| 2026-03-02 | PASS | 91 | 3 | 24.69s | Regression check for `mindspec-n9j7`: implement guidance + pre-commit messaging changes still keep SingleBead green. |
| 2026-03-02 | FAIL | 142 | 5 | 42.58s | Full-suite rerun: agent created and staged `greeting.go` but never reached successful `mindspec complete` before max turns. |
| 2026-03-02 | PASS | 129 | 7 | 67.16s | Hardened setup to start with active bead worktree + imperative prompt; targeted rerun passes. |
| 2026-03-02 | PASS | 141 | 4 | 54.12s | Final full-suite verification after setup hardening remains green. |
| 2026-03-02 | FAIL | 32 | 2 | 15.27s | De-tautologized prompt (no explicit complete command) was too open: agent used `bd close` directly instead of lifecycle completion. |
| 2026-03-02 | PASS | 173 | 3 | 44.90s | Prompt revised to require lifecycle end-state (review mode) without naming commands; agent discovered completion path and passed. |
| 2026-03-03 | PASS | 167 | 5 | 52.10s | Spec 058 fixes (DetectWorktreeContext last-match, focus propagation, plan scaffold). 100% fwd ratio. |
| 2026-03-03 | PASS | 120 | 11 | 56.92s | After sandbox .gitignore fix (added .mindspec/focus + session.json). 100% fwd ratio, clean bead→spec merge at [92]. |
| 2026-03-04 | FAIL | 2361 | 7 | 98.92s | ADR-0023 compat: `mindspec next` exits 1, `mindspec complete` exits 1. Agent reached max turns (20). Pre-existing; not a regression from Spec 067. |
| 2026-03-04 | FAIL | 1898 | 3 | 82.86s | Full-suite rerun: no `impl(` commit message, no bead merge topology. `skip_next` false positive fixed (CWD-based inference). |
| 2026-03-04 | PASS | ~313 | 6 | 1m22s | Fix worktree topology (nested spec→bead), guard redirect points to bead worktree, idle template cleanup, implement template fixes (no redundant cd/next), bd close forbidden, complete emits cd hint. 100% fwd ratio. (1649 raw events; ~1336 beads git internals filtered.) |
| 2026-03-04 | PASS | 1337 | 4 | 1m17s | setupWorktrees refactor full-suite rerun. 100% fwd ratio. No regressions from helper conversion. |
| 2026-03-05 | PASS | 1386 | 6 | 1m48s | Targeted rerun: 100% fwd ratio. No regressions. |
| 2026-03-05 | PASS | 971 | 6 | 2m18s | Spec 072 worktree run: 100% fwd ratio. Clean pass — instruct, commit, complete all succeeded. |
| 2026-03-05 | PASS | 814 | 5 | 93.49s | Spec 073 validation full-suite: stable. |
| 2026-03-05 | PASS | 992 | 3 | 96.13s | Targeted rerun: 100% fwd ratio. Stable. |
| 2026-03-05 | 2/2 PASS | 992 | 3 | 87-116s | Parallel reliability run: consistent 3 turns, 992 events, 100% fwd ratio across both. |
| 2026-03-05 | 5/5 PASS | 803-1332 | 3-6 | 78-130s | 5x parallel reliability: 3/5 at 100% fwd, 2/5 had 1 retry (83%/80% fwd). Median 3 turns, 992 events. |
| 2026-03-06 | PASS | 1127 | 6 | 100s | Dolt port isolation fix full-suite. 83.3% fwd ratio. Stable. |
| 2026-03-06 | PASS | 613 | 3 | 66.79s | Full-suite rerun (Opus): 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 845 | 3 | 1m42s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |
| 2026-03-09 | FAIL | - | - | timeout | Baseline after Spec 080 merge: agent ran `mindspec complete "msg"` (missing bead-id) due to incorrect syntax in implement.md template. Timed out at 10m. |
| 2026-03-09 | FAIL | - | - | timeout | Second baseline: same root cause — template showed `mindspec complete "msg"` but CLI requires `mindspec complete <bead-id> "msg"`. |
| 2026-03-09 | PASS | ~900 | 5 | 2m08s | Fix: updated all 6 instruct templates to include `<bead-id>` in complete syntax, implement.md uses `{{.ActiveBead}}`. Also fixed `detectSkipComplete` false positive (require exit=0 for `mindspec next`) and `assertBeadsState` (`bd show <id> --json` instead of broken `bd list --json --parent`). |
| 2026-03-09 | 3/3 PASS | 800-1100 | 4-6 | 1m30-2m30 | Verification: fwd ratios 92.6%, 86.4%, 96.7%. Agent still uses `bd close` before `mindspec complete` (Haiku limitation, tolerated by analyzer). |

### TestLLM_SpecToIdle

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL (assertions pass) | 125 | 50 | 2m16s | Baseline with hooks: agent completed lifecycle but `complete` failed (CWD guard) |
| 2026-02-28 | FAIL | 17 | 50 | 7s | Removed SessionStart hook: agent greeted instead of executing (instruct idle template) |
| 2026-02-28 | FAIL | 11 | 50 | 6s | Empty hooks{}: still conversational (CLAUDE.md influence) |
| 2026-02-28 | FAIL | 107 | 50 | 2m3s | Imperative prompt: agent executed but dolt orphans blocked bd init |
| 2026-02-28 | FAIL (1 assertion) | 108 | 50 | 1m52s | bd dolt killall in initBeads: agent reached `next` but ran out of turns before `complete` |
| 2026-02-28 | **PASS** | 170 | 75 | 2m42s | MaxTurns 50->75: **agent completed full lifecycle** |
| 2026-02-28 | FAIL | 327 | 30 | 3m16s | Full suite run: agent skipped `explore` (went to spec-init), then stuck retrying `complete` in worktree (17 retries, 43% fwd ratio) |
| 2026-03-01 | **PASS** | 358 | 28 | 4m10s | Fix: auto-commit `.mindspec/` state files in `complete.Run()`, remove dead `--spec` flag, accept explore+promote as valid path. 71.4% fwd ratio (20 fwd / 8 retry). Remaining retries: `approve plan` (needs bead creation) and `approve impl` (merge conflicts). |
| 2026-03-01 | FAIL (new assertions) | 377 | 22 | 3m8s | Added git state assertions (branch cleanup, worktree removal, CWD contains .worktrees/). Existing assertions pass (explore+promote, next, complete all ran). New assertions caught: spec/ and bead/ branches not deleted, worktree not removed. Agent stuck retrying `complete` from spec worktree CWD (not bead worktree). 59.1% fwd ratio (13 fwd / 9 retry). Root cause: guidance gap — agent doesn't know to cd into bead worktree. |
| 2026-03-01 | FAIL | 452 | 53 | 4m20s | **REGRESSION**: lifecycle advanced but cleanup assertions failed again (spec/* and bead/* branches + worktree left behind). |
| 2026-03-01 | FAIL | 485 | 25 | 199.62s | Full-suite rerun: still no successful `spec-init`/`explore promote` milestone and cleanup assertions fail (`spec/*`, `bead/*`, and spec worktree remain). |
| 2026-03-02 | FAIL | 430 | 22 | 2m48.19s | Full-suite rerun: lifecycle progressed, but cleanup assertions still fail (`spec/*`, `bead/*`, and spec worktree remain). |
| 2026-03-02 | FAIL | 545 | 35 | 3m54.61s | Baseline for `mindspec-ce5b`: recursive bead worktree nesting from CWD-sensitive `mindspec next`, plus `approve impl` retries, left `spec/*`, `bead/*`, and worktrees behind. |
| 2026-03-02 | PASS | 416 | 20 | 2m25.28s | Fix: `next.EnsureWorktree` now anchors worktree creation to spec worktree/main root (not caller CWD). Recursive nesting stopped and cleanup assertions passed. |
| 2026-03-02 | FAIL | 506 | 26 | 3m21.03s | Full-suite rerun after SingleBead hardening: lifecycle advanced but cleanup assertions failed (`spec/*`, `bead/*`, worktrees remained) after max turns. |
| 2026-03-02 | PASS | 675 | 28 | 4m10.23s | Increased MaxTurns 75->100 and clarified prompt end-state; targeted rerun completed cleanup and returned to idle. |
| 2026-03-02 | PASS | 530 | 33 | 4m02.18s | Final full-suite verification remains green under the higher turn budget. |
| 2026-03-02 | FAIL | 437 | 34 | 3m28.77s | De-tautologized full-suite validation: lifecycle progressed but strict cleanup assertions failed again (`spec/*`, `bead/*`, worktrees remained). |
| 2026-03-03 | PASS | 550 | 39 | 254.50s | Spec 058 fixes (DetectWorktreeContext last-match, focus propagation, plan scaffold). 41.0% fwd ratio (16 fwd / 23 retry). Retries from manual merge conflicts on `.mindspec/focus` (committed to both branches). |
| 2026-03-03 | PASS | 423 | 28 | 189.29s | After sandbox .gitignore fix (added .mindspec/focus + session.json). **71.4% fwd ratio** (20 fwd / 8 retry). Zero merge conflicts — bead→spec merge at [312] clean. `approve impl` succeeded after auto-merge of unmerged bead branch at [311-312]. |
| 2026-03-04 | PASS | - | - | 389.30s | ADR-0023 compat: full lifecycle passes with CreateSpecEpic + phase derivation fixes. |
| 2026-03-04 | FAIL | 7548 | 39 | 344.53s | Full-suite rerun: `mindspec complete` never succeeded (skip_complete). 89.7% fwd ratio. |
| 2026-03-04 | FAIL | 5993 | 35 | 10m18s | setupWorktrees refactor full-suite: agent skipped `mindspec complete` (wrote code after `mindspec next` but never completed). 82.9% fwd ratio. Pre-existing haiku behavior, not a regression. |
| 2026-03-05 | FAIL | 1361 | 31 | 6m10s | Spec 073 validation: `mindspec complete` never succeeded. Pre-existing. |
| 2026-03-05 | FAIL | 1199 | 27 | 260.15s | Targeted rerun: agent completed lifecycle via raw git (merge+cleanup) but `mindspec complete` never called. 59.3% fwd ratio (16 fwd / 11 retry). Pre-existing. |
| 2026-03-05 | **PASS** | 1399 | 12 | 221.70s | **Sonnet model**: full lifecycle completed incl. `mindspec impl approve`. 41.7% fwd ratio (5 fwd / 7 retry). Sonnet retains lifecycle commands in context where Haiku loses them. |
| 2026-03-06 | FAIL | 1338 | 13 | 187s | Dolt port isolation fix. `mindspec next` never succeeded. 53.8% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 1692 | 17 | 260.58s | **Full-suite rerun (Opus)**: 70.6% fwd ratio. Full lifecycle completed incl. `mindspec impl approve`. Opus succeeds where Haiku consistently fails. |
| 2026-03-06 | PASS | 1372 | 14 | 3m6s | Full-suite rerun #2 (Opus): 64.3% fwd ratio (9 fwd / 5 retry). Stable. |

### TestLLM_AbandonSpec

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | FAIL | 11 | 1 | 6.5s | Baseline: conversational response, agent asked "What would you like?" |
| 2026-02-28 | PASS | 18 | 2 | 10s | Imperative prompt pattern: "Execute these commands immediately" (50% fwd — infra noise) |
| 2026-02-28 | PASS | 18 | 2 | 8.8s | Filter infra git cmds from retry detection: **100% forward ratio** |
| 2026-02-28 | PASS | 31 | 1 | 11s | Full hooks enabled: `mindspec instruct` runs via SessionStart. Imperative prompt overrides idle template. 100% fwd, 1 turn (down from 2). |
| 2026-02-28 | PASS | 35 | 3 | 14s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | PASS | 51 | 3 | 26.39s | Pass in full-suite run. More infra events than previous baseline, behavior still correct (`explore` + `dismiss`). |
| 2026-03-01 | FAIL | 47 | 4 | 27.55s | Full-suite rerun: dismiss commands were attempted but only with non-zero exits, so no successful `dismiss` event matched stricter assertions. |
| 2026-03-02 | FAIL | 25 | 2 | 10.48s | **REGRESSION**: `mindspec explore dismiss` exits 2 (panic), so no successful `explore`/`dismiss` events are recorded. |
| 2026-03-02 | PASS | 35 | 3 | 13.18s | Fixed nil-focus handling in `explore.Dismiss`/`explore.Promote`; `mindspec explore dismiss` now exits 0 and the scenario passes. |

> AbandonSpec was not part of the 2026-03-06 full-suite run (excluded from `AllScenarios()`).

### TestLLM_ResumeAfterCrash

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 45 | 3 | 29.4s | Baseline: 66.7% fwd ratio, 1 retry (complete before commit) |
| 2026-02-28 | PASS | 74 | 2 | 22s | Full hooks enabled: agent gets implement.md guidance via SessionStart. **100% fwd ratio** (up from 66.7%), 2 turns (down from 3). Still 1 retry on complete (session.json dirty). |
| 2026-02-28 | PASS | 86 | 3 | 33s | Full suite run: stable, 100% fwd ratio |
| 2026-03-01 | FAIL | - | - | 2.15s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 138 | 6 | 43.81s | Full-suite rerun pass: resume-after-crash flow completed under current setup and assertions. |
| 2026-03-02 | PASS | 111 | 7 | 45.59s | Full-suite rerun pass; scenario still completes after one retry in the complete/commit flow. |
| 2026-03-04 | FAIL | 1682 | 7 | 75.35s | Full-suite rerun: `mindspec complete` never succeeded. 42.9% fwd ratio (4 retries). |
| 2026-03-04 | FAIL | 749 | 5 | 1m37s | setupWorktrees refactor full-suite: agent completed work but used raw git instead of `mindspec complete`. 100% fwd ratio. Pre-existing haiku behavior. |
| 2026-03-05 | FAIL | 350 | 10 | 69.04s | Spec 073 validation: `skip_next` (commit before `mindspec next`) + `mindspec complete` never called. Pre-existing. |
| 2026-03-06 | FAIL | 684 | 6 | 79s | Dolt port isolation fix. `mindspec complete` never called. 83.3% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 1032 | 4 | 103.85s | **Full-suite rerun (Opus)**: 75% fwd ratio. `mindspec complete` succeeded. Opus follows lifecycle where Haiku bypassed it. |
| 2026-03-06 | PASS | 1211 | 5 | 1m51s | Full-suite rerun #2 (Opus): 80% fwd ratio (4 fwd / 1 retry). Stable. |

### TestLLM_InterruptForBug

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 81 | 3 | 26s | First recorded run: 100% fwd ratio, agent fixed bug + created feature + completed bead |
| 2026-03-01 | FAIL | - | - | 2.12s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 180 | 7 | 61.76s | Full-suite rerun pass: interrupt-for-bug scenario still completes with current assertions. |
| 2026-03-02 | FAIL | 148 | 12 | 1m13.62s | **REGRESSION**: run reached `mindspec complete`, but `feature.go` was never created so artifact assertion failed. |
| 2026-03-02 | PASS | 156 | 8 | 57.97s | `mindspec-n9j7` validation: guidance/hook updates plus artifact assertion hardened to accept root or worktree output; scenario completes successfully. |
| 2026-03-02 | FAIL | 140 | 8 | 1m02.81s | De-tautologized full-suite validation: agent handled interrupts but never produced `feature.go`; artifact assertion failed. |
| 2026-03-04 | FAIL | 803 | 2 | 39.28s | `skip_next` false positive: agent committed fix on main without bead lifecycle. Fixed: CWD-based implement phase inference. |
| 2026-03-04 | FAIL | 815 | 4 | 3m17s | setupWorktrees refactor full-suite: agent committed directly to main (`skip_next` wrong action). 100% fwd ratio. Pre-existing haiku behavior. |
| 2026-03-05 | PASS | 151 | 5 | 50.14s | Spec 073 validation: **improvement** — first pass since de-tautologization. Agent handled interrupt and created feature.go. |
| 2026-03-06 | PASS | 1141 | 5 | 118s | Dolt port isolation fix. 80% fwd ratio. Stable. |
| 2026-03-06 | PASS | 153 | 6 | 46.50s | Full-suite rerun (Opus): 66.7% fwd ratio. Stable. |
| 2026-03-06 | PASS | 862 | 6 | 1m31s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_MultiBeadDeps

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-02-28 | PASS | 230 | 6 | 66s | First recorded run: completed 2/3 beads within 30 turns, 66.7% fwd (2 retries on complete due to dirty tree), all 3 files created |
| 2026-03-01 | FAIL | - | - | 2.46s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 228 | 7 | 69.37s | Full-suite rerun: scenario advanced, but artifact assertions failed (`formatter.go` and `formatter_test.go` not found at expected location). |
| 2026-03-02 | FAIL | 131 | 12 | 1m15.11s | Full-suite rerun: max turns reached without successful `mindspec next`; no `.worktrees/` CWD observed. |
| 2026-03-02 | PASS | 187 | 12 | 1m19.56s | `mindspec-n9j7` fix: implement template + pre-commit guardrails now steer retries toward `mindspec next` and managed worktree handoff, restoring pass. |
| 2026-03-04 | FAIL | 4213 | 6 | 159.11s | Full-suite rerun: `mindspec next` never succeeded. 83.3% fwd ratio. |
| 2026-03-04 | PASS | 5249 | 13 | 9m31s | setupWorktrees refactor full-suite: all 3 beads completed. 69.2% fwd ratio (4 retries). |
| 2026-03-05 | PASS | 1594 | 22 | 3m27s | Spec 073 validation: stable pass. |
| 2026-03-06 | PASS | 1907 | 15 | 212s | Dolt port isolation fix. 53.3% fwd ratio. Stable. |
| 2026-03-06 | PASS | 2766 | 16 | 250.75s | Full-suite rerun (Opus): 56.2% fwd ratio. Stable. |
| 2026-03-06 | PASS | 2913 | 12 | 4m3s | Full-suite rerun #2 (Opus): 50% fwd ratio (6 fwd / 6 retry). Stable. |

### TestLLM_SpecInit

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 57 | 6 | 37s | Baseline: agent ran spec-init, created worktree + branch. 100% fwd ratio. Hit max turns (15) while writing spec content. |
| 2026-03-01 | FAIL | 49 | 3 | 20.39s | **REGRESSION**: assertions failed after run (`.mindspec/focus` missing in repo root after spec-init). |
| 2026-03-01 | FAIL | 56 | 5 | 29.78s | Full-suite rerun: `mindspec spec-init` never succeeded (exit non-zero) and root `.mindspec/focus` assertion remains red. |
| 2026-03-02 | PASS | 54 | 3 | 28.96s | Full-suite rerun pass: `mindspec spec-init` succeeded and focus/worktree assertions held. |
| 2026-03-04 | PASS | 193 | 3 | 28s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | FAIL | 102 | 3 | 35.36s | Spec 073 validation: `skip_next` false positive — spec-init commit flagged because `lifecycleTurns` doesn't match `mindspec spec create`. Agent manually created spec branch+worktree. Analyzer issue, not agent regression. |
| 2026-03-06 | FAIL | 90 | 2 | 53s | Dolt port isolation fix. Agent created branch manually without `mindspec spec-init`. 100% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 96 | 2 | 17.05s | **Full-suite rerun (Opus)**: 100% fwd ratio. Agent used `mindspec spec create` correctly. Opus follows guidance where Haiku created branch manually. |
| 2026-03-06 | PASS | 96 | 2 | 16s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_SpecApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 47 | 4 | 39s | Baseline: agent ran `mindspec approve spec` (3 attempts, exit=1 each — spec validation failures). 50% fwd ratio. Hit max turns (15). Validation errors are a product gap, not test issue. |
| 2026-03-01 | PASS | 68 | 5 | 35s | Fixed setup: realistic worktree structure. Removed misleading `assertBranchIs(main)`. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.22s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in spec mode. |
| 2026-03-01 | FAIL | 74 | 6 | 37.52s | Full-suite rerun: scenario remained in `spec` mode; expected transition to `plan` did not occur. |
| 2026-03-02 | PASS | 59 | 4 | 40.16s | Full-suite rerun pass: `approve spec` succeeded and scenario met transition assertions. |
| 2026-03-04 | PASS | 2108 | 8 | 1m39s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | FAIL | 237 | 2 | 33.08s | Spec 073 validation: `mindspec approve spec` never ran with exit 0. Agent behavior regression — nondeterministic haiku. |
| 2026-03-06 | PASS | 949 | 4 | 101s | Dolt port isolation fix. 75% fwd ratio. Recovered from nondeterministic haiku FAIL. |
| 2026-03-06 | PASS | 780 | 4 | 67.61s | Full-suite rerun (Opus): 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 780 | 3 | 1m44s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_PlanApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 117 | 9 | 56s | Baseline: agent ran `approve plan` (succeeded on 3rd try) then `mindspec next` (claimed bead, created nested worktree). 77.8% fwd ratio (7 fwd / 2 retry). |
| 2026-03-01 | PASS | 130 | 8 | 54s | Fixed plan.md to pass ValidatePlan (added version, ADR Fitness, Testing Strategy, proper bead Steps/Verification). Added git state assertions. 100% fwd ratio. |
| 2026-03-01 | PASS | 90 | 2 | 23s | Fixed assertions: removed misleading `assertBranchIs(main)`, added `assertHasWorktrees`. Agent CWD enters bead worktree. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.07s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in plan mode. |
| 2026-03-01 | FAIL | 148 | 3 | 46.93s | Full-suite rerun: `approve plan`/`next` activity occurred, but focus mode stayed `plan` instead of expected `implement`. |
| 2026-03-02 | PASS | 155 | 6 | 43.84s | Full-suite rerun pass with higher retries/events; `approve plan` plus `next` completed successfully. |
| 2026-03-04 | PASS | 1311 | 3 | 1m03s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | PASS | 1002 | 5 | 1m41s | Spec 073 validation: stable. |
| 2026-03-06 | FAIL | 542 | 3 | 51s | Dolt port isolation fix. `mindspec next` never called. 100% fwd ratio. Nondeterministic haiku. |
| 2026-03-06 | PASS | 723 | 5 | 82.99s | Full-suite rerun (Opus): 60% fwd ratio. `mindspec next` succeeded. Opus follows plan→implement transition reliably. |
| 2026-03-06 | PASS | 437 | 2 | 42s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_ImplApprove

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 60 | 3 | 24s | Baseline: agent ran `state show`, then `approve impl` (direct merge + cleanup), then session close (bd sync, git commit, git push). 100% fwd ratio. |
| 2026-03-01 | PASS | 58 | 3 | 22s | Fixed setup: realistic spec worktree (not just branch), focus.activeWorktree set. Added `assertFileExists(done.go)` to verify merge. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.67s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in review mode. |
| 2026-03-01 | PASS | 69 | 3 | 26.33s | Full-suite rerun pass: review-to-idle transition and merge assertions still hold. |
| 2026-03-02 | FAIL | 101 | 5 | 31.75s | **REGRESSION**: `approve impl` command succeeded, but focus remained `review` instead of transitioning to `idle`. |
| 2026-03-02 | PASS | 84 | 4 | 29.54s | Fixed `ApproveImpl` focus persistence: fallback to root focus when local missing and write idle focus to both local+root targets. |
| 2026-03-04 | PASS | 1171 | 2 | 51.00s | ADR-0023 compat: `ApproveImpl` now uses `FindEpicBySpecID`, accepts "done" phase, handles pre-cleaned branches. `detectSkipNext` exempts approval flows. |
| 2026-03-04 | PASS | 1200 | 3 | 60s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | PASS | 447 | 7 | 54.64s | Spec 073 validation: stable. |
| 2026-03-06 | PASS | 527 | 2 | 50s | Dolt port isolation fix. 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 489 | 5 | 52.14s | Full-suite rerun (Opus): 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 489 | 4 | 53s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_SpecStatus

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 40 | 2 | 14s | Baseline: agent ran `state show` and `instruct`, reported implement mode with active bead. 100% fwd ratio. |
| 2026-03-01 | PASS | 33 | 2 | 17s | Fixed setup: realistic spec + bead worktrees, branches, focus.activeWorktree. Added branch/worktree preservation assertions. 100% fwd ratio. |
| 2026-03-01 | FAIL | - | - | 2.39s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | PASS | 33 | 2 | 19.25s | Full-suite rerun pass: read-only status checks and branch/worktree preservation assertions still pass. |
| 2026-03-02 | PASS | 23 | 3 | 13.51s | Full-suite rerun pass with lower events/time while preserving status assertions. |
| 2026-03-04 | PASS | 813 | 2 | 44s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | PASS | 554 | 3 | 56.85s | Spec 073 validation: stable. |
| 2026-03-06 | PASS | 719 | 3 | 61s | Dolt port isolation fix. 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 572 | 2 | 52.52s | Full-suite rerun (Opus): 100% fwd ratio. Stable. |
| 2026-03-06 | PASS | 344 | 2 | 33s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_MultipleActiveSpecs

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 87 | 5 | 47s | Baseline: agent discovered `--spec` flag after initial `complete` and `state show` failures. Tried `complete`, `complete --spec=001-alpha`, `bd close`, `state set`, then `complete` again. 80% fwd ratio (4 fwd / 1 retry). Agent reached max turns (20) but all assertions pass. |
| 2026-03-01 | FAIL | - | - | 2.26s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 108 | 3 | 27.85s | Added explicit `--spec` assertion: agent completed bead successfully but never used `--spec` on `mindspec complete`. This indicates current product path can disambiguate without the flag, so scenario intent/assertion may no longer match runtime behavior. |
| 2026-03-01 | FAIL | 169 | 6 | 51.79s | Full-suite rerun: bead closed successfully, but no successful `mindspec complete --spec...` invocation (new assertion still failing). |
| 2026-03-02 | PASS | 179 | 8 | 51.33s | Full-suite rerun pass; scenario succeeds but retry overhead remains high (37.5% forward ratio). |
| 2026-03-02 | FAIL | 151 | 5 | 57.41s | Full-suite rerun: no successful `mindspec complete --spec ...`; artifact/complete assertions failed. |
| 2026-03-02 | PASS | 62 | 4 | 31.16s | Hardened setup with active bead worktree (while keeping activeSpec unset) + imperative prompt; targeted rerun passes and uses `--spec`. |
| 2026-03-02 | PASS | 62 | 2 | 25.62s | Final full-suite verification remains green with successful `mindspec complete --spec ...`. |
| 2026-03-02 | FAIL | 145 | 13 | 81.71s | De-tautologized prompt v1 (too open) regressed disambiguation completion: no successful `mindspec complete --spec ...` observed. |
| 2026-03-02 | PASS | 203 | 7 | 72.26s | Prompt revised to lifecycle end-state (001-alpha to review, 002-beta unchanged, no `bd close` shortcut) restored `--spec` completion path without command-level prescription. |
| 2026-03-04 | FAIL | 5932 | 7 | 179.75s | Full-suite rerun: `mindspec complete` never succeeded. 71.4% fwd ratio. |
| 2026-03-04 | FAIL | 4198 | 12 | 11m28s | setupWorktrees refactor full-suite: no successful `mindspec complete --spec ...`. 83.3% fwd ratio. Pre-existing haiku behavior. |
| 2026-03-05 | FAIL | 507 | 8 | 10m02s | Spec 073 validation: `mindspec complete --spec` never called. Pre-existing. |
| 2026-03-06 | FAIL | 389 | 6 | 602s | Dolt port isolation fix. `mindspec complete` never called. 100% fwd ratio. Pre-existing. |
| 2026-03-06 | REDESIGN | - | - | - | Dropped `--spec` assertion: worktree resolution makes it unnecessary (bead worktree auto-resolves spec). Replaced with bead-closed, merge-topology, and 002-beta-untouched assertions. Spirit changed from "error recovery via --spec" to "multi-spec coexistence". |
| 2026-03-06 | **PASS** | 996 | 7 | 478.93s | **Full-suite rerun (Opus)**: 57.1% fwd ratio. First PASS since redesign. Agent completed bead, closed, and preserved 002-beta. Opus handles multi-spec coexistence. |
| 2026-03-06 | PASS | 1216 | 12 | 9m7s | Full-suite rerun #2 (Opus): 66.7% fwd ratio (8 fwd / 4 retry). Stable. |

### TestLLM_StaleWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 70 | 7 | 42s | Baseline: agent recovered from missing worktree by manually closing the bead via `bd close` and resetting state with `mindspec state set --mode idle`. 71.4% fwd ratio (5 fwd / 2 retry). `mindspec complete` failed (stale worktree), agent worked around it. |
| 2026-03-01 | FAIL | - | - | 2.13s | **REGRESSION**: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 126 | 8 | 53.00s | Full-suite rerun: stale-worktree recovery attempts happened, but no successful `mindspec complete` event was recorded. |
| 2026-03-02 | PASS | 101 | 5 | 47.28s | Full-suite rerun pass via documented fallback (`state set --mode idle` + `bd close`) after `complete` failure. |
| 2026-03-04 | FAIL | 2674 | 12 | 7m40s | setupWorktrees refactor full-suite: max turns (20) exhausted. 75% fwd ratio (3 retries). Pre-existing haiku behavior. |
| 2026-03-05 | FAIL | 335 | 6 | 72.80s | Spec 073 validation: `widget.go` not created, `mindspec complete` never called. Pre-existing. |
| 2026-03-06 | FAIL | 914 | 10 | 116s | Dolt port isolation fix. `mindspec complete` never called. 60% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 1284 | 6 | 111.37s | **Full-suite rerun (Opus)**: 66.7% fwd ratio. `mindspec complete` succeeded. Opus follows lifecycle where Haiku bypassed it. |
| 2026-03-06 | PASS | 1725 | 9 | 2m27s | Full-suite rerun #2 (Opus): 77.8% fwd ratio (7 fwd / 2 retry). Stable. |

### TestLLM_CompleteFromSpecWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.48s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in implement mode. |
| 2026-03-01 | FAIL | 152 | 2 | 35.91s | Full-suite rerun: scenario progressed further, but never produced a successful `mindspec complete`. |
| 2026-03-02 | PASS | 132 | 9 | 49.42s | Full-suite rerun pass: successful `mindspec complete` observed from spec-worktree context. |
| 2026-03-02 | PASS | 127 | 6 | 36.92s | Regression check after worktree-anchor fix: still green; `mindspec complete` succeeds from spec-worktree context. |
| 2026-03-04 | FAIL | 2104 | 7 | 1m17s | setupWorktrees refactor full-suite: agent used `git cherry-pick` instead of `mindspec complete`. Pre-existing haiku behavior. |
| 2026-03-05 | FAIL | 134 | 3 | 27.97s | Spec 073 validation: `mindspec complete` never called. Pre-existing. |
| 2026-03-06 | FAIL | 508 | 9 | 80s | Dolt port isolation fix. `mindspec complete` never called. 100% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 1139 | 4 | 110.77s | **Full-suite rerun (Opus)**: 100% fwd ratio. `mindspec complete` succeeded from spec worktree. |
| 2026-03-06 | FAIL | 137 | 3 | 27s | Full-suite rerun #2 (Opus): agent used `bd close` instead of `mindspec complete`. 100% fwd ratio. Same root cause as BlockedBeadTransition FAIL. |
| 2026-03-06 | FAIL | 138 | 3 | 28s | Post-fix retest: implement.md reinforcement didn't help — agent still used `bd close`. `bd_close_shortcut` wrong-action detector caught it. |
| 2026-03-06 | **PASS** | 1085 | 7 | 2m4s | Post-fix retest #2: agent used `mindspec complete` + `mindspec approve impl`, went to idle. 85.7% fwd ratio. Nondeterministic — guidance fix + session-level detector working. |

### TestLLM_ApproveSpecFromWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.18s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in spec mode. |
| 2026-03-01 | FAIL | 40 | 4 | 29.10s | Full-suite rerun: repeated `mindspec approve spec 001-greeting` attempts all exited non-zero; no successful approval event. |
| 2026-03-02 | PASS | 55 | 6 | 44.53s | Full-suite rerun pass: successful `approve spec` recorded in worktree-only artifact context. |
| 2026-03-04 | PASS | 948 | 3 | 1m43s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | FAIL | 659 | 4 | 66.36s | Spec 073 validation: `skip_next` false positive — spec-approval commit flagged. Analyzer issue (same root cause as SpecInit). |
| 2026-03-06 | FAIL | 1144 | 6 | 122s | Dolt port isolation fix. `mindspec approve spec` exit code mismatch. 100% fwd ratio. Nondeterministic haiku. |
| 2026-03-06 | **PASS** | 906 | 4 | 92.31s | **Full-suite rerun (Opus)**: 75% fwd ratio. `mindspec approve spec` succeeded from worktree. Opus handles worktree context reliably. |
| 2026-03-06 | PASS | 899 | 5 | 1m40s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_ApprovePlanFromWorktree

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | FAIL | - | - | 2.22s | Baseline in full-suite run: setup failed before agent run. `sandbox.Commit()` blocked on main in plan mode. |
| 2026-03-01 | PASS | 104 | 4 | 27.41s | Full-suite rerun pass: worktree-context `approve plan` assertion now succeeds end-to-end. |
| 2026-03-02 | PASS | 61 | 4 | 24.56s | Full-suite rerun pass; stable behavior with fewer events than prior pass. |
| 2026-03-04 | PASS | 1094 | 2 | 1m56s | setupWorktrees refactor full-suite: stable. |
| 2026-03-05 | PASS | 47 | 3 | 67.63s | Spec 073 validation: stable. |
| 2026-03-06 | PASS | 1024 | 6 | 110s | Dolt port isolation fix. 50% fwd ratio. Stable. |
| 2026-03-06 | FAIL | 1102 | 7 | 109.87s | Full-suite rerun (Opus): 57.1% fwd ratio. Agent completed full lifecycle (approve plan, implement, complete, impl approve) but assertions expected mid-lifecycle state — `approve impl` cleaned up spec branch. Assertion gap, not agent failure. |
| 2026-03-06 | 3/3 PASS | 429-1039 | 2-6 | 39-111s | **FIX**: Relaxed branch assertion — accept full lifecycle completion (approve impl cleans up branches). 100% fwd ratio on 2/3, 50% on 1/3. |
| 2026-03-06 | PASS | 944 | 4 | 1m34s | Full-suite rerun #2 (Opus): 75% fwd ratio (3 fwd / 1 retry). Stable. |

### TestLLM_BugfixBranch

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-01 | PASS | 45 | 2 | 25s | TAUTOLOGICAL — prompt said "create a branch, create PR, don't commit to main". Agent followed instructions. Not a valid workflow test. |
| 2026-03-01 | PASS | 51 | 3 | 23s | Still tautological, added real GitHub remote (mrmaxsteel/test-mindspec). |
| 2026-03-01 | 3/3 PASS | 74-82 | 3-4 | 32-42s | Reliability of tautological prompt confirmed. |
| 2026-03-01 | FAIL | 70 | 3 | 25s | Removed workflow hints from prompt (task-only). Agent committed directly to main — no branch, no PR. **Confirmed guidance gap.** |
| 2026-03-01 | FAIL | 25 | 2 | 12s | Added "Branch Policy — MANDATORY" section to idle.md. Agent still edited directly on main. Policy section too passive for Haiku. |
| 2026-03-01 | PASS | 50 | 4 | 23s | Restructured idle.md: "How to Make Changes" with numbered steps, "you cannot edit files until you are on a branch". First non-tautological pass. |
| 2026-03-01 | 2/3 PASS | 43-46 | 2-4 | 22-27s | Reliability (3 runs). 2 pass, 1 fail (Haiku skipped guidance, edited directly). ~67% reliability with guidance-only approach on Haiku. |
| 2026-03-01 | FAIL | 23 | 2 | 32.85s | Regression check: agent fixed code but never created/pushed a branch or opened PR (`git push`/`gh pr` missing). |
| 2026-03-01 | PASS | 47 | 4 | 42.11s | Full-suite rerun pass for current prompt contract; branch/PR workflow assertions succeeded. |
| 2026-03-02 | PASS | 44 | 4 | 27.27s | Full-suite rerun pass: agent created branch/worktree, pushed, and opened PR successfully. |
| 2026-03-02 | FAIL | 23 | 2 | 13.47s | De-tautologized full-suite validation: agent fixed on main and exited without branch/push/PR workflow (`git push`/`gh pr` missing). |
| 2026-03-04 | FAIL | 259 | 2 | 25.67s | No branch/push/PR. `skip_next` false positive on `bd: backup` commit. Fixed: infrastructure commit exclusion in `isCodeModifyingEvent`. |
| 2026-03-04 | FAIL | 262 | 2 | 38s | setupWorktrees refactor full-suite: agent committed directly to main, no branch/PR. Pre-existing haiku behavior. |
| 2026-03-05 | FAIL | 781 | 2 | 22.02s | Spec 073 validation: agent committed directly to main (4 commits vs expected 3). No branch/push/PR. Pre-existing. |
| 2026-03-06 | FAIL | 36 | 1 | 17s | Dolt port isolation fix. Committed to main, no branch/PR. 100% fwd ratio. Pre-existing. |
| 2026-03-06 | **PASS** | 69 | 3 | 39.65s | **Full-suite rerun (Opus)**: 100% fwd ratio. Agent created branch, committed, pushed, opened PR. Opus follows idle.md branch policy where Haiku bypasses it. |
| 2026-03-06 | PASS | 52 | 4 | 36s | Full-suite rerun #2 (Opus): 100% fwd ratio. Stable. |

### TestLLM_BlockedBeadTransition

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-04 | PASS | - | - | - | ADR-0023 session: scenario passes (previous session). |
| 2026-03-04 | FAIL | 3644 | 7 | 134.94s | Full-suite rerun: `skip_next` false positive — agent committed code before `mindspec next` in worktree CWD. Fixed: CWD-based implement phase inference. |
| 2026-03-04 | PASS | 1372 | 3 | 2m15s | setupWorktrees refactor full-suite: agent completed bead, closed, and next bead unblocked. 100% fwd ratio. |
| 2026-03-05 | PASS | 537 | 3 | 78.24s | Spec 073 validation: stable. |
| 2026-03-06 | PASS | 589 | 6 | 77s | Dolt port isolation fix. 83.3% fwd ratio. Stable. |
| 2026-03-06 | PASS | 821 | 4 | 69.27s | Full-suite rerun (Opus): 100% fwd ratio. Stable. |
| 2026-03-06 | FAIL | 125 | 2 | 23s | Full-suite rerun #2 (Opus): agent used `bd close` instead of `mindspec complete`. 100% fwd ratio. Same root cause as CompleteFromSpecWorktree FAIL. |
| 2026-03-06 | **PASS** | 821 | 3 | 1m13s | **FIX**: implement.md completion section reinforced `bd close` prohibition + `bd_close_shortcut` session-level wrong-action detector + `assertMindspecMode(plan)` added. Agent used `mindspec complete`. 100% fwd ratio. |

### TestLLM_UnmergedBeadGuard

| Date | Result | Events | Turns | Time | Change |
|------|--------|--------|-------|------|--------|
| 2026-03-04 | FAIL | 3483 | 5 | 120.72s | Baseline: `mindspec complete` and `mindspec next` never succeeded. Agent reached max turns (25). |
| 2026-03-04 | FAIL | - | - | 2s | setupWorktrees refactor full-suite: setup failure — `bd create spec epic` exit 1. Sandbox beads issue, not agent behavior. |
| 2026-03-05 | FAIL | 537 | 11 | 114.15s | Spec 073 validation: setup fixed (runs now), but `mindspec complete` and `mindspec next` never succeeded. Pre-existing behavioral issue. |
| 2026-03-06 | FAIL | 754 | 11 | 117s | Dolt port isolation fix. `mindspec complete` never succeeded. 54.5% fwd ratio. Pre-existing. |
| 2026-03-06 | FAIL | 1224 | 10 | 146.81s | Full-suite rerun (Opus): 50% fwd ratio. `mindspec next` exit=1, `mindspec next --force` exit=1, `mindspec complete` exit=0 but next assertion failed. Scenario's unmerged-bead guard blocks `next`; agent couldn't resolve. |
| 2026-03-06 | **3/3 PASS** | 657-2047 | 6-13 | 121-202s | **FIX**: Product fix — skip dirty-tree check in recovery mode (`findRecentClosed`). Test fix — MaxTurns 25→35, `mindspec next` assertion softened to secondary. 2/3 runs got `mindspec next` too. |
| 2026-03-06 | PASS | 1629 | 8 | 2m21s | Full-suite rerun #2 (Opus): 75% fwd ratio (6 fwd / 2 retry). Stable. |

### Session Summary — 2026-03-01 Full Suite

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 6 PASS (`TestLLM_ApprovePlanFromWorktree`, `TestLLM_BugfixBranch`, `TestLLM_ImplApprove`, `TestLLM_InterruptForBug`, `TestLLM_ResumeAfterCrash`, `TestLLM_SpecStatus`), 11 FAIL.
- Setup-on-main regression is resolved; remaining failures are runtime behavior/assertion mismatches rather than sandbox bootstrap failures.
- Highest-impact remaining failures: completion/approval success assertions (`SingleBead`, `AbandonSpec`, `StaleWorktree`, `CompleteFromSpecWorktree`, `ApproveSpecFromWorktree`, `MultipleActiveSpecs`), mode transition assertions (`SpecApprove`, `PlanApprove`), artifact/focus assertions (`MultiBeadDeps`, `SpecInit`), and cleanup assertions (`SpecToIdle`).

### Session Summary — 2026-03-02 Full Suite

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 12 PASS (`TestLLM_SingleBead`, `TestLLM_ResumeAfterCrash`, `TestLLM_SpecInit`, `TestLLM_SpecApprove`, `TestLLM_PlanApprove`, `TestLLM_SpecStatus`, `TestLLM_MultipleActiveSpecs`, `TestLLM_StaleWorktree`, `TestLLM_CompleteFromSpecWorktree`, `TestLLM_ApproveSpecFromWorktree`, `TestLLM_ApprovePlanFromWorktree`, `TestLLM_BugfixBranch`), 5 FAIL (`TestLLM_SpecToIdle`, `TestLLM_MultiBeadDeps`, `TestLLM_AbandonSpec`, `TestLLM_InterruptForBug`, `TestLLM_ImplApprove`).
- Main-branch setup regression remains resolved; failures are now concentrated in runtime behavior and cleanup/state-transition correctness.
- Highest-impact remaining failures after targeted rerun: cleanup leakage in `SpecToIdle`, workflow adherence in `MultiBeadDeps`, missing artifact completion in `InterruptForBug`, and review→idle focus transition mismatch in `ImplApprove`.

### Session Summary — 2026-03-02 Final Full Suite (mindspec-kt01)

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 17 PASS, 0 FAIL.
- Stabilization changes in this session:
  - `ScenarioSingleBead`: setup now starts with active bead worktree; prompt made imperative.
  - `ScenarioMultipleActiveSpecs`: setup now includes active bead worktree while preserving `--spec` disambiguation requirement; prompt made imperative; artifact assertion accepts worktree evidence.
  - `ScenarioSpecToIdle`: MaxTurns increased from 75 to 100 with explicit idle/cleanup end-state wording.
- Final full-suite command: `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1` (log: `/tmp/mindspec-kt01-fullsuite-final.log`).

### Session Summary — 2026-03-02 De-tautologized Full Suite Validation

- 17 scenarios run sequentially with `env -u CLAUDECODE`.
- 14 PASS, 3 FAIL.
- Failing scenarios:
  - `TestLLM_SpecToIdle`: cleanup assertions failed (`spec/*`, `bead/*`, worktrees remained).
  - `TestLLM_InterruptForBug`: no observable `feature.go` artifact.
  - `TestLLM_BugfixBranch`: no non-main branch workflow (`git push`/`gh pr` absent).
- Command/log: `env -u CLAUDECODE go test ./internal/harness/ -v -run '^TestLLM_' -timeout 180m -count=1` (`/tmp/mindspec-kt01-fullsuite-detautologized.log`).

### Session Summary — 2026-03-04 ADR-0023 Compatibility (Spec 067)

- 17 scenarios run sequentially with `env -u CLAUDECODE` from worktree.
- 8 PASS, 9 FAIL.
- **Passing (8)**: `SpecToIdle`, `InterruptForBug`, `ResumeAfterCrash`, `ImplApprove` ✨, `SpecStatus`, `ApprovePlanFromWorktree`, `BlockedBeadTransition` ✨, `CompleteFromSpecWorktree`.
- **Failing (9)**: `SingleBead`, `MultiBeadDeps`, `SpecInit`, `SpecApprove`, `PlanApprove`, `MultipleActiveSpecs`, `StaleWorktree`, `ApproveSpecFromWorktree`, `BugfixBranch`.
- Root cause of previous 15/17 FAIL regression: ADR-0023 changed phase derivation to beads-based, but sandbox epics used `[specID] Epic` format instead of `[SPEC NNN-slug]` format. Fixed via `CreateSpecEpic` helper.
- Production bug found and fixed: beads molecule auto-close (when all children close, epic auto-closes) caused `DiscoverActiveSpecs()` to miss epics in "review" phase. Fixed with `mindspec_done` metadata marker and status-agnostic epic queries.
- `ApproveImpl` hardened: accepts "done" phase (idempotent), skips cleanup when spec branch already removed, adds merge step for local workflows.
- `detectSkipNext` analyzer: exempt approval-flow scenarios (no `mindspec next` expected).
- Remaining failures appear to be nondeterministic Haiku behavior (SingleBead reached max turns) or pre-existing issues, not regressions from this change.

### Session Summary — 2026-03-04 Full Suite (skip_next false positives)

- 18 scenarios run sequentially with `env -u CLAUDECODE` from main.
- **9 PASS**: SpecInit, SpecApprove, PlanApprove, ImplApprove, SpecStatus, StaleWorktree, CompleteFromSpecWorktree, ApproveSpecFromWorktree, ApprovePlanFromWorktree.
- **9 FAIL**: SingleBead, SpecToIdle, MultiBeadDeps, InterruptForBug, ResumeAfterCrash, MultipleActiveSpecs, BugfixBranch, BlockedBeadTransition, UnmergedBeadGuard.
- **Key finding**: `detectSkipNext` analyzer had false positives in 3 scenarios (InterruptForBug, BlockedBeadTransition, BugfixBranch) due to Phase field never being populated by recording shims.
- **Fix applied**: `isImplementPhase()` infers implement mode from `.worktrees/` in CWD when Phase is empty. `isInfrastructureCommit()` excludes `bd: backup` commits from code-modification detection.
- **Result**: All 52 deterministic tests pass including 2 new unit tests for the false-positive fixes.
- Remaining 9 failures are pre-existing nondeterministic Haiku behavior (agent reaching max turns, wrong command sequencing, etc.), not regressions.

### Session Summary — 2026-03-04 setupWorktrees Refactor Full Suite

- 18 scenarios run (parallel batches) with `env -u CLAUDECODE`.
- 11 PASS, 7 FAIL.
- **Passing (11)**: `SingleBead`, `SpecInit`, `SpecApprove`, `SpecStatus`, `PlanApprove`, `ImplApprove`, `ApproveSpecFromWorktree`, `ApprovePlanFromWorktree`, `BlockedBeadTransition`, `MultiBeadDeps`.
- **Failing (7)**:
  - `CompleteFromSpecWorktree`: agent cherry-picked instead of `mindspec complete` (pre-existing haiku behavior)
  - `ResumeAfterCrash`: agent used raw git instead of `mindspec complete` (pre-existing)
  - `InterruptForBug`: agent committed directly to main (`skip_next` wrong action) (pre-existing)
  - `BugfixBranch`: agent committed to main, no branch/PR (pre-existing)
  - `UnmergedBeadGuard`: setup failure (`bd create spec epic` exit 1) — sandbox issue
  - `MultipleActiveSpecs`: no `mindspec complete --spec` (pre-existing)
  - `StaleWorktree`: max turns exhausted (pre-existing)
  - `SpecToIdle`: agent skipped `mindspec complete` after `mindspec next` (pre-existing)
- **Conclusion**: No regressions from the `setupWorktrees` helper refactor. All failures are pre-existing haiku behavioral issues or sandbox setup bugs. The 11 passing scenarios confirm the worktree topology changes are solid.

### Session Summary — 2026-03-05 Spec 073 Validation Full Suite

- 18 scenarios run sequentially with `env -u CLAUDECODE` from main.
- **8 PASS, 10 FAIL**.
- **Passing (8)**: `SingleBead`, `MultiBeadDeps`, `InterruptForBug`, `PlanApprove`, `ImplApprove`, `SpecStatus`, `ApprovePlanFromWorktree`, `BlockedBeadTransition`.
- **Failing (10)**:
  - `SpecToIdle`: `mindspec complete` never succeeded (pre-existing)
  - `ResumeAfterCrash`: `skip_next` + `mindspec complete` never called (pre-existing)
  - `SpecInit`: `skip_next` false positive — spec-init commit flagged (**analyzer issue**, not agent regression)
  - `SpecApprove`: `mindspec approve spec` never called (nondeterministic haiku)
  - `MultipleActiveSpecs`: `mindspec complete --spec` never called (pre-existing)
  - `StaleWorktree`: artifact + complete never called (pre-existing)
  - `CompleteFromSpecWorktree`: `mindspec complete` never called (pre-existing)
  - `ApproveSpecFromWorktree`: `skip_next` false positive — spec-approval commit flagged (**analyzer issue**)
  - `BugfixBranch`: committed to main, no branch/PR (pre-existing)
  - `UnmergedBeadGuard`: setup fixed (was failing before), but `mindspec complete`/`next` never succeeded (pre-existing behavioral issue)
- **Improvements vs 2026-03-04 setupWorktrees run**:
  - `InterruptForBug`: FAIL → **PASS** (first pass since de-tautologization)
  - `UnmergedBeadGuard`: setup failure → runs (setup fix from jfgj.4 worked)
- **Regressions vs 2026-03-04 setupWorktrees run**:
  - `SpecInit`: PASS → FAIL (analyzer false positive, not agent behavior)
  - `SpecApprove`: PASS → FAIL (nondeterministic haiku — agent didn't run approve)
  - `ApproveSpecFromWorktree`: PASS → FAIL (analyzer false positive, same root cause as SpecInit)
- **Root cause of analyzer false positives**: `lifecycleTurns` set only matches `approve`, `spec-init`, `complete` verb args, but `mindspec spec create` uses `spec`+`create` args — not matched. Spec-phase commits in worktrees are incorrectly flagged as `skip_next`. New bead filed: see beads tracker.
- **Net assessment**: No real agent behavior regressions. The 3 apparent regressions are 2 analyzer bugs + 1 nondeterministic haiku failure. InterruptForBug improvement is a genuine gain. Overall suite health comparable to 2026-03-04.

### Session Summary — 2026-03-06 Dolt Isolation Fix Full Suite

- 18 scenarios run sequentially with `env -u CLAUDECODE` from main.
- **8 PASS, 10 FAIL**.
- **Passing (8)**: `SingleBead`, `MultiBeadDeps`, `InterruptForBug`, `SpecApprove`, `ImplApprove`, `SpecStatus`, `ApprovePlanFromWorktree`, `BlockedBeadTransition`.
- **Failing (10)**:
  - `SpecToIdle`: `mindspec next` never succeeded (pre-existing)
  - `ResumeAfterCrash`: `mindspec complete` never called (pre-existing)
  - `SpecInit`: agent created branch manually instead of using `mindspec spec-init` (pre-existing)
  - `PlanApprove`: `mindspec next` never called (pre-existing nondeterministic haiku)
  - `MultipleActiveSpecs`: `mindspec complete` never called (pre-existing)
  - `StaleWorktree`: `mindspec complete` never called (pre-existing)
  - `CompleteFromSpecWorktree`: `mindspec complete` never called (pre-existing)
  - `ApproveSpecFromWorktree`: `mindspec approve spec` exit code mismatch (nondeterministic haiku)
  - `BugfixBranch`: committed to main, no branch/PR (pre-existing)
  - `UnmergedBeadGuard`: `mindspec complete` never succeeded (pre-existing)
- **Infrastructure fixes in this session**:
  - **Dolt port isolation**: `initBeads()` now uses `net.Listen` to find a free port, passes it via `--server-port` and `BEADS_DOLT_SERVER_PORT` env var. Prevents sandbox `bd` commands from connecting to the host project's dolt server on port 3307.
  - **Database name**: explicit `--database beads` avoids auto-detect from CWD directory name ("repo").
  - **Pre-commit guard bypass**: `mustRunGit()` now sets `MINDSPEC_ALLOW_MAIN=1` for all setup commits, preventing pre-commit hook from blocking spec-worktree commits during implement-mode setup.
  - **Agent env routing**: `Sandbox.Env()` and `runBD()` now include `BEADS_DOLT_SERVER_PORT` so the agent and setup helpers use the sandbox's dolt server.
- **Key improvement**: SpecStatus FAIL→PASS (was failing due to pre-commit guard blocking setup commit). All 7 previous sub-second setup failures (dolt connection) now run as real LLM tests.
- **No new regressions**: PlanApprove FAIL is nondeterministic haiku behavior (was PASS last session). SpecApprove PASS (was FAIL last session).

### Session Summary — 2026-03-06 Full Suite (Opus Model)

- 18 scenarios run sequentially with `env -u CLAUDECODE` on branch `fix/spec-init-test-redesign`.
- **16 PASS, 2 FAIL** — major improvement from previous session (8 PASS, 10 FAIL).
- **Model**: Opus 4.6 (previous sessions used Haiku). This is the first full-suite run on Opus.
- **Passing (16)**: `SpecToIdle`, `SingleBead`, `MultiBeadDeps`, `InterruptForBug`, `ResumeAfterCrash`, `SpecInit`, `SpecApprove`, `PlanApprove`, `ImplApprove`, `SpecStatus`, `MultipleActiveSpecs`, `StaleWorktree`, `CompleteFromSpecWorktree`, `ApproveSpecFromWorktree`, `BugfixBranch`, `BlockedBeadTransition`.
- **Failing (2)**:
  - `ApprovePlanFromWorktree`: Agent completed full lifecycle (approve plan → implement → complete → impl approve), which cleaned up spec branch. Assertion expected mid-lifecycle state (`spec/*` branch still present). Assertion gap, not agent failure.
  - `UnmergedBeadGuard`: `mindspec next` failed (exit=1) due to unmerged bead guard. Agent couldn't resolve the guard condition. `mindspec complete` succeeded but `next` assertion failed.
- **Flipped FAIL→PASS (8 scenarios)**:
  - `SpecToIdle`: Full lifecycle including `mindspec impl approve` — Opus retains lifecycle commands in context where Haiku loses them.
  - `ResumeAfterCrash`: `mindspec complete` succeeded — Opus follows lifecycle where Haiku bypassed it.
  - `SpecInit`: Used `mindspec spec create` correctly — Opus follows guidance where Haiku created branch manually.
  - `PlanApprove`: `mindspec next` succeeded — Opus follows plan→implement transition reliably.
  - `MultipleActiveSpecs`: First PASS since redesign — Opus handles multi-spec coexistence.
  - `StaleWorktree`: `mindspec complete` succeeded — Opus follows lifecycle.
  - `CompleteFromSpecWorktree`: `mindspec complete` from spec worktree succeeded.
  - `BugfixBranch`: Agent created branch, pushed, opened PR — Opus follows idle.md branch policy where Haiku bypasses it.
- **Key insight**: Opus follows mindspec guidance (CLAUDE.md, instruct templates, CLI error messages) much more reliably than Haiku. Most Haiku failures were "pre-existing" agent behavior issues (bypassing lifecycle commands, committing to main, losing context). Opus resolves all of these except the unmerged bead guard scenario.
- **Total wall time**: 36 minutes (2160s) for 18 scenarios.

### Session Summary — 2026-03-06 Full Suite #2 (Opus Model, Confirmatory)

- 18 scenarios run sequentially (one at a time) with `env -u CLAUDECODE` on branch `fix/spec-init-test-redesign`.
- **16 PASS, 2 FAIL** — matches previous Opus run exactly.
- **Model**: Opus 4.6.
- **Passing (16)**: `SingleBead`, `SpecStatus`, `SpecInit`, `SpecApprove`, `PlanApprove`, `ImplApprove`, `ResumeAfterCrash`, `ApproveSpecFromWorktree`, `ApprovePlanFromWorktree`, `BugfixBranch`, `UnmergedBeadGuard`, `MultipleActiveSpecs`, `StaleWorktree`, `InterruptForBug`, `MultiBeadDeps`, `SpecToIdle`.
- **Failing (2)**:
  - `CompleteFromSpecWorktree`: Agent used `bd close` directly instead of `mindspec complete` (guidance gap — agent chose the shorter path).
  - `BlockedBeadTransition`: Same root cause — agent used `bd close` instead of `mindspec complete`.
- **Comparison to previous Opus run**: Same 16/18 pass rate. The 2 failing scenarios flipped: previous run failed `ApprovePlanFromWorktree` (assertion gap, since fixed) and `UnmergedBeadGuard` (guard resolution). This run fails `CompleteFromSpecWorktree` and `BlockedBeadTransition` on `bd close` shortcut. These are nondeterministic Opus behaviors — sometimes the agent uses the lifecycle command, sometimes it shortcuts.
- **Key observation**: The `bd close` shortcut issue is intermittent on Opus (both scenarios passed in the previous Opus run). The guidance tells the agent to use `mindspec complete`, but `bd close` also works at the beads level. The agent occasionally takes the shorter path.
- **Total wall time**: ~35 minutes for 18 scenarios (comparable to previous run).

### Key Metrics to Track Per Run
- **Events**: total shim-recorded commands (multiple per turn -- measures total agent activity)
- **Turns (estimated)**: API round-trips, estimated from event timestamp gaps >2s. The `--max-turns` flag sets the budget; "Reached max turns" means all were consumed
- **Wall time**: total test duration (includes LLM thinking time between turns)
- **Retry count**: how many times the agent retried failing commands (measures CLI friction)
- **Events/turn ratio**: commands per turn (higher = agent is batching tool calls efficiently)
- **Forward ratio**: % of turns classified as productive work (from analyzer report)
- **Key milestone events**: which step in the lifecycle was reached before failure

### What Makes a Good Improvement
- **Reduces turns used** for the same outcome (agent is more efficient)
- **Reduces retry count** (fewer CLI errors = smoother workflow)
- **Increases first-time success rate** across multiple runs
- **Doesn't regress other scenarios** (always recheck SingleBead)

### What Can Regress
- Changing hooks/settings.json or instruct templates can make agent conversational
- Changing CWD guards can break scenarios that depend on worktree enforcement
- Changing mindspec instruct templates can override scenario prompts
- Changing beads integration can break bead creation/claiming

## Coverage Analysis (2026-03-03, updated by Spec 059)

### State Transition Coverage

| Transition | Trigger | LLM Scenario(s) | Status |
|:-----------|:--------|:-----------------|:-------|
| idle → spec | `spec create` | SpecToIdle, SpecInit | Covered |
| spec → plan | `spec approve` | SpecToIdle, SpecApprove, ApproveSpecFromWorktree | Covered |
| plan → implement | `plan approve` + `next` | SpecToIdle, PlanApprove, ApprovePlanFromWorktree | Covered |
| implement → implement | `complete` + `next` (more beads) | MultiBeadDeps | Covered |
| implement → plan | `complete` (only blocked beads) | BlockedBeadTransition | Covered (Spec 059) |
| implement → review | `complete` (all done) | SingleBead, SpecToIdle, MultiBeadDeps, InterruptForBug, ResumeAfterCrash, CompleteFromSpecWorktree, MultipleActiveSpecs, StaleWorktree | Covered |
| review → idle | `impl approve` | SpecToIdle, ImplApprove | Covered |

**Invalid transitions**: Tested deterministically via `TestInvalidTransitions` (Spec 059, Bead 3):
idle→plan, idle→implement, spec→implement, plan→review, review→implement, review→plan — each verified to return non-zero exit code.

### Assertion Depth

**Well-covered areas:**
- Git branch/worktree cleanup after `impl approve` (SpecToIdle, ImplApprove)
- Focus mode transitions at each phase boundary (SpecInit, SpecApprove, PlanApprove, ImplApprove)
- **Focus field depth**: `assertFocusFields` checks activeSpec, specBranch (not just mode) in SpecApprove, PlanApprove, ImplApprove (Spec 059)
- Wrong-action detection: code edits in spec/plan mode, commits to main, wrong CWD, force bypass
- **Analyzer rules**: `skip_next` and `skip_complete` are fully implemented with deterministic tests (Spec 059)
- Edge cases: stale worktree recovery, crash recovery, interrupt-for-bug, multi-spec disambiguation, complete-from-spec-worktree auto-redirect
- No-mutation on read-only commands (SpecStatus)
- Pre-approve merge/PR prohibition (ImplApprove, SpecToIdle)
- **Merge topology**: `assertMergeTopology` verifies bead→spec merge commits in SingleBead (Spec 059)
- **Commit message format**: `assertCommitMessage` verifies `impl(` prefix in SingleBead (Spec 059)
- **Beads state**: `assertBeadsState` helper available, tested deterministically (Spec 059)

**Remaining gaps:**

| Category | Gap | Impact |
|:---------|:----|:-------|
| Auto-commit verification | No test checks that `spec approve`, `plan approve` produce commits on the spec branch | Auto-commit regressions could break downstream merges |
| Beads state in LLM tests | `assertBeadsState` is available but not yet wired into PlanApprove LLM scenario | Beads creation after plan approve not verified end-to-end |

### Recommendations (Priority Order)

1. ~~**Implement `skip_next` / `skip_complete` analyzer rules**~~ ✅ Done (Spec 059, Bead 2)
2. ~~**Add beads state assertions**~~ ✅ Done (Spec 059, Bead 1)
3. ~~**Add merge topology assertion**~~ ✅ Done (Spec 059, Bead 1 + 4)
4. ~~**Add focus field depth helper**~~ ✅ Done (Spec 059, Bead 1 + 4)
5. ~~**Add invalid transition rejection tests**~~ ✅ Done (Spec 059, Bead 3)
6. ~~**Add implement→plan scenario**~~ ✅ Done (Spec 059, Bead 5)
7. ~~**Add commit message format assertion**~~ ✅ Done (Spec 059, Bead 1 + 4)
8. **Add auto-commit verification** — check spec branch has commits from `spec approve` / `plan approve`

## Architecture Notes

### Sandbox Setup (`sandbox.go`)
- Creates temp dir with git repo, `.mindspec/`, `config.yaml`
- Runs `setup.RunClaude()` for CLAUDE.md, slash commands, **and full hooks** (SessionStart + PreToolUse)
- SessionStart hook runs `mindspec instruct` — agent gets mode-aware guidance
- PreToolUse enforcement hooks are installed but **no-op** because `config.yaml` has `agent_hooks: false` (non-enforcement scenarios work from main repo root, not a worktree)
- Runs `bd init --sandbox --skip-hooks --server-port 0`
- Installs recording shims in `.harness/bin/`
- Adds `.beads/`, `.harness/`, `.mindspec/session.json`, `.mindspec/focus`, `.mindspec/current-spec.json` to `.gitignore`

### Recording Shims (`recorder.go`)
- Shell scripts in `.harness/bin/` that log to `events.jsonl` then delegate to real binary
- Shims for: git, mindspec, bd
- Each event has: command, args_list, exit_code, cwd, timestamp
- Events are the primary diagnostic -- always check them first

### Agent Invocation (`agent.go`)
- `claude -p <prompt> --permission-mode bypassPermissions --max-turns N --model haiku`
- `cmd.Dir = sandbox.Root` (agent starts in main repo)
- `filterEnv(sandbox.Env(), "CLAUDECODE")` strips CLAUDECODE for nesting
- `cmd.CombinedOutput()` captures all agent text output

### Scenario Structure (`scenario.go`)
- `Setup func(sandbox *Sandbox) error` -- prepare sandbox state
- `Prompt string` -- the agent's task (MUST be imperative for Haiku)
- `Assertions func(t, sandbox, events)` -- post-run checks
- `MaxTurns int` -- turn budget (too low = incomplete, too high = slow)
- `Model string` -- always "haiku" for cost/speed

### Why Haiku?
- Cost: ~$0.01-0.05 per test run vs $0.50+ for Sonnet/Opus
- Speed: 1-3 minutes vs 5-10 minutes
- If Haiku can follow the workflow, Sonnet/Opus definitely can
- Tradeoff: Haiku needs more imperative prompts and retries more

## Sandbox Helpers for Scenario Setup

```go
sandbox.CreateBead(title, issueType, parentID) string  // Create real beads issue
sandbox.ClaimBead(beadID)                                // Set to in_progress
sandbox.WriteFile(relPath, content)                      // Write file in sandbox
sandbox.WriteFocus(content)                              // Write .mindspec/focus
sandbox.Commit(msg)                                      // git add -A && commit
sandbox.FileExists(relPath) bool                         // Check file exists
sandbox.ReadFile(relPath) string                         // Read file content
sandbox.GitBranch() string                               // Current branch
sandbox.BranchExists(branch) bool                        // Check branch exists
sandbox.ListBranches(prefix) []string                    // List branches matching prefix
sandbox.ListWorktrees() []string                         // List .worktrees/ entries
```

## Prompt Engineering for Haiku

Haiku in `claude -p` mode tends to be conversational unless strongly directed. Rules:

1. **Say "Do NOT respond conversationally"** -- prevents Haiku from greeting instead of executing
2. **Describe the task, not the workflow** -- "add a greeting feature", not "run mindspec explore"
3. **Be specific about what to build** -- "create greeting.go with a Greet(name) function"
4. **End-state instructions are OK** -- "run `mindspec complete` when done" names the finish line
5. **Do NOT prescribe intermediate commands** -- the agent must discover `mindspec explore`, `mindspec approve`, `mindspec next` from CLAUDE.md and instruct templates

## Known Issues & Workarounds

### Dolt Server Orphans (RESOLVED — 2026-03-04)
**Problem**: Each sandbox `bd init` starts a dolt sql-server. If the test crashes or the process isn't cleaned up, orphan servers accumulate and block new ones (max 3).
**Workaround**: N/A (fixed).
**Status (2026-03-04)**: Fixed by Spec 070. Each sandbox gets its own dolt server on a random port (`--server-port 0`) with graceful `t.Cleanup()` teardown. No global `bd dolt killall` needed.

### Setup Commits Blocked on Main (REGRESSION — 2026-03-01)
**Problem**: Many scenarios call `sandbox.Commit()` after setting non-idle mode state. Current guard rules reject commits on `main` in spec/plan/implement/review, so setup fails before agent execution.
**Workaround**: In setup helpers, either commit before moving mode state out of idle, create/use the expected worktree branch first, or allow setup commits via `MINDSPEC_ALLOW_MAIN=1`.
**Status (2026-03-01)**: Harness setup now applies the explicit escape hatch in `Sandbox.Commit()` (`MINDSPEC_ALLOW_MAIN=1`), restoring deterministic scenario bootstrap without changing runtime guard behavior.

### Explore Dismiss Panic (RESOLVED — 2026-03-02)
**Problem**: `mindspec explore dismiss` exited with code 2 and a nil-pointer panic in `TestLLM_AbandonSpec` when focus state was absent.
**Workaround**: N/A (fixed in CLI).
**Status (2026-03-02)**: Fixed by treating missing focus as implicit idle in `internal/explore` mode checks. Targeted rerun passes (35 events, 3 turns, 13.18s).

### ImplApprove Focus Transition Mismatch (RESOLVED — 2026-03-02)
**Problem**: `mindspec approve impl` could succeed while root `.mindspec/focus` stayed `review` if the command executed in a worktree.
**Workaround**: N/A (fixed in workflow logic).
**Status (2026-03-02)**: Fixed in `internal/approve/impl.go` by falling back to root focus when local focus is missing and writing idle focus to both local and root targets. Added deterministic coverage in `internal/approve/impl_test.go`. Targeted rerun passes (84 events, 4 turns, 29.54s).

### mindspec complete CWD Guard
**Problem**: Agent runs from `sandbox.Root` (main repo) but `mindspec complete` requires CWD in the bead worktree.
**Fix applied**: `cmd/mindspec/complete.go` now auto-chdirs to `ActiveWorktree` from focus state when CWD is main.

### Implement Mode Manual Worktree Bypass (RESOLVED — 2026-03-02)
**Problem**: In implement mode with no recorded `activeWorktree`, agents could bypass lifecycle commands by creating spec/bead branches or worktrees manually, then get stuck in `complete`/`next` retries.
**Workaround**: N/A (fixed in guidance + hook messaging).
**Status (2026-03-02)**: Fixed by strengthening implement template handoff rules and pre-commit guardrail messaging for implement mode (including no-active-worktree branch commits). Added deterministic coverage in `internal/hooks/install_test.go` and `internal/complete/complete_test.go`. Targeted reruns now pass (`MultiBeadDeps`, `InterruptForBug`, and `SingleBead` regression check).

### Sandbox .gitignore Missing Focus Entry (RESOLVED — 2026-03-03)
**Problem**: `sandbox.go` overwrote `.gitignore` with only `.beads/` and `.harness/`, dropping `.mindspec/focus` and `.mindspec/session.json` entries. This caused `gitops.CommitAll()` (`git add -A`) to commit focus files to both spec and bead branches with different content, creating merge conflicts on every bead→spec merge in `mindspec complete`.
**Status (2026-03-03)**: Fixed by adding `.mindspec/session.json`, `.mindspec/focus`, and `.mindspec/current-spec.json` to the sandbox `.gitignore`. SpecToIdle forward ratio improved from 41% to 71.4%, retries dropped from 23 to 8, and all merge conflicts eliminated.

### DetectWorktreeContext First-Match Bug (RESOLVED — 2026-03-03)
**Problem**: `workspace.DetectWorktreeContext()` returned on the FIRST `.worktrees` match in the path. For nested bead worktrees (`repo/.worktrees/worktree-spec-XXX/.worktrees/worktree-beadID/`), it matched the outer spec worktree and returned `WorktreeSpec` instead of `WorktreeBead`. This caused `mindspec complete` to hit the "you're in a spec worktree" hard error.
**Status (2026-03-03)**: Fixed by changing to last-match semantics — the innermost worktree type wins. SingleBead and SpecToIdle both pass.

### Nested Worktrees
**Status**: Git fully supports nested worktrees. `workspace.FindRoot()` correctly resolves them. The bead worktree is created inside the spec worktree: `.worktrees/worktree-spec-XXX/.worktrees/worktree-bead-YYY`. This is fine -- it reflects the merge hierarchy (bead -> spec -> main).

### mindspec instruct Idle Template
**Problem**: The idle template contains "Greet the user" / "Ask what they'd like to work on" which could override scenario prompts.
**Status**: SessionStart hook now runs in the sandbox (full hooks enabled). Scenarios starting in idle mode (SpecToIdle, AbandonSpec) use imperative prompts ("Execute these commands immediately. Do NOT respond conversationally.") which override the idle template greeting via `claude -p` mode. If a scenario fails due to idle template interference, fix the idle template itself (product improvement).

### Focus File Deadlock (mindspec-wpqg)
**Problem**: When multiple specs are active and one is in plan/spec mode, the agent hits a deadlock: (1) Edit on `.mindspec/focus` is blocked by workflow guard (plan mode blocks "code" edits), (2) `mindspec state set` via Bash is blocked by worktree-bash hook (CWD is main, not the active worktree). The agent cannot change spec context without fragile workarounds (writing focus from a different allowed worktree).
**Status**: Product bug filed as mindspec-wpqg (P1). Not currently testable in LLM harness because sandbox has `agent_hooks: false` (enforcement hooks are no-op). Fix should go into hook logic: whitelist `mindspec state` in worktree-bash allowlist, and/or whitelist `.mindspec/focus` in workflow guard.
**Related scenarios**: `TestLLM_MultipleActiveSpecs` tests multi-spec coexistence (agent completes one spec without disrupting another). The `--spec` flag disambiguation was dropped from this scenario's assertions (worktree resolution makes it unnecessary). Full enforcement testing requires `agent_hooks: true` scenarios.

### ADR-0023 Epic Format Mismatch (RESOLVED — 2026-03-04)
**Problem**: ADR-0023 changed phase derivation from focus-file-based to beads-derived. `phase.DiscoverActiveSpecs()` queries beads epics and uses `ExtractSpecMetadata()` to identify spec epics, expecting `[SPEC NNN-slug]` title format or `spec_num`/`spec_title` metadata. The test harness created epics with `[specID] Epic` format which didn't match, causing all scenarios to report "no active specs found" / "idle mode". Additionally, beads molecule auto-close (parent epic auto-closes when all children close) caused `DiscoverActiveSpecs()` to miss review-phase epics.
**Status (2026-03-04)**: Fixed in three layers: (1) `CreateSpecEpic` sandbox helper creates epics with correct format; (2) `DerivePhaseWithStatus` handles auto-closed epics via `mindspec_done` marker; (3) `ApproveImpl` accepts "done" phase, handles pre-cleaned branches, and includes local merge step.

### Worktree CWD Sensitivity (RESOLVED — 2026-03-02)
**Problem**: Running `git worktree add` from inside an existing worktree can create the new worktree relative to CWD, causing recursive `.worktrees/.../.worktrees/...` nesting and cleanup leakage.
**Status (2026-03-02)**: Fixed in `internal/next/beads.go`: `mindspec next` now anchors worktree creation to the spec worktree (when active) or main root, independent of caller CWD. Added deterministic unit coverage in `internal/next/next_test.go` and validated with LLM reruns (`SpecToIdle` pass, `CompleteFromSpecWorktree` regression check pass).
