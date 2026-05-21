# ADR-0032: Semantic ADR Coverage Gates with Override and Supersede Flags

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: validation, adr, lifecycle
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
   it belongs).

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
