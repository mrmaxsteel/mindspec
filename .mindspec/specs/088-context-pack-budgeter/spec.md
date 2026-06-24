---
approved_at: "2026-05-21T01:08:47Z"
approved_by: user
status: Approved
---
# Spec 088-context-pack-budgeter: Context pack budgeter — deterministic markdown bundle with `--max-tokens` budget and pluggable `Tokenizer` interface

## Goal

`mindspec context bead <id> --max-tokens N` emits a deterministic
markdown bundle whose estimated token count is `<= N` (per the
documented tokenizer), with a stable section order, source
attribution, and a trailing `## Provenance` block. Re-running with
identical inputs produces byte-identical output (and an identical
SHA over the output). This spec is **F3** of the converged
transformation plan; F4 (spec 085, `executor.Executor` boundary),
F2 (spec 086, doc-sync gate + per-domain `OWNERSHIP.yaml`), and F1
(spec 087, ADR semantic gates) have all landed, so the
`OWNERSHIP.yaml` machinery, the `contextpack.ParseSpec` parser, and
the `adr.Store.Show` surface that this spec depends on are all on
`main`.

## Background

F3 stands on F1 (087). F1 finalized `contextpack.ParseSpec` as the
single source of truth for a spec's `## Impacted Domains` (parser
at `internal/contextpack/spec.go` lines 18-87) and validated it
against the canonical four-domain identifier set
(`context-system`, `core`, `execution`, `workflow`). F3 reuses that
parser plus the same canonical identifier set to resolve which
domain docs (`overview.md` + `interfaces.md` under
`.mindspec/domains/<name>/`) to include in the context bundle.

F3 also reads each plan's cited ADRs through the same
`adr.Store.Show(id)` surface that F1 uses, then extracts each ADR's
`## Decision` section verbatim. The plan's bead-scoped subsection
is read from `plan.md` (the spec's plan file lives at
`.mindspec/specs/<spec>/plan.md`, and beads are addressed by
heading per the existing plan layout).

The transformation plan section that governs this spec is at
`/Users/Max/replit/mindspec-transformation-plan.md` lines ~128-165
and is converged. This spec implements that block verbatim; it does
not redesign.

### Tokenizer contract

The `Tokenizer` interface lives in a NEW package
`internal/tokenize/`:

```
type Tokenizer interface {
    Count(s string) int
    Name() string
}
```

The default implementation is `Approx` with `Name() == "approx"`.
Its `Count(s)` returns `int(float64(utf8.RuneCountInString(s)) /
3.7)` (rounded down), which is documented as accurate to **±3%**
on English+code reference text in the 500-2000 token range. The
`±3%` figure is the *documented contract*, not the implementation
detail: a future BPE-backed `Tokenizer` may drop in as long as it
satisfies the same contract on the reference corpus. Callers MUST
NOT depend on the precise rune-ratio constant.

### Determinism rules

- All map iteration goes through sorted-key helpers (no direct
  `range` over a `map`).
- Section order is fixed (see Requirement 9).
- The trailing `## Provenance` block lists, in deterministic order,
  the SHA-256 of every byte sequence the bundle consumed (bead
  JSON, spec.md, plan.md, each cited ADR file, each domain doc
  file, each literal `file_paths` file). The provenance block is
  itself part of the output, so the SHA-of-the-output is stable
  across re-runs.

## Impacted Domains

- context-system

## Affected packages (per domain)

- **`internal/contextpack/`** (domain: `context-system`) — the
  centerpiece. A NEW sibling file `budgeter.go` is added to export
  the new entry point `BuildBead(beadID string, maxTokens int, tok
  tokenize.Tokenizer) ([]byte, error)`. `BuildBead` emits the
  Req-9 section layout (`## Bead` / `## Spec` / `## Cited ADRs` /
  `## Plan` / `## Domain Docs` / `## File Paths` / `## Provenance`)
  and is the entry point for all new callers, including the CLI.
  The existing `RenderBeadContext(beadID string) (string, error)`
  in `beadctx.go` is preserved **as-is** (its current implementation,
  its current section headings — `## Acceptance Criteria`,
  `## Work Chunk`, `## Key File Paths`, etc.) so existing callers
  and existing `beadctx_test.go` golden assertions continue to pass
  unchanged. `RenderBeadContext` is marked as deprecated in its
  doc-comment with a pointer to `BuildBead`; it is NOT a wrapper
  around `BuildBead` (the layouts differ). `spec.go::ParseSpec` is
  reused unchanged.
- **`internal/tokenize/`** (NEW package, domain: `context-system`)
  — defines the `Tokenizer` interface and the default `Approx`
  implementation. Pure-function helper; does NOT import
  `executor`, `os/exec`, or `internal/gitutil`. Has zero external
  dependencies beyond the standard library (`unicode/utf8` for
  `Approx`).
- **`cmd/mindspec/`** (domain: `execution`, READ-ONLY for this
  spec — no behavioural change to the `execution` domain) —
  `context.go` is extended to add a `--max-tokens N` flag to the
  `mindspec context bead <id>` subcommand. The flag defaults to
  `0`, which is interpreted as "no budget" and preserves the
  existing zero-budget behaviour. Because this is a pure
  flag-plumbing change with no new behaviour in the `execution`
  domain (the actual budgeting logic lives in
  `internal/contextpack/`), `execution` is NOT listed in
  `## Impacted Domains`. The plan-time ADR coverage gate (Req 8 of
  spec 087) therefore needs ADR-0033 to cover `context-system`
  only — see `## ADR Touchpoints`.
- **`internal/adr/`** (domain: `core`, READ-ONLY) — `show.go` and
  `parse.go` are read for the existing `adr.Store.Show(id) (*ADR,
  error)` surface and the `## Decision` section extraction helper.
  No structural change.
- **`internal/bead/`** (domain: `core`, READ-ONLY) — `bdcli.go`
  `RunBD` is read for the existing `bd show <id> --json` surface
  reused by `beadctx.go::beadShowFn`.

### `OWNERSHIP.yaml` update

The existing `.mindspec/domains/context-system/OWNERSHIP.yaml`
currently claims `internal/contextpack/**` only. This spec adds the
new package `internal/tokenize/**` under the same domain. The
manifest is updated as part of this spec (Requirement 12) and the
doc-sync gate from spec 086 confirms the change at plan approval.

## ADR Touchpoints

- [ADR-0033-tokenizer-interface.md](../../adr/ADR-0033-tokenizer-interface.md)
  (**new**): Records the `Tokenizer` interface contract
  (`Count(s string) int` + `Name() string`), the documented ±3%
  tolerance for the default `Approx` implementation, the
  `runes/3.7` ratio as an implementation detail (not part of the
  contract), the determinism rules for the context bundle (sorted
  map iteration, fixed section order, SHA-256 provenance block,
  rune-aligned tail-shaving via `utf8.DecodeLastRuneInString`,
  dynamic provenance reserve sized from the rendered Provenance
  block), the six-tier ranking, the constant-length `[truncated]`
  marker (size dropped from the marker so the tail-shave is a
  convergent fixed-point — the per-input SHA in Provenance
  identifies which input was truncated), the
  `bead.metadata.spec_id` resolution rule (no fallback scan), and
  the failure mode when the must-tier plus `provReserve` exceeds
  the budget. **Domain(s): `context-system`.**
  ADR-0033 MUST list `context-system` in its `Domains` field so
  the spec 087 plan-time cite-relevant gate
  (`checkADRCitations` + `checkADRCoverage`) passes against this
  spec's single impacted domain.
- [ADR-0030-executor-boundary.md](../../adr/ADR-0030-executor-boundary.md):
  Cited as the I/O boundary contract. F3 reads ADR files, spec
  files, plan files, and domain docs via plain
  `os.ReadFile`/`filepath` — these are repository-relative reads,
  not git/process operations, so they remain outside the executor
  boundary by ADR-0030's scope (executor is git/process only). The
  bead JSON is fetched via the existing `bead.RunBD` indirection
  (already a test seam at `contextpack/beadctx.go:12`), which sits
  on the `bd` side of the executor split per ADR-0030 ("`bd` stays
  in `internal/bead`"). Citing ADR-0030 documents that F3 inherits
  this boundary without extending it.
- [ADR-0032-adr-semantic-gates.md](../../adr/ADR-0032-adr-semantic-gates.md):
  F3 reads cited ADRs from the plan as part of ranking tier 3.
  ADR-0032 defined how citations are parsed and validated at
  plan-approval time; F3 reuses that parsed citation list shape
  (via `contextpack.ParseSpec` for impacted domains and via the
  plan's frontmatter / body citation list, however the plan parser
  currently exposes it). Cited so that future changes to citation
  semantics propagate cleanly to F3.
- **ADR number reservation.** At spec-draft time the highest
  existing ADR is `ADR-0032-adr-semantic-gates.md`, so `ADR-0033`
  is the intended number. The implementer MUST re-check
  `.mindspec/adr/` at PR-open time; if `ADR-0033` has been
  claimed by a sibling spec landing first, renaming the file and
  updating cross-references (Background, this section, Acceptance
  Criteria, ADR-0033's own filename) is a **1-bead followup** under
  this spec, not a spec amendment.

## Requirements

### Hard Constraints (from converged plan)

1. **HC-1 F3 lands AFTER F1 (spec 087).** F1 has merged; the
   `contextpack.ParseSpec` parser and the canonical four-domain
   identifier set are on `main`. The doc-sync gate from F2 (086)
   is in force; the ADR semantic gate from F1 (087) is in force.
   This spec is unblocked.
2. **HC-2 Solo-developer UX preserved.** The new flag
   `--max-tokens N` is opt-in. `mindspec context bead <id>`
   without `--max-tokens` (or with `--max-tokens 0`) preserves the
   existing zero-budget behaviour: no truncation, all tiers
   included verbatim, provenance block still emitted (determinism
   is a property of the bundle, not the budget — see Req 11).
3. **HC-3 Existing test suite preserved.** No test is skipped,
   excluded, or marked `t.Skip` relative to `main`. New tests are
   additive. The existing `contextpack/beadctx_test.go` suite
   continues to pass because `RenderBeadContext` is preserved
   verbatim (not rewired through `BuildBead`); `BuildBead` is a
   separate new entry point with its own new tests (see
   `## Acceptance Criteria`).
4. **HC-4 `viz/agentmind/bench` excluded.** F3 does not read,
   reference, or include any file under those trees in the context
   bundle. Domain-doc inclusion is restricted to the four canonical
   domains, none of which claim those paths. Optional `file_paths`
   literal inclusion (tier 6 in the ranking; section 7 in the
   output) explicitly rejects any path whose first segment is in
   `{viz, agentmind, bench}` with the error
   `"file_paths entry %q is under an excluded tree (viz/agentmind/bench)"`.
5. **HC-5 Every commit `go build ./... && go test -short ./...`
   green.** Including the `internal/tokenize/` introduction
   commit, the `BuildBead` body commit, the `OWNERSHIP.yaml`
   update commit, and the CLI-flag commit.
6. **HC-6 AST boundary lint from spec 085 stays green.** The new
   `internal/tokenize/` package MUST NOT import `os/exec`,
   `internal/gitutil`, or `internal/executor`. It is a
   pure-function helper. `internal/contextpack/budgeter.go` (or
   the extended `beadctx.go`) inherits the same constraint
   already in force on `internal/contextpack/`.

### Spec-specific (from F3 design)

7. **`Tokenizer` interface (new package
   `internal/tokenize/`).** A new package exports:
   ```
   type Tokenizer interface {
       Count(s string) int
       Name() string
   }

   type Approx struct{}

   func (Approx) Count(s string) int { /* runes / 3.7 */ }
   func (Approx) Name() string       { return "approx" }
   ```
   The `runes / 3.7` ratio is an implementation detail; the
   *contract* is "±3% of a reference BPE on English+code in the
   500-2000 token range" (documented in package doc-comments and
   in ADR-0033). The package has a `doc.go` (or top-of-file
   doc-comment) that states this contract verbatim.
8. **`BuildBead(beadID string, maxTokens int, tok
   tokenize.Tokenizer) ([]byte, error)`** in
   `internal/contextpack/` (file: `budgeter.go`, or appended to
   `beadctx.go`). Inputs gathered:
   - Bead JSON via `beadShowFn("show", beadID, "--json")` (existing
     test seam at `beadctx.go:12`).
   - The spec file at `.mindspec/specs/<spec>/spec.md`,
     where `<spec>` is taken from the bead JSON's
     `metadata.spec_id` field. **No fallback scan.** If the bead
     JSON lacks `metadata.spec_id`, `BuildBead` returns the error
     `"bead JSON for <id> lacks metadata.spec_id; cannot resolve
     spec"` and returns `nil` for the byte slice. The repo-root
     scan path is explicitly removed to keep `BuildBead`
     deterministic and free of filesystem-order dependence.
   - The plan file at `.mindspec/specs/<spec>/plan.md`,
     restricted to the bead's section. Section resolution scans
     the plan top-down and **the first level-2 (or deeper) heading
     whose text contains `beadID` OR the bead's `Title` wins**
     (top-down, first-match). If no heading matches, the entire
     plan body is included.
   - Each cited ADR's `## Decision` section, loaded via
     `adr.Store.Show(id)` against the in-scope `adr.NewFileStore`
     rooted at `.mindspec/adr/`.
   - For each impacted domain in `spec.Domains`: `overview.md` and
     `interfaces.md` under `.mindspec/domains/<domain>/`.
     Missing files are skipped silently (warning emitted as a
     comment line in the output, not an error).
   - Optional `file_paths` from the bead's `metadata.file_paths`
     list — literal file contents, each tail-truncated at tier 6
     to fit the remaining budget.
9. **Fixed section order in the output bundle.** In this exact
   order, with these exact heading strings:
   1. `# Bead Context: <Title>` (level-1 header; line includes
      `**Bead**: <id> | **~<n> tokens**` per the existing format,
      where `<n>` is `tok.Count(rendered)` rounded to the nearest
      whole token).
   2. `## Bead` — description + acceptance criteria + design
      (must-include tier; Requirement 10 governs the failure mode
      if this tier alone exceeds the budget).
   3. `## Spec` — spec Goal + impacted-domains list + spec
      acceptance criteria.
   4. `## Cited ADRs` — each cited ADR's id + title + verbatim
      `## Decision` section, ordered by ADR id ascending.
   5. `## Plan` — the bead's section of the plan file.
   6. `## Domain Docs` — for each impacted domain (ordered by
      canonical-identifier sort), `### <domain>` then
      `#### overview.md` then `#### interfaces.md`, each verbatim,
      tail-truncated as needed.
   7. `## File Paths` (only if `file_paths` is non-empty) — for
      each path (sorted ascending), `### <path>` then a fenced
      code block with the file's literal contents, tail-truncated
      as needed.
   8. `## Provenance` — see Requirement 11.
10. **Ranking and tail-shaving truncation strategy.** The six
    ranking tiers map to sections 2-7 in the order listed in
    Requirement 9 (tier 1 = section 2, tier 2 = section 3, etc.).
    Let `provReserve` be the dynamic provenance reserve computed
    per Req 11 (the token cost of the fully-rendered Provenance
    block for this invocation's inputs).
    - **Tier 1 (must):** bead description + acceptance criteria +
      design. If `tok.Count(tier1) + provReserve > maxTokens`,
      `BuildBead` returns the error `"bead context exceeds
      --max-tokens N; raise budget or split bead"` with `N` filled
      in. NO partial output, NO truncation of tier 1; the returned
      `[]byte` is `nil`.
    - **Tiers 2-6:** included in order; each tier's content is
      tail-shaved (truncated from the end) until
      `tok.Count(everything_so_far) + provReserve <= maxTokens`.
      Tail-shaving is **rune-aligned**: after picking a candidate
      byte offset, back up to the nearest valid rune boundary
      using `utf8.DecodeLastRuneInString` (loop while the decoded
      rune is `utf8.RuneError` with size `1`). This guarantees the
      output remains valid UTF-8 and that the truncation point is
      deterministic regardless of how many bytes a final rune
      occupies.
    - When tail-shaving fires within a tier, the literal marker
      `[truncated]` (no size, no byte count) is appended at the
      truncation point. The marker has constant length, which is
      required for the shave to converge as a fixed-point — a
      marker whose length varied with N would re-exceed the budget
      after appending. The Provenance block's per-input SHA lines
      identify exactly which inputs were truncated; no per-marker
      size is needed.
    - Within tier 5 (domain docs, section 6 in output order),
      truncation happens per file (per `overview.md` /
      `interfaces.md`), not per domain. Within tier 6
      (`file_paths`, section 7 in output order), truncation
      happens per file.
    - `maxTokens == 0` is interpreted as "no budget": all tiers
      are included verbatim, no truncation, no marker, no
      must-tier-overflow error. (`provReserve` is still computed
      but not enforced.)
11. **`## Provenance` block.** Appears at the end of every bundle
    (whether or not `maxTokens > 0`). Content (in this exact
    order):
    ```
    ## Provenance

    - tokenizer: <tok.Name()>
    - max_tokens: <N>
    - estimated_tokens: <tok.Count(output_without_this_block)>
    - inputs:
      - bead:<id> sha256:<hex>
      - spec:<path> sha256:<hex>
      - plan:<path> sha256:<hex>
      - adr:<ID> sha256:<hex>          (one line per cited ADR, sorted)
      - domain:<name>/overview.md sha256:<hex>    (sorted)
      - domain:<name>/interfaces.md sha256:<hex>  (sorted)
      - file:<path> sha256:<hex>       (one line per file_paths entry, sorted)
    ```
    The block's content is included in the final SHA-of-output but
    `estimated_tokens` reflects the body only (no
    chicken-and-egg).

    **Dynamic provenance reserve.** The Provenance block grows
    linearly with input count, so a fixed 50-token reserve is
    unsafe (a bead citing 5 ADRs + 4 domains + 6 file_paths can
    blow past 50 tokens on the SHA lines alone). Instead,
    `BuildBead` computes the reserve dynamically:

    1. After resolving all inputs (bead JSON, spec, plan, ADRs,
       domain docs, file_paths) and their SHA-256 hashes, render
       the Provenance block exactly as it will appear in the
       output (using the resolved `tok.Name()`, the resolved
       `max_tokens` value, and a placeholder for
       `estimated_tokens` of the same digit width as the eventual
       value — see step 3).
    2. Compute `provReserve := tok.Count(renderedProvBlock)`.
       This is the headroom used in Req 10's tier-1 check and in
       the per-tier tail-shave inequality.
    3. `estimated_tokens` is rendered as a fixed-width decimal
       (right-justified, width = number of digits in `maxTokens`,
       or 6 if `maxTokens == 0`) so the Provenance block's byte
       length is independent of the final body token count. This
       removes the chicken-and-egg loop.

    The fixed `50` constant is removed.
12. **`OWNERSHIP.yaml` update.** As part of this spec,
    `.mindspec/domains/context-system/OWNERSHIP.yaml` is
    updated to add `internal/tokenize/**` alongside the existing
    `internal/contextpack/**` claim:
    ```
    paths:
      - internal/contextpack/**
      - internal/tokenize/**
    ```
    The doc-sync gate from spec 086 enforces this change at plan
    approval; the schema rejection of `viz/`/`agentmind/`/`bench/`
    first-segments (spec 086) continues to apply.
13. **CLI surface.** `cmd/mindspec/context.go` gains a
    `--max-tokens N` flag on the `bead` subcommand (`mindspec
    context bead <id> --max-tokens N`). Default value `0`
    (unbudgeted). Negative values are rejected at flag-parse time
    with `"--max-tokens must be >= 0"`. The flag is plumbed into a
    new call `contextpack.BuildBead(beadID, maxTokens,
    tokenize.Approx{})`. The legacy `contextpack.RenderBeadContext`
    is preserved as-is for back-compat — it is NOT rewired through
    `BuildBead` (see Requirement 14).
14. **Backward compatibility for `RenderBeadContext`.** The
    existing exported `contextpack.RenderBeadContext(beadID
    string) (string, error)` (used by anything that imported
    `internal/contextpack` before this spec) is preserved **as-is**
    — same body, same section headings (`## Acceptance Criteria`,
    `## Work Chunk`, `## Key File Paths`, etc.), same callers. It
    is NOT rewired through `BuildBead` (the layouts differ: Req 9
    defines a new layout for `BuildBead` that is intentionally
    distinct from the legacy `RenderBeadContext` shape). A
    deprecation doc-comment is added pointing new code at
    `BuildBead`. Existing tests in
    `internal/contextpack/beadctx_test.go` continue to pass
    unchanged precisely because the function they exercise did not
    change. New tests for `BuildBead` are added under the names
    listed in `## Acceptance Criteria`. Once all in-tree callers
    have migrated to `BuildBead`, a future spec may remove
    `RenderBeadContext`; that removal is out of scope here.
15. **Package documentation.** Both new/extended packages get
    doc-comments:
    - `internal/tokenize/doc.go` (or top-of-file): documents the
      `Tokenizer` interface, the ±3% contract, and that the
      default `Approx` implementation uses `runes/3.7` for
      English+code in the 500-2000 token range.
    - `internal/contextpack/budgeter.go`: documents the six-tier
      ranking, the rune-aligned tail-shaving truncation strategy
      (`utf8.DecodeLastRuneInString` back-up), the constant-length
      `[truncated]` marker, the must-tier-overflow error message,
      the dynamic `provReserve` algorithm (compute the Provenance
      block first, count its tokens, use that as the headroom),
      and the rule that `BuildBead` requires `bead.metadata.spec_id`
      with no fallback scan.

## Scope

### In Scope

- New package `internal/tokenize/` with the `Tokenizer` interface
  and the default `Approx` implementation per Requirement 7.
- New entry point `contextpack.BuildBead(beadID, maxTokens, tok)`
  with the six-tier ranking and tail-shaving truncation per
  Requirements 8-10.
- Fixed section order in the output bundle per Requirement 9.
- `## Provenance` block with SHA-256 over every input artefact and
  a **dynamic** token reserve sized from the rendered block per
  Requirement 11 (the fixed 50-token reserve was removed during
  round-1 review).
- `OWNERSHIP.yaml` update for `internal/tokenize/**` per
  Requirement 12.
- `--max-tokens N` flag on `mindspec context bead <id>` per
  Requirement 13.
- Preservation of `RenderBeadContext` as-is (no rewiring; new
  layout lives in `BuildBead` only) per Requirement 14.
- Package doc-comments documenting the tokenizer contract and the
  budgeter behaviour per Requirement 15.
- Constant-length `[truncated]` marker (no size suffix) used at
  every tail-shaving truncation point, applied on rune boundaries
  via `utf8.DecodeLastRuneInString`, in sections 5, 6, 7 of the
  Requirement-9 output order.
- ADR-0033 drafted and accepted as part of this spec, with
  `Domains: [context-system]` to satisfy the spec 087 plan-time
  cite-relevant gate.

### Out of Scope

- **Real BPE tokenizer implementation** — the `Tokenizer`
  interface is pluggable, but the only implementation shipped in
  this spec is `Approx`. A future spec may add a BPE
  implementation (e.g., a tiktoken port) and switch the CLI
  default.
- **Cross-spec context packs** — `BuildBead` is bead-scoped only.
  Building a spec-wide or repo-wide context bundle is a future
  spec.
- **Live token reporting during generation** — no streaming,
  no progress indicator. `BuildBead` returns the final bundle or
  an error.
- **Per-section budget hints** — the operator passes a single
  `--max-tokens` value; the tiered ranking decides how to spend
  it. A future spec may add per-section overrides.
- **Two-pass token-count reconciliation between Provenance and
  body** — `provReserve` is computed once from the rendered
  Provenance block (with `estimated_tokens` rendered at fixed
  width per Req 11), then held constant across all tail-shave
  iterations. No iterative re-render of Provenance is performed.
- **Tokenizer selection via CLI flag** — the CLI always uses
  `Approx` in this spec. A future spec may add
  `--tokenizer <name>`.
- **Concurrent budget enforcement against multiple beads in one
  invocation** — single-bead only.
- **`viz/agentmind/bench` inclusion under any flag** — HC-4 is
  absolute; no escape hatch.

## Acceptance Criteria

- [ ] `TestContextPackDeterministic` passes: two runs of
  `BuildBead(<id>, 2000, tokenize.Approx{})` against an identical
  on-disk fixture produce byte-identical output AND the SHA-256
  over the output is identical across runs.
- [ ] `TestContextPackBudget` passes: `BuildBead(<id>, 2000,
  tokenize.Approx{})` against a populated fixture (with cited
  ADRs, domain docs, and `file_paths` whose total raw size exceeds
  2000 tokens) returns a bundle for which
  `tokenize.Approx{}.Count(output) <= 2000` AND every must-tier
  section (the `## Bead` section) is present in full.
- [ ] `TestContextPackTruncationMarker` passes: a budget that
  forces truncation in the domain-docs tier (section 6 in
  Requirement 9's ordering) produces output containing at least
  one literal `[truncated]` marker (constant string, no size
  suffix). The byte immediately preceding the marker is the last
  byte of a complete UTF-8 rune (rune-aligned shave, verified by
  `utf8.ValidString` on the entire output).
- [ ] `TestContextPackErrorOnMustTierOverflow` passes: a bead
  whose `description + acceptance criteria + design` plus the
  dynamic `provReserve` exceeds `maxTokens` (e.g., a fixture
  with a 4000-rune design and `--max-tokens 100`) causes
  `BuildBead` to return an error whose message contains
  `"bead context exceeds --max-tokens"` and the literal budget
  value (`100`). NO partial output is returned (the `[]byte`
  return is `nil`).
- [ ] `TestContextPackErrorOnMissingSpecID` passes: a bead JSON
  fixture whose `metadata` lacks `spec_id` causes `BuildBead` to
  return an error whose message contains
  `"lacks metadata.spec_id"`. NO partial output. NO repo-root
  fallback scan is attempted (asserted by inspecting that no
  `filepath.Walk` is invoked under `.mindspec/specs/` —
  e.g., via a test seam that fails the test if a walk runs).
- [ ] `TestContextPackProvenanceReserveIsDynamic` passes: two
  fixtures — one with a small input set (1 ADR, 1 domain, 0
  file_paths) and one with a large input set (5 ADRs, 4 domains,
  6 file_paths) — produce Provenance blocks whose
  `tok.Count(provBlock)` values differ by more than 50 tokens,
  proving the reserve is not a fixed constant. Both bundles
  satisfy `tok.Count(output) <= maxTokens`.
- [ ] `TestTokenizerApproxToleranceWithinThreePercent` passes:
  the test file `internal/tokenize/approx_test.go` contains a
  hand-counted reference fixture — a ~100-token English-prose
  sample (the corpus is documented inline as a Go string literal,
  and the reference count is recorded as a `const
  referenceTokens = N` derived by hand-counting whitespace-
  delimited words plus punctuation tokens, with the counting
  rule stated in a comment). The assertion is
  `|Approx{}.Count(corpus) - referenceTokens| <=
  ceil(referenceTokens * 0.03)`. No external BPE model file is
  required; the test is self-contained and falsifiable.
- [ ] `TestProvenanceBlockContainsInputSHA` passes: the output
  bundle's tail contains a `## Provenance` block listing
  `sha256:` for each of: the bead JSON, the spec.md, the plan.md,
  every cited ADR, every included domain doc file, and every
  `file_paths` entry. Each SHA is a valid 64-character lowercase
  hex string.
- [ ] `TestRenderBeadContextBackCompatPreserved` passes: every
  existing test in `internal/contextpack/beadctx_test.go` passes
  unchanged because `RenderBeadContext` is preserved verbatim
  (its body, its section headings, its callers are untouched by
  this spec). `BuildBead` is a separate new entry point and does
  NOT replace `RenderBeadContext`.
- [ ] `TestContextPackRejectsExcludedFilePath` passes: a bead
  with `metadata.file_paths` including a path under `viz/`,
  `agentmind/`, or `bench/` causes `BuildBead` to return an error
  whose message contains `"excluded tree"` and names the
  offending path. No partial output.
- [ ] `TestContextPackRejectsNegativeBudget` passes: invoking
  `mindspec context bead <id> --max-tokens -1` returns an error
  containing `"--max-tokens must be >= 0"`.
- [ ] `TestContextPackSectionOrder` passes: the output bundle's
  level-2 headings appear in the exact order specified by
  Requirement 9 (`## Bead`, `## Spec`, `## Cited ADRs`, `## Plan`,
  `## Domain Docs`, optional `## File Paths`, `## Provenance`).
  No interleaving, no duplicates.
- [ ] `TestTokenizeNoForbiddenImports` passes: `go list -deps
  ./internal/tokenize/...` does NOT include `os/exec`,
  `github.com/mrmaxsteel/mindspec/internal/gitutil`, or
  `github.com/mrmaxsteel/mindspec/internal/executor`. The
  package is a pure-function helper.
- [ ] `ADR-0033-tokenizer-interface.md` exists under
  `.mindspec/adr/`, has `Status: Accepted`, lists
  `Domains: [context-system]` so the spec 087 plan-time
  cite-relevant gate passes, cites ADR-0030 and ADR-0032, and
  records the ±3% contract + the determinism rules + the
  six-tier ranking + the must-tier-overflow failure mode.
- [ ] `.mindspec/domains/context-system/OWNERSHIP.yaml`
  includes both `internal/contextpack/**` and
  `internal/tokenize/**` under `paths:`.
- [ ] All existing tests still pass; AST boundary lint from
  spec 085 (`internal/lint/boundary_test.go::TestEnforcementHasNoGitLeaks`)
  stays green. The new `internal/tokenize/` package and the
  extended `internal/contextpack/` files do not regress the
  boundary contract.
- [ ] `go build ./... && go test -short ./...` is green on every
  commit of the F3 branch (verified by per-commit CI or by
  `git rebase -x`).

## Validation Proofs

- `go test ./internal/contextpack -run TestContextPackDeterministic -v`
  — PASS; two runs produce identical bytes and identical SHA.
- `go test ./internal/contextpack -run TestContextPackBudget -v` —
  PASS; `tokenize.Approx{}.Count(output) <= 2000`; must-tier
  sections present.
- `go test ./internal/contextpack -run TestContextPackTruncationMarker -v`
  — PASS; literal `[truncated]` marker present in output and
  `utf8.ValidString(output)` is true (rune-aligned shave).
- `go test ./internal/contextpack -run TestContextPackErrorOnMustTierOverflow -v`
  — PASS; error message contains `"bead context exceeds --max-tokens"`
  and the budget value; returned `[]byte` is `nil`.
- `go test ./internal/tokenize -run TestTokenizerApproxToleranceWithinThreePercent -v`
  — PASS; against the hand-counted ~100-token English-prose
  fixture documented inline in `approx_test.go`,
  `|Approx{}.Count(corpus) - referenceTokens| <=
  ceil(referenceTokens * 0.03)`.
- `go test ./internal/contextpack -run TestProvenanceBlockContainsInputSHA -v`
  — PASS; SHA lines present for every input artefact, each a
  valid 64-character lowercase hex string.
- `go test ./internal/contextpack -run TestRenderBeadContextBackCompatPreserved -v`
  — PASS; the existing `beadctx_test.go` golden fixtures pass
  unchanged because `RenderBeadContext` is preserved verbatim
  (not delegated through `BuildBead`).
- `go test ./internal/contextpack -run TestContextPackErrorOnMissingSpecID -v`
  — PASS; error contains `"lacks metadata.spec_id"`; no
  `filepath.Walk` is triggered.
- `go test ./internal/contextpack -run TestContextPackProvenanceReserveIsDynamic -v`
  — PASS; small-input vs large-input Provenance token counts
  differ by more than 50 tokens.
- `go test ./internal/contextpack -run TestContextPackRejectsExcludedFilePath -v`
  — PASS; error names the offending path and contains
  `"excluded tree"`.
- `go test ./cmd/mindspec -run TestContextPackRejectsNegativeBudget -v`
  — PASS (CLI-level test; error contains
  `"--max-tokens must be >= 0"`).
- `go test ./internal/contextpack -run TestContextPackSectionOrder -v`
  — PASS; level-2 headings in fixed order.
- `go list -deps ./internal/tokenize/... | grep -E
  '(os/exec|internal/gitutil|internal/executor)'` — exits non-zero
  (no matches), proving the boundary.
- `go test ./internal/lint -run TestEnforcementHasNoGitLeaks -v` —
  PASS (boundary lint still green).
- `go build ./... && go test -short ./...` — exit 0 on every
  commit of this spec's branch.
- Manual: `mindspec context bead <id> --max-tokens 2000 > out1.md;
  mindspec context bead <id> --max-tokens 2000 > out2.md;
  diff out1.md out2.md` — exits 0 (no diff). `sha256sum out1.md
  out2.md` — both hashes identical.
- Manual: `mindspec context bead <id> --max-tokens 100` against a
  bead with a 4000-rune design — exits non-zero, stderr contains
  `"bead context exceeds --max-tokens 100"`.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-21
- **Notes**: Approved via mindspec approve spec