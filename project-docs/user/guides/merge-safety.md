# Merge Safety, Conflict Recovery, and Adopting an Externally-Merged Spec

Spec 127 changed what `mindspec` does when a lifecycle merge would
destroy work, when it conflicts, and when a spec's content reached
`main` outside the lifecycle entirely. This guide covers the three
operator surfaces that shipped: the work-destruction preflight and its
`--allow-net-deletion` override, the `--resolve-merge` conflict
re-entry, and the `mindspec impl adopt` verb.

## The work-destruction preflight (`--allow-net-deletion`)

Every lifecycle merge — `mindspec complete`'s bead→spec merge and
`mindspec impl approve`'s bead→spec and spec→main merges — is preceded
by a work-destruction evaluation of the actual merge preview. Five
outcomes are possible:

| Outcome | Disposition |
|:--------|:------------|
| `ancestor` | Proceed as a no-op (nothing to merge — the normal post-conflict re-run case) |
| `clean` | Proceed (no destructive class was detected) |
| `superseded` | REFUSE — the branch's content already landed via another route |
| `stale-deletion` | REFUSE — the merge would delete target content the branch never authored |
| `evidence-error` | REFUSE — the evaluation could not run; absence of evidence is never treated as safety |

A refusal names the paths at risk and mutates nothing. If you have
inspected the named paths and the deletion is intended, re-run with the
audited override:

```bash
mindspec complete <bead-id> --allow-net-deletion "<reason>"
mindspec impl approve <spec-id> --allow-net-deletion "<reason>"
```

The reason is required, recorded on durable metadata, and registered in
the escape-hatch friction registry (like `--allow-doc-skew`), so
repeated use is visible to friction reporting. There is no silent form.

## Conflicted merges: preserved, never aborted (`--resolve-merge`)

When a lifecycle merge conflicts, `mindspec` now stops **in place**: the
conflicted merge is left exactly as git created it — it is never
aborted, and nothing checks out over it. The refusal names the
conflicted files and prints pinned resolution steps ending in a
re-entry invocation:

```bash
# after resolving each named file and `git add`-ing exactly those paths:
mindspec complete <bead-id> --resolve-merge
mindspec impl approve <spec-id> --resolve-merge
```

Semantics worth knowing:

- Re-entry completes the **same** preserved merge (verified against the
  merge subject and a merge-start marker ref) — it never starts a new
  one over your resolution. If the merge in progress belongs to a
  different source than the verb expects, or its ownership cannot be
  determined, the verb refuses and preserves it.
- A plain re-run **without** `--resolve-merge` also refuses over a
  resolved-and-staged index — the flag is the deliberate act of
  completing a staged resolution, so a routine retry can never silently
  commit someone's half-checked resolution.
- If the source branch advanced while the conflict sat resolved, the
  re-entry incorporates the drift — the fresh preflight is evaluated at
  the current tip, and the drift commit is included, never silently
  dropped.
- Never run `git merge --abort` on a preserved lifecycle merge as a
  routine step: your staged resolution is the thing the design is
  protecting. Re-entry (or resolution, then re-entry) is the path out.
- A refusal about "blocking paths" that does **not** mention conflicts
  is a different condition: the worktree has staged-elsewhere or
  unmerged-path state that is not a preserved lifecycle merge. Clear the
  named paths by hand; `--resolve-merge` does not apply there.

## `mindspec impl adopt` — the externally-merged spec

For the GH #218 case: a spec's implementation reached `main` via an
external route (a GitHub PR, a hand merge), the spec branch may be
deleted or stale, and `impl approve` would either fail or re-merge
stale content. Adopt is the audited, **merge-free** terminal
transition:

```bash
mindspec impl adopt <spec-id> --reason "<why the content already reached main outside the lifecycle>"
```

What it does: closes the epic, writes the finalize-export artifact, and
records an audit marker — it **never merges anything**, and its durable
post-state is identical to a normal `impl approve`'s (epic closed,
`mindspec_phase=done`, export present).

What it verifies: positive landed evidence — a corroborated remote
spec-branch ref, or the epic's surviving bead branches each confirmed
landed on `main` — before writing a VERIFIED marker.

What it refuses:

- Any invocation without `--reason` (refused before any I/O; the reason
  is the audit trail).
- A spec that is not in review state (adopt is a terminal transition,
  not a shortcut past the lifecycle gates).
- Absent, negative, or erroring landed evidence — unless you add the
  attestation escape:

```bash
mindspec impl adopt <spec-id> --reason "<why>" --attest-unverified
```

which proceeds anyway but durably records that landing was **not**
verified, including which evidence state triggered the escape. Use it
only when you have verified the landing yourself by inspection;
`bd show` on the epic shows the marker either way.

Related recovery surfaces reference this verb: a `plan approve` or
`impl approve` refusal on a missing spec branch names adopt as the
path for the externally-merged case, and the orphaned-bead hints
propose inspection (never `mindspec complete`, never a pasteable
force-delete) when merge provenance is not positively established.
