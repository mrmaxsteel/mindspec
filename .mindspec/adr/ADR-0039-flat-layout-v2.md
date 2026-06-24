# ADR-0039: Flat `.mindspec/` Layout v2 — Per-Artifact Three-Tier Resolver, Permanent Multi-Prefix Matchers, and Co-Located Reviews

- **Date**: 2026-06-24
- **Status**: Proposed
- **Domain(s)**: core, workflow, execution, context-system
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0022](ADR-0022.md) (Worktree-Aware Spec Resolution — this ADR EXTENDS its worktree → canonical → legacy resolution order into a per-artifact, three-tier flat → canonical → legacy resolver; it is an extension, not a supersession), [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (Panel Gate as Enforced Contract — AMENDED here for the reviews LOCATION only), [ADR-0018](ADR-0018.md) (Lean Bootstrap — the glossary/policies drops this layout completes are ADR-0018-consistent), [ADR-0023](ADR-0023.md) (Beads as Single State Authority — the flatten is forward-only; the cross-layout merge guard hard-fails the regression direction)

---

## Context

Through spec 105 the lifecycle artifacts lived under a NESTED
`.mindspec/docs/{specs,adr,domains,core}` tree, with user/dogfood
documentation interleaved under `.mindspec/docs/{user,installation,research}`
and panel-review artifacts in a repo-root `review/` tree. The nesting bought
nothing: the only consumer of the `docs/` segment was string-matching code, and
the interleaved dogfood docs and homeless repo-root reviews created recurring
friction (adwu — homeless reviews; the dogfood docs polluting the lifecycle
ownership scans).

Spec 106 FLATTENS the tree: lifecycle artifacts move up one level to live
DIRECTLY under `.mindspec/` (keeping the names `adr` and `core`), the dogfood
docs are evicted to a TOP-LEVEL `project-docs/` tree (explicitly NOT a root
`docs/`, which would alias the legacy read tier), and panel reviews are
co-located under the spec they review. The move is a one-way, irreversible
filesystem cut executed by a deterministic, crash-resumable, history-preserving
mover (Reqs 4/5/11).

This ADR FREEZES the layout decisions the framework and its skills now build on,
so a future editor does not silently re-introduce the nesting, break the
backward-compatible read path, or weaken the permanent git-ref matcher posture.

## Decision

### 1. The flat `.mindspec/{specs,adr,domains,core}` + top-level `project-docs/` layout

Lifecycle artifacts live at `.mindspec/specs/`, `.mindspec/adr/`,
`.mindspec/domains/`, `.mindspec/core/`, and `.mindspec/context-map.md`. There
is NO intermediate `.mindspec/docs/` directory in the shipped (flat) shape.
User/dogfood documentation lives at a TOP-LEVEL `project-docs/`
(`user/`, `installation/`, `research/`) — never under `.mindspec/`, and never at
a root `docs/` (which is the legacy read tier and would alias it). Greenfield
`mindspec init`/bootstrap is **born flat**: it writes `.mindspec/{specs,domains}`
directly and `DetectLayout` classifies the result `flat`.

### 2. The per-artifact three-tier first-exists-wins resolver + whole-tree `DetectLayout`

Every artifact accessor (`SpecDir`, `ADRDir`, `DomainDir`, `CoreDir`,
`ContextMapPath`, `RecordingDir`, and the `SpecsDir`/`DomainsDir` enumeration
roots) resolves with **flat → canonical → legacy** precedence, first-exists-wins:

1. **flat** — `.mindspec/{specs,adr,domains,core}` / `.mindspec/context-map.md`.
2. **canonical** — `.mindspec/docs/{specs,adr,domains,core}` /
   `.mindspec/docs/context-map.md`.
3. **legacy** — root `docs/{specs,adr,domains,core}`.

With NO flat tree present the resolver returns the canonical/legacy path
BYTE-FOR-BYTE as before spec 106 — the refactor is behavior-preserving for
pre-flatten checkouts. A whole-tree `DetectLayout(root)` probe classifies the
tree `{flat | canonical | legacy | greenfield | mixed}`; `mixed` (a flat
lifecycle tree coexisting with ANY canonical/legacy tree) is a HARD ERROR, with
the sole exception of a recorded in-progress `.mindspec/migrations/<run-id>/`
recovery. The write-default keys off this whole-tree classification, never a
per-id probe.

### 3. PERMANENT multi-prefix git-ref matcher posture

The git-DIFF-STRING gate matchers (doc-sync `isDocFile`/`isSourceFile`, the
spec-artifact literals, the cmd-docs accept-set, the domain enumerators, the
ownership pair `LoadOwnership`/`LoadOwnershipAtRef` + `domainManifestRelPaths`,
and `isProcessArtifact`) recognize ALL THREE prefixes (flat + canonical +
legacy) **PERMANENTLY**. This multi-prefix posture is DECOUPLED from the
filesystem read-tier deprecation lifecycle: even after the canonical/legacy
READ tiers are someday removed, historical refs, old branches, and external
forks keep emitting the canonical/legacy diff-path strings forever, so the
matchers must keep classifying them. The matcher posture is a permanent
compatibility surface, not a transitional shim.

### 4. Reviews co-location (ADR-0037 amendment)

Panel-review artifacts (`panel.json`, BRIEF, verdicts, consolidated lists) are
co-located under the spec they review at
`.mindspec/specs/<id>/reviews/<panel-slug>/` — a sibling of `recording/` —
instead of a repo-root `review/` tree. The `mindspec complete` panel gate is
LAYOUT-AWARE: on a canonical/legacy (pre-move) tree it scans BOTH the repo-root
`review/<slug>/panel.json` AND the co-located `<spec-dir>/reviews/<slug>/panel.json`
(the transition union); on a flat (post-move) tree it scans the co-located
reviews ONLY and ignores the repo-root `review/`. This AMENDS ADR-0037's
registration LOCATION only — the registration, round derivation, N−1 threshold,
staleness, dirty-tree, and fail-open/closed semantics are all untouched.

### 5. The flatten is forward-only and directionally guarded (ADR-0023)

The move is irreversible (the lifecycle cannot rewind). A DIRECTIONAL
cross-layout merge guard at the real local executor merge seams HARD-FAILS the
REGRESSION direction — a canonical/legacy-layout source merging onto a flat
target (which would resurrect pre-flatten `.mindspec/docs/...` paths) — with a
"rebase onto post-flatten main" recovery line, while explicitly ALLOWING the
MIGRATION direction (a flat source onto a canonical/legacy target) so the
flatten itself can land. The regression block is exempt inside a recorded
in-progress migration run-state.

## Consequences

- **Positive**: a shorter, friction-free tree; homeless repo-root reviews (adwu)
  resolved by co-location; dogfood docs no longer pollute the lifecycle
  ownership scans; born-flat greenfield projects never carry the vestigial
  `docs/` nesting; pre-flatten checkouts still resolve byte-for-byte.
- **Negative / cost**: every artifact accessor gained a tier and every gate
  matcher gained two prefixes (a permanent surface), and the harness/testdata
  fixtures + path-bearing skills had to be re-pointed at the flat shape. The
  one-way move requires the directional merge guard and the crash-resumable
  mover to land safely.
- **Compatibility**: NO existing Accepted ADR is violated or superseded. The
  flatten EXTENDS ADR-0022's resolution order (an additional first-precedence
  tier); ADR-0022 needs no `Superseded-by`, and when it is later flipped to
  Accepted it and this ADR are consistent. The reviews relocation is an
  ADR-0037 AMENDMENT (location only), not a supersede.

## Alternatives Considered

### 1. Keep the nested `.mindspec/docs/` tree

Rejected: the `docs/` segment bought nothing — no consumer needed it — while
the interleaved dogfood docs and homeless repo-root reviews were recurring
friction. The nesting was vestigial.

### 2. A single whole-tree layout switch instead of per-artifact resolvers

Rejected: a per-id or whole-tree-only switch cannot express a partially-migrated
tree and would force an all-or-nothing cut. The per-artifact first-exists-wins
resolver preserves byte-identical reads on pre-flatten trees and localizes the
flat preference to each accessor.

### 3. Deprecate the canonical/legacy git-ref matchers on the same timeline as the read tiers

Rejected: historical refs, old branches, and external forks emit the
canonical/legacy diff-path strings FOREVER, independent of when the filesystem
read tiers are removed. Coupling the matcher lifecycle to the read-tier
lifecycle would silently un-classify those paths and trip gates on old refs. The
multi-prefix matcher posture is therefore PERMANENT and decoupled.

### 4. Evict dogfood docs to a root `docs/` directory

Rejected: a root `docs/` is the LEGACY read tier and would alias it, re-creating
the exact ambiguity the flatten removes. Dogfood docs go to a top-level
`project-docs/` that no read tier claims.
