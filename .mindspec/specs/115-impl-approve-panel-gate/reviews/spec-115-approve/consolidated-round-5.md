# spec-115-approve round-5 consolidated (design-decision brief for round 6)

Round-5 spec-approval panel @ `e6dfa4cf`: **0 APPROVE / 9 REQUEST_CHANGES** — UNANIMOUS. All nine slots (F1/F2/F3 fable, O1/O2/O3 opus, G1/G2/G3 codex) independently reproduced the SAME empirical fact and reached the SAME conclusion.

## The conclusive finding (9/9, orchestrator-re-verified)
The round-4 fix re-grounded `BranchExistsE` on `git show-ref --verify refs/heads/<name>` WITHOUT `--quiet`, asserting a clean 0/1/128 (present/absent/infra) three-way. **This is empirically FALSE** on git 2.51.2 AND Apple git 2.50.1 (reproduced by 9 reviewers + the orchestrator, across loose/packed/reftable backends):

| probe | present | genuinely absent | existing + unreadable store |
|:------|:--------|:-----------------|:----------------------------|
| `show-ref --verify` (NO `--quiet`) | 0 | **128** (`fatal: not a valid ref`) | **128** |
| `show-ref --verify --quiet` | 0 | 1 | 1 |
| `rev-parse --verify --quiet` | 0 | 1 | 1 |
| `for-each-ref refs/heads/bead/` | 0 (lists) | 0 empty | 0 empty |

`show-ref` only yields exit 1 for a missing ref WITH `--quiet` (it `die()`s → 128 otherwise). So:
- **no-`--quiet`**: absent=128 collides with infra=128 → the spec's 0/1/else mapping false-refuses EVERY normal merged-and-deleted branch (128→error→refuse). Its own Falsified-if fires. My round-4 evidence table conflated the `--quiet` absent-code (1) with the no-`--quiet` infra-code (128) — a mistake.
- **`--quiet`**: absent=1 collides with infra=1 → masks an unreadable store as absent (G2's round-4 fail-open).

**CONCLUSION (proven): no single git ref-probe distinguishes "genuinely absent" from "ref-store unreadable" by exit code alone** (except `git show-ref --exists`, git ≥2.43 — see Approach A). Chasing an exit-code classifier is the unwinnable spiral the spec-114 "delete > patch" lesson warns about (rounds 2→3→4→5, a deeper edge each time).

## The load-bearing NEW evidence — the fail-open is NOT a real un-gated-merge hole
Orchestrator-reproduced (single corrupt loose ref `bead/orphan`, rest of store healthy):
- `MergeBase("main", specBranch)` at `impl.go:249` (which runs BEFORE the orphan scan) → **exit 0** here (store mostly healthy), BUT for a WHOLE-store-unreadable `refs/heads` (`chmod 000`) → **exit 128** → ApproveImpl aborts fail-closed BEFORE the scan. So whole-store-unreadable never reaches the probe.
- For the residual (single corrupt bead ref, store otherwise fine): the `--quiet` probe masks it as absent, BUT FinalizeEpic's own merge of that ref → **exit 1 (fail-closed)**, AND the corrupt orphan has **no worktree** → FinalizeEpic's worktree-listing loop never even attempts to merge it.

**So every infra sub-case that the probe could misclassify is independently fail-closed** — by `MergeBase@:249` (whole store) or by FinalizeEpic's merge failing / no-worktree (single ref). The probe's absent-vs-infra ambiguity cannot produce an un-gated merge. This is the key input to the decision.

## The design decision (round 6) — pick ONE approach
This is a genuine fork with a project-level tradeoff. The round-6 revision implements the chosen approach; the round-6 9-slot panel (3 codex) ratifies or breaks it.

- **A — `git show-ref --exists`** (git ≥2.43): a purpose-built genuine three-way (present=0 / absent=2 / error=1 — orchestrator/F3/G1 verified). CLEAN, no refutation, no fail-open. COST: introduces a hard git-version floor (currently UNDOCUMENTED in the project) — a user-facing portability requirement, needs CI enforcement + a graceful path for older git (which reintroduces the classification problem on the fallback).
- **B — `--quiet` probe + an explicit store-health precheck**: `--quiet` gives present(0)/not-present(nonzero) and NEVER false-refuses genuine absence; gate the "not-present → absent" conclusion behind a store-health signal that fails loud on an unreadable store (e.g. reuse the `MergeBase@:249` precondition, or an explicit `git show-ref --head` / `rev-parse --verify HEAD` at the scan). Version-agnostic, fail-closed at the gate. COST: a second probe = a bit more surface.
- **C — revert the probe to simple `--quiet`/bool + AUDITED REFUTATION** (the spec-114 "delete the classifier" move): use the simplest never-false-refusing existence check; document the absent-vs-infra ambiguity as an audited residual, refuted by the evidence above (MergeBase@:249 catches whole-store-unreadable pre-scan; FinalizeEpic's merge-failure / no-worktree catches the single-corrupt-ref case → no un-gated merge). Version-agnostic, minimal, no new machinery. COST: relies on the reader accepting the refutation; must survive codex trying to break it.
- **D — worktree-enumeration reframe**: detect orphans via the SAME `git worktree list` FinalizeEpic merges from, so scan and merge cannot disagree; fail closed if that one command errors. Robust by construction. COST: diverges from the shared `FindOrphanedClosedBeads` predicate that complete/next/doctor use (bigger blast radius).

**Orchestrator recommendation: a C/B synthesis** — use `--quiet` for present/absent (never false-refuses), make the scan's fail-closed posture rest on the store-health signal that already exists (`MergeBase@:249`) rather than per-ref exit-code classification, and carry the audited refutation (evidence above) for the single-corrupt-ref residual. This applies the 114 lesson (stop classifying per-ref exit codes), is version-agnostic (no floor), and closes the real risk (un-gated merge) structurally. A is the clean alternative IF a git≥2.43 floor is acceptable — that portability call is the decision.

## Disposition
A DESIGN-DECISION panel (5 slots) picks the approach; round 6 implements it; round 7 (narrow) verifies. Findings never out-voted: the 9/9 RC is fixed by re-grounding (A/B) or by the audited refutation (C) — not by shipping the broken 0/1/128 claim.
