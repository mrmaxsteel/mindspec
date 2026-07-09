# wu7t-finalize-fix — round 1 consolidated changes required

**Tally: 3 APPROVE (R1/R2/R3, all Opus) / 3 REQUEST_CHANGES (R4/R5/R6, all Sonnet) / 0 REJECT — below threshold 5/6 → FIX ROUND.**
Reviewed SHA: 6f558bd6. All asks deduped and ranked. Groups 1–4 are must-fix; Group 5 is a comment; the Deferred list goes to follow-up beads, not this round.

## Group 1 — Retry idempotency of the chore-branch flow (R4 #1 empirical, R5 #2, R1 dev-C nit) — CODE DEFECT, must fix

R4 reproduced in a bare-remote fixture: if `finalizeOrphanedSpecBranch` already pushed `chore/finalize-<specID>` once and `impl approve` is retried (e.g. a later step failed), the retry recreates the branch fresh from origin/main and the **non-force push is rejected as non-fast-forward**, hard-failing impl approve until manual intervention. Fix so a retry succeeds: the chore branch is machine-owned and regenerated from live Dolt each run — push with `--force-with-lease` (or detect the remote branch and reconcile) so a fresh-from-origin/main recreation always lands. Also: a **leftover temp finalize worktree from a crashed prior run** currently fails `WorktreeAdd` with a raw git error (R5) — remove/prune it first so retries are self-healing. And fix the factually wrong comment claiming the spec branch "also survives locally" (cleanup deletes it — R1).

## Group 2 — Operator-guidance composition in `implApproveTail` (R6 #3, R4 #2) — UX DEFECT in the target scenario, must fix

`result.Pushed` is true whenever MergeStrategy=="pr", so in the orphaned case the OLD "Branch pushed to remote. Create a PR to merge into main" block prints TOGETHER with the new NOTE — contradictory instructions about a dead branch at the exact moment clarity matters. Gate the old block on `result.FinalizeBranch == ""` so the orphan case prints ONLY the NOTE. Add a test covering the composition (orphan case → NOTE only; normal case → old message only).

## Group 3 — Error-path contract: never lose the baseline spec-branch push (R5 #1, R3 low) — must fix

Today an error inside `finalizeOrphanedSpecBranch` returns early from FinalizeEpic and SKIPS the spec-branch push, contradicting the adjacent "always pushes" comment. Reorder: `PushBranch(specBranch)` FIRST (baseline behavior, unconditional), then orphan-detect + chore-branch flow. A chore-path failure then still surfaces as an error but the baseline push already happened. This also resolves R3's low finding (NOTE lost when spec push fails after orphan push) by making the ordering coherent.

## Group 4 — Test coverage (R5 #3, R4 hygiene) — must fix

(a) Retry case: run the orphan path twice against the same fixture; second run must succeed (pins Group 1). (b) Leftover-temp-worktree case (pins the prune). (c) Fetch-failure fallback case: fetch fails → warn + baseline behavior, no orphan path. (d) Direct unit tests for `gitutil.FetchRemoteBranch` and the `workspace.FinalizeBranch`/`FinalizeWorktreeName`/`FinalizeWorktreePath` helpers, matching sibling precedent (`TestFetchRemote_RunsFetch`, `TestSpecWorktreeName`).

## Group 5 — Documentation (R1 #1 medium) — comment only

Document the squash-merge blind spot at the detection site: `IsAncestor(preFinalizeTip, origin/main)` only detects merge-commit/FF/rebase merges; a squash-merged impl PR discards the SHAs and detection misses (old behavior recurs). One comment stating the assumption; the workflow-level fix is a follow-up bead (orchestrator files it — not this round).

## Deferred to follow-up beads (do NOT do in this round)

- Squash-merge-tolerant detection (e.g. content-based JSONL comparison) — R1 medium.
- doctor/instruct surfacing of an outstanding unmerged `chore/finalize-*` branch / stale committed JSONL — R6 #5.
- Offline-fallback loudness beyond the stderr warning — R5 note (accepted fail-safe).
- `CommitCount`/`DiffStat` computed vs local (possibly stale) main in the orphan case — R4 minor.
- `WorktreeRemoveForce` lacking `RejectOptionLike` (pre-existing, untouched) — R5 note.
