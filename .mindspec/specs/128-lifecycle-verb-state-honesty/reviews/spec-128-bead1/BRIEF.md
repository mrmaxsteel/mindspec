<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead1`
- **Round**: 4
- **Branch**: `bead/mindspec-ta30.1`
- **Reviewed commit**: `9324d8316fea30cfda553aeac5cc3ef8bd6ec7bc`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-4.json` beside this BRIEF
(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`
§ Panel Artifact Schema). Required and optional top-level fields:

- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.
- `hard_block` (optional boolean, a top-level sibling of `verdict` — never
  a per-finding field): set `true` only for an evidence-bearing halt that
  no vote count may override.
- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
  `findings`: reviewer-authored context, read presentation-only by
  `mindspec panel tally` — not consumed by the gate decision.
<!-- /mindspec:panel-header -->

## Summary

This is a **bead implementation review** (gate: bead). Under review: commit `5101a07a` on `bead/mindspec-ta30.1` — spec 128 Bead 1: the stored-phase write lock (`internal/phase/phaselock.go` + unix/windows impls), the lifecycle backward-move comparator (`order.go`), `phase.SanctionedDerivedSync` (fenced, zero callers at this bead), all nine `mindspec_phase` writers wrapped in `WithPhaseWriteLock` (values unchanged), `repair phase --force-backward` with reason-in-output, the AC-5(vi) AST writer-inventory anti-drift test, and the AC-5(ii)/(iii)/(v)/(vii) fixtures. Review it against the spec's AC-5 legs (ii, iii, v, vi, vii) and the plan's Bead 1 section (both in `.mindspec/specs/128-lifecycle-verb-state-honesty/` on this branch); the staged impl prompt is at `reviews/prep/bead1_impl_prompt.md`.

**Implementer-flagged deviations (assess each explicitly — A/B/C/D)**:
- **A**: `WithPhaseWriteLock(root, epicID, mergeMetadata, fn)` signature — `fn` returns `(map[string]interface{}, error)` and each call site passes its OWN pre-existing MergeMetadata seam, preserving all existing fault-injection tests. This deliberately diverges from plan.md's literal shorthand sketch (`fn func(stored string) (write string, ok bool, err error)`, no epicID/mergeMetadata) which the implementer argues is unworkable (impl.go's done-write needs a map; seams would be orphaned). Judge the divergence AND whether the chosen shape keeps the AC-5(vi) test honest.
- **B**: `phase.EnsureMigrated`/`EnsureMigratedWithCache` gained a `root string` first parameter (3 production + 5 test call sites) — mechanical, to locate the lock.
- **C**: out-of-scope touches to `internal/guard/registries.go` + `internal/lint/testdata/destructive_seed_manifest.txt` — repair's new refusal tripped the repo-wide opaque-operand lint ratchet; one paired entry added mirroring the existing repairPhaseRunE rationale. Judge whether the registry entry's disclosed-residual rationale is honest.
- **D**: `TestApproveImplCallOrder`'s two AST anchors rewritten to match the new call shape (same ordering assertions claimed).

**Schema note (GH #254 countermeasure)**: the header's schema citation is DANGLING. The field list in the header is authoritative: `verdict` exactly APPROVE / REQUEST_CHANGES / REJECT (never PASS); `hard_block` top-level only. Malformed verdicts silently lose your vote.

## Files in Scope

`git show 5101a07a` in /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.1 — 25 files, +1736/−65. Key: internal/phase/{order,phaselock,phaselock_unix,phaselock_windows,sanctioned_sync}.go + tests; the nine wrap sites (internal/approve/{plan,spec,impl,adopt}.go, internal/complete/complete.go, cmd/mindspec/{release,repair}.go, internal/phase/migrate.go); cmd/mindspec/repair.go; gitignore mechanisms (gitutil.RuntimeIgnoreEntries + bootstrap.starterGitignore).

## Acceptance Criteria (verbatim from spec.md)

AC-5 legs (ii), (iii), (v), (vi), (vii) — read them in `spec.md` § Acceptance Criteria on this branch. The bead claims exactly these; (i)/(iv)/(viii) are bead 2's.

## Prior-Round Asks

Rounds 1-3: 0/6+hard_block → 3A/3RC+renewed-hard_block → **6/6 UNANIMOUS PASS at `293ef0ee`** (R6 cleared its own hard block by scratch re-derivation). Round 4 exists ONLY because GH #186 bit live during the merge terminal: `mindspec complete`'s pre-gate auto-commit landed `9324d831` — a 3-line `.beads/issues.jsonl` tracker-only delta (verified: `git diff 293ef0ee 9324d831 -- . ':!.beads'` is EMPTY) — before the doc-sync gate refused (docs are bead 10's owned scope; the retry will carry the audited `--allow-doc-skew`), and the retry stale-blocked against the tool's own commit. This is the EXACT defect this spec's bead 6 fixes; it is now live evidence in the spec's own delivery.

Round-4 duty (deliberately minimal): verify yourself that the `293ef0ee..9324d831` delta is confined to `.beads/issues.jsonl` (no source/test/doc content), confirm your round-3 verdict stands for the unchanged content, and re-verdict at `9324d831`. Nothing else changed; nothing else needs re-review.
## Lens

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Author-of-record** — does the diff deliver the plan's Bead 1 steps and ONLY them (deviations A-D each explicitly acceptable or not)? Values genuinely unchanged at all nine wrap sites? The bead-1 fence exactly zero-callers-outside-allowlist? |
| R2 | Claude | **Codebase-pin** — named files/tests exist and are green; the AC-5(vi) AST test actually red-on-revert (does unwrapping one site turn it red? does a NEW unwrapped writer somewhere else get caught?); the D anchors still pin the same ordering; the C ratchet entry matches the house pattern. |
| R3 | Claude | **Contract stability** — the WithPhaseWriteLock/SanctionedDerivedSync/IsBackwardMove public surface bead 2 will consume: is deviation A's signature the right long-term shape, is the fence tightenable to exactly-one without rework, does the fresh-read-under-lock contract close the TOCTOU the spec claims (AC-5(v))? |
| R4 | Codex | **Empirical prober** — build and run: `go build ./...`, `go test ./internal/phase/... ./cmd/mindspec/... -short` (NEVER without -short), the phaselock tests under -race; try to defeat the lock with a hand-rolled concurrent writer; verify the leftover-lockfile and timeout-refusal behaviors; check `.mindspec/phase.lock` never becomes blockable git dirt. |
| R5 | Codex | **Schema/correctness** — the comparator's total order + conservative-unknown rule vs the spec's order; the Windows LockFileEx path's API usage (retained handle, owner-only release, no FILE_SHARE_DELETE — review the syscall code closely, it cannot run on this Mac); migrate.go's absent-check-inside-critical-section claim; the force-reason metadata write. |
| R6 | Codex | **Next-bead integration** — will bead 2 (value semantics: plan-approve writes plan, next's first-claim advance, release→SanctionedDerivedSync wiring, EnsurePhaseSemanticsV2) consume this cleanly? Anything in the lock/fence/seam shapes that forces bead-2 rework? Attack the migration-contention test's realism (AC-5(vii)). |

## Reviewer conduct (binding)

- READ-ONLY on repo state: build/test freely IN THE BEAD WORKTREE, but no commits, no lifecycle verbs, no edits to tracked files; scratch under absolute /tmp paths only; leave the worktree byte-identical (git status clean check before you finish — running tests may create the .mindspec/phase.lock file transiently; ensure it is gone/ignored).
- Verify, never confirm; re-derive claims; contradiction licensed (BRIEF errors are findings).
- Write your verdict JSON to the exact path in your prompt BEFORE terminating.
