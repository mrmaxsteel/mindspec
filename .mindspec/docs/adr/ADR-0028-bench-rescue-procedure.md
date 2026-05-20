# ADR-0028: Bench Rescue Procedure via Annotated Tag

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: extraction, recovery, bench
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0027](ADR-0027-mindspec-otel-only.md)

---

## Status

Accepted. Finalized in spec 084 Bead 3 alongside the actual tag creation
(`pre-spec-084-bench-delete`, annotated, created locally in the spec
084 PR branch; push to origin happens as part of the merge step) and
the addition of `BENCH-MOVED.md` at the repo root.

## Context

Spec 084 deletes `internal/bench/` from mindspec entirely. The user's stated
intent is that bench is "destined for its own repo" but no such repo exists
yet. The deletion must therefore preserve a documented, single-command way
for someone to retrieve the bench code from history if they want to seed
the future bench repo.

A naive answer — "just look at git history" — is brittle:

1. The mindspec PR for spec 084 may be squash-merged, which collapses all
   bead commits into a single commit whose parent is the pre-spec-084 main
   tip. Squashing means **no merge commit retains the pre-delete state's
   SHA in a discoverable way**.
2. Even with a merge commit, future readers would have to traverse the
   commit graph to find the pre-delete state; the procedure should be
   `git checkout <human-readable-handle>`.

## Decision

Before any deletion commit in Bead 3, push an **annotated git tag**
`pre-spec-084-bench-delete` pointing at the parent commit of the first
deletion commit. The tag is:

- **Annotated** (not lightweight) so it carries metadata and message.
- **Pushed to origin as part of the merge step** so it survives any
  local-only state loss. The tag is created locally in the spec 084 PR
  branch; the push to `origin` happens at merge time, before the
  squash-merge lands on `main`.
- **Pinned in BENCH-MOVED.md** at the repo root, which the deletion commit
  also adds.

The BENCH-MOVED.md procedure is:

```bash
git fetch origin --tags
git checkout pre-spec-084-bench-delete -- internal/bench/
# or, to view without checking out:
git show pre-spec-084-bench-delete:internal/bench/
```

This procedure is squash-merge-resilient: the tag is independent of whether
the PR squash-merges or merge-commits, and it survives any future history
rewrites on `main` because tags are first-class refs.

## Consequences

**Positive:**
- One-command rescue, no graph traversal required.
- Survives squash-merge.
- The tag is the official entry-point referenced by ADR-0027 §4 and the
  spec 084 acceptance criteria (Test K).

**Negative:**
- One extra tag in the repo's tag list. (Tag pollution is minimal; the
  project already publishes `agentmind/v0.0.1` and other extraction tags.)
- Requires push access at Bead 3 time. (Already a precondition for any
  bead-side work in this repo's workflow.)

## Alternatives considered

- **No tag; rely on the PR's merge commit SHA in BENCH-MOVED.md**: rejected
  because squash-merge defeats this (panel R6 caught it).
- **Inline-rescue commit (extract bench as a separate "bench-pre-delete"
  branch and push it)**: rejected as heavier than a tag; the branch would
  serve no other purpose.
- **Don't bother; let users mine git history themselves**: rejected because
  the rescue procedure is the load-bearing answer to "but I needed bench."

## References

- [Spec 084-mindspec-otel-only](../specs/084-mindspec-otel-only/spec.md)
  acceptance criterion: Test K (tag-before-delete safety)
- [ADR-0027](ADR-0027-mindspec-otel-only.md) §4 — bench deletion rationale
- BENCH-MOVED.md (added by Bead 3)
