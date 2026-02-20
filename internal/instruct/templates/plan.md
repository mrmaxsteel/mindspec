# MindSpec — Plan Mode

**Active Spec**: `{{.ActiveSpec}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## Objective

Turn the approved spec into bounded, executable work chunks (implementation beads).

## Required Review (before planning)

1. Read accepted ADRs for impacted domains
2. Read domain docs (`overview.md`, `architecture.md`, `interfaces.md`)
3. Check Context Map (`.mindspec/docs/context-map.md`) for neighboring context contracts
4. Verify existing constraints and invariants
5. **ADR Fitness Evaluation**: After reviewing ADRs, actively evaluate whether each relevant ADR still represents the best architectural choice for the work being planned. Do not blindly conform — if a better design would diverge from an accepted ADR, propose the divergence with justification. Prefer adherence when ADRs are sound; propose superseding when they are not. Document your evaluation in the `## ADR Fitness` section of the plan.

## Permitted Actions

- Create/edit `.mindspec/docs/specs/{{.ActiveSpec}}/plan.md`
- Define implementation beads as work chunks in the plan (the spec-lifecycle formula creates the molecule at spec-init; implementation beads are tracked via the molecule's step mapping)
- Propose new ADRs if divergence detected (`mindspec adr create --supersedes <old-id>`)
- Update documentation to clarify scope

## Forbidden Actions

- Writing implementation code (`cmd/`, `internal/`, or equivalent)
- Writing test code
- Widening scope beyond the spec's defined user value

## Required Output

Implementation beads, each with:
- Small scope (one slice of value)
- 3-7 step micro-plan
- Explicit verification steps that reference **concrete test artifacts** (test file paths like `_test.go`, test commands like `make test`, `go test`, `pytest`, or `mindspec validate`)
- Dependencies between beads
- ADR citations

Required plan sections:
- `## ADR Fitness` — evaluate whether each relevant ADR remains the best choice; if no ADRs are relevant, explain why (this section is **required** even when no ADRs apply)
- `## Testing Strategy` — declare the overall test approach (unit, integration, e2e) and shared test infrastructure
- `## Provenance` — map each spec acceptance criterion to the bead verification steps that satisfy it (output provenance)

## Human Gates

- **Plan approval**: Run `mindspec approve plan <id>` when the plan is ready
- **ADR divergence**: If a better design would diverge from an accepted ADR, **stop planning**. Present: (1) which ADR, (2) why it should be superseded, (3) the proposed alternative. Wait for human approval before proceeding. Use `mindspec adr create --supersedes <ADR-NNNN>` to create the superseding ADR once approved.

## Next Action
{{- if .PlanApproved}}

Plan is approved. Commit approval artifacts first, then run `mindspec next` to claim the first bead and enter Implementation Mode. `mindspec next` requires a clean working tree and will fail on uncommitted changes. Do NOT manually set state to implement — `mindspec next` handles bead selection and state transition together.
{{- else}}

Complete the plan at `.mindspec/docs/specs/{{.ActiveSpec}}/plan.md`, then run `mindspec approve plan {{.ActiveSpec}}`.
{{- end}}

## Session Close

Before ending a session: commit all changes, run quality gates if code changed, update bead status, and push to remote (if configured). Work is not complete until changes are committed and pushed.
