---
adr_citations:
    - id: ADR-0003
      sections:
        - Decision
    - id: ADR-0016
      sections:
        - Decision
approved_at: "2026-03-08T16:12:04Z"
approved_by: user
bead_ids:
    - mindspec-3rkt.1
    - mindspec-3rkt.2
    - mindspec-3rkt.3
spec_id: 076-plan-decomposition-quality
status: Approved
version: "1"
---
# Plan: 076-plan-decomposition-quality

## ADR Fitness

**ADR-0003 (Centralized Agent Instruction Emission)**: Sound. This spec extends the plan-mode instruct template — the exact mechanism ADR-0003 establishes for delivering guidance. The decomposition heuristics section becomes part of the runtime-assembled instruction output, consistent with the "dynamic over static" principle.

**ADR-0016 (Bead Creation Timing)**: Sound. Decomposition quality checks validate the plan markdown *before* beads are created at plan-approval time. This aligns with ADR-0016's "lazy creation" model — the plan is freely editable during drafting, and quality warnings fire during `mindspec validate plan` to inform the agent before approval.

## Testing Strategy

Unit tests in `internal/validate/plan_test.go` covering:
- `ExtractPathRefs()`: path extraction from various text patterns (Go files, packages, test commands)
- `ParseBeadSections()`: verifying `StepLines` capture alongside existing fields
- `checkDecompositionQuality()`: threshold-based warnings for R_scope, dependency depth, parallelism ratio, bead count
- Negative tests confirming well-structured plans produce no warnings
- Backwards compatibility: approved plans skip decomposition checks

No integration tests needed — all changes are pure functions operating on parsed plan content.

## Bead 1: Research Document and Instruct Template Heuristics

**Steps**
1. Create `.mindspec/docs/research/scaling-agent-systems.md` with full citation of Kim et al. (2025), key findings (sequential degradation, optimal redundancy R≈0.41, 3-4 agent sweet spot, capability saturation, tool fragmentation penalty), metric definitions, and threshold rationale
2. Add a `## Decomposition Heuristics` section to `internal/instruct/templates/plan.md` after the "Required Output" section, containing six research-backed rules framed as design-time questions (independence test, merge signal, trivial-work test, target count, serial chain limit, tool grouping), each with a brief "why" referencing the research
3. Add a reference line in the instruct template pointing agents to `.mindspec/docs/research/scaling-agent-systems.md` for full context

**Verification**
- [ ] `.mindspec/docs/research/scaling-agent-systems.md` exists with citation, key findings, metrics, thresholds
- [ ] `./bin/mindspec instruct` in plan mode emits output containing "Decomposition Heuristics" and all six rules
- [ ] Each heuristic is framed as a question and includes a "why"
- [ ] `go test ./internal/instruct/...` passes (template renders without errors)

**Depends on**
None

## Bead 2: ParseBeadSections StepLines and ExtractPathRefs

**Steps**
1. Extend `BeadSection` struct in `internal/validate/plan.go` to include `StepLines []string` field
2. Update `ParseBeadSections()` to capture raw step lines (the numbered text) into `StepLines` alongside existing `StepsCount`
3. Implement `ExtractPathRefs(text string) []string` function using regex to match Go file paths (`*.go`), package paths (`internal/foo/bar`, `cmd/mindspec/...`), and test paths (`go test ./...`), deduplicating results
4. Write unit tests in `internal/validate/plan_test.go`: `TestExtractPathRefs` covering Go files, packages, test commands, and edge cases (URLs, non-paths); `TestParseBeadSections_StepLines` verifying capture

**Verification**
- [ ] `go test ./internal/validate/ -run TestExtractPathRefs` passes
- [ ] `go test ./internal/validate/ -run TestParseBeadSections_StepLines` passes

**Depends on**
None

## Bead 3: Decomposition Quality Validator

**Steps**
1. Implement `checkDecompositionQuality(sections []BeadSection) []Warning` in `internal/validate/plan.go` that computes: R_scope (scope redundancy from path refs in steps+verification), dependency chain depth (longest path in DAG built from `DependsOn` text using `(?i)bead\s+(\d+)` regex), parallelism ratio (beads with zero inbound deps / total), and bead count
2. Wire `checkDecompositionQuality()` into `ValidatePlan()`, skipping for already-approved plans (consistent with existing `isApproved` pattern)
3. Each warning must include the computed metric value and the threshold (e.g., "scope redundancy R=0.65 exceeds threshold 0.50")
4. Write unit tests: `TestDecompositionQuality` covering all five warning conditions (R_scope high, R_scope low, chain depth > 3, parallelism < 0.25, bead count > 6); `TestDecompositionQuality_NoWarnings` confirming a well-structured plan produces no warnings

**Verification**
- [ ] `go test ./internal/validate/ -run TestDecompositionQuality` passes — all threshold warnings fire correctly
- [ ] `go test ./internal/validate/ -run TestDecompositionQuality_NoWarnings` passes
- [ ] `go test ./internal/validate/...` passes (full suite including existing tests)

**Depends on**
Bead 2

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Research doc exists with citation, findings, metrics, thresholds | Bead 1 verification (file exists with required content) |
| Instruct template includes Decomposition Heuristics section | Bead 1 verification (`mindspec instruct` output) |
| Each heuristic framed as question with research "why" | Bead 1 verification (manual check of template content) |
| `mindspec instruct` in plan mode emits heuristics | Bead 1 verification |
| `ParseBeadSections()` returns `StepLines` | Bead 2 verification (`TestParseBeadSections_StepLines`) |
| `ExtractPathRefs()` extracts Go paths correctly | Bead 2 verification (`TestExtractPathRefs`) |
| Warns when R_scope > 0.50 | Bead 3 verification (`TestDecompositionQuality`) |
| Warns when R_scope < 0.15 with >2 beads | Bead 3 verification (`TestDecompositionQuality`) |
| Warns when dependency chain depth > 3 | Bead 3 verification (`TestDecompositionQuality`) |
| Warns when parallelism ratio < 0.25 | Bead 3 verification (`TestDecompositionQuality`) |
| Warns when bead count > 6 | Bead 3 verification (`TestDecompositionQuality`) |
| Warnings include metric value and threshold | Bead 3 verification (assertion on warning text) |
| Approved plans skip decomposition checks | Bead 3 verification (`TestDecompositionQuality_NoWarnings` or separate test) |
| `go test ./internal/validate/...` passes | Bead 3 verification (full suite) |
