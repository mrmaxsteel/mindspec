# spec-128-plan-approve — consolidated concrete_changes_required, round 2

Tally: 2 APPROVE (R1, R4 — both zero asks) / 4 REQUEST_CHANGES (R2 ×2, R3 ×2, R5 ×3, R6 ×3). 10 asks in 5 groups. Artifact-gate screen: clear.

## G1 — Snapshot temp index must be SEEDED from the bead tip (R2 + R5 + R6, 3-slot independent convergence on a real synthetic-deletion bug) — MUST FIX
As drafted, the temp index starts EMPTY; `add -A` with the named-artifact exclusion makes the tracked `.beads/issues.jsonl` ABSENT from T, and `commit-tree T -p <tip>` materializes a commit that DELETES the artifact from the bead branch (silently merged whenever artifactDirt was empty and no sync commit follows — R6 reproduced the synthetic deletion in review). Fix: `GIT_INDEX_FILE=<tmp> git -C wtPath read-tree <tip>` BEFORE `add -A -- . ':!<named-artifact>'` then `write-tree` — exclusion then means unchanged-at-tip (blob/mode retained), never absent. Update bead 6 step 1, the OQ-1 protocol paragraph, fixture (iii); add a red-on-revert leg asserting the user commit's diff vs tip NEVER touches the named artifact (only the separate artifact-sync commit updates it).

## G2 — Post-materialization real-index transition (R2 + R5 convergent) — MUST FIX
After the fast-forward update-ref of the checked-out bead branch, the REAL index is stale: newly-tracked-via-T files appear as phantom staged deletions; exec.Status surfaces (residual-dirt WARN, complete.go:928-933) report spurious paths; fixture (iv)'s interval-edit assertion is polluted. Fix bead 6 step 4: index-only refresh (`git reset --quiet --mixed` in wtPath; worktree untouched), with ordering, failure/retry handling, and expected-old-tip protection on update-ref specified (not just commit-tree+update-ref as the whole op). Note CommitPaths is already safe (pathspec-limited, gitops.go:1699). Extend bead 6 red-on-revert: (a) excluded artifact unchanged in T and user commit, changed only in sync commit; (b) unchanged-after-snapshot worktree is CLEAN after materialization; (c) interval edit remains ordinary unstaged dirt, never an index/HEAD split. Update the ADR-0041 amendment draft to describe the corrected protocol.

## G3 (R3 ×2) — Fence satisfiability + chunk file — MUST FIX (mechanical)
1. Bead-1's sanctioned-entry fence as written ("exactly ONE production caller") is unsatisfiable at bead 1's own gate (release isn't wired until bead 2). Restate: bead 1's enumeration test asserts NO production caller outside allowlist {cmd/mindspec/release.go} (zero passes); bead 2 tightens to exactly-one — or move the fence test to bead 2 and say so.
2. work_chunks id 2: add internal/approve/impl.go (bead 2 step 7 edits its :277 invocation site).

## G4 (R6) — Legacy-v2 story must cover READ paths — MUST FIX
As planned, `state show`/instruct tails can expose the old stored=implement/all-open mismatch indefinitely until a mutating verb happens to run. Name the invocation seam for read/derivation entry points and add an upgrade-before-first-claim state-show fixture — or define an equally honest NON-MUTATING compatibility treatment that removes the obsolete warning safely (a read path must not take the write lock casually; the treatment must be explicit either way).

## G5 (R6) — Windows stale-lock takeover must be ownership/liveness-aware and race-safe — MUST FIX
Age-only deletion can remove a still-live holder, and two simultaneous stale contenders can unlink each other's fresh locks. Design the takeover with holder identity + liveness checks and a race-safe replace step; add fixtures: live holder older than the bound (NOT removed), concurrent stale takeovers (exactly one winner), plus the planned crashed-holder and stale-file tests.
