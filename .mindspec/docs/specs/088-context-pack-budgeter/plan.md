---
adr_citations:
    - id: ADR-0033
approved_at: "2026-05-21T01:20:21Z"
approved_by: user
bead_ids:
    - mindspec-58tn.1
    - mindspec-58tn.2
    - mindspec-58tn.3
spec_id: 088-context-pack-budgeter
status: Approved
version: "1"
---
# Plan: 088-context-pack-budgeter

## ADR Fitness

- **ADR-0033** (new — "Pluggable Tokenizer Interface and Deterministic
  Context Pack Budgeting"): the stub at
  `.mindspec/docs/adr/ADR-0033-tokenizer-interface.md` already carries
  `Status: Accepted` (line 4), `Domain(s): context-system` (line 5),
  and a Decision section recording three sub-decisions verbatim: (1)
  the `Tokenizer` interface `Count(s string) int` + `Name() string`
  in the new `internal/tokenize/` package with `Approx` as the default
  `runes/3.7` implementation; (2) the six-tier ranking with rune-
  aligned tail-shaving (`utf8.DecodeLastRuneInString`) and the
  constant-length `[truncated]` marker (no size suffix, so the shave
  is a convergent fixed-point); (3) deterministic output with sorted
  map iteration, fixed section order, and a trailing `## Provenance`
  block of SHA-256 hashes over every input source. The narrative
  "Status" paragraph (lines 14-16) still reads "Stub created during
  spec 088-context-pack-budgeter drafting. Finalized in spec 088
  Bead N alongside the budgeter + Tokenizer implementation." — Bead 3
  step 4 replaces that paragraph with "Finalized in spec 088 Bead 3
  alongside the `internal/tokenize/` package, the
  `internal/contextpack/budgeter.go::BuildBead` entry point, and the
  `--max-tokens` flag in `cmd/mindspec/context.go`." The Decision
  section already documents the `bead.metadata.spec_id` resolution
  rule with no fallback scan AND the dynamic `provReserve` algorithm
  (sized from the rendered Provenance block, not a fixed constant)
  per spec Requirements 8 and 11 — no Decision-section edit is
  required at finalization. ADR-0033's authored `**Domain(s)**:
  context-system` line satisfies the spec 087 plan-time cite-
  relevant gate (`checkADRCitations` + `checkADRCoverage`) against
  this spec's single impacted domain `context-system` exactly per
  Requirement 12 of ADR-0032.

  **ADR number reservation.** At plan-draft time the highest
  existing ADR is `ADR-0033-tokenizer-interface.md` (the stub this
  spec finalizes), so no renumber is needed. If a sibling spec
  lands claiming `0033` first between plan-draft and impl, Bead 3
  step 4 renumbers to the next free integer (`git mv` the ADR file,
  update this plan's `adr_citations` frontmatter, the spec.md
  Background + ADR Touchpoints + Acceptance Criteria sections, and
  any test that cites the ADR number) as a 1-bead followup before
  merge.

- **ADR-0030** ("Executor as the Git/Process I/O Boundary"):
  prerequisite. F3 reads ADR files, spec.md, plan.md, and domain
  docs via plain `os.ReadFile`/`filepath` — these are repository-
  relative reads, not git/process operations, so they remain outside
  the executor boundary by ADR-0030's scope (executor is git/process
  only). The `bead show <id> --json` lookup goes through the
  existing `bead.RunBD` indirection (test seam at
  `internal/contextpack/beadctx.go:12`), which sits on the `bd`
  side of the executor split per ADR-0030 ("`bd` stays in
  `internal/bead`"). The new `internal/tokenize/` package and the
  new `internal/contextpack/budgeter.go` file MUST NOT import
  `os/exec`, `internal/gitutil`, or `internal/executor` — the
  `internal/lint/boundary_test.go::TestEnforcementHasNoGitLeaks`
  invariant from spec 085 Bead 4 continues to hold. No
  contradiction with ADR-0030.

- **ADR-0032** ("Semantic ADR Coverage Gates with Override and
  Supersede Flags"): the plan-time gate this plan must satisfy. The
  spec's `## Impacted Domains` parses to `["context-system"]` (a
  single canonical identifier per the F1 four-domain identifier
  set). The plan cites ADR-0033 which declares
  `Domain(s): context-system`; the case-folded set intersection
  (`intersectFold`) is non-empty so `checkADRCitations` emits NO
  `adr-cite-irrelevant` Issue. The same single ADR satisfies
  `checkADRCoverage` for the single impacted domain. ADR-0030 and
  ADR-0032 are co-cited for the boundary contract and the gate
  contract respectively — neither needs to cover `context-system`
  on its own because ADR-0033 already does. F3 is therefore
  PLAN-COVERED for ADR-0032's plan-time gate with no override or
  supersede flag required.

## Testing Strategy

This spec's failure mode is **unreliable LLM context packs**: the
existing `RenderBeadContext` emits a fixed bundle with no token
budget, so large beads produce overflowing markdown that downstream
LLMs truncate unpredictably (mid-rune, mid-section, sometimes mid-
ADR-Decision). The defense is mechanical exit-code enforcement at
TWO points: (1) the `Tokenizer` interface gives `BuildBead` a
falsifiable budget counter (with a documented ±3% contract,
asserted by `TestTokenizerApproxToleranceWithinThreePercent`), and
(2) `BuildBead` returns either a bundle whose
`tok.Count(output) <= maxTokens` OR an explicit error naming the
budget and the offending tier — never a silently-truncated bundle
that satisfies neither contract.

**Bead ordering note.** Bead 1 (`internal/tokenize/` package +
`Tokenizer` interface + `Approx` impl + `OWNERSHIP.yaml` update)
lands FIRST because Bead 2's `BuildBead` depends on the interface
for its budget counter. Bead 2 (`BuildBead` + six-tier ranking +
tail-shave + SHA provenance) depends on Bead 1 and is the largest
bead. Bead 3 (CLI flag wiring + ADR-0033 finalization) depends on
Bead 2 — the flag plumbs `maxTokens` into the new `BuildBead`
entry point and the ADR narrative edit closes out the spec.

Per the converged plan's HC-5 the per-commit gate is CI; locally
each bead's verification block ends with the exact command pair
`go build ./... && go test -short ./...` passing. HC-3 (existing
tests preserved, no skips relative to `main`) is enforced per-bead:
Bead 3 step 5 records a `go test -v ./...` test-name + status diff
vs `main` in its final commit message. The legacy
`RenderBeadContext` is preserved verbatim per Requirement 14 — its
`internal/contextpack/beadctx_test.go` golden assertions continue
to pass unchanged because that function is NOT rewired through
`BuildBead`.

**New test additions across the three beads:**

- **Bead 1** (`internal/tokenize/approx_test.go` — new file, and
  the boundary lint placement chosen per step 5):
  - `TestTokenizerApproxToleranceWithinThreePercent` — the test
    file contains a hand-counted reference fixture as a `const`
    `referenceCorpus` Go string literal (a ~100-token English-
    prose sample) and a `const referenceTokens = N` derived by
    hand-counting whitespace-delimited words plus punctuation
    tokens, with the counting rule stated in a comment block. The
    assertion is
    `|Approx{}.Count(corpus) - referenceTokens| <=
    int(math.Ceil(float64(referenceTokens) * 0.03))`. No external
    BPE model file is required; the test is self-contained and
    falsifiable.
  - `TestTokenizeNoForbiddenImports` (placement per step 5 —
    either extend `internal/lint/boundary_test.go` to ALSO
    cover `internal/tokenize/...`, or add a sibling
    `internal/tokenize/boundary_test.go` using
    `go/packages`) — asserts that `internal/tokenize/...` does
    NOT transitively import `os/exec`,
    `github.com/mrmaxsteel/mindspec/internal/gitutil`, or
    `github.com/mrmaxsteel/mindspec/internal/executor`. Test
    name matches the spec Acceptance Criterion verbatim.

- **Bead 2** (`internal/contextpack/budgeter_test.go` — new file):
  - `TestContextPackDeterministic` — calls
    `BuildBead("mindspec-088.X", 2000, tokenize.Approx{})` twice
    against an identical on-disk fixture (a `t.TempDir()`-rooted
    repo skeleton with a bead JSON, a spec.md, a plan.md, two
    cited ADRs, and two domain doc files, all created from
    string literals at test setup time). Assert `bytes.Equal`
    on the two outputs AND `sha256.Sum256` equality on the two
    outputs.
  - `TestContextPackBudget` — same fixture but with the cited
    ADR Decision sections inflated to ~3000 tokens combined.
    Assert `tokenize.Approx{}.Count(output) <= 2000` AND the
    `## Bead` section (must-tier, section 2 in Req-9 order)
    appears in full (string-match its expected content) in the
    output.
  - `TestContextPackTruncationMarker` — same shape but with a
    budget chosen to force truncation in the domain-docs tier
    (section 6). Assert the output contains at least one
    literal `[truncated]` substring (constant string, no size
    suffix), `utf8.ValidString(output)` is true (rune-aligned
    shave verified end-to-end), and the marker appears WITHIN a
    `## Domain Docs` subsection (not in `## Bead`, `## Spec`,
    `## Cited ADRs`, or `## Plan`).
  - `TestContextPackErrorOnMustTierOverflow` — fixture with a
    4000-rune bead `Design` field and `--max-tokens 100`.
    Assert `BuildBead` returns an error whose `.Error()`
    contains the substring `"bead context exceeds --max-tokens"`
    AND the literal budget value `"100"`; assert the returned
    `[]byte` is `nil` (no partial output).
  - `TestContextPackErrorOnMissingSpecID` — fixture whose
    `bd show <id> --json` payload's `metadata` map omits
    `spec_id`. Assert `BuildBead` returns an error whose
    `.Error()` contains the substring `"lacks metadata.spec_id"`
    AND the returned `[]byte` is `nil`. Asserts no fallback scan
    is attempted: the test installs a `filepath.Walk` recorder
    seam (introduced in Bead 2 step 4 as the package-level var
    `walkFn = filepath.Walk` with a `SetWalkForTest` helper) and
    asserts the recorder records ZERO invocations under
    `.mindspec/docs/specs/`.
  - `TestProvenanceBlockContainsInputSHA` — assert the output's
    tail contains `## Provenance` followed by `sha256:` lines
    for each input artefact (`bead:<id>`, `spec:<path>`,
    `plan:<path>`, one `adr:<ID>` line per cited ADR, one
    `domain:<name>/overview.md` and one
    `domain:<name>/interfaces.md` line per domain, one
    `file:<path>` line per `file_paths` entry). Each SHA is a
    valid 64-character lowercase hex string (validated via
    `regexp.MustCompile("^[a-f0-9]{64}$")`).
  - `TestContextPackProvenanceReserveIsDynamic` — two fixtures:
    fixture A has 1 cited ADR + 1 domain + 0 `file_paths`;
    fixture B has 5 cited ADRs + 4 domains (the four canonical
    domains, with overview+interfaces each) + 6 `file_paths`.
    Render both Provenance blocks via the same helper
    `renderProvBlock`. Assert
    `|tok.Count(provB) - tok.Count(provA)| > 50`, proving the
    reserve is NOT a fixed constant. Additionally assert
    `tok.Count(output) <= maxTokens` for both fixtures (i.e.,
    the dynamic reserve correctly accommodates each block).
  - `TestContextPackSectionOrder` — assert the output's level-2
    headings appear in the exact Req-9 order (`## Bead`,
    `## Spec`, `## Cited ADRs`, `## Plan`, `## Domain Docs`,
    optional `## File Paths`, `## Provenance`) with no
    interleaving and no duplicates.
  - `TestContextPackRejectsExcludedFilePath` — fixture with a
    `file_paths` entry under `viz/`, `agentmind/`, or `bench/`
    causes `BuildBead` to return an error whose message
    contains `"excluded tree"` and names the offending path. No
    partial output (returned `[]byte` is `nil`).

- **Bead 3** (`cmd/mindspec/context_test.go` — new file, OR
  extension of an existing `cmd/mindspec` test file if
  `context_test.go` does not yet exist):
  - `TestContextPackRejectsNegativeBudget` — invoke the cobra
    command tree with `args: []string{"context", "bead",
    "test-bead", "--max-tokens", "-1"}` and assert the
    returned error's `.Error()` contains the substring
    `"--max-tokens must be >= 0"`. The test installs a
    `beadShowFn` stub via `contextpack.SetBeadShowForTest` to
    avoid invoking the real `bd`.

## Provenance

Each spec.md Acceptance Criterion maps to the bead whose
verification proves it satisfied:

- `TestTokenizerApproxToleranceWithinThreePercent` → **Bead 1**
  step 6.
- `TestTokenizeNoForbiddenImports` → **Bead 1** step 6.
- `TestContextPackDeterministic` → **Bead 2** step 7.
- `TestContextPackBudget` → **Bead 2** step 7.
- `TestContextPackTruncationMarker` → **Bead 2** step 7.
- `TestContextPackErrorOnMustTierOverflow` → **Bead 2** step 7.
- `TestContextPackErrorOnMissingSpecID` → **Bead 2** step 7
  (asserts the walk recorder records zero invocations).
- `TestContextPackProvenanceReserveIsDynamic` → **Bead 2** step 7.
- `TestProvenanceBlockContainsInputSHA` → **Bead 2** step 7.
- `TestContextPackSectionOrder` → **Bead 2** step 7.
- `TestContextPackRejectsExcludedFilePath` → **Bead 2** step 7.
- `TestRenderBeadContextBackCompatPreserved` → **Bead 2** step 8
  (the existing `beadctx_test.go` golden assertions continue to
  pass unchanged because `RenderBeadContext` is preserved
  verbatim — `BuildBead` is a separate new entry point and the
  back-compat anchor is the existing test running unchanged).
- `TestContextPackRejectsNegativeBudget` → **Bead 3** step 3
  (CLI-level negative-flag rejection at flag-parse time).
- `cmd/mindspec/context.go` exposes `--max-tokens N` on the
  `mindspec context bead <id>` subcommand → **Bead 3** step 2.
- `ADR-0033-tokenizer-interface.md` exists with `Status:
  Accepted`, `Domain(s): context-system`, citing ADR-0030 and
  ADR-0032, recording the ±3% contract + determinism rules +
  six-tier ranking + must-tier-overflow failure mode → **Bead 3**
  step 4 (narrative Status edit; the body Decision section
  already records these per the stub at lines 36-68 and needs no
  edit at finalization).
- `.mindspec/docs/domains/context-system/OWNERSHIP.yaml`
  includes both `internal/contextpack/**` and
  `internal/tokenize/**` under `paths:` → **Bead 1** step 4
  (the OWNERSHIP.yaml update lands in the same bead that
  introduces the new package so the doc-sync gate from spec 086
  passes on that commit).
- All existing tests still pass; AST boundary lint stays green
  → enforced per-bead (every bead's verification ends with
  `go build ./... && go test -short ./...`); Bead 1 step 6
  additionally re-runs `TestEnforcementHasNoGitLeaks` explicitly
  because Bead 1 introduces the new `internal/tokenize/` package
  that the lint guards, and Bead 2 step 7 re-runs it because
  Bead 2 introduces the new `internal/contextpack/budgeter.go`
  file.
- `go build ./... && go test -short ./...` green on every commit
  → enforced as the final verification step of every bead.

## Bead 1 — `internal/tokenize/` package + `Tokenizer` interface + `Approx` impl + OWNERSHIP.yaml update

**Domain.** `context-system` (the new package and the OWNERSHIP.yaml
update both live under the `context-system` domain per the spec's
single-domain manifest).

**Depends on.** Nothing in this spec.

**Steps**

1. Create new directory `internal/tokenize/` at the repo root.
2. Create `internal/tokenize/tokenize.go` exporting the interface
   and the default implementation EXACTLY per spec Requirement 7:

   ```go
   // Package tokenize defines the Tokenizer interface used by
   // internal/contextpack to budget bead context bundles. The
   // default Approx implementation uses runes/3.7 (rounded down)
   // and is documented as accurate to +/-3% of a reference BPE
   // tokenizer on English+code text in the 500-2000 token range.
   // Callers MUST NOT depend on the precise rune-ratio constant;
   // a future BPE-backed Tokenizer may drop in as long as it
   // satisfies the same +/-3% contract on the reference corpus.
   //
   // This package has zero external dependencies beyond the
   // standard library (unicode/utf8 for Approx). It MUST NOT
   // import os/exec, internal/gitutil, or internal/executor —
   // the spec 085 boundary lint (TestEnforcementHasNoGitLeaks)
   // enforces this constraint.
   package tokenize

   import "unicode/utf8"

   // Tokenizer counts approximate tokens in a string per a
   // documented contract (see package doc). Pluggable; the
   // default implementation is Approx.
   type Tokenizer interface {
       Count(s string) int
       Name() string
   }

   // Approx is the default Tokenizer: runes/3.7 rounded down.
   // Accurate to +/-3% on English+code in the 500-2000 token
   // range. Name returns "approx".
   type Approx struct{}

   // Count returns int(float64(utf8.RuneCountInString(s)) / 3.7).
   func (Approx) Count(s string) int {
       return int(float64(utf8.RuneCountInString(s)) / 3.7)
   }

   // Name returns "approx".
   func (Approx) Name() string { return "approx" }
   ```

   The `Name()` return string is `"approx"` (NOT the stub ADR's
   `"approx-3.7"` — the spec Requirement 11 example shows
   `tokenizer: <tok.Name()>` and the spec 088 Acceptance Criteria
   match against the rendered Provenance block, so the contract
   is `"approx"`; ADR-0033's finalization narrative edit in
   Bead 3 step 4 may update the ADR's example string if desired
   but is NOT required because the example is documentary).

3. Create `internal/tokenize/approx_test.go` with the hand-counted
   reference fixture per the Testing Strategy block. The corpus
   is a Go string literal embedded inline; the reference token
   count is a `const referenceTokens = N` derived by hand-counting
   (counting rule documented in a comment block above the
   constant). The assertion is
   `diff := Approx{}.Count(referenceCorpus) - referenceTokens; if
   diff < 0 { diff = -diff }; tolerance :=
   int(math.Ceil(float64(referenceTokens) * 0.03)); if diff >
   tolerance { t.Fatalf(...) }`.

   The corpus SHOULD be approximately 100 tokens of English prose
   (NOT code, NOT non-ASCII) so the ±3% contract is exercised in
   the documented range. Pick prose that exercises punctuation
   boundaries (commas, periods, hyphens) without leaning on
   Unicode edge cases — those are covered by the rune-counting
   primitive (`utf8.RuneCountInString`) and not by the Approx
   ratio.

4. Update
   `.mindspec/docs/domains/context-system/OWNERSHIP.yaml` to ADD
   `internal/tokenize/**` under `paths:`:

   ```yaml
   paths:
     - internal/contextpack/**
     - internal/tokenize/**
   ```

   The doc-sync gate from spec 086 enforces this change at plan
   approval; the schema rejection of `viz/`/`agentmind/`/`bench/`
   first-segments (spec 086) continues to apply (neither path
   triggers it). The update lands in the SAME commit that creates
   the `internal/tokenize/` directory so the doc-sync gate passes
   on that commit (HC-5).

5. Add a forbidden-imports check. Two acceptable placements:
   (a) extend the existing `internal/lint/boundary_test.go` to
   also assert `internal/tokenize/...` has no `os/exec` /
   `internal/gitutil` / `internal/executor` transitive import,
   OR (b) add a new file `internal/tokenize/boundary_test.go`
   that uses `go/packages` (`packages.Load` with
   `packages.NeedDeps | packages.NeedImports`) to walk the
   transitive import set and fails on any forbidden import.
   Placement (a) is preferred (single lint surface) but (b) is
   acceptable if the spec-085 harness's test fixture loader does
   not yet support multi-package assertions — the implementer
   chooses based on the actual harness shape.

   Either way, the test name is `TestTokenizeNoForbiddenImports`
   to match the spec Acceptance Criterion verbatim.

6. Run the new tests and the boundary lint.

**Verification**

- [ ] `go test ./internal/tokenize -run
  TestTokenizerApproxToleranceWithinThreePercent -v` — PASS;
  log shows the actual `Approx{}.Count(corpus)` and the
  reference count, with `|diff| <= ceil(0.03 * N)`.
- [ ] `go test ./internal/tokenize -run
  TestTokenizeNoForbiddenImports -v` OR the equivalent test in
  `internal/lint/` per the placement chosen in step 5 — PASS.
- [ ] `go list -deps ./internal/tokenize/... | grep -E
  '(os/exec|internal/gitutil|internal/executor)'` — exits
  non-zero (no matches), proving the boundary at the
  command-line level.
- [ ] `go test ./internal/lint -run TestEnforcementHasNoGitLeaks
  -v` — PASS (the existing boundary lint still green after the
  new package is added to the tree).
- [ ] `git diff
  .mindspec/docs/domains/context-system/OWNERSHIP.yaml` — shows
  exactly ONE line addition (`- internal/tokenize/**`).
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `internal/tokenize/tokenize.go` exists with the `Tokenizer`
  interface and the `Approx` struct as specified.
- `TestTokenizerApproxToleranceWithinThreePercent` passes
  against the inline hand-counted fixture.
- `TestTokenizeNoForbiddenImports` passes; the new package
  transitively imports nothing under `os/exec`,
  `internal/gitutil`, or `internal/executor`.
- `.mindspec/docs/domains/context-system/OWNERSHIP.yaml`
  includes both `internal/contextpack/**` and
  `internal/tokenize/**` under `paths:`.
- The spec 085 boundary lint (`TestEnforcementHasNoGitLeaks`)
  still passes.
- All existing tests still pass (HC-3).

## Bead 2 — `BuildBead` + six-tier ranking + rune-aligned tail-shave + SHA provenance

**Domain.** `context-system` (the new `budgeter.go` lives in
`internal/contextpack/` which the spec's single impacted-domain
manifest claims).

**Depends on.** Bead 1 (imports `internal/tokenize` for the
`Tokenizer` interface and the `Approx` default implementation).

**Steps**

1. Read `internal/contextpack/beadctx.go` lines 1-94 and confirm
   the existing `RenderBeadContext` shape: the package-level test
   seam `beadShowFn = bead.RunBD` at line 12, the
   `SetBeadShowForTest` helper at lines 15-19, and the
   `beadShowEntry` struct at lines 21-29. `BuildBead` REUSES the
   `beadShowFn` seam for its bead JSON fetch — do NOT introduce a
   parallel seam.
2. Read `internal/contextpack/spec.go` lines 18-87 and confirm
   `ParseSpec(specDir string) (*SpecMeta, error)` returns a
   `*SpecMeta` whose `Domains` field is `[]string` (lower-cased
   at parse time). `BuildBead` uses this to enumerate domain
   docs for tier 5 (section 6 in Req-9 output order).
3. Read `internal/adr/show.go` lines 25+ and confirm
   `adr.Show(root, id) (*ADR, error)` is the API; the spec also
   mentions `adr.Store.Show(id)` via `adr.NewFileStore(root)`
   from `internal/adr/filestore.go:14`. EITHER surface is
   acceptable for tier 3 — the implementer chooses based on what
   the existing call sites in `internal/validate/plan.go` use
   today (the gate code from spec 087 uses
   `adr.NewFileStore(root)` + `store.Get(id)`). Use the same
   pattern in `budgeter.go` for consistency.
4. Create new file `internal/contextpack/budgeter.go` exporting:

   ```go
   // BuildBead emits a deterministic markdown bundle for the
   // given bead id whose estimated token count (per the supplied
   // Tokenizer) is <= maxTokens, or returns an error when the
   // must-tier alone exceeds the budget.
   //
   // Section order (fixed):
   //   1. # Bead Context: <Title>
   //   2. ## Bead          (must-tier; errors on overflow)
   //   3. ## Spec
   //   4. ## Cited ADRs    (verbatim ## Decision per cited ADR)
   //   5. ## Plan          (bead's section of plan.md)
   //   6. ## Domain Docs   (overview + interfaces per domain)
   //   7. ## File Paths    (only if file_paths non-empty)
   //   8. ## Provenance    (SHA-256 of every input artefact)
   //
   // Tail-shaving on tiers 2-6 is rune-aligned via
   // utf8.DecodeLastRuneInString; the truncation marker is the
   // constant string "[truncated]" (no size suffix — the constant
   // length is required for the shave to converge as a
   // fixed-point). The provenance reserve is dynamic: the
   // Provenance block is rendered first with a fixed-width
   // estimated_tokens placeholder, its token count becomes the
   // headroom for the budget check, and no second-pass re-render
   // is performed.
   //
   // BuildBead requires bead.metadata.spec_id; on missing it
   // returns (nil, error) containing "lacks metadata.spec_id".
   // No repo-root fallback scan is attempted — the filepath.Walk
   // seam (walkFn = filepath.Walk) records zero invocations on
   // this path, asserted by TestContextPackErrorOnMissingSpecID.
   func BuildBead(beadID string, maxTokens int, tok tokenize.Tokenizer) ([]byte, error)
   ```

   Introduce the test seam `var walkFn = filepath.Walk` at the
   top of `budgeter.go` plus a `SetWalkForTest` helper mirroring
   `SetBeadShowForTest`. `BuildBead` MUST NOT call
   `filepath.Walk` directly — all walks go through `walkFn`. The
   no-fallback-scan rule (spec Requirement 8) makes this seam a
   negative-recorder asset for
   `TestContextPackErrorOnMissingSpecID`.

   Constants in `budgeter.go`:

   ```go
   const truncationMarker = "[truncated]"
   ```

   The marker is exactly that string with NO size suffix per
   spec Requirement 10 and ADR-0033 sub-decision 2.

5. Implement `BuildBead` body. The algorithm (matching spec
   Requirements 8-11 verbatim):

   1. Fetch bead JSON via `beadShowFn("show", beadID, "--json")`.
      Parse the first entry as `beadShowEntry` (reuse the
      existing type from `beadctx.go`; do not introduce a
      parallel type).
   2. Extract `specID := e.Metadata["spec_id"]` as a string. On
      missing or non-string: return `nil, fmt.Errorf("bead JSON
      for %s lacks metadata.spec_id; cannot resolve spec",
      beadID)`. NO repo-root walk.
   3. Resolve spec dir as
      `filepath.Join(".mindspec/docs/specs", specID)`. Read
      spec.md, plan.md.
   4. Parse spec.md via `ParseSpec(specDir)` to get impacted
      domains. Sort them lexicographically.
   5. Resolve cited ADRs from the plan frontmatter (reuse the
      same parser the spec 087 gate uses — see
      `internal/validate/plan.go::parsePlanFrontmatter`; if that
      helper is not exported, use the same YAML approach via
      `gopkg.in/yaml.v3` against the frontmatter bytes). Sort by
      ADR id ascending. For each, load via
      `adr.NewFileStore(root).Show(id)` (or `adr.Show(root, id)`,
      per step 3) and extract the verbatim `## Decision`
      section.
   6. Resolve plan bead section via top-down scan: parse plan.md
      line-by-line; the first level-2 (or deeper) heading whose
      heading text contains `beadID` OR `e.Title` wins; capture
      lines until the next heading of the same or shallower
      level. If no heading matches, capture the entire body.
   7. Resolve domain docs: for each impacted domain (sorted),
      attempt `os.ReadFile(".mindspec/docs/domains/<domain>/overview.md")`
      and same for `interfaces.md`. On `os.IsNotExist`, emit a
      single comment-line warning into the output bundle and
      proceed (NOT an error per spec Requirement 8 last
      sub-bullet).
   8. Resolve `file_paths`: from `e.Metadata["file_paths"]`. If
      any entry's first path segment is in
      `{viz, agentmind, bench}`, return `nil, fmt.Errorf(
      "file_paths entry %q is under an excluded tree (viz/agentmind/bench)",
      path)`. Else sort ascending and read each via
      `os.ReadFile`.
   9. Compute SHA-256 over each input artefact's raw bytes (the
      bead JSON bytes, the spec.md bytes, the plan.md bytes,
      each ADR file's full bytes, each domain doc file's bytes,
      each file_paths file's bytes). Hex-encode lowercase.
   10. Render the Provenance block FIRST per spec Requirement
       11: tokenizer name, max_tokens, fixed-width
       estimated_tokens placeholder (width = number of decimal
       digits in `maxTokens`, or 6 when `maxTokens == 0`), then
       the sorted SHA lines. Capture
       `provReserve := tok.Count(renderedProvBlock)`.
   11. Render tier 1 (must-tier; section 2 `## Bead`): the
       bead's `Description + AcceptanceCriteria + Design` per
       spec Requirement 10. If `tok.Count(tier1Text) +
       provReserve > maxTokens` AND `maxTokens > 0`: return
       `nil, fmt.Errorf("bead context exceeds --max-tokens %d;
       raise budget or split bead", maxTokens)`.
   12. Render tiers 2-6 (sections 3-7) in Req-9 order. For each
       tier, after appending its candidate content, if
       `tok.Count(everythingSoFar) + provReserve > maxTokens`
       AND `maxTokens > 0`: tail-shave THIS tier's
       most-recently-appended content until the inequality
       holds. Tail-shave algorithm (per spec Requirement 10):

       ```
       for tok.Count(everythingSoFar) + provReserve > maxTokens:
           candidate := lastTierBytes truncated to some shorter length
           // back up to nearest valid rune boundary
           for !utf8.ValidString(candidate) ||
               (lastRune, size) := utf8.DecodeLastRuneInString(candidate);
               lastRune == utf8.RuneError && size == 1:
               candidate = candidate[:len(candidate)-1]
           candidate += truncationMarker
           replace lastTierBytes with candidate
       ```

       The marker has constant length so this loop is a
       convergent fixed-point — the count after appending the
       marker is monotonically non-increasing in the input
       length, and the marker itself contributes a fixed
       additive token cost that the shave accounts for.

       Within tier 5 (`## Domain Docs`) the shave is PER FILE
       (per `overview.md` / `interfaces.md`), not per domain;
       within tier 6 (`## File Paths`) the shave is PER FILE.

   13. Append the rendered Provenance block.
   14. Compute the FINAL `estimatedTokens := tok.Count(output
       MINUS the Provenance block bytes)` per spec Requirement
       11 ("`estimated_tokens` reflects the body only, no
       chicken-and-egg") and patch the placeholder. Because the
       placeholder is fixed-width and the actual value is at
       most that width, the patch is a literal slice overwrite
       — it does NOT shift the block's byte length and
       therefore does NOT invalidate the `provReserve` sized in
       step 10.
   15. Return the rendered bytes and `nil`.

   `maxTokens == 0` semantics: skip the budget enforcement at
   steps 11 and 12 entirely (no overflow error, no shave). The
   Provenance block is still computed and emitted (deterministic
   by Req-2). This matches spec HC-2.

6. Add a `renderProvBlock` package-internal helper used both by
   step 10 (during normal generation) AND by
   `TestContextPackProvenanceReserveIsDynamic` (via a
   `_test.go`-only accessor function in
   `internal/contextpack/budgeter_test.go` such as
   `func renderProvBlockForTest(...) string { return
   renderProvBlock(...) }`). This avoids exporting the helper in
   the public API while keeping the test deterministic.

7. Add new tests to `internal/contextpack/budgeter_test.go` per
   the Testing Strategy block:
   - `TestContextPackDeterministic`
   - `TestContextPackBudget`
   - `TestContextPackTruncationMarker`
   - `TestContextPackErrorOnMustTierOverflow`
   - `TestContextPackErrorOnMissingSpecID`
   - `TestProvenanceBlockContainsInputSHA`
   - `TestContextPackProvenanceReserveIsDynamic`
   - `TestContextPackSectionOrder`
   - `TestContextPackRejectsExcludedFilePath`

   Each test uses `t.TempDir()` to build a fixture repo
   skeleton (`.mindspec/docs/specs/<id>/spec.md`, `plan.md`,
   `.mindspec/docs/adr/ADR-XXXX-*.md`,
   `.mindspec/docs/domains/<name>/overview.md`,
   `.mindspec/docs/domains/<name>/interfaces.md`,
   `.mindspec/docs/domains/<name>/OWNERSHIP.yaml`) from string
   literals, swaps `beadShowFn` via
   `contextpack.SetBeadShowForTest` to return a fixture
   `bd show` payload, swaps `walkFn` via the new
   `SetWalkForTest` to a recorder (for the missing-spec-id
   case), and asserts on the returned bytes + error.

8. Add a doc-comment to `RenderBeadContext` (the existing
   function in `beadctx.go`) marking it as deprecated and
   pointing new callers at `BuildBead`. Per spec Requirement
   14, the body of `RenderBeadContext` is UNCHANGED — only the
   doc-comment is added. Example:

   ```go
   // RenderBeadContext renders a markdown context bundle for
   // the given bead. The output layout is the legacy
   // (## Acceptance Criteria, ## Work Chunk, ## Key File Paths)
   // shape preserved for back-compat. New code SHOULD call
   // BuildBead instead, which emits the spec 088 budgeted
   // layout (## Bead, ## Spec, ## Cited ADRs, ## Plan,
   // ## Domain Docs, ## File Paths, ## Provenance) and
   // supports --max-tokens budgeting.
   //
   // Deprecated: use BuildBead.
   ```

   The existing `internal/contextpack/beadctx_test.go` golden
   assertions continue to pass unchanged — this is the
   `TestRenderBeadContextBackCompatPreserved` anchor for the
   Provenance block.

**Verification**

- [ ] `go test ./internal/contextpack -run
  'TestContextPackDeterministic|TestContextPackBudget|TestContextPackTruncationMarker|TestContextPackErrorOnMustTierOverflow|TestContextPackErrorOnMissingSpecID|TestProvenanceBlockContainsInputSHA|TestContextPackProvenanceReserveIsDynamic|TestContextPackSectionOrder|TestContextPackRejectsExcludedFilePath'
  -v` — all PASS; the determinism test logs identical
  `sha256.Sum256` across two runs; the budget test logs
  `tok.Count(output) <= 2000`; the truncation test logs
  `utf8.ValidString(output) == true`; the must-tier-overflow
  test logs an error containing `"bead context exceeds
  --max-tokens 100"`; the missing-spec-id test logs an error
  containing `"lacks metadata.spec_id"` AND the walk recorder
  shows zero invocations; the provenance-reserve test logs the
  two reserve sizes differing by >50 tokens.
- [ ] `go test ./internal/contextpack -v` — all tests PASS,
  zero regressions vs `main` (the existing
  `beadctx_test.go` golden assertions still green —
  `TestRenderBeadContextBackCompatPreserved` anchor).
- [ ] `go test ./internal/lint -run TestEnforcementHasNoGitLeaks
  -v` — PASS (new `budgeter.go` does not regress 085's
  boundary).
- [ ] `grep -nE 'os/exec|internal/gitutil|exec\.Command'
  internal/contextpack/budgeter.go` — exits 1 (no matches).
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `internal/contextpack/budgeter.go` exports `BuildBead(beadID
  string, maxTokens int, tok tokenize.Tokenizer) ([]byte,
  error)` per spec Requirement 8.
- `TestContextPackDeterministic` passes; two runs produce
  byte-identical output AND identical `sha256.Sum256`.
- `TestContextPackBudget` passes;
  `tokenize.Approx{}.Count(output) <= 2000` AND the `## Bead`
  must-tier section is present in full.
- `TestContextPackTruncationMarker` passes; the literal
  `[truncated]` marker appears in the domain-docs tier and
  `utf8.ValidString(output)` is true (rune-aligned shave).
- `TestContextPackErrorOnMustTierOverflow` passes; the error
  message contains `"bead context exceeds --max-tokens"` AND
  the literal budget value; the returned `[]byte` is `nil`.
- `TestContextPackErrorOnMissingSpecID` passes; the error
  message contains `"lacks metadata.spec_id"`; the
  `filepath.Walk` recorder records ZERO invocations under
  `.mindspec/docs/specs/`.
- `TestProvenanceBlockContainsInputSHA` passes; every input
  artefact has a `sha256:` line that is a valid 64-character
  lowercase hex string.
- `TestContextPackProvenanceReserveIsDynamic` passes; the
  small-input and large-input Provenance token counts differ
  by more than 50 tokens AND both bundles satisfy
  `tok.Count(output) <= maxTokens`.
- `TestContextPackSectionOrder` passes; the level-2 headings
  appear in the exact Req-9 order with no interleaving and no
  duplicates.
- `TestContextPackRejectsExcludedFilePath` passes; a
  `file_paths` entry under `viz/`, `agentmind/`, or `bench/`
  causes an error containing `"excluded tree"`.
- `RenderBeadContext` is marked `// Deprecated:` and its body
  is unchanged; the existing `beadctx_test.go` golden
  assertions still pass.
- The new `budgeter.go` does NOT import `os/exec`,
  `internal/gitutil`, or `internal/executor`; the spec 085
  boundary lint stays green.
- All existing `internal/contextpack` tests still pass (HC-3).

## Bead 3 — CLI flag `--max-tokens` + ADR-0033 finalization

**Domain.** `context-system` (this bead's CLI plumbing is pure
flag-wiring with no behavioural change to the `execution` domain —
see spec lines 107-118 — so the impacted domain remains the spec's
single declared `context-system`; the ADR-0033 narrative edit also
lives under `context-system` per ADR-0033's own `Domain(s)` field).

**Depends on.** Bead 2 (the CLI flag calls `BuildBead`).

**Steps**

1. Read `cmd/mindspec/context.go` lines 1-37 and confirm the
   existing `contextBeadCmd` cobra command shape: `Use: "bead
   <bead-id>"`, `Args: cobra.ExactArgs(1)`, `RunE` invokes
   `contextpack.RenderBeadContext(beadID)`. The bead reuses the
   existing command and ADDS a `--max-tokens` flag plus a branch
   on its value.

2. Extend `cmd/mindspec/context.go`. Add the flag registration in
   `init()`:

   ```go
   func init() {
       contextBeadCmd.Flags().Int("max-tokens", 0,
           "Budget for the rendered bundle in approx tokens (0 = unbudgeted, preserves legacy output)")
       contextCmd.AddCommand(contextBeadCmd)
   }
   ```

   Extend `RunE` to parse and validate the flag, then branch:

   ```go
   RunE: func(cmd *cobra.Command, args []string) error {
       beadID := args[0]
       maxTokens, _ := cmd.Flags().GetInt("max-tokens")
       if maxTokens < 0 {
           return fmt.Errorf("--max-tokens must be >= 0")
       }
       if cmd.Flags().Changed("max-tokens") {
           // New code path: spec 088 budgeted bundle via BuildBead.
           out, err := contextpack.BuildBead(beadID, maxTokens, tokenize.Approx{})
           if err != nil {
               return fmt.Errorf("building bead context: %w", err)
           }
           fmt.Print(string(out))
           return nil
       }
       // Legacy code path: preserve byte-identical output for callers
       // that never pass --max-tokens (HC-2 solo-developer UX).
       rendered, err := contextpack.RenderBeadContext(beadID)
       if err != nil {
           return fmt.Errorf("rendering bead context: %w", err)
       }
       fmt.Print(rendered)
       return nil
   },
   ```

   **Branch-on-Changed vs branch-on-zero.** The spec
   Requirement 13 says default `0` is "unbudgeted" and preserves
   "the existing zero-budget behaviour". The legacy
   `RenderBeadContext` path is preserved verbatim per Req 14 and
   its golden tests assert byte-identical output, so the safest
   way to honour BOTH contracts is: when `--max-tokens` was NOT
   passed at all (`cmd.Flags().Changed("max-tokens") == false`),
   take the legacy path; when `--max-tokens 0` WAS passed
   explicitly, take the new `BuildBead` path (which honours
   `maxTokens == 0` as unbudgeted per spec Req 10 last bullet —
   the new layout is emitted, but no truncation). This gives the
   operator an explicit opt-in to the new layout via
   `--max-tokens 0` while keeping the existing golden assertions
   green by default.

   Import `tokenize` at the top of `context.go`:
   `"github.com/mrmaxsteel/mindspec/internal/tokenize"`.

3. Create new test file `cmd/mindspec/context_test.go` (or
   extend an existing file in `cmd/mindspec/`) with
   `TestContextPackRejectsNegativeBudget` per the Testing
   Strategy block. The test invokes the cobra command tree with
   `--max-tokens -1` (using the existing `cobra` testing
   pattern visible in `cmd/mindspec/complete_test.go` and
   `cmd/mindspec/impl_test.go` — set `rootCmd.SetArgs([]string{
   "context", "bead", "test-bead", "--max-tokens", "-1"})` and
   call `rootCmd.Execute()`; assert the returned error contains
   `"--max-tokens must be >= 0"`). Install a `beadShowFn` stub
   via `contextpack.SetBeadShowForTest` returning an empty
   array to avoid invoking real `bd`. The negative-flag check
   fires BEFORE the `beadShowFn` call so the stub return value
   is irrelevant; the test still installs it as a defensive
   guard.

4. Finalize ADR-0033. Edit
   `.mindspec/docs/adr/ADR-0033-tokenizer-interface.md` lines
   13-16 ("## Status" section). Replace the placeholder
   paragraph ("Stub created during spec
   088-context-pack-budgeter drafting. Finalized in spec 088
   Bead N alongside the budgeter + Tokenizer implementation.")
   with: "Finalized in spec 088 Bead 3 alongside the
   `internal/tokenize/` package (Bead 1), the
   `internal/contextpack/budgeter.go::BuildBead` entry point
   (Bead 2), and the `--max-tokens` flag in
   `cmd/mindspec/context.go` (this bead)." The
   `**Status**: Accepted` frontmatter field (line 4) and the
   `**Domain(s)**: context-system` field (line 5) are already
   set and require NO change.

   The Decision section (lines 36-68) already records the
   six-tier ranking + the rune-aligned tail-shave + the
   constant-length `[truncated]` marker + the dynamic
   `provReserve` algorithm + the SHA-256 provenance block + the
   `bead.metadata.spec_id` no-fallback-scan rule per spec
   Requirements 7-11. No Decision-section edit is required at
   finalization. Verify via `git diff` that the only mutation
   in this commit is the step-4 "Status" paragraph narrative
   edit.

5. Run the test diff vs `main` and record the test-name +
   status delta in the final commit message of this bead (HC-3
   enforcement; no pre-existing test SKIPPED or REMOVED — only
   ADDITIONS: `TestTokenizerApproxToleranceWithinThreePercent`,
   `TestTokenizeNoForbiddenImports`,
   `TestContextPackDeterministic`, `TestContextPackBudget`,
   `TestContextPackTruncationMarker`,
   `TestContextPackErrorOnMustTierOverflow`,
   `TestContextPackErrorOnMissingSpecID`,
   `TestProvenanceBlockContainsInputSHA`,
   `TestContextPackProvenanceReserveIsDynamic`,
   `TestContextPackSectionOrder`,
   `TestContextPackRejectsExcludedFilePath`,
   `TestContextPackRejectsNegativeBudget`).

**Verification**

- [ ] `go test ./cmd/mindspec -run
  TestContextPackRejectsNegativeBudget -v` — PASS; error
  contains `"--max-tokens must be >= 0"`.
- [ ] `git diff
  .mindspec/docs/adr/ADR-0033-tokenizer-interface.md` — shows
  exactly ONE narrative-text edit from step 4 (the "Status"
  paragraph). The `**Status**`, `**Domain(s)**`, and Decision
  sections are UNCHANGED.
- [ ] Manual smoke (recorded in commit message, not in CI):
  `mindspec context bead <id>` (no `--max-tokens`) — exit 0,
  legacy layout (existing `## Acceptance Criteria` / `## Work
  Chunk` / `## Key File Paths` shape).
  `mindspec context bead <id> --max-tokens 2000` — exit 0, new
  budgeted layout (`## Bead` / `## Spec` / `## Cited ADRs` /
  `## Plan` / `## Domain Docs` / [`## File Paths`] /
  `## Provenance`); re-run identical command, `diff` the two
  outputs, exit 0.
  `mindspec context bead <id> --max-tokens 100` against a bead
  with a large `Design` — exit non-zero, stderr contains
  `"bead context exceeds --max-tokens 100"`.
  `mindspec context bead <id> --max-tokens -1` — exit non-zero,
  stderr contains `"--max-tokens must be >= 0"`.
- [ ] Full spec-088 test sweep:
  `go test ./internal/tokenize ./internal/contextpack
  ./cmd/mindspec -run
  'TestTokenizerApproxToleranceWithinThreePercent|TestTokenizeNoForbiddenImports|TestContextPackDeterministic|TestContextPackBudget|TestContextPackTruncationMarker|TestContextPackErrorOnMustTierOverflow|TestContextPackErrorOnMissingSpecID|TestProvenanceBlockContainsInputSHA|TestContextPackProvenanceReserveIsDynamic|TestContextPackSectionOrder|TestContextPackRejectsExcludedFilePath|TestContextPackRejectsNegativeBudget'
  -v` — all PASS.
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `cmd/mindspec/context.go` registers `--max-tokens N` on the
  `mindspec context bead <id>` subcommand with default `0` and
  the `"--max-tokens must be >= 0"` rejection at flag-parse
  time.
- `TestContextPackRejectsNegativeBudget` passes; invoking
  `mindspec context bead <id> --max-tokens -1` returns an
  error containing `"--max-tokens must be >= 0"`.
- When `--max-tokens` is passed (any non-negative value
  including 0), the command invokes
  `contextpack.BuildBead(beadID, maxTokens, tokenize.Approx{})`
  and emits the new spec-088 layout. When `--max-tokens` is
  NOT passed, the command invokes the legacy
  `contextpack.RenderBeadContext(beadID)` to preserve the
  existing golden assertions (HC-2 / HC-3).
- `ADR-0033-tokenizer-interface.md` "## Status" section
  narrative paragraph (line ~15) reads as edited in step 4
  above (no "Bead N" placeholder).
- ADR-0033's `**Status**: Accepted` and `**Domain(s)**:
  context-system` fields remain UNCHANGED; the spec 087
  plan-time `checkADRCitations` + `checkADRCoverage` gate
  passes against this spec's single impacted domain.
- All pre-existing tests still pass (HC-3); no test is
  SKIPPED, EXCLUDED, or REMOVED relative to `main`. The only
  CHANGE to existing code outside the new files is the
  `// Deprecated:` doc-comment on `RenderBeadContext` (Bead 2
  step 8) and the `--max-tokens` flag wiring in
  `cmd/mindspec/context.go` (this bead's step 2).
