---
molecule_id: mindspec-mol-630
step_mapping:
    implement: mindspec-mol-018
    plan: mindspec-mol-nuf
    plan-approve: mindspec-mol-nfn
    review: mindspec-mol-4nu
    spec: mindspec-mol-vo5
    spec-approve: mindspec-mol-92a
    spec-lifecycle: mindspec-mol-630
---

# Spec 039-plan-quality-gates: Plan Quality Gates

## Goal

Strengthen `mindspec validate plan` so that architecture governance (ADR evaluation) and test-driven development planning are **hard errors** that block plan approval — not soft warnings the agent can ignore. Plans that pass validation should give the human reviewer confidence that architectural fitness was evaluated and every bead has a testable verification strategy.

## Background

Today, `mindspec validate plan` checks structural completeness (frontmatter fields, bead sections with steps/verification/dependencies) and ADR citations. However:

1. **ADR governance is advisory** — missing ADR citations and a missing `## ADR Fitness` section produce warnings, not errors. An agent can get a plan approved with zero architectural evaluation.
2. **Testing strategy is unchecked** — bead verification steps are counted (`- [ ]` items exist?) but not checked for testability. A bead can pass validation with verification like "- [ ] Confirm it works" — no test files, no `make test`, no concrete assertions.
3. **Output provenance is missing** — MindSpec already tracks *input provenance* (context packs record what was loaded and why), but there's no *output provenance* — no check that the plan's bead verification steps trace back to the spec's acceptance criteria. The provenance chain is open-ended: we know what informed the plan, but not what spec requirements the plan satisfies.

The human gate (plan-approve) is the judgment layer — but it needs a structural scaffold to review against. Without hard gates, the human has to catch everything manually.

## Impacted Domains

- validation: new and promoted checks in `internal/validate/plan.go`
- workflow: plan-mode instruct template updated to reference new requirements

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): instruct emission — plan template must communicate new requirements to agents
- [ADR-0008](../../adr/ADR-0008.md): human gates — this spec strengthens what the human gate can rely on, not replaces it
- [ADR-0013](../../adr/ADR-0013.md): formula lifecycle — plan-approve step calls `mindspec approve plan` which runs validation; tighter validation means the formula gate is more meaningful

## Requirements

1. **Promote ADR Fitness section to error** — `mindspec validate plan` must emit an error (not warning) when no `## ADR Fitness` section is present. This is the primary architecture governance gate. When no ADRs are relevant to the change, the agent must still include the section and explain *why* no ADRs apply — the section's presence proves the evaluation happened.
2. **Promote ADR citations to warning-with-fitness-dependency** — empty `adr_citations` in frontmatter remains a warning *only if* the `## ADR Fitness` section is present (the section explains why none apply). If both `adr_citations` is empty *and* `## ADR Fitness` is missing, emit an error — the plan shows no evidence of architectural evaluation.
3. **Require Testing Strategy section** — `mindspec validate plan` must emit an error when the plan lacks a `## Testing Strategy` section. This section declares the overall test approach (unit, integration, e2e) and any shared test infrastructure.
4. **Require testable bead verification** — each bead's `**Verification**` section must contain at least one item referencing a concrete test artifact: a test file path (e.g., `_test.go`, `.test.ts`), a test command (e.g., `make test`, `go test`, `pytest`), or `mindspec validate`. Verification items that don't reference any test artifact produce an error.
5. **Output provenance** — `mindspec validate plan` must warn when the plan does not contain a `## Provenance` section that maps spec acceptance criteria to bead verification steps. This closes the provenance loop: input provenance (context packs) records what informed the work; output provenance records what spec requirements the plan satisfies.
6. **Update plan-mode instruct template** — the plan.md template in `internal/instruct/templates/plan.md` must list the new required sections and verification expectations under "Required Output."
7. **Backwards compatibility** — existing approved plans (status: Approved) must not retroactively fail validation. The new checks apply only when `status` is `Draft` or empty.

## Scope

### In Scope
- `internal/validate/plan.go` — new and promoted checks
- `internal/validate/plan_test.go` — test coverage for all new checks
- `internal/instruct/templates/plan.md` — updated guidance

### Out of Scope
- Changes to `mindspec validate spec` (spec validation is a separate concern)
- LLM-based evaluation of plan quality (judgment stays with the human gate)
- Changes to the beads formula or gate types
- Enforcement of *which* tests to write (that's implementation-mode concern)

## Non-Goals

- Replacing human judgment with automated quality scoring
- Enforcing test coverage thresholds or specific testing frameworks
- Validating that referenced test files actually exist (that's an implementation-time check)
- Modifying the spec-lifecycle formula steps or gate types

## Acceptance Criteria

- [ ] `mindspec validate plan` without `## ADR Fitness` section produces an error (not warning)
- [ ] `mindspec validate plan` with empty `adr_citations` but present `## ADR Fitness` section produces a warning (not error)
- [ ] `mindspec validate plan` with empty `adr_citations` and missing `## ADR Fitness` produces an error
- [ ] `mindspec validate plan` without `## Testing Strategy` section produces an error
- [ ] `mindspec validate plan` with a bead verification item lacking any test artifact reference produces an error for that bead
- [ ] `mindspec validate plan` without `## Provenance` section produces a warning
- [ ] Existing approved plans (status: Approved in frontmatter) skip the new checks and still pass validation
- [ ] Plan-mode instruct template lists `## Testing Strategy`, `## Provenance`, and testable verification as required output
- [ ] All new checks have corresponding unit tests in `plan_test.go`
- [ ] `make test` passes with no regressions

## Validation Proofs

- `make test`: all tests pass, including new plan validation tests
- `mindspec validate plan 039-plan-quality-gates`: the plan for this spec itself passes the strengthened validation (dogfooding)

## Open Questions

- [x] Should ADR citations be an error or remain a warning? → **Conditional.** Empty citations are a warning if `## ADR Fitness` explains why none apply. Missing both is an error — the plan shows no architectural evaluation at all.
- [x] Should the testable-verification check be strict (require file path patterns) or loose (any mention of "test")? → **Pattern-based.** Check for `_test.go`, `.test.ts`, `.test.js`, `.spec.ts`, `make test`, `go test`, `pytest`, `npm test`, `mindspec validate`. Extensible list, not regex-on-"test".
- [x] Should output provenance be an error or warning? → **Warning.** It's high-value but harder to validate structurally — we don't parse spec AC programmatically today.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-20
- **Notes**: Approved via mindspec approve spec