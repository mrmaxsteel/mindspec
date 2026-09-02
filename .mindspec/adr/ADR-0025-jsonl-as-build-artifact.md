# ADR-0025: `.beads/issues.jsonl` Is a Build Artifact, Not User Authorship

- **Date**: 2026-04-22
- **Status**: Accepted
- **Domain(s)**: workflow, execution, bootstrap
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0023](ADR-0023.md) (beads as single state authority), [ADR-0006](ADR-0006.md) (protected main + PR-based merging)

---

## Context

Per [ADR-0023](ADR-0023.md), Dolt is the single authoritative store for beads state. `.beads/issues.jsonl` is a deterministic projection: `bd export` rewrites it from the current Dolt rows, and `bd`'s pre-commit hook runs the same export before every commit. The JSONL is tracked in git so its content is durable off-machine even in projects with no configured Dolt remote.

Two properties follow from that design:

1. The JSONL is **derived** — given Dolt state at time T, `bd export` produces byte-identical output every time. A JSONL diff carries no information that Dolt did not already carry.
2. The JSONL is **co-managed by tooling** — `bd`'s auto-export (throttled to 60s) and pre-commit hook, `mindspec`'s commit helpers, and future automation all rewrite it. It is not hand-edited.

Nevertheless, mindspec's dirty-tree guard treated the JSONL like any other file. A throttled auto-export would leave the file dirty on main after an ad-hoc `bd create` from any worktree; the next `mindspec next` would abort with "workspace has uncommitted changes". Users worked around it with a stash → `mindspec next` → pop dance that had no workflow meaning and frequently failed when combined with other runtime artifact drift.

Treating the JSONL as user-authored work caused the friction. Fixing it at the beads layer (e.g. `export.auto=false`) widens the durability window — in projects with no Dolt remote, the committed JSONL is the only off-machine copy of issue state. The fix therefore belongs in mindspec: classify the JSONL as a build artifact, not user work.

## Decision

`.beads/issues.jsonl` is a **build artifact** co-managed by `bd export` and mindspec's commit points. Tooling must not treat its diff as user authorship.

Concretely:

1. **Dirty-tree guards ignore it.** `mindspec next`, `mindspec approve`, and any future workflow guard that refuses to claim work when the tree is dirty must classify dirty paths and proceed when the only dirty path is a listed artifact. User-authored dirt still blocks — the guard's purpose is to protect user code, not to enforce hygiene on derived files.
2. **Before any such guard decides, mindspec runs `bd export` from the main repo root.** This normalizes the diff against stale throttled exports so the guard's classification reflects current Dolt state, not a 60-second-old snapshot.
3. **Every executor-driven commit refreshes the JSONL from Dolt before staging.** `internal/executor/mindspec_executor.go:CommitAll` runs `bd export` against the main repo's `.beads/` before `git add -A`. The committed JSONL is therefore byte-identical to a fresh `bd export` at commit time, preserving off-machine durability even in projects with no Dolt remote.
4. **The JSONL remains visible.** `git status`, `git diff`, and `git blame` still show it. The decision is about how automated guards interpret its diffs, not about hiding the file.
5. **The artifact list is explicit and small.** Today it contains one entry: `.beads/issues.jsonl`. Future additions (e.g. `.beads/events.jsonl`) are one-liners in the classifier. The list is not a glob — only named, mindspec-recognized artifacts qualify.

### What this does not change

- [ADR-0023](ADR-0023.md) stands: Dolt is the single authority. The JSONL is a projection whose content is recoverable at any time via `bd export`.
- [ADR-0006](ADR-0006.md) stands: main is protected, merges go through PRs. Auto-handling the JSONL does not open a back door to direct main writes — the guard still refuses on user-authored dirt, and the executor still commits through worktree-scoped branches.
- `export.auto` is left at bd's default (`true`). Silencing auto-export would widen the durability window and also short-circuit `bd`'s pre-commit hook (`exportJSONLForCommit` gates on `export.auto`).

## Consequences

**Positive**

- `mindspec next` no longer aborts when an ad-hoc `bd create` has left the main worktree's JSONL dirty. The friction pattern that users worked around with stash/branch dances is gone.
- Every mindspec-driven PR-merged commit carries current beads state, making `git push` the off-machine durability guarantee even without a Dolt remote.
- The stance is cross-cutting (workflow, executor, bootstrap, doctor) and now has a single ADR number to cite rather than recurring prose.

**Negative**

- The dirty-tree guard is no longer a pure `git status --porcelain` check; it classifies paths. The classifier list is a small, mindspec-owned piece of policy that must stay in sync with what the project considers an artifact.
- A future bug that left the JSONL dirty for reasons other than auto-export (e.g. a corrupt Dolt state that `bd export` writes differently than the committed blob) would pass the guard silently. Mitigation: `mindspec doctor` surfaces beads-config drift and durability risk (Spec 082, Bead 5).

## Scope

This ADR governs how automated mindspec guards and executors treat `.beads/issues.jsonl`. It does not change bd's behavior, does not change the file's git-tracking status, and does not override user-authored edits to the file (which would remain visible in `git diff` and are not the kind of "dirt" this ADR addresses — ad-hoc user edits to the JSONL are unsupported per ADR-0023).

## Amendment (Spec 128, R5.1): clarifying "still blocks" for the worktree-creating `next` claim path

Decision item 1 above reads, unqualified: "User-authored dirt still blocks — the guard's purpose is to protect user code, not to enforce hygiene on derived files." Taken literally, that sentence also blocks dirt no claim step could ever have caused — an unrelated file the human happens to be editing in the same checkout. This amendment narrows that one sentence, for exactly one guard, without touching anything else this ADR decided.

**Scope fence (binding):** this amendment governs ONLY the dirty-tree guard on the worktree-creating `mindspec next` claim path (settled fact: this is the only kind of claim `next` performs). `mindspec spec approve`'s tree-clean check, `mindspec release`'s dirty-worktree guard, and any other present or future dirty-tree guard keep decision item 1's original, unqualified sentence — user-authored dirt still blocks them in full, with no entanglement carve-out.

**The derived enumeration (spec 128 R5.1):** the repo-relative paths a worktree-creating claim writes in the invoking checkout are, at this base, exactly (a) `.mindspec/session.json` — gitignored (`.gitignore:3`), so it can never appear as blockable dirt; (b) `.beads/issues.jsonl` — already the sole named artifact under decision item 5 above, already exempt; and (c) the new bead worktree directory, created only AFTER the guard runs, and itself gitignored (`.worktrees/`). None of the three can ever surface as tracked user dirt. The claim-entangleable user-dirt set is therefore EMPTY today.

**The narrowed rule:** for the `next` claim path only, "user-authored dirt still blocks" is scoped to dirt on a path the claim can itself write — the derived enumeration above, an empty set at this base. An unrelated dirty user file, directory, or modified tracked file in the invoking checkout no longer blocks a worktree-creating claim. The guard's protective posture is otherwise unchanged: it still never advises stash, discard, or commit of the human's work on their behalf; when it DOES block (the seam-injected case where the enumeration is non-empty), the existing failure message contract is unchanged. Because the block's removal must not also remove visibility, the claim prints a one-line informational notice on stderr naming the ignored dirt before proceeding.

**Anti-drift:** the enumeration is pinned in code as an explicit list (empty beyond the exempt artifact today), the same discipline decision item 5 already applies to the artifact list itself. A future claim step that writes a new TRACKED path outside the three above must append it to that list, which re-arms the block for that one path; a test asserts the pinned list matches the paths a real claim run actually writes, so the enumeration cannot silently go stale.
