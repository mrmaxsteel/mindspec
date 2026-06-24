# Spec 036-init-migrate-plan-apply: Split Greenfield Init from Migration Plan/Apply

## Goal

Make project bootstrapping predictable and low-friction by separating concerns:
- `mindspec init` is fast greenfield scaffolding only (no repository-wide analysis, no LLM calls).
- Existing-repository onboarding moves to a dedicated migration workflow (`mindspec migrate`) with Terraform-like `plan` then `apply` semantics.

The migration workflow must generate a detailed, reviewable change plan with provenance/rationale for every decision before any destructive changes are applied.

## Background

Current `mindspec init` behavior mixes two very different user intents:
1. quick project bootstrap, and
2. brownfield migration of existing documentation.

This creates UX confusion (`--dry-run`, `--brownfield`, `--apply`, prior `--report-only`) and obscures the safety boundary between analysis and mutation.

For migration, users need an explicit review gate similar to Terraform:
- `plan` computes and records intended changes,
- human reviews the plan with rationale and traceability,
- `apply` executes exactly that approved plan.

## Impacted Domains

- **core**: CLI command model split (`init` vs `migrate`) and orchestration behavior.
- **context-system**: deterministic discovery/classification/synthesis and provenance capture.
- **workflow**: explicit review/apply lifecycle and reproducibility guarantees.
- **observability**: run artifacts, plan hashes, source hashes, and apply traceability.

## ADR Touchpoints

- [ADR-0014](../../adr/ADR-0014.md): canonical docs remain under `.mindspec/docs/` and policies under `.mindspec/policies.yml`.
- [ADR-0005](../../adr/ADR-0005.md): migration planning/apply state must be explicit and persisted.
- [ADR-0012](../../adr/ADR-0012.md): compose deterministic stages cleanly; avoid opaque wrappers.

## Requirements

1. `mindspec init` must be greenfield-only and remain fast; it must not perform repo-wide migration analysis.
2. `mindspec init` must not invoke LLM providers under any mode.
3. Brownfield onboarding must move to a dedicated command family: `mindspec migrate`.
4. `mindspec migrate plan` must perform full migration analysis (deterministic rules + LLM assistance for low-confidence decisions).
5. `mindspec migrate plan` must produce a detailed plan artifact before any canonical docs/archive writes occur.
6. Plan artifacts must be written under `.mindspec/migrations/<run-id>/` and include machine + human formats (e.g. `plan.json`, `plan.md`).
7. Plan output must include per-change provenance:
   - source file paths,
   - source hashes,
   - target canonical path,
   - action (`create`, `update`, `merge`, `split`, `archive-only`, `drop`),
   - rationale for the action,
   - confidence and whether LLM contributed.
8. Plan output must include traceable reasoning for merges/splits (e.g. why `XXX.md` and `YYY.md` map to `ZZZ.md`).
9. `mindspec migrate apply` must consume a previously generated plan and execute only that plan.
10. `mindspec migrate apply` must not perform new LLM calls; all inference belongs to `plan`.
11. Apply must verify source hashes from the approved plan and fail with drift diagnostics if inputs changed.
12. Apply must be transactional (stage -> validate -> atomic promote -> archive) with rollback-safe failure behavior.
13. Archive behavior must remain explicit (`copy` or `move`) and be executed only during apply.
14. Migration lineage manifest must map `source -> canonical -> archive` with hashes and run metadata.
15. `mindspec migrate plan` must support JSON output for tooling and human-readable output for reviewers.
16. Command semantics must be clear and minimal:
   - `mindspec init` (greenfield apply)
   - `mindspec init --dry-run` (greenfield no-write)
   - `mindspec migrate plan` (brownfield analysis + plan generation)
   - `mindspec migrate apply --run-id <id>` (apply reviewed plan)
17. Legacy `mindspec init --brownfield ...` flags must be removed (hard break acceptable).
18. `mindspec doctor` must validate migration artifacts required for plan/apply traceability.
19. Dogfooding requirement: this repository must adopt the new command model and execute migration via `migrate plan` then `migrate apply` as proof.

## Scope

### In Scope

- CLI command surface redesign:
  - `cmd/mindspec/init.go` limited to greenfield bootstrap.
  - new `cmd/mindspec/migrate.go` with `plan` and `apply` subcommands.
- Migration plan artifact schema and rendering (`.mindspec/migrations/<run-id>/plan.json|plan.md`).
- Deterministic + LLM-assisted planning pipeline for migration decisions.
- Apply engine that executes approved plans with drift checks and transactional writes.
- Lineage/provenance manifest updates.
- Tests and docs for the new workflow.
- Dogfooding this repo through the new flow.

### Out of Scope

- Non-markdown input formats (PDF, DOCX, SaaS wiki connectors).
- Interactive merge-resolution UI.
- Auto-approval of migration plans.
- Continuous scheduled re-migration.

## Non-Goals

- Preserving brownfield behavior under `mindspec init` for backward compatibility.
- Running implicit migration side effects during `init`.
- Hiding LLM-driven classification decisions from users.

## Command UX Contract

- `init` means bootstrap only.
- `migrate plan` means compute + explain what will change.
- `migrate apply` means execute a reviewed plan.
- No hidden mode switching via overloaded flags.

## Plan/Apply Data Contract

Plan artifacts under `.mindspec/migrations/<run-id>/` must include:
- `inventory.json`
- `classification.json`
- `extraction.json`
- `plan.json`
- `plan.md`
- `validation.json`
- `state.json`

Apply artifacts must include:
- `apply.json` with timestamps, promoted paths, archive results, and drift checks
- updated `.mindspec/lineage/manifest.json`

## Acceptance Criteria

- [ ] `mindspec init` performs greenfield scaffold only and triggers no migration scanning/LLM behavior.
- [ ] `mindspec init --brownfield` is rejected as unknown/invalid usage.
- [ ] `mindspec migrate plan` generates plan artifacts without mutating canonical docs or archive trees.
- [ ] Plan output clearly shows per-operation provenance and rationale.
- [ ] Plan output explicitly identifies merge/split decisions and their justification.
- [ ] `mindspec migrate apply --run-id <id>` applies only the approved plan and performs no new LLM calls.
- [ ] Apply fails with clear drift diagnostics if any source hash differs from plan-time values.
- [ ] Successful apply writes canonical docs, archive outputs, and lineage manifest updates transactionally.
- [ ] Re-running `migrate apply` on an already-applied unchanged plan is idempotent.
- [ ] `mindspec doctor` validates required migration artifacts and canonical outputs.
- [ ] This repository demonstrates the new user path (`migrate plan` then `migrate apply`) as dogfood proof.

## Validation Proofs

- `mindspec init --help`: shows greenfield-only semantics (no brownfield/migration flags).
- `mindspec migrate plan --help`: documents analysis + review workflow and plan outputs.
- `mindspec migrate apply --help`: documents run-id apply contract and drift checks.
- `mindspec migrate plan --json`: emits machine-readable plan with provenance fields.
- `mindspec migrate apply --run-id <id>` with unchanged sources: succeeds and updates canonical outputs.
- `mindspec migrate apply --run-id <id>` after source edit: fails with drift report.
- `mindspec doctor`: passes on migrated repository.

## Open Questions

All resolved for planning:
- [x] Rename from "brownfield" user terminology to `migrate` command family.
- [x] Keep `init` fast and greenfield-only.
- [x] Require explicit plan review before apply.
- [x] Perform LLM-assisted reasoning only during plan, never during apply.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-17
- **Notes**: Approved via mindspec approve spec