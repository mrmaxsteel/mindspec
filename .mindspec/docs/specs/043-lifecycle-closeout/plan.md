---
adr_citations:
    - id: ADR-0002
      sections:
        - Decision
        - Decision Details
    - id: ADR-0008
      sections:
        - Decision
        - Consequences
    - id: ADR-0013
      sections:
        - Decision
        - How mindspec uses it
    - id: ADR-0015
      sections:
        - Decision
        - Spec-to-molecule binding
approved_at: "2026-02-20T13:16:55Z"
approved_by: user
last_updated: 2026-02-20T00:00:00Z
spec_id: 043-lifecycle-closeout
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: internal/approve/spec.go, .mindspec/docs/user/templates/spec.md
      title: Canonical spec approval frontmatter
      verify:
        - go test ./internal/approve -run ApproveSpec
        - mindspec validate spec 043-lifecycle-closeout
    - depends_on:
        - 1
      id: 2
      scope: internal/approve/impl.go
      title: Molecule-wide close-out in approve impl
      verify:
        - go test ./internal/approve -run ApproveImpl
        - go test ./cmd/mindspec -run approve
    - depends_on:
        - 1
      id: 3
      scope: internal/validate/spec.go, internal/validate/plan.go
      title: Frontmatter-gate consistency validators
      verify:
        - go test ./internal/validate -run 'Spec|Plan'
        - mindspec validate spec 043-lifecycle-closeout
    - depends_on:
        - 2
        - 3
      id: 4
      scope: .mindspec/docs/core/USAGE.md, .mindspec/docs/core/MODES.md, .mindspec/docs/user/templates/spec.md
      title: Docs and template contract alignment
      verify:
        - go test ./internal/instruct -run Template
        - mindspec validate plan 043-lifecycle-closeout
---

# Plan: Spec 043 — Lifecycle Close-Out Reconciliation

**Spec**: [spec.md](spec.md)

## ADR Fitness

### ADR-0002: Beads as passive tracking substrate
Verdict: Conform. The plan keeps MindSpec as orchestrator and uses Beads only as execution state storage. Molecule reconciliation closes existing Beads records but does not shift orchestration authority into Beads.

### ADR-0008: Human gates as dependency markers
Verdict: Conform with refinement. The plan preserves dual signals (document approval + gate status) and adds validator checks to detect drift when users bypass CLI commands and edit approval state directly.

### ADR-0013: Formula-defined lifecycle orchestration
Verdict: Conform. Reconciliation is data-driven off formula-poured molecule metadata (`molecule_id`, `step_mapping`) rather than hardcoded step assumptions, so formula-driven lifecycle remains canonical.

### ADR-0015: Molecule-derived lifecycle state
Verdict: Conform. Closing parent + all mapped lifecycle steps on `approve impl` aligns with per-spec molecule-derived completion semantics (`all closed = done`) while `state.json` remains a convenience cursor.

## Testing Strategy

- Unit tests in `internal/approve/*_test.go` for spec approval frontmatter writing, full-molecule close-out behavior, idempotent reruns, and warning-on-partial-failure behavior.
- Unit tests in `internal/validate/*_test.go` for spec/plan frontmatter to gate consistency warnings.
- Regression coverage for legacy specs that still use markdown-only approval records.
- CLI-level smoke checks using `mindspec validate spec <id>` and `mindspec validate plan <id>` for warning surface behavior.
- Full suite confirmation via `make test`.

## Provenance

| Spec Acceptance Criterion | Bead / Verification |
|---|---|
| `approve impl` closes parent epic + all lifecycle steps | Bead 043-B; `go test ./internal/approve -run ApproveImpl` |
| Already-closed molecule members are treated as success | Bead 043-B; `go test ./internal/approve -run ApproveImpl` |
| Unexpected per-member closure failures warn and continue | Bead 043-B; `go test ./internal/approve -run ApproveImpl` |
| `approve spec` writes canonical YAML approval fields | Bead 043-A; `go test ./internal/approve -run ApproveSpec` |
| `validate spec` warns on `status: Approved` with open spec gate | Bead 043-C; `go test ./internal/validate -run Spec` |
| `validate plan` warns on `status: Approved` with open plan gate | Bead 043-C; `go test ./internal/validate -run Plan` |
| Legacy spec handling is graceful (migration or actionable warning) | Bead 043-A and 043-C; `go test ./internal/approve -run ApproveSpec`, `go test ./internal/validate -run Spec` |
| USAGE docs define molecule-wide close-out contract | Bead 043-D; `mindspec validate plan 043-lifecycle-closeout` |
| MODES docs distinguish `complete` vs `approve impl` | Bead 043-D; `mindspec validate plan 043-lifecycle-closeout` |
| Spec template documents canonical approval frontmatter | Bead 043-A and 043-D; `go test ./internal/instruct -run Template` |

## Bead 043-A: Canonical spec approval frontmatter

Add canonical YAML approval fields to spec artifacts and approval write-path.

**Scope**: `internal/approve/spec.go`, `.mindspec/docs/user/templates/spec.md`, parser helpers used by spec approval.

**Steps**:
1. Add helper logic in `internal/approve/spec.go` to parse/write YAML frontmatter in `spec.md`.
2. Update `ApproveSpec` to write `status: Approved`, `approved_at`, and `approved_by` in frontmatter.
3. Preserve existing markdown `## Approval` section for backward readability while making YAML canonical.
4. Add legacy-path handling when approval frontmatter is absent (migrate in place or emit actionable warning).
5. Update `.mindspec/docs/user/templates/spec.md` to include canonical approval frontmatter fields for newly initialized specs.
6. Add/extend tests in `internal/approve/spec_test.go` for canonical write behavior and legacy migration handling.

**Verification**:
- [ ] `go test ./internal/approve -run ApproveSpec` passes
- [ ] `go test ./internal/approve -run Frontmatter` passes

**Depends on**: nothing

## Bead 043-B: Molecule-wide reconciliation on impl approval

Close the entire formula-poured lifecycle atomically (best-effort, idempotent) during implementation approval.

**Scope**: `internal/approve/impl.go`, `internal/approve/impl_test.go`.

**Steps**:
1. Resolve close-out targets from metadata (`molecule_id` plus unique `step_mapping` values) instead of hardcoded step names.
2. Update `ApproveImpl` to iterate all targets and attempt closure for each member.
3. Treat already-closed members as success with no warning.
4. For unexpected member-level closure failures, append warnings and continue closing remaining targets.
5. Keep review-mode/spec-id safety checks unchanged before any close-out actions.
6. Add tests for full closure set, idempotent rerun behavior, and partial-failure warning behavior.

**Verification**:
- [ ] `go test ./internal/approve -run ApproveImpl` passes
- [ ] `go test ./internal/approve -run Impl` passes

**Depends on**: Bead 043-A

## Bead 043-C: Frontmatter ↔ gate consistency validation

Detect and surface approval drift when users edit frontmatter manually without running approval commands.

**Scope**: `internal/validate/spec.go`, `internal/validate/plan.go`, `internal/validate/*_test.go`.

**Steps**:
1. Add spec validator logic that reads `spec.md` frontmatter `status` and compares it to `spec-approve` gate state from molecule mapping.
2. Add plan validator logic that compares `plan.md` frontmatter `status` against `plan-approve` gate state.
3. Emit warnings (not hard errors) on approved-frontmatter/open-gate mismatch to keep checks actionable but non-blocking.
4. Handle missing/legacy metadata paths with explicit warnings instead of panic/failure.
5. Add tests for matching states, mismatched states, and missing metadata behavior.

**Verification**:
- [ ] `go test ./internal/validate -run Spec` passes
- [ ] `go test ./internal/validate -run Plan` passes
- [ ] `mindspec validate spec 043-lifecycle-closeout` runs without validator crashes

**Depends on**: Bead 043-A

## Bead 043-D: Documentation and mode contract alignment

Align user docs and templates with the new close-out and canonical frontmatter contract.

**Scope**: `.mindspec/docs/core/USAGE.md`, `.mindspec/docs/core/MODES.md`, `.mindspec/docs/user/templates/spec.md`.

**Steps**:
1. Update USAGE Phase 9 to state that `approve impl` reconciles parent + all molecule steps.
2. Update MODES implementation exit text to distinguish `mindspec complete` (per-bead progress) from `mindspec approve impl` (lifecycle close-out).
3. Ensure docs describe validator behavior for frontmatter/gate drift as warning-level feedback.
4. Cross-check wording for consistency with ADR-0013 and ADR-0015 terminology.
5. Add/adjust doc-related tests if present; otherwise verify via existing template/instruct coverage.

**Verification**:
- [ ] `go test ./internal/instruct -run Template` passes
- [ ] `mindspec validate plan 043-lifecycle-closeout` passes

**Depends on**: Bead 043-B, Bead 043-C

## Dependency Graph

```text
043-A (canonical spec frontmatter)
  ├── 043-B (impl molecule reconciliation)
  └── 043-C (validator consistency checks)
        └── 043-D (docs + template alignment)
```
