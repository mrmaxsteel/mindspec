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
