# ADR-0033: Pluggable Tokenizer Interface and Deterministic Context Pack Budgeting

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: context-system
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0030](ADR-0030-executor-boundary.md) (executor surface; F3 uses `FileAtRef` for some reads), [ADR-0032](ADR-0032-adr-semantic-gates.md) (ADR semantic gates; F3 reads cited ADRs from plan frontmatter)

---

## Status

Stub created during spec 088-context-pack-budgeter drafting. Finalized
in spec 088 Bead N alongside the budgeter + Tokenizer implementation.

## Context

Today `mindspec context bead <id>` (via
`internal/contextpack/RenderBeadContext`) emits a fixed markdown bundle
with no token budget — large beads produce overflowing output that LLMs
truncate unpredictably. There is no Tokenizer abstraction in the
codebase; size estimates rely on raw byte length, which under-counts
multi-byte UTF-8 sequences and over-counts dense ASCII code, producing
unreliable budget decisions at the boundary.

F3 of the converged transformation plan introduces a deterministic
budget-aware bundler (`BuildBead`) with a pluggable Tokenizer interface.
The default implementation is `Approx`, which uses `runes/3.7` with a
documented ±3% tolerance against GPT-style BPE on representative
corpora. Output is byte-identical across runs given identical inputs:
all map iteration is sorted, section order is stable, and a trailing
`## Provenance` block records SHA-256 of every input source.

## Decision

Three sub-decisions:

1. **Tokenizer interface in new `internal/tokenize/` package.** The
   surface is `type Tokenizer interface { Count(s string) int; Name()
   string }`. The default implementation `Approx` returns
   `len([]rune(s)) / 3.7` (rounded), with `Name()` returning
   `"approx-3.7"`. The interface is pluggable so future BPE-based
   tokenizers (e.g., a tiktoken-backed implementation) can drop in
   without touching the budgeter. Rejected alternatives: hard-code a
   GPT-2 BPE table (heavy dependency, license friction); rely on byte
   count (~30% inaccurate for code-heavy bundles and worse for non-
   ASCII).

2. **Six-tier ranking with tail-shaving on UTF-8 rune boundaries.** The
   bundler classifies content into six tiers (must, plan-design, cited-
   ADRs, prior-bead-summaries, ownership, optional-context). The must-
   tier (bead description + acceptance criteria + design) errors if it
   alone exceeds budget — no silent truncation of essential content.
   Optional tiers are truncated tail-first on rune boundaries (never
   mid-rune). The truncation marker is the constant string
   `[truncated]` with no size annotation, deliberately avoiding the
   fixed-point convergence issue where the marker length itself changes
   the truncation point.

3. **Deterministic output with SHA provenance.** All map iteration is
   sorted by key; section order is fixed by tier and then by stable
   secondary sort. The bundle ends with a `## Provenance` block listing
   the SHA-256 of each input source (spec.md, plan.md, each cited ADR,
   each OWNERSHIP.yaml read, each prior bead summary). Re-running with
   identical inputs produces byte-identical output, enabling diff-based
   review of bundle drift across plan revisions.

## Consequences

- (+) Deterministic, reproducible bundles — same inputs always yield
  byte-identical output, suitable for caching and diff review.
- (+) Pluggable tokenizer interface — future precision improvements
  (real BPE) land without touching budget logic.
- (+) Deterministic output enables review/diff workflows — reviewers
  can see exactly what changed in a context pack between plan
  revisions.
- (−) The ±3% `Approx` tolerance may under-estimate token count for
  non-English or symbol-dense content, risking occasional overflow at
  the budget boundary.
- (−) The budgeter adds ~50ms to context-pack emit time vs the current
  unbudgeted renderer (rune counting + SHA-256 over all inputs).
- (−) The existing `RenderBeadContext` path is preserved separately
  during the transition, doubling the context-pack API surface until
  callers migrate.

## Rollback

Revert the spec 088 PR. The new `internal/tokenize/` package is a leaf
import consumed only by `internal/contextpack/`; removing it is purely
mechanical. The pre-existing `RenderBeadContext` path remains intact
throughout the transition, so rollback restores prior behavior without
data migration. ADR-0033 itself remains harmless in the tree.

## Related

- [ADR-0030](ADR-0030-executor-boundary.md) — executor surface; F3
  consumes `Executor.FileAtRef` for reading cited ADRs and prior bead
  artifacts at specific revisions.
- [ADR-0032](ADR-0032-adr-semantic-gates.md) — ADR semantic gates; F3
  reads the set of cited ADRs from plan frontmatter (the same list the
  F1 gates validate) to populate the cited-ADRs tier.
