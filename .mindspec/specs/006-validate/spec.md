# Spec 006: Workflow Validation (`mindspec validate`)

## Goal

Give agents and humans a single command to validate that specs, plans, and documentation meet MindSpec quality standards. `mindspec validate` consolidates doc-sync checks, spec/plan structural validation, and ADR citation verification — catching problems before they reach approval gates.

## Background

Today, quality checks are manual: reviewers eyeball spec completeness, plan coverage, and doc-sync compliance. The `/spec-approve` and `/plan-approve` workflows include inline validation, but those checks are embedded in skill prompts and not reusable outside the approval flow. A standalone `validate` command lets agents self-check before requesting approval and lets humans audit artifacts at any time.

The backlog defines three sub-commands:
- `mindspec validate docs` — doc-sync compliance
- `mindspec validate spec <id>` — spec quality checks
- `mindspec validate plan <id>` — plan quality checks

## Impacted Domains

- **workflow**: Validation becomes a formal workflow step, usable pre-approval
- **core**: CLI command registration, replaces the validate stub

## ADR Touchpoints

- [ADR-0003](../../adr/ADR-0003.md): Centralized Agent Instruction Emission — defines `mindspec validate` as part of the CLI contract
- [ADR-0005](../../adr/ADR-0005.md): Explicit State Tracking — validate can check state consistency

## Requirements

1. **Spec validation**: Check that a spec has all required sections filled (Goal, Impacted Domains, ADR Touchpoints, Requirements, Scope, Acceptance Criteria, Approval), open questions are resolved, and acceptance criteria are specific and measurable (not vague)
2. **Plan validation**: Check that a plan has YAML frontmatter with required fields, at least one implementation bead defined, each bead has a micro-plan (steps), each bead has verification steps, inter-bead dependencies are declared, ADRs are cited, and bead IDs in frontmatter exist in Beads
3. **Doc-sync validation**: Given a list of changed files (from git diff or explicit), check that corresponding documentation exists and has been updated
4. **Machine-readable output**: Support `--format=json` for programmatic consumption
5. **Exit codes**: Return non-zero exit code when validation fails, for use in scripts and hooks
6. **Aggregated reporting**: Show all issues found, not just the first failure

## Scope

### In Scope

- `internal/validate/` package: spec, plan, and doc-sync validation logic
- `cmd/mindspec/validate.go`: replace the stub in `cmd/mindspec/stubs.go`
- `mindspec validate spec <id>`: structural quality checks on spec.md
- `mindspec validate plan <id>`: structural quality checks on plan.md
- `mindspec validate docs [--diff=<ref>]`: doc-sync compliance checks

### Out of Scope

- Semantic validation (e.g., "is this acceptance criterion actually good?") — that's human judgment
- ADR divergence detection at the code level (Spec 014)
- Automated fixing of validation issues
- Pre-commit hook integration (can be added later using the exit code)

## Non-Goals

- Replacing human review — validate catches structural issues, not design quality
- Blocking commits — validate reports issues but does not prevent git operations
- Validating code correctness — that's what tests are for

## Acceptance Criteria

- [ ] `mindspec validate spec <id>` reports missing/empty required sections
- [ ] `mindspec validate spec <id>` flags unresolved open questions
- [ ] `mindspec validate spec <id>` warns on vague acceptance criteria (contains "works correctly", "is fast", "properly handles", etc.)
- [ ] `mindspec validate plan <id>` checks YAML frontmatter for required fields (status, spec_id, version)
- [ ] `mindspec validate plan <id>` verifies at least one bead section exists with steps and verification
- [ ] `mindspec validate plan <id>` checks that ADR citations are present
- [ ] `mindspec validate plan <id>` verifies bead IDs in frontmatter exist in Beads
- [ ] `mindspec validate docs` compares changed Go files against doc update expectations
- [ ] All sub-commands support `--format=json` for structured output
- [ ] All sub-commands return non-zero exit code on validation failure
- [ ] `make test` passes with tests covering each validation check
- [ ] Existing commands (`instruct`, `next`, `state`, `doctor`, `glossary`, `context`) are unaffected

## Validation Proofs

- `make build && ./bin/mindspec validate spec 005-next`: Validates an existing approved spec (should pass)
- `./bin/mindspec validate plan 005-next`: Validates an existing approved plan (should pass)
- `./bin/mindspec validate spec 006-validate`: Validates this spec itself
- `make test`: All tests pass including validate package tests

## Open Questions

None — all resolved.

## Design Decisions (resolved during spec)

- **Doc-sync heuristic**: Convention-based for v1. Map `internal/<pkg>/` changes to `docs/domains/<domain>/` updates, `cmd/mindspec/` changes to CLAUDE.md/CONVENTIONS.md. Report missing doc-sync as warnings (not hard errors). Configurable mapping deferred to a later spec if conventions prove insufficient.
- **Bead ID verification**: `validate plan` checks that bead IDs in frontmatter actually exist in Beads (via `bd show <id> --json`), not just structural completeness.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-12
- **Notes**: Approved via /spec-approve workflow
