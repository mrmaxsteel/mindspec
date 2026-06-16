# ADR-0032: Semantic ADR Coverage Gates with Override and Supersede Flags

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: validation, adr, lifecycle, workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0030](ADR-0030-executor-boundary.md) (executor-boundary; F1 uses `Executor.ChangedFiles`/`MergeBase`), [ADR-0031](ADR-0031-doc-sync-gate.md) (doc-sync gate; F1 follows the same enforcement+override pattern)

---

## Status

Finalized in spec 087 Bead 4 alongside the semantic-gate
implementation. Plan-time gates land in Bead 1 (`checkADRCoverage` +
`walkSupersededChain` + `IsDomainCovered`); per-bead divergence check
lands in Bead 2 (`internal/validate/divergence.go::ValidateDivergence`
+ filled `CheckADRDivergence` body); `--override-adr` /
`--supersede-adr` CLI flags + `adr.CreateWithID` + audit metadata land
in Bead 3.

## Context

Today `internal/validate/plan.go::checkADRCitations` (~line 366) verifies
each cited ADR exists and is `Accepted`, but does NOT check whether the
ADR's `Domains` field is relevant to the spec's impacted-domains. A spec
can cite any set of ADRs and pass plan validation. Per-bead, the
`CheckADRDivergence` stub added by spec 086 returns an empty `Result` —
no actual gating happens at `complete` or `approve impl` time.

F1 of the converged transformation plan promotes both checks to errors:
plan approval fails on irrelevant or missing coverage; bead complete
fails when the diff touches a domain whose ADRs weren't cited. Override
flags `--override-adr` and `--supersede-adr` provide explicit,
audit-trailed escape hatches so cross-domain refactors and ADR evolution
aren't blocked.

## Decision

Four sub-decisions:

1. **Domain identifier is the `OWNERSHIP.yaml` directory name.** All
   three artifacts (spec.md `## Impacted Domains`, `OWNERSHIP.yaml`
   location, ADR `Domains` field) MUST use the same short-tag identifier
   set (e.g., `core`, `execution`). Comparison is case-folded,
   trim-whitespace, exact set intersection. No aliases or hierarchy in
   v1. Rejected alternatives: path-like identifiers (ambiguous —
   `internal/foo` vs `foo`); free-form tags (impossible to validate
   mechanically).

2. **Plan-time gate: cite-relevant + coverage-complete.** Extends
   `checkADRCitations` to intersect `ADR.Domains` with the spec's
   impacted-domains — empty intersection is an error. A new
   `checkADRCoverage` ensures every impacted domain has at least one
   cited Accepted ADR whose `Domains` contains it. Rejected: cite-
   relevant only (allows uncovered domains to slip through); a separate
   `mindspec adr verify` step (defers the check past plan approval where
   it belongs). *(Amended — the "at least one cited Accepted ADR"
   predicate is now tri-state; see the Amendment section below.)*

3. **Bead-time gate: divergence check via `Executor.ChangedFiles` +
   `attributeDomain`.** `internal/validate/divergence.go::ValidateDivergence`
   computes the diff range, maps paths to domains via the F2
   `OWNERSHIP.yaml` machinery, and errors when a touched domain isn't in
   the plan's cited ADR coverage. The `internal/validate/adr_divergence.go`
   stub from spec 086 calls into this. `approve impl` runs the same
   check as a backstop with broader scope (main → spec branch).

4. **Override flags with split audit trail.** `--override-adr "<reason>"`
   records `mindspec_adr_override_*` keys in bead metadata (one-shot
   pass-through, reason required). `--supersede-adr ADR-NNNN` is a
   richer form: it creates a new ADR with `Status: Proposed` and
   `Domains` seeded from the violated domain, AND records
   `mindspec_adr_supersede_*` metadata, AND bypasses the gate (the gate
   is not re-run since the new ADR is `Proposed` not `Accepted`; full
   upgrade to `Accepted` is a follow-up). No env-var escape hatch.
   Metadata writes happen AFTER terminal mutation success, consistent
   with ADR-0031 discipline.

## Consequences

- (+) Plan-time and bead-time gates mechanically enforce ADR coverage —
  drift between code, domains, and decisions stops compounding.
- (+) Overrides are auditable — every bypass leaves a reason, actor, and
  timestamp in bead metadata.
- (+) `--supersede-adr` creates the placeholder ADR rather than papering
  over the violation, preserving the decision trail.
- (−) Cross-domain refactors need the override flag or an explicit
  supersede.
- (−) Existing repos must update spec impacted-domains to use canonical
  short tags matching `OWNERSHIP.yaml` directory names.
- (−) ADR authors must populate the `Domains` field carefully — sloppy
  domain tagging poisons both gates.

## Amendment — tri-state coverage (2026-06-11, PR #126)

PR #126 (`fix(validate): ADR-lane batch`, bead mindspec-53qx, panel
verdict UPHOLD_WITH_CONDITIONS) amends sub-decision 2's coverage
predicate, deliberately reversing spec 087 plan revision 11. The
"at least one cited **Accepted** ADR" requirement at plan time created
a chicken-and-egg for spec-introduced ADRs — legitimately `Proposed`
until the implementation that validates them ships — pressuring
authors to flip ADRs to `Accepted` prematurely.

Coverage is now **tri-state**, with the Accepted obligation moved to
the lifecycle gate where the implementation actually ships:

- **`coveredAccepted`** — a cited Accepted ADR (directly, or
  transitively via a cited Superseded chain head) declares the domain.
  Silent pass at every gate. Unchanged.
- **`coveredProposedOnly`** — the only covering cited ADR(s) are
  `Proposed`. Plan time: passes with an advisory
  `adr-coverage-proposed` warning. Bead complete: passes with an
  advisory `adr-divergence-proposed` warning (mid-implementation
  Proposed is the legitimate state). **Impl approve: ERROR** — the
  `adr-divergence-proposed` failure demands the ADR be flipped to
  `Accepted` now that the implementation ships, with the existing
  `--override-adr` / `--supersede-adr` flags as the audit-trailed
  escape. This closes the loop the plan-time tolerance opens.
- **`notCovered`** — no cited ADR of any tolerated status declares the
  domain. Error (`adr-coverage-missing` / `adr-divergence-uncovered`).
  Unchanged.

Citing the Proposed ADR in `adr_citations` is the explicit opt-in;
uncited Proposed ADRs never satisfy coverage. Implementation:
`internal/validate/plan.go::coverageOf` (plan lane) and
`internal/validate/divergence.go::ValidateDivergence` (bead-complete
and impl-approve lanes, selected by the `implApprove` parameter).

## Amendment — Impacted-Domains normalization (2026-06-16, spec 100)

Spec 100 (`ownership-gate-resolution`, bead `mindspec-4ft2`, GH #147 +
#145.1) amends **sub-decision 1**. Sub-decision 1 originally REJECTED
path-like identifiers as ambiguous (`internal/foo` vs `foo`), so a
spec whose `## Impacted Domains` entries are FILE PATHS (e.g.
`internal/genevieve/review.py`) — the genevieve-style real-world case —
failed every gate as `adr-divergence-unowned` / `adr-coverage-missing` /
`adr-cite-irrelevant`, forcing `--override-adr` on every bead.

The canonical `OWNERSHIP.yaml` directory NAME remains the identifier the
gates compare. What changes is how an author's path-like entry reaches
it: instead of being rejected outright, a path-like Impacted-Domains
entry is **NORMALIZED to its owning-domain dir-name** when exactly one
domain's `OWNERSHIP.yaml` `paths:` glob claims it. A single shared
helper (`internal/validate/ownership_resolve.go::normalizeImpactedDomains`)
resolves each raw entry — an entry that already names a domain dir is
kept verbatim; a path-like entry is glob-matched against every domain's
EXPLICIT `paths:` manifest and replaced with the owning domain's name —
and the bead-time divergence gate AND both plan-time gates
(`checkADRCoverage`, `checkADRCitations`) consume the normalized set.

Resolution is total and unambiguous: an entry owned by **zero** domains,
or by **more than one** domain, is a hard `impacted-domains-resolve`
ERROR naming the entry (and, for the ambiguous case, the conflicting
owners). No domain is ever synthesized — this is the ZFC-clean reading of
[ADR-0036](ADR-0036-ownership-discovery.md): it consumes declared data
and explicit globs, with no path-PREFIX inference (which would
re-introduce the synthesized-fallback ZFC violation ADR-0036 removed).
The per-file attribution and blast-radius guard of sub-decision 3 are
PRESERVED unchanged — the candidate set stays the resolved DECLARED
domains, so a changed file owned by an undeclared domain still fails.
`workflow` is added to this ADR's `Domain(s)` line because spec 100's
workflow source implements the gate mechanism this ADR governs.

## Rollback

Revert spec 087 PR's merge commit (`git revert -m 1 <merge-sha>`). The
gate code reverts to no-ops (`CheckADRDivergence` returns empty,
`checkADRCitations` stops intersecting domains, `checkADRCoverage`
disappears). Override and supersede metadata keys
(`mindspec_adr_override_*`, `mindspec_adr_supersede_*`) are forward-
compatible — older binaries ignore them. ADR-0032 itself remains
harmless in the tree.

## Related

- [ADR-0030](ADR-0030-executor-boundary.md) — executor surface; F1
  consumes `Executor.ChangedFiles` and `MergeBase` for divergence input.
- [ADR-0031](ADR-0031-doc-sync-gate.md) — doc-sync override pattern; F1
  mirrors the same enforcement+override+metadata discipline.
