---
adr_citations:
    - id: ADR-0030
    - id: ADR-0031
    - id: ADR-0032
approved_at: "2026-05-20T23:32:06Z"
approved_by: user
bead_ids:
    - mindspec-zy4u.1
    - mindspec-zy4u.2
    - mindspec-zy4u.3
    - mindspec-zy4u.4
spec_id: 087-adr-semantic-gate
status: Approved
version: "1"
---
# Plan: 087-adr-semantic-gate

## ADR Fitness

- **ADR-0032** (new — "Semantic ADR Coverage Gates with Override and
  Supersede Flags"): the stub at
  `.mindspec/docs/adr/ADR-0032-adr-semantic-gates.md` already carries
  `Status: Accepted` (line 4) and records four sub-decisions: (1) the
  canonical domain identifier is the `OWNERSHIP.yaml` directory name
  with case-folded, trim-whitespace, exact set-intersection
  comparison (no aliases / hierarchy in v1); (2) the plan-time gate
  extends `checkADRCitations` with a cite-relevant intersection check
  AND adds a `checkADRCoverage` helper requiring at least one cited
  Accepted ADR per impacted domain; (3) the bead-time gate is
  `validate.ValidateDivergence` (new file) computing
  `exec.ChangedFiles(base, head)`, mapping paths to domains via
  `attributeDomain` from spec 086, and erroring on uncovered or
  unowned files — `approve impl` runs the same check as a backstop
  with broader scope; (4) `--override-adr "<reason>"` and
  `--supersede-adr ADR-NNNN` both BYPASS the gate, write to DISTINCT
  audit metadata namespaces (`mindspec_adr_override_*` vs
  `mindspec_adr_supersede_*`), and `--supersede-adr` additionally
  pre-creates a placeholder ADR with `Status: Proposed` via the new
  `adr.CreateWithID` helper (revision 1, Bead 3 step 3a) using the
  user-supplied id verbatim. The narrative "Status" paragraph
  (lines 13-16) still reads "Stub created during spec
  087-adr-semantic-gate drafting. Finalized in spec 087 Bead N…" —
  Bead 4 step 1 replaces that paragraph with "Finalized in spec
  087 Bead 4 alongside the semantic-gate implementation in
  `internal/validate/{plan,divergence,adr_divergence}.go`, the
  override/supersede flag plumbing in `cmd/mindspec/`, and the
  metadata-seam wiring in `internal/{complete,approve}/`." The
  ADR's authored `**Domain(s)**: validation, adr, lifecycle` line
  is GRANDFATHERED and NOT retagged — see revision 4 below and
  Bead 4 step 2.

  **Canonical-tag grandfathering (revision 4).** Requirement 16's
  canonical-tag-shape rule (`OWNERSHIP.yaml` directory name as the
  domain identifier) applies PROSPECTIVELY to new specs and new
  ADRs only. ADR-0030 ("Executor…"), ADR-0031 ("Doc-Sync…"), and
  ADR-0032 ("Semantic ADR Coverage…") retain their authored
  free-form domain tags (`execution`, `lifecycle`, `validation`,
  `adr`, `doc-sync`, etc.) and are explicitly grandfathered. The
  plan-time `intersectFold` check in Bead 1 step 3 SHOULD continue
  to use whatever tags those legacy ADRs already declare; new
  specs that need to cite a grandfathered ADR for coverage MAY
  declare matching legacy tags in their `## Impacted Domains`
  section to satisfy the gate, or MAY add a canonical tag to the
  ADR via a dedicated 1-line follow-up bead (NOT in this spec).
  Bead 4 in this plan therefore does NOT mutate
  `ADR-0032 **Domain(s)**`.
- **ADR-0030** ("Executor as the Git/Process I/O Boundary"):
  prerequisite. F1's diff input flows through
  `executor.Executor.ChangedFiles` and `executor.Executor.MergeBase`
  per ADR-0030's boundary doctrine. The new
  `internal/validate/divergence.go` MUST NOT import `os/exec` or
  `internal/gitutil`, and MUST NOT call `exec.Command("git", ...)`
  or `exec.Command("bd", ...)`. The
  `internal/lint/boundary_test.go::TestEnforcementHasNoGitLeaks`
  invariant from spec 085 Bead 4 continues to hold: this plan adds
  NO new `os/exec` or `internal/gitutil` imports to any of
  `internal/{validate, approve, complete}`. No contradiction with
  ADR-0030.
- **ADR-0031** ("Doc-Sync as an Enforcement Gate with Per-Domain
  OWNERSHIP.yaml"): sibling and direct precedent. F1 follows
  ADR-0031's three discipline points VERBATIM:
  (a) gate runs BEFORE every mutating/terminal operation;
  (b) explicit recorded override flag (mirrors `--allow-doc-skew`);
  (c) override metadata writes happen AFTER terminal mutation
  success via the existing test seams (`completeMergeMetadataFn`,
  `implMergeMetadataFn`). F1 also reuses the `OWNERSHIP.yaml`
  machinery ADR-0031 introduced (`loadOwnership`,
  `attributeDomain` in `internal/validate/ownership.go`). ADR-0032
  cites ADR-0031 as immediate precedent and explicitly notes
  `--override-adr`, `--supersede-adr`, and `--allow-doc-skew` are
  INDEPENDENT overrides (any combination may co-exist on a single
  invocation EXCEPT `--override-adr` and `--supersede-adr` which
  are the only mutually exclusive pair).
- **ADR number reservation.** At plan-draft time the highest
  existing ADR is `ADR-0032-adr-semantic-gates.md` (the stub this
  spec finalizes), so no renumber is needed. If a sibling spec
  lands claiming `0032` first between plan-draft and impl, Bead 4
  step 1 renumbers to the next free integer (`git mv` the file,
  update this plan's `adr_citations` frontmatter, the spec.md
  Background / ADR Touchpoints / Acceptance Criteria sections, and
  any test that cites the ADR number) as a 1-bead followup before
  merge.

## Testing Strategy

This spec's failure mode is **silent ADR drift**: a spec cites
unrelated ADRs (or none) for the domains it actually touches; a
bead's diff slips a `internal/core/foo.go` edit past the gate while
the plan only cites execution-domain ADRs; an `approve impl`
finalize succeeds without ever noticing the spec branch grew a
core/ touch the plan never sanctioned. The defense is mechanical
exit-code enforcement at THREE points: plan validation,
`complete.Run`, and `ApproveImpl`. Both lifecycle commands MUST
return a non-nil error on divergence unless the operator passed
`--override-adr "<reason>"` (one-shot pass-through) or
`--supersede-adr ADR-NNNN` (one-shot pass-through PLUS pre-creates a
placeholder ADR with `Status: Proposed`), in which case the
respective audit metadata is recorded under DISTINCT namespaces
with a UTC RFC3339 timestamp and best-effort actor identity AFTER
the terminal mutation succeeds.

**Bead ordering note.** Bead 1 (plan-time gates: extend
`checkADRCitations` with cite-relevant + add `checkADRCoverage` +
implement superseded-chain walker with cycle detection) lands
FIRST because Bead 2's `ValidateDivergence` reuses the
case-folded set-intersection helper and the superseded-chain
walker. Bead 2 (`ValidateDivergence` new file + fill
`CheckADRDivergence` body + widen signature) depends on Bead 1
for the chain-walker and coverage-check helpers. Bead 3 (CLI
flags + override/supersede flow + audit metadata) depends on
BOTH Beads 1 and 2 — the override path needs both gates wired
to have anything to skip, and the supersede path needs the
structured `[]DivergenceFinding` slice (revision 2) from
Bead 2's `CheckADRDivergence` to seed the new ADR's `Domains`
field via `findings[i].Domain` (NO `fmt.Sprintf` parsing of
Issue messages). Bead 4 (ADR-0032 finalization + AST call-order
test confirmation) depends on Bead 3 and is the smallest bead;
it MAY be folded into Bead 3 if the implementer prefers a
3-bead decomposition, but is kept separate here to isolate the
"narrative-only ADR text replacement" step from code changes.

Per the converged plan's HC-5 the per-commit gate is CI; locally
each bead's verification block ends with the exact command pair
`go build ./... && go test -short ./...` passing. HC-3 (existing
tests preserved, no skips relative to `main`) is enforced
per-bead: Bead 3 step 9 records a `go test -v ./...` test-name +
status diff vs `main` in its final commit message.

**New test additions across the four beads:**

- **Bead 1** (`internal/validate/plan_test.go` — extend existing):
  - `TestPlanRejectsIrrelevantADRCitation` — fixture spec with
    `## Impacted Domains` parsing to `["core"]` and an
    `adr_citations` list citing an ADR whose frontmatter
    `**Domain(s)**: execution` does NOT overlap. Assert
    `ValidatePlan` returns a `*Result` whose `Issues` contains an
    entry where `issue.Name == "adr-cite-irrelevant"` and
    `issue.Severity == validate.SevError` and `issue.Message`
    names the ADR id, its declared `Domains` slice, and the
    spec's parsed impacted-domains slice. Positive companion
    case: an ADR with `**Domain(s)**: core, execution` and the
    same `["core"]` spec passes (no irrelevant-cite Issue
    emitted).
  - `TestPlanRejectsUncoveredDomain` — fixture spec with
    `## Impacted Domains` parsing to `["core", "execution"]`
    where the plan cites only an Accepted ADR with
    `**Domain(s)**: execution`. Assert `ValidatePlan` returns a
    `*Result` whose `Issues` contains an entry where
    `issue.Name == "adr-coverage-missing"` and
    `issue.Severity == validate.SevError` and `issue.Message`
    contains the hint string `mindspec adr create --domain core`.
    Positive companion case: same spec citing TWO Accepted ADRs
    whose `Domains` together cover both `core` and `execution`
    passes (no `adr-coverage-missing` Issue).
  - `TestSupersededADRDoesNotSatisfyCoverage` — fixture plan
    citing ONLY an ADR with `Status: Superseded` (whose
    `SupersededBy` ADR exists, has `Status: Accepted`, and whose
    `Domains` would cover the impacted domain — but is NOT itself
    cited). Assert `checkADRCoverage` emits
    `"adr-coverage-missing"`. Companion case: same plan with the
    superseding chain head ALSO cited passes (no
    `adr-coverage-missing` for that domain).
  - `TestSupersedeChainCycleDetected` — fixture three ADRs whose
    `SupersededBy` links form a cycle (A → B → C → A); cite A.
    Assert `checkADRCoverage` emits `"adr-supersede-cycle"`
    naming the starting ADR id, and the walker terminates (no
    infinite loop — bounded by the 10-element visited set).
  - `TestSupersedeChainTooLong` — fixture 11 ADRs in a linear
    `SupersededBy` chain; cite the head. Assert `checkADRCoverage`
    emits `"adr-supersede-chain-too-long"` naming the starting
    ADR id and noting max length 10, and the walker terminates.

- **Bead 2** (`internal/validate/divergence_test.go` — new file,
  and `internal/complete/complete_test.go` — extend existing):
  - `TestCompleteRejectsUndeclaredDomainTouch` (in
    `complete_test.go`) — `complete.Run` with a `MockExecutor`
    returning a `ChangedFiles` result containing
    `internal/core/foo.go`, against a fixture plan whose cited
    ADRs have only `**Domain(s)**: execution`. Assert
    `complete.Run` returns a non-nil error whose `.Error()`
    contains `"adr-divergence-uncovered"`, names
    `internal/core/foo.go`, names the resolved manifest path
    (`.mindspec/docs/domains/core/OWNERSHIP.yaml`) or the
    `<fallback: internal/core/**>` marker, and names the
    uncovered domain `"core"`. The bead is NOT closed —
    asserted via swap-and-restore of the package-level
    `closeBeadFn` variable at `complete.go:23` (no invocation
    recorded); `exec.CompleteBead` is NOT invoked (mock
    recorder).
  - `TestUnownedFileRejected` (in `divergence_test.go`) —
    `ValidateDivergence` called with a `ChangedFiles` result
    containing `internal/some-new-dir/foo.go` (a path no
    `OWNERSHIP.yaml` in the spec's impacted-domains set claims).
    Assert the returned `*Result` contains an Issue where
    `issue.Name == "adr-divergence-unowned"` and `issue.Message`
    names the file plus the impacted-domains set the walker
    consulted.
  - `TestVizAgentmindBenchFiltered` (in `divergence_test.go`)
    — `ValidateDivergence` called with a `ChangedFiles` result
    containing `viz/foo.go`, `agentmind/bar.go`, and
    `bench/baz.go` (and no other files). Assert the returned
    `*Result.HasFailures()` is false because all three files
    are filtered out BEFORE `attributeDomain` is called.
    Companion case: same input plus `internal/core/x.go`
    triggers a single `adr-divergence-uncovered` Issue for the
    core file only.
  - `TestCheckADRDivergenceReturnsPopulated` (in
    `divergence_test.go`) — REPLACES the spec-086 stub
    `TestCheckADRDivergenceReturnsEmpty` (revision 10).
    `CheckADRDivergence` invoked against a real divergence
    fixture (an ADR plan citing only execution-domain ADRs +
    a `MockExecutor` `ChangedFiles` containing
    `internal/core/foo.go`) returns a `*Result` with
    `HasFailures() == true` and a `[]DivergenceFinding` of
    length >= 1 whose first element has `Domain == "core"`,
    `Path == "internal/core/foo.go"`, `Kind == "uncovered"`.
    The old stub test (asserting an empty Result on a
    placeholder body) is removed in the same commit; the
    commit message documents the rename for HC-3 traceability.
  - `TestDivergenceFindingsSeededFromStructuredField` (in
    `divergence_test.go`) — asserts the supersede flow's
    domain-capture seam: when `ValidateDivergence` returns
    `findings`, the FIRST `findings[i].Domain` (where
    `i` selects the first `Kind == "uncovered"` finding) is
    the structured value the supersede flow consumes — NO
    parsing of `*Result.Issues[i].Message` is performed.
    Asserted by a direct call: build a fixture violation, call
    `ValidateDivergence`, assert `findings[0].Domain ==
    "core"` and `findings[0].Kind == "uncovered"`.

- **Bead 3** (`internal/complete/complete_test.go` and
  `internal/approve/impl_test.go` and
  `cmd/mindspec/complete_test.go` — extend existing):
  - `TestOverrideUnblocks` (in `complete_test.go`) — same
    fixture as `TestCompleteRejectsUndeclaredDomainTouch` but
    with `opts.OverrideADR = "wip — core ADR coming in
    followup"`. Assert `complete.Run` returns success; the
    bead's metadata (asserted via the stub
    `completeMergeMetadataFn` recorder introduced by Bead 3
    step 5) contains `mindspec_adr_override_reason` (=
    verbatim reason), `mindspec_adr_override_at` (parseable
    RFC3339), and `mindspec_adr_override_by` (non-empty).
    The test also asserts WRITE ORDER (mirroring spec 086
    panel CONSENSUS revision 4): the recorder records call
    order and asserts the metadata-write seam was called
    STRICTLY AFTER `exec.CompleteBead` returned nil. A
    failing-`CompleteBead` sub-case asserts NO metadata-write
    invocation occurred.
  - `TestSupersedeUnblocks` (in `complete_test.go`) — same
    fixture with `opts.SupersedeADR = "ADR-0099"` (and
    `ADR-0099` not yet existing on disk). Assert
    `complete.Run` returns success; the new ADR file MUST
    exist on disk at `.mindspec/docs/adr/ADR-0099-*.md` with
    parseable frontmatter showing `Status: Proposed` and
    `**Domain(s)**` containing the previously-violated domain
    (`core`); the bead's metadata contains
    `mindspec_adr_supersede_id = "ADR-0099"`,
    `mindspec_adr_supersede_reason` containing the substring
    `"ADR-0099"`, `mindspec_adr_supersede_at` (parseable
    RFC3339), and `mindspec_adr_supersede_by` (non-empty).
    Asserts the placeholder ADR is created BEFORE the
    gate-skip path runs (the file must exist on disk even on
    a downstream `CompleteBead` failure — see Bead 3 step 4
    for the ordering rule).
  - `TestOverrideMetadataGoesThroughSeam` (in
    `complete_test.go`) — with the metadata seam swapped for a
    recording stub, the override write is captured by the
    stub (NOT by direct `bead.MergeMetadata`). This asserts
    the seam introduced by Bead 3 step 5 is the ONLY write
    path. Mirror test in `impl_test.go` asserts the same for
    `implMergeMetadataFn`.
  - `TestApproveImplBackstopRunsDivergence` (in
    `impl_test.go`) — `ApproveImpl` with a `MockExecutor`
    whose `ChangedFiles(base="main", head="spec/087-...")`
    returns a list containing `internal/core/foo.go` while
    the plan cites only execution-domain ADRs. Assert
    `ApproveImpl` returns a non-nil error containing
    `"adr-divergence-uncovered"`; `implRunBDCombinedFn` is
    NOT invoked; `bead.MergeMetadata` for
    `mindspec_phase: done` is NOT invoked;
    `exec.FinalizeEpic` is NOT invoked (mock recorder).
    Companion: same scenario with
    `opts.OverrideADR = "<reason>"` succeeds and writes the
    three `mindspec_adr_override_*` keys to the EPIC's
    metadata AFTER `exec.FinalizeEpic` returned nil.
  - `TestOverrideAndSupersedeMutuallyExclusive` (in
    `cmd/mindspec/complete_test.go`) — invoking
    `mindspec complete <bead> --override-adr "x"
    --supersede-adr ADR-0099` returns an error containing
    `"mutually exclusive"`. Mirror in `impl_test.go` for
    `mindspec approve impl`.
  - `TestOverrideEmptyReasonRejected` (in
    `cmd/mindspec/complete_test.go`) — invoking
    `--override-adr ""` returns an error containing
    `"requires a non-empty reason"`. Mirror in
    `impl_test.go`.

- **Bead 4** (no new tests):
  - The existing `TestApproveImplCallOrder` test in
    `internal/approve/impl_test.go` (from spec 086 panel
    CONSENSUS revision 9, which asserts the symbol-call ORDER
    of doc-sync → adr-divergence → epic-close → phase-metadata
    → commit-count → FinalizeEpic) MUST continue to pass. The
    spec-086 AST harness asserts the symbol NAME and call
    ORDER, not the argument list, so the signature widening
    from Bead 2 is non-breaking. Bead 4's verification block
    re-runs this test and the boundary lint
    (`TestEnforcementHasNoGitLeaks`) to confirm no regression.

## Provenance

Each spec.md Acceptance Criterion maps to the bead whose
verification proves it satisfied:

- `TestPlanRejectsIrrelevantADRCitation` → **Bead 1** step 7.
- `TestPlanRejectsUncoveredDomain` → **Bead 1** step 7.
- `TestSupersededADRDoesNotSatisfyCoverage` → **Bead 1** step 7.
- `TestSupersedeChainCycleDetected` → **Bead 1** step 7
  (spec-AC name preserved per revision 9).
- `TestSupersedeChainTooLong` → **Bead 1** step 7
  (spec-AC name preserved per revision 9).
- `TestCompleteRejectsUndeclaredDomainTouch` → **Bead 2** step 6.
- `TestCompleteRejectsUnownedFile` → **Bead 2** step 6 (named
  `TestUnownedFileRejected` in the new `divergence_test.go`).
- `TestVizAgentmindBenchFiltered` → **Bead 2** step 6.
- `TestSupersedeUnblocks` → **Bead 3** step 8 (asserts the new
  ADR file exists on disk at the user-supplied id verbatim
  per revision 1 / `adr.CreateWithID`).
- `TestSupersedeRejectsExistingID` → **Bead 3** step 8
  (collision path on `adr.CreateWithID`; revision 1).
- `TestCreateWithIDUsesSuppliedID`,
  `TestCreateWithIDRejectsExisting` → **Bead 3** step 3a
  (the new `internal/adr` helper unit tests; revision 1).
- `TestCheckADRDivergenceReturnsPopulated` → **Bead 2**
  step 6 (replaces the spec-086 stub
  `TestCheckADRDivergenceReturnsEmpty`; revision 10).
- `TestDivergenceFindingsSeededFromStructuredField` →
  **Bead 2** step 6 (asserts structured-field consumption
  in the supersede flow; revision 2).
- `TestOverrideUnblocks` → **Bead 3** step 8.
- `TestOverrideMetadataGoesThroughSeam` → **Bead 3** step 8.
- `TestApproveImplBackstopRunsDivergence` → **Bead 3** step 8.
- `TestOverrideAndSupersedeMutuallyExclusive` → **Bead 3** step 8.
- `TestOverrideEmptyReasonRejected` → **Bead 3** step 8.
- `TestCheckADRDivergenceSignatureWidened` → **Bead 2** step 5
  (the widening commit; the AST test reads the symbol's
  reflected signature).
- `cmd/mindspec/complete.go` + `cmd/mindspec/impl.go` expose
  `--override-adr` and `--supersede-adr` independently of and
  composable with `--allow-doc-skew` → **Bead 3** step 7.
- `ADR-0032-adr-semantic-gates.md` exists with `Status:
  Accepted` citing ADR-0030/0031, recording the algorithm and
  semantics per Requirements 7-17 → **Bead 4** step 1.
- All existing tests still pass; AST boundary lint stays green
  → enforced per-bead (every bead's verification ends with
  `go build ./... && go test -short ./...`); Bead 2 step 6
  additionally re-runs `TestEnforcementHasNoGitLeaks`
  explicitly because Bead 2 introduces the new
  `divergence.go` file that the lint guards.
- `go build ./... && go test -short ./...` green on every
  commit → enforced as the final verification step of every
  bead.

## Bead 1 — Plan-time gates: cite-relevant + coverage + superseded-chain walker

**Domain.** `core` (the validate package is core-domain per the
spec 086 OWNERSHIP.yaml seeding).

**Depends on.** Nothing in this spec.

**Steps**

1. Read `internal/validate/plan.go` lines 366-385
   (`checkADRCitations`) and confirm the current signature is
   `checkADRCitations(r *Result, store adr.Store, citations
   []ADRCitation)`. Confirm `adr.Store.Get(id)` returns
   `(*ADR, error)` where `ADR.Domains` is `[]string`
   (lower-cased at parse time per `internal/adr/parse.go:66`)
   and `ADR.Status` is the raw string from the
   `**Status**:` line. Confirm
   `contextpack.ParseSpec(specDir)` returns `(*SpecMeta,
   error)` where `SpecMeta.Domains` is `[]string` (lower-cased
   at parse time per `internal/contextpack/spec.go:70`).
2. Widen `checkADRCitations` to accept the spec's parsed
   impacted-domains: change the signature to
   `checkADRCitations(r *Result, store adr.Store, citations
   []ADRCitation, impactedDomains []string)`. Update the
   single existing caller in `plan.go` (line ~102) to pass
   the spec's impacted-domains list. The caller already
   operates against a known spec dir; load the impacted
   domains via `contextpack.ParseSpec(specDir)` at the
   call-site and pass the resulting `Domains` slice.
3. Inside `checkADRCitations`, after the existing
   superseded/proposed warning logic at lines 374-384,
   compute `overlap := intersectFold(a.Domains,
   impactedDomains)`. When `len(overlap) == 0`, emit
   `r.AddError("adr-cite-irrelevant", fmt.Sprintf("cited ADR
   %s declares domains %v which do not intersect spec
   impacted domains %v", cite.ID, a.Domains,
   impactedDomains))`. Preserve the existing Superseded /
   Proposed warning behaviour verbatim — the new error is
   ADDITIVE.
4. Add an unexported helper `intersectFold(a, b []string)
   []string` in `plan.go` that returns the case-folded,
   trim-whitespace exact set intersection of `a` and `b`.
   Use `strings.EqualFold` / `strings.TrimSpace`; build a
   `map[string]struct{}` over normalised `a` for O(n+m).
   This is the canonical domain-overlap algorithm per
   Requirement 16.
5. Add a new unexported helper `checkADRCoverage(r *Result,
   store adr.Store, citations []ADRCitation, impactedDomains
   []string)` in `plan.go`. For each domain in
   `impactedDomains`: scan the cited ADRs. A domain is
   COVERED when there exists at least one cited ADR `a` such
   that `strings.EqualFold(a.Status, "Accepted")` AND
   `intersectFold(a.Domains, []string{domain})` is non-empty.
   A cited ADR with `Status: Superseded` does NOT satisfy
   coverage UNLESS the superseding chain head (resolved via
   step 6 below) is ALSO cited and itself satisfies the same
   condition. ADRs with `Status: Proposed` (including
   placeholder ADRs pre-created by the supersede flow) do NOT
   satisfy coverage — this is documented behaviour per
   revision 11. On uncovered domain: `r.AddError(
   "adr-coverage-missing", fmt.Sprintf("impacted domain %q
   has no cited Accepted ADR; run: mindspec adr create
   --domain %s", d, d))`.

   Extract the predicate as an exported helper
   `IsDomainCovered(store adr.Store, citations []ADRCitation,
   domain string) bool` (revision 5 — single source of truth).
   This helper is the canonical "domain X is covered by an
   Accepted cited ADR, transitively through one supersede
   chain hop" predicate. Bead 2's `ValidateDivergence` MUST
   import and call this same helper for its
   `coveredDomains` decision — NO inline duplicate of the
   Accepted+intersect logic in `divergence.go`.

   **Call-site (revision 6).** Insert the `checkADRCoverage`
   call in `ValidatePlan` between the existing
   `checkADRCitations` invocation (line 102) and the `##
   ADR Fitness`-section presence check (line 106). The
   citations branch becomes:

   ```go
   } else {
       store := adr.NewFileStore(root)
       impacted, _ := loadImpactedDomains(specDir)
       checkADRCitations(r, store, fm.ADRCitations, impacted)
       checkADRCoverage(r, store, fm.ADRCitations, impacted)
   }
   ```

   `loadImpactedDomains(specDir)` is a tiny helper added in
   step 2 that wraps `contextpack.ParseSpec(specDir)` and
   returns `meta.Domains, err`. The `specDir` value is
   already in scope at `plan.go` line ~95 — `ValidatePlan`
   receives the plan-file path and derives `specDir =
   filepath.Dir(planPath)`.
6. Add an unexported helper
   `resolveSupersedeChainHead(store adr.Store, startID
   string) (*adr.ADR, *Issue)` in `plan.go`. Walk from
   `startID` following `ADR.SupersededBy` links. Maintain
   `visited := map[string]struct{}{}` and bound chain length
   at 10. On revisit: return `(nil, &Issue{Name:
   "adr-supersede-cycle", Severity: SevError, Message:
   fmt.Sprintf("superseded chain has a cycle starting at
   %s", startID)})`. On length > 10: return `(nil, &Issue{
   Name: "adr-supersede-chain-too-long", Severity: SevError,
   Message: fmt.Sprintf("superseded chain starting at %s
   exceeds max length 10", startID)})`. On terminal (an
   ADR with empty `SupersededBy`): return the terminal
   `*adr.ADR` and nil. `checkADRCoverage` integrates the
   walker: when a cited ADR has `Status: Superseded`,
   resolve its chain head; if the walker returns an Issue,
   `r.AddError(...)` with that Issue's Name + Message and
   skip the cited ADR for coverage purposes.
7. Add the new tests listed in the Testing Strategy block
   above to `internal/validate/plan_test.go`:
   `TestPlanRejectsIrrelevantADRCitation`,
   `TestPlanRejectsUncoveredDomain`,
   `TestSupersededADRDoesNotSatisfyCoverage`,
   `TestSupersedeChainCycleDetected`, `TestSupersedeChainTooLong`
   (names match spec.md AC verbatim per revision 9).
   Each test constructs a minimal `adr.Store` test double
   (or uses the existing `adr.FileStore` against a
   `t.TempDir()` fixture directory) plus a fixture spec.md
   and plan.md. Assertions are on `r.Issues` entries
   (Name, Severity, Message substrings) per the existing
   pattern in `plan_test.go`.

**Verification**

- [ ] `go test ./internal/validate -run
  'TestPlanRejectsIrrelevantADRCitation|TestPlanRejectsUncoveredDomain|TestSupersededADRDoesNotSatisfyCoverage|TestSupersedeChainCycleDetected|TestSupersedeChainTooLong'
  -v` — all five PASS; error messages in log show the ADR id,
  declared Domains, impacted Domains, and (for coverage) the
  `mindspec adr create --domain <d>` hint.
- [ ] `go test ./internal/validate -v` — all tests PASS,
  zero regressions vs `main`.
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `TestPlanRejectsIrrelevantADRCitation` passes against the
  extended `checkADRCitations`.
- `TestPlanRejectsUncoveredDomain` passes against the new
  `checkADRCoverage`.
- `TestSupersededADRDoesNotSatisfyCoverage` passes (cited
  Superseded ADR without cited chain head does NOT satisfy
  coverage).
- `TestSupersedeChainCycleDetected` passes (walker terminates
  on cycle and emits `"adr-supersede-cycle"`).
- `TestSupersedeChainTooLong` passes (walker terminates on
  chain > 10 and emits `"adr-supersede-chain-too-long"`).
- The new exported `IsDomainCovered` predicate is callable
  from `internal/validate/divergence.go` (revision 5 single
  source of truth).
- All existing `plan_test.go` tests still pass; the existing
  Superseded/Proposed warning behaviour is preserved
  verbatim (the new errors are ADDITIVE).

## Bead 2 — `ValidateDivergence` + fill `CheckADRDivergence` body

**Domain.** `core` (validate package).

**Depends on.** Bead 1 (uses `intersectFold` and
`resolveSupersedeChainHead`).

**Steps**

1. Read `internal/validate/adr_divergence.go` lines 1-25 to
   confirm the current stub signature
   `CheckADRDivergence(root, diffRef string, exec
   executor.Executor) *Result` and the preserved
   `SubCommand: "adr-divergence"` label.
2. Read `internal/validate/ownership.go` lines 100-115 to
   confirm `attributeDomain(root, sourcePath string, domains
   []string) (string, *Ownership, error)` returns
   `("", nil, nil)` when no manifest in the supplied domains
   list claims the file. This is the unowned-file signal F1
   surfaces as `"adr-divergence-unowned"`.
3. Read `internal/approve/impl.go` lines 121-141 and
   `internal/complete/complete.go` lines 153-171 to confirm
   the current `CheckADRDivergence` call sites and that
   `specDir`, `base`, and `beadID` (or `""` on the impl
   path) are all in scope at those lines.
4. Create new file `internal/validate/divergence.go`
   exporting (signature matches spec Requirement 9 per
   revision 7 — `citations` and `store` are loaded INSIDE,
   not passed in):

   ```go
   // DivergenceFinding is the structured record consumed by
   // the supersede flow (revision 2). The string-formatted
   // Issue messages on *Result stay for humans; this slice
   // is the machine-readable seam.
   type DivergenceFinding struct {
       Domain       string // empty when Kind == "unowned"
       Path         string
       ManifestPath string // empty when Kind == "unowned" or fallback
       Kind         string // "uncovered" | "unowned"
   }

   func ValidateDivergence(
       exec executor.Executor,
       root, specDir, beadID string,
       base, head string,
   ) (*Result, []DivergenceFinding)
   ```

   The function implementation:
   - Initialise `r := &Result{SubCommand: "adr-divergence"}`
     and `findings := []DivergenceFinding{}`.
   - Load `meta, err := contextpack.ParseSpec(specDir)`;
     on error `r.AddError("adr-divergence-spec",
     err.Error())` and return `(r, nil)`.
   - Load citations + store INSIDE the function (revision 7):
     `fm, err := parsePlanFrontmatter(planPathFromSpecDir(
     specDir))` (existing parser used by `plan.go`); `store
     := adr.NewFileStore(root)`. On loader error
     `r.AddError("adr-divergence-load", err.Error())` and
     return `(r, nil)`.
   - `changed, err := exec.ChangedFiles(base, head)`; on
     error `r.AddError("adr-divergence-diff", err.Error())`
     and return `(r, nil)`.
   - Filter `changed`: drop any entry whose first path
     segment (text before the first `/`) is in the
     existing `excludedFirstSegments` set from
     `ownership.go:31-35` (`viz`, `agentmind`, `bench`).
     This is HC-4 layer 2, revision 5.
   - Sort the spec's impacted-domains lexicographically
     (per `attributeDomain`'s caller contract at
     `ownership.go:94-99`) and pass that slice to each
     `attributeDomain` call.
   - For each remaining changed file `f`:
     - `domain, own, err := attributeDomain(root, f,
       sortedImpactedDomains)`; on error
       `r.AddError("adr-divergence-attribute", ...)`
       and continue.
     - On `domain == ""` (unowned file):
       `r.AddError("adr-divergence-unowned",
       fmt.Sprintf("file %s is not claimed by any
       OWNERSHIP.yaml for the spec's impacted domains
       %v; add it to an existing manifest or create a
       new domain dir at .mindspec/docs/domains/<name>/OWNERSHIP.yaml",
       f, meta.Domains))`. Also append a `DivergenceFinding{
       Path: f, Kind: "unowned"}` to `findings`.
     - On `domain != ""` but the domain is NOT covered by
       any Accepted cited ADR — use the Bead 1 single-source
       predicate `IsDomainCovered(store, fm.ADRCitations,
       domain)` (revision 5). Compute `manifestRef :=
       own.ManifestPath`; when empty, set `manifestRef =
       fmt.Sprintf("<fallback: internal/%s/**>", domain)`.
       `r.AddError("adr-divergence-uncovered",
       fmt.Sprintf("file %s attributed to domain %q
       (manifest: %s) but no cited ADR covers %q", f,
       domain, manifestRef, domain))`. Also append a
       `DivergenceFinding{Domain: domain, Path: f,
       ManifestPath: own.ManifestPath, Kind:
       "uncovered"}` to `findings`.
   - Return `(r, findings)`.

   The supersede flow (Bead 3 step 4) reads the FIRST
   `findings[i]` where `Kind == "uncovered"` to seed the
   placeholder ADR's `Domains` field — NO `fmt.Sprintf`
   round-tripping of `r.Issues[i].Message` (revision 2).

   IMPORT DISCIPLINE (HC-6): the new file MUST NOT import
   `os/exec` or
   `github.com/mrmaxsteel/mindspec/internal/gitutil`,
   and MUST NOT call `exec.Command("git", ...)` or
   `exec.Command("bd", ...)`. All git reads go through
   `executor.Executor`.
5. Replace the body of
   `internal/validate/adr_divergence.go::CheckADRDivergence`
   (lines 20-25). Widen the signature to ALSO return
   `[]DivergenceFinding` (revision 2):

   ```go
   func CheckADRDivergence(
       root, diffRef string,
       exec executor.Executor,
       specDir string,
       beadID string,
   ) (*Result, []DivergenceFinding)
   ```

   Implementation: on empty `specDir`, return a `*Result`
   with a single `r.AddError("adr-divergence-load",
   "specDir required")` and `nil` findings. Otherwise
   compute `headRef` — `"HEAD"` when `beadID != ""`
   (complete path) and the spec branch when `beadID == ""`
   (impl path — derive via
   `workspace.SpecBranch(filepath.Base(specDir))`). Then
   delegate: `return ValidateDivergence(exec, root,
   specDir, beadID, diffRef, headRef)`. `ValidateDivergence`
   loads citations + store internally (revision 7). Preserve
   `SubCommand: "adr-divergence"`.
6. Add the new tests to
   `internal/validate/divergence_test.go` (new file) and
   `internal/complete/complete_test.go` (extend):
   - `TestUnownedFileRejected`,
     `TestVizAgentmindBenchFiltered`,
     `TestCheckADRDivergenceSignatureWidened` (asserts via
     reflection that
     `validate.CheckADRDivergence`'s exported signature
     matches the widened form including the
     `[]DivergenceFinding` return),
     `TestCheckADRDivergenceReturnsPopulated`,
     `TestDivergenceFindingsSeededFromStructuredField` — all
     in `divergence_test.go`.
   - `TestCompleteRejectsUndeclaredDomainTouch` — in
     `complete_test.go`, using the existing `MockExecutor`
     pattern. Asserts `closeBeadFn` and `exec.CompleteBead`
     are NOT invoked on divergence error.
   - REPLACE the spec-086 stub test
     `TestCheckADRDivergenceReturnsEmpty` (present per
     spec 086 plan Bead 2 step 11) with
     `TestCheckADRDivergenceReturnsPopulated` in the same
     commit (revision 10). The replacement is a rename +
     body rewrite asserting a non-empty Result and
     `[]DivergenceFinding` on a real divergence fixture.
     The commit message documents the rename for HC-3
     traceability — the symbol-presence half of HC-3 is
     preserved because the replacement test occupies the
     same file and asserts the SAME function
     (`CheckADRDivergence`) at greater depth. Confirm
     `TestApproveImplCallOrder` still passes against the
     widened signature (it asserts the symbol NAME, not the
     argument list, so the return-tuple widening is
     non-breaking).
7. Update the two call sites to match the widened signature
   AND the new `[]DivergenceFinding` return:
   - `internal/complete/complete.go:165`: change to
     `adrResult, adrFindings := validate.CheckADRDivergence(
     root, base, exec, specDir, beadID)`. `specDir` is
     derived via `workspace.SpecDir(root, specID)` at this
     point in the function (`specID` is already in scope
     from step 1 of `Run`). `beadID` is the function
     parameter already in scope. `adrFindings` is later
     consumed by Bead 3 step 4's supersede wrap.
   - `internal/approve/impl.go:138`: change to
     `adrResult, adrFindings := validate.CheckADRDivergence(
     root, base, exec, specDir, "")`. `specDir` is already
     in scope at line 98. The empty `beadID` triggers the
     broader spec-branch diff range inside
     `ValidateDivergence`. `adrFindings` is consumed by
     Bead 3 step 6's mirrored supersede wrap.

**Verification**

- [ ] `go test ./internal/validate -run
  'TestUnownedFileRejected|TestVizAgentmindBenchFiltered|TestCheckADRDivergenceSignatureWidened'
  -v` — all three PASS.
- [ ] `go test ./internal/complete -run
  TestCompleteRejectsUndeclaredDomainTouch -v` — PASS;
  error message in log names file → manifest → domain.
- [ ] `go test ./internal/approve -run
  TestApproveImplCallOrder -v` — PASS (no regression vs
  `main`).
- [ ] `go test ./internal/lint -run
  TestEnforcementHasNoGitLeaks -v` — PASS (new
  `divergence.go` does not regress 085's boundary).
- [ ] `grep -nE 'os/exec|internal/gitutil|exec\.Command'
  internal/validate/divergence.go` — exits 1 (no matches).
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `TestUnownedFileRejected` passes; error message names
  the file and the impacted-domains slice.
- `TestVizAgentmindBenchFiltered` passes; viz/agentmind/bench
  files filtered BEFORE `attributeDomain` is called.
- `TestCheckADRDivergenceSignatureWidened` passes;
  `validate.CheckADRDivergence`'s reflected signature
  matches the widened form including the
  `[]DivergenceFinding` second return value.
- `TestCheckADRDivergenceReturnsPopulated` passes; the
  spec-086 stub `TestCheckADRDivergenceReturnsEmpty` is
  REPLACED (revision 10) — the new test asserts a
  non-empty `*Result` and non-empty `[]DivergenceFinding`
  on a real divergence fixture.
- `TestDivergenceFindingsSeededFromStructuredField` passes;
  the supersede flow reads `findings[i].Domain`
  structurally — NO `fmt.Sprintf` parsing of
  `r.Issues[i].Message` (revision 2).
- `TestCompleteRejectsUndeclaredDomainTouch` passes;
  `closeBeadFn` and `exec.CompleteBead` are NOT invoked
  on divergence error.
- `TestApproveImplCallOrder` (spec 086) still passes
  against the widened signature.
- `TestEnforcementHasNoGitLeaks` (spec 085 boundary lint)
  still passes; the new `divergence.go` does NOT import
  `os/exec` or `internal/gitutil`.
- All existing `internal/validate`, `internal/complete`,
  `internal/approve` tests still pass.

## Bead 3 — CLI flags + override/supersede flow + audit metadata

**Domain.** `execution` (cmd/mindspec/) and `workflow`
(internal/complete + internal/approve).

**Depends on.** Beads 1 and 2.

**Steps**

1. Read `cmd/mindspec/complete.go` lines 74-103 and
   `cmd/mindspec/impl.go` lines 31-65 to confirm the
   existing `--allow-doc-skew` flag wiring pattern: flag
   registered on the cobra command, value read via
   `cmd.Flags().GetString`, empty-string rejection via
   `cmd.Flags().Changed(...) && strings.TrimSpace(...) ==
   ""`, and value passed into `CompleteOpts` / `ImplOpts`.
   The new flags follow this pattern verbatim.
2. Extend `CompleteOpts` in
   `internal/complete/complete.go` (lines 45-47) with two
   new string fields:
   `OverrideADR string` — verbatim reason; empty means no
   override.
   `SupersedeADR string` — ADR id (e.g., `"ADR-0099"`);
   empty means no supersede.
   Mirror the godoc comment style used for `AllowDocSkew`,
   noting (a) gate-skip semantics, (b)
   post-terminal-mutation metadata write, (c)
   `SupersedeADR` additionally pre-creates a placeholder
   ADR via `internal/adr.Create` with `Status: Proposed`
   BEFORE the gate-skip path.
3. Extend `ImplOpts` in `internal/approve/impl.go` (lines
   37-39) with the same two fields and the same godoc
   pattern.
3a. **PREREQUISITE (revision 1, BLOCKER).** Add a new
    exported function
    `adr.CreateWithID(root, id, title string, opts CreateOpts)
    (string, error)` to `internal/adr/create.go`. Contract:

    - `id` is the user-supplied ADR id (e.g., `"ADR-0099"`)
      and MUST already have passed
      `idvalidate.ADRID(id)` at the call site (Bead 3 step 7
      enforces this at CLI flag-parse time).
    - Compute `outPath := workspace.ADRFilePath(root, id)`.
    - If `outPath` already exists (`os.Stat` returns nil),
      return `"", fmt.Errorf("ADR %s already exists at %s",
      id, outPath)`.
    - Otherwise reuse the existing template-fill logic from
      `adr.Create` (lines 104-125 of `create.go`) but
      SUBSTITUTE the user-supplied `id` instead of calling
      `NextID(root)`. Preserve the `**Status**: Proposed`
      and `**Domain(s)**` substitution behaviour exactly.
    - The supersede-update sub-step (lines 127-132) does
      NOT apply — `CreateWithID` is for the placeholder
      flow, which has no "old" ADR to update.

    `adr.Create` is UNCHANGED — the existing `mindspec adr
    create` CLI path continues to use `NextID`. Add a unit
    test `TestCreateWithIDRejectsExisting` in
    `internal/adr/create_test.go` asserting the collision
    error. Add a positive test `TestCreateWithIDUsesSuppliedID`
    asserting the resulting file path matches the requested
    id verbatim. This sub-step CLOSES the unanimous BLOCKER
    cited by all six reviewers: `TestSupersedeUnblocks`
    becomes deterministic.

4. In `internal/complete/complete.go::Run` at lines
   165-171 (the `CheckADRDivergence` call site), the gate
   is invoked EXACTLY ONCE per `Run` (revision 3 — no
   probe-call). The gate-wrap shape:

   ```go
   adrResult, adrFindings := validate.CheckADRDivergence(
       root, base, exec, specDir, beadID,
   )

   // Pre-create the placeholder ADR FIRST when supersede
   // is requested, so the new file exists on disk even if
   // a downstream step fails.
   var newADRID string
   if opts.SupersedeADR != "" {
       // Seed Domains from the structured findings slice
       // (revision 2). When no violation exists (defensive
       // invocation), domains is empty and the user
       // populates later.
       var domains []string
       for _, f := range adrFindings {
           if f.Kind == "uncovered" && f.Domain != "" {
               domains = []string{f.Domain}
               break
           }
       }
       title := "Placeholder for " + opts.SupersedeADR
       newPath, err := adr.CreateWithID(
           root, opts.SupersedeADR, title,
           adr.CreateOpts{Domains: domains},
       )
       if err != nil {
           return nil, fmt.Errorf("--supersede-adr: %w", err)
       }
       newADRID = opts.SupersedeADR
       _ = newPath // path is asserted by TestSupersedeUnblocks
   }

   // Decide whether the gate failure is fatal.
   if opts.OverrideADR == "" && opts.SupersedeADR == "" &&
       adrResult.HasFailures() {
       return nil, fmt.Errorf(
           "adr-divergence: %s\nhint: re-run with --override-adr \"<reason>\" or --supersede-adr ADR-NNNN to bypass",
           joinResultErrorMessages(adrResult),
       )
   }
   ```

   **Gate-runs-once contract (revision 3).** The plan
   guarantees `CheckADRDivergence` is invoked EXACTLY ONCE
   per `Run`. The supersede path consumes the `adrFindings`
   slice for `Domains` seeding; the failure-decision is
   simply bypassed when an override or supersede flag is
   set. There is NO probe-call.

   **Structured capture (revision 2).** The `Domains` seed
   value is read from `adrFindings[i].Domain` (structured
   field) — NO `fmt.Sprintf` parsing of
   `adrResult.Issues[i].Message`. The `[]DivergenceFinding`
   slice is the contract Bead 2 added to
   `CheckADRDivergence` precisely for this consumer.

   **ID determinism (revision 1).** `adr.CreateWithID`
   (Bead 3 step 3a) uses the user-supplied id verbatim and
   errors if a file at that id already exists. The error
   message matches the substring `"already exists"` which
   `TestSupersedeRejectsExistingID` asserts.

5. AFTER the terminal mutation at
   `internal/complete/complete.go:201-206`
   (`exec.CompleteBead`), at the same scope as the
   existing spec-086 `AllowDocSkew` write block (lines
   208-225), add two parallel blocks using the
   `completeMergeMetadataFn` test seam. **REQUIRED rename
   (revision 8).** This bead RENAMES `mergeMetadataFn` →
   `completeMergeMetadataFn` at line 31 of `complete.go`
   to match spec.md Requirement 12 verbatim — this is
   contract, not optional. The rename is a single
   identifier change (the package-level var declaration
   plus every call site within
   `internal/complete/complete.go`). Update the in-package
   test
   `internal/complete/complete_test.go::TestOverrideMetadataGoesThroughSeam`
   to swap the new name:

   ```go
   if opts.OverrideADR != "" && completeErr == nil {
       meta := buildSkewMetadata(opts.OverrideADR,
           "mindspec_adr_override_reason",
           "mindspec_adr_override_at",
           "mindspec_adr_override_by",
       )
       if err := completeMergeMetadataFn(beadID, meta); err != nil {
           fmt.Printf("Warning: could not record adr-override metadata on %s: %v\n", beadID, err)
       }
   }
   if opts.SupersedeADR != "" && completeErr == nil {
       meta := map[string]interface{}{
           "mindspec_adr_supersede_id":     newADRID,
           "mindspec_adr_supersede_reason": fmt.Sprintf("superseded by %s", newADRID),
           "mindspec_adr_supersede_at":     time.Now().UTC().Format(time.RFC3339),
           "mindspec_adr_supersede_by":     gitUserEmailOr("unknown"),
       }
       if err := completeMergeMetadataFn(beadID, meta); err != nil {
           fmt.Printf("Warning: could not record adr-supersede metadata on %s: %v\n", beadID, err)
       }
   }
   ```

   `newADRID` is the value captured from step 4's
   `adr.Create` return path (parse the id from the
   filename). `gitUserEmailOr("unknown")` is a small helper
   that calls `gitUserEmailFn()` (already in the
   package-level var block at line 32) and falls back to
   `"unknown"` on error or empty string. Reuse
   `buildSkewMetadata` from spec 086 (already in
   `internal/complete/complete.go` per
   `complete.go:217-221` usage); it accepts the reason
   string and the three key names.
6. Symmetric changes to `internal/approve/impl.go`:
   - Wrap the `CheckADRDivergence` call at lines 138-141
     with the same override/supersede gate-skip logic;
     widen to `adrResult, adrFindings :=
     validate.CheckADRDivergence(root, base, exec,
     specDir, "")`. The gate is invoked EXACTLY ONCE here
     (revision 3 — no probe-call).
   - Pre-create the placeholder ADR on `SupersedeADR`
     set using `adr.CreateWithID` (revision 1) seeded
     from `adrFindings[i].Domain` (revision 2) — same
     logic as Bead 3 step 4. The placeholder ADR is
     written to disk BEFORE the gate-failure decision so
     the file exists even when downstream steps fail.
   - AFTER `exec.FinalizeEpic` at line 171, mirror the
     two metadata-write blocks from step 5 but target the
     EPIC (`epicID`), using `implMergeMetadataFn` (line
     25). The seam name is already `implMergeMetadataFn`
     (no rename needed on the impl side — the existing
     name matches the spec). Use the same key names
     (`mindspec_adr_override_*`,
     `mindspec_adr_supersede_*`) — the namespaces are
     consistent across both lifecycle commands per
     Requirement 13 and Acceptance Criterion
     `TestApproveImplBackstopRunsDivergence`.
7. Wire the two CLI flags in `cmd/mindspec/complete.go`
   (extend lines 103-104) and `cmd/mindspec/impl.go`
   (extend lines 31-32):

   ```go
   completeCmd.Flags().String("override-adr", "",
       "Override the ADR-divergence gate with a recorded reason (records mindspec_adr_override_* on bead metadata)")
   completeCmd.Flags().String("supersede-adr", "",
       "Pre-create a placeholder ADR (Status: Proposed) and bypass the divergence gate (records mindspec_adr_supersede_* on bead metadata)")
   ```

   In each command's `RunE` (around `complete.go:78-84`
   and `impl.go:59-65`), AFTER the existing
   `--allow-doc-skew` parsing block, add:

   ```go
   overrideADR, _ := cmd.Flags().GetString("override-adr")
   if cmd.Flags().Changed("override-adr") && strings.TrimSpace(overrideADR) == "" {
       return fmt.Errorf("--override-adr requires a non-empty reason")
   }
   supersedeADR, _ := cmd.Flags().GetString("supersede-adr")
   if cmd.Flags().Changed("supersede-adr") {
       if err := idvalidate.ADRID(supersedeADR); err != nil {
           return fmt.Errorf("--supersede-adr: %w", err)
       }
   }
   if cmd.Flags().Changed("override-adr") && cmd.Flags().Changed("supersede-adr") {
       return fmt.Errorf("--override-adr and --supersede-adr are mutually exclusive")
   }
   ```

   Then pass both into the opts struct:
   `complete.CompleteOpts{AllowDocSkew: allowDocSkew,
   OverrideADR: overrideADR, SupersedeADR: supersedeADR}`
   (and the equivalent for `approve.ImplOpts`).
8. Add the new tests listed in the Testing Strategy block:
   `TestOverrideUnblocks`, `TestSupersedeUnblocks`,
   `TestSupersedeRejectsExistingID` (new — asserts the
   `adr.CreateWithID` collision path; revision 1),
   `TestOverrideMetadataGoesThroughSeam`,
   `TestApproveImplBackstopRunsDivergence`,
   `TestOverrideAndSupersedeMutuallyExclusive`,
   `TestOverrideEmptyReasonRejected`. Also add the unit
   tests for the new `adr.CreateWithID` helper
   (`TestCreateWithIDUsesSuppliedID`,
   `TestCreateWithIDRejectsExisting`) in
   `internal/adr/create_test.go`. Use the existing
   `MockExecutor` patterns from spec 086; swap
   `completeMergeMetadataFn` / `implMergeMetadataFn` for
   recording stubs to capture write order + arguments;
   assert write-ordering ALWAYS strictly after the terminal
   mutation per the spec-086 panel CONSENSUS rev 4
   pattern (failing-`CompleteBead` /
   failing-`FinalizeEpic` sub-cases assert NO metadata
   write).

   `TestSupersedeUnblocks` MUST assert the new ADR file
   exists at the user-supplied id path verbatim — e.g.,
   `.mindspec/docs/adr/ADR-0099-placeholder-for-adr-0099.md`
   when `--supersede-adr ADR-0099` was passed. This is the
   deterministic falsifiability fix revision 1 enables.
9. After implementing steps 1-8, run the test diff vs
   `main` and record the test-name + status delta in the
   final commit message of this bead (HC-3 enforcement;
   no pre-existing test SKIPPED or REMOVED — only
   ADDITIONS and the rename-or-removal of the now-obsolete
   `TestCheckADRDivergenceReturnsEmpty` per Bead 2 step 6).

**Verification**

- [ ] `go test ./internal/complete -run
  'TestOverrideUnblocks|TestSupersedeUnblocks|TestOverrideMetadataGoesThroughSeam'
  -v` — all PASS; logs show metadata write strictly
  after `CompleteBead`.
- [ ] `go test ./internal/approve -run
  TestApproveImplBackstopRunsDivergence -v` — PASS; logs
  show the EPIC metadata write strictly after
  `FinalizeEpic`.
- [ ] `go test ./cmd/mindspec -run
  'TestOverrideAndSupersedeMutuallyExclusive|TestOverrideEmptyReasonRejected'
  -v` — PASS.
- [ ] Manual smoke (recorded in commit message, not in
  CI): `mindspec complete <bead>` against a bead touching
  an uncovered domain — exit non-zero, error names file
  + manifest + domain. Re-run with `--override-adr "test
  override"` — exit 0, metadata records reason. Re-run a
  different uncovered bead with `--supersede-adr
  ADR-0099` — exit 0, new ADR file on disk with
  `Status: Proposed`.
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `TestOverrideUnblocks` passes; metadata write strictly
  after `CompleteBead` returns nil; failing-`CompleteBead`
  sub-case asserts no write.
- `TestSupersedeUnblocks` passes; new ADR file exists on
  disk at `.mindspec/docs/adr/<user-supplied-id>-*.md`
  (e.g. `ADR-0099-placeholder-for-adr-0099.md` when
  `--supersede-adr ADR-0099` was passed; deterministic
  per revision 1's `adr.CreateWithID`) with `Status:
  Proposed` and `**Domain(s)**` containing the captured
  violated domain (sourced from
  `[]DivergenceFinding[i].Domain` per revision 2 — NO
  string parsing); bead metadata contains the four
  `mindspec_adr_supersede_*` keys.
- `TestSupersedeRejectsExistingID` passes; invoking
  `--supersede-adr ADR-0099` when `ADR-0099` already
  exists on disk returns an error containing the
  substring `"already exists"` and DOES NOT mutate any
  ADR file or write any metadata.
- `TestCreateWithIDUsesSuppliedID` and
  `TestCreateWithIDRejectsExisting` (in
  `internal/adr/create_test.go`) pass — the new
  `adr.CreateWithID` API behaves as contracted in Bead 3
  step 3a (revision 1 BLOCKER closure).
- `completeMergeMetadataFn` (renamed from `mergeMetadataFn`
  per revision 8) is the only metadata-write path on the
  complete side; `implMergeMetadataFn` is the only one on
  the impl side.
- `CheckADRDivergence` is invoked EXACTLY ONCE per
  `Run` / `ApproveImpl` (revision 3 gate-runs-once
  contract); the override/supersede paths consume
  `adrFindings` and skip the failure decision — they do
  NOT issue a second call.
- `TestOverrideMetadataGoesThroughSeam` passes; the
  recording stub captured the write (NOT
  `bead.MergeMetadata` directly).
- `TestApproveImplBackstopRunsDivergence` passes; on the
  failure path `FinalizeEpic` is NOT invoked and
  `mindspec_phase: done` is NOT written; on the override
  path the three `mindspec_adr_override_*` keys land on
  the EPIC metadata after `FinalizeEpic` returns nil.
- `TestOverrideAndSupersedeMutuallyExclusive` passes;
  passing both flags returns the exact substring
  `"mutually exclusive"`.
- `TestOverrideEmptyReasonRejected` passes; passing
  `--override-adr ""` returns the exact substring
  `"requires a non-empty reason"`.
- The two new flags are additive to `--allow-doc-skew`
  — passing `--allow-doc-skew "<reason>" --override-adr
  "<reason>"` on a single invocation succeeds (no
  mutual-exclusivity error); the only mutually
  exclusive pair is `--override-adr` vs
  `--supersede-adr`.
- All pre-existing tests still pass (HC-3); the only
  modification to existing tests is the spec-086 stub
  `TestCheckADRDivergenceReturnsEmpty` rename or
  removal per Bead 2 step 6.

## Bead 4 — ADR-0032 finalization + AST call-order test confirmation

**Domain.** `core` (the ADR file lives under
`.mindspec/docs/adr/` which the OWNERSHIP.yaml for `core`
claims per spec 086 seeding).

**Depends on.** Bead 3.

**Steps**

1. Edit
   `.mindspec/docs/adr/ADR-0032-adr-semantic-gates.md`
   lines 13-16 ("## Status" section). Replace the
   narrative placeholder paragraph ("Stub created during
   spec 087-adr-semantic-gate drafting. Finalized in spec
   087 Bead N alongside the semantic-gate
   implementation.") with: "Finalized in spec 087 Bead 4
   alongside the semantic-gate implementation in
   `internal/validate/{plan,divergence,adr_divergence}.go`,
   the override/supersede flag plumbing in
   `cmd/mindspec/`, and the metadata-seam wiring in
   `internal/{complete,approve}/`." The
   `**Status**: Accepted` frontmatter field (line 4) is
   already set and requires no change.
2. **DO NOT mutate `**Domain(s)**` (revision 4).**
   ADR-0032's authored line `**Domain(s)**: validation,
   adr, lifecycle` is GRANDFATHERED and remains as
   authored. The canonical-tag-shape rule
   (Requirement 16) applies PROSPECTIVELY — new specs and
   new ADRs SHOULD use canonical
   (`OWNERSHIP.yaml`-directory-name) tags, but
   ADR-0030/0031/0032 retain their free-form authored
   tags. Mutating ADR-0032's `Domain(s)` here would
   create an asymmetric retag (ADR-0030 and ADR-0031
   keep theirs), would cascade through the new
   `intersectFold` plan-time check, and risks impacting
   unrelated specs that cite ADR-0032 with different
   domain expectations. Confirm via
   `git diff .mindspec/docs/adr/ADR-0032-adr-semantic-gates.md`
   that the only mutation in this commit is the step-1
   "Status" paragraph narrative edit. No other change to
   the ADR file.
3. Re-run `TestApproveImplCallOrder` from
   `internal/approve/impl_test.go` to confirm the
   widened-signature divergence call still parses in the
   AST harness. The harness asserts the symbol NAME and
   ORDER (per spec 086 panel CONSENSUS rev 9), not the
   argument list, so widening is non-breaking. No code
   change is required at this bead; the verification step
   below is the green-test confirmation.
4. Re-run `TestEnforcementHasNoGitLeaks` to confirm the
   `internal/validate/divergence.go` file Bead 2
   introduced still satisfies the spec 085 boundary
   invariant.

**Verification**

- [ ] `git diff
  .mindspec/docs/adr/ADR-0032-adr-semantic-gates.md` —
  shows exactly ONE narrative-text edit from step 1 (the
  "Status" paragraph). The `**Domain(s)**` line is
  UNCHANGED (revision 4 grandfathering).
- [ ] `go test ./internal/approve -run
  TestApproveImplCallOrder -v` — PASS.
- [ ] `go test ./internal/lint -run
  TestEnforcementHasNoGitLeaks -v` — PASS.
- [ ] Full spec-087 test sweep:
  `go test ./internal/validate ./internal/complete
  ./internal/approve ./internal/adr ./cmd/mindspec -run
  'TestPlanRejectsIrrelevantADRCitation|TestPlanRejectsUncoveredDomain|TestSupersededADRDoesNotSatisfyCoverage|TestSupersedeChainCycleDetected|TestSupersedeChainTooLong|TestUnownedFileRejected|TestVizAgentmindBenchFiltered|TestCheckADRDivergenceSignatureWidened|TestCheckADRDivergenceReturnsPopulated|TestDivergenceFindingsSeededFromStructuredField|TestCompleteRejectsUndeclaredDomainTouch|TestOverrideUnblocks|TestSupersedeUnblocks|TestSupersedeRejectsExistingID|TestCreateWithIDUsesSuppliedID|TestCreateWithIDRejectsExisting|TestOverrideMetadataGoesThroughSeam|TestApproveImplBackstopRunsDivergence|TestOverrideAndSupersedeMutuallyExclusive|TestOverrideEmptyReasonRejected'
  -v` — all PASS.
- [ ] `go build ./... && go test -short ./...` — exit 0.

**Acceptance Criteria**

- `ADR-0032-adr-semantic-gates.md` Status section
  narrative paragraph (line ~15) reads as edited in
  step 1 above (no "Bead N" placeholder).
- ADR-0032 `**Domain(s)**` field (line 5) is UNCHANGED
  from the authored `validation, adr, lifecycle` —
  grandfathered per revision 4. Requirement 16's
  canonical-tag rule applies PROSPECTIVELY to new
  ADRs/specs only.
- `TestApproveImplCallOrder` still passes against the
  widened `CheckADRDivergence` signature.
- `TestEnforcementHasNoGitLeaks` still passes (no
  regression in the new `divergence.go` file).
- All spec.md Acceptance Criteria checkboxes (lines
  524-635 of spec.md) are satisfied — Bead 4's
  verification block re-runs the full spec-087 test
  sweep end-to-end as the final confirmation before
  the follow-up `mindspec approve impl
  087-adr-semantic-gate`.
