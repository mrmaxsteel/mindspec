---
adr_citations:
    - id: ADR-0014
      sections:
        - Decision
        - Decision Details
    - id: ADR-0005
      sections:
        - Decision
        - State File Schema (v1)
    - id: ADR-0012
      sections:
        - Decision
        - Principles
approved_at: "2026-02-17T22:50:44Z"
approved_by: user
spec_id: 036-init-migrate-plan-apply
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: cmd/mindspec/init.go, cmd/mindspec/root.go, new cmd/mindspec/migrate*.go
      title: 'Split CLI contract: init-only and new migrate command'
      verify:
        - init help no longer contains brownfield/migration flags
        - migrate command exposes plan/apply subcommands with clear usage
    - depends_on:
        - 1
      id: 2
      scope: internal/brownfield (renamed/reframed migration package), plan structs, JSON/Markdown renderers
      title: Plan artifact schema and provenance model
      verify:
        - plan.json includes source hashes, actions, targets, rationale, confidence
        - plan.md shows human-readable merge/split justifications
    - depends_on:
        - 2
      id: 3
      scope: discovery/classification/extraction/synthesis orchestration for migrate plan
      title: Deterministic + LLM-assisted migrate plan pipeline
      verify:
        - deterministic stages are stable for unchanged inputs
        - LLM is used only for low-confidence decisions during plan
    - depends_on:
        - 3
      id: 4
      scope: apply engine, staging/promotion/archive, source-hash drift guardrails
      title: Plan-driven migrate apply with drift checks and transactions
      verify:
        - apply performs zero LLM calls
        - apply fails on source drift and succeeds when hashes match
        - apply updates canonical docs/archive/lineage transactionally
    - depends_on:
        - 4
      id: 5
      scope: cmd tests, brownfield/migration tests, doctor checks, error messaging
      title: Validation, doctor, and regression tests
      verify:
        - go test ./... passes
        - doctor validates required migration artifacts and canonical outputs
        - CLI errors are explicit for invalid command/flag combinations
    - depends_on:
        - 5
      id: 6
      scope: run migrate plan + apply on this repo, update docs/examples to new UX
      title: Dogfood command model in this repository
      verify:
        - repo usage docs show init-only + migrate plan/apply flow
        - dogfood evidence artifacts are committed and reproducible
---

# Plan: Spec 036 — Split Greenfield Init from Migration Plan/Apply

**Spec**: [spec.md](spec.md)

## Context

This plan formalizes a Terraform-like migration workflow:
- `mindspec migrate plan` computes and records all intended changes, including LLM-assisted decisions where deterministic rules are insufficient.
- Human reviews the plan (with provenance/rationale).
- `mindspec migrate apply --run-id <id>` applies exactly that plan, with no new inference.

The plan also removes overloaded `init` semantics so bootstrap stays fast, predictable, and safe.

## ADR Fitness

| ADR | Verdict | Notes |
|-----|---------|-------|
| ADR-0014 | Conform | Canonical locations remain `.mindspec/docs/` + `.mindspec/policies.yml`; this spec changes command UX and execution lifecycle, not storage targets. |
| ADR-0005 | Conform | Plan/apply artifacts and state transitions remain explicit under `.mindspec/migrations/<run-id>/`. |
| ADR-0012 | Conform | Pipeline stages remain explicit and composable; no opaque wrappers around decisioning/apply behavior. |

## Bead 036-A: Split CLI contract (`init` vs `migrate`)

**Scope**: Make `init` greenfield-only and introduce `migrate` command group.

**Steps**:
1. Remove migration flags/paths from `mindspec init`.
2. Add `mindspec migrate` root with `plan` and `apply` subcommands.
3. Update help/usage text to remove ambiguity.
4. Add command-level tests for valid/invalid combinations.

**Verification**:
- [ ] `mindspec init --help` contains only greenfield semantics.
- [ ] `mindspec migrate plan --help` and `mindspec migrate apply --help` are explicit.
- [ ] Deprecated brownfield flags are rejected with actionable diagnostics.

**Depends on**: nothing

## Bead 036-B: Plan artifact schema and explainability output

**Scope**: Define durable migration plan artifact contracts.

**Steps**:
1. Define plan operation schema with `sources`, `source_hashes`, `target`, `action`, `rationale`, `confidence`, `llm_used`.
2. Add machine-readable output (`plan.json`).
3. Add human-readable output (`plan.md`) with grouped operations and merge/split reasoning.
4. Include run metadata (`model`, prompt version, deterministic stage hashes).

**Verification**:
- [ ] `plan.json` includes required provenance fields for every operation.
- [ ] `plan.md` is review-friendly and explains decision rationale.
- [ ] Plan artifact schema is stable and test-covered.

**Depends on**: 036-A

## Bead 036-C: Deterministic + LLM-assisted `migrate plan`

**Scope**: Implement end-to-end planning pipeline.

**Steps**:
1. Run deterministic discovery and classification in stable path order.
2. Route only low-confidence docs to LLM classification/synthesis assistance.
3. Persist intermediate artifacts (`inventory`, `classification`, `extraction`) and final plan artifacts.
4. Record unresolved/conflict decisions explicitly in plan output.

**Verification**:
- [ ] Unchanged inputs produce stable deterministic artifacts.
- [ ] LLM usage appears only for low-confidence operations.
- [ ] Plan generation succeeds with rich diagnostics and provenance.

**Depends on**: 036-B

## Bead 036-D: Plan-driven `migrate apply`

**Scope**: Execute reviewed plans safely and deterministically.

**Steps**:
1. Load plan by run-id and validate schema/version.
2. Re-hash source inputs and compare against plan-time hashes.
3. Fail fast on drift with actionable diff output.
4. Stage canonical outputs, validate, atomically promote, then archive according to policy.
5. Emit apply report and lineage updates.

**Verification**:
- [ ] Apply performs zero LLM calls.
- [ ] Drifted source input blocks apply.
- [ ] Successful apply updates canonical docs and archive atomically.

**Depends on**: 036-C

## Bead 036-E: Quality gates and documentation updates

**Scope**: Ensure test coverage and operational checks align with new model.

**Steps**:
1. Update unit/integration tests for new command surface.
2. Extend doctor checks for required plan/apply artifacts and consistency checks.
3. Update usage docs to demonstrate `init` vs `migrate plan/apply`.
4. Add regression tests for no-LLM init and no-LLM apply behavior.

**Verification**:
- [ ] `go test ./...` passes.
- [ ] `mindspec doctor` validates post-migration state.
- [ ] Docs no longer instruct users to run brownfield flags on `init`.

**Depends on**: 036-D

## Bead 036-F: Dogfood in this repository

**Scope**: Apply the new flow to the MindSpec repo itself.

**Steps**:
1. Run `mindspec migrate plan` in this repo and review `plan.md`/`plan.json`.
2. Run `mindspec migrate apply --run-id <id>` after review.
3. Verify canonical docs, archive, lineage, and doctor/validate checks.
4. Commit dogfood artifacts and command-model documentation updates.

**Verification**:
- [ ] Dogfood run demonstrates explicit plan review before apply.
- [ ] Repository passes `mindspec doctor` and relevant validate checks post-apply.
- [ ] Migration evidence artifacts are committed.

**Depends on**: 036-E

## Dependency Graph

```text
036-A (CLI split)
  -> 036-B (plan schema + renderers)
    -> 036-C (migrate plan pipeline)
      -> 036-D (plan-driven apply)
        -> 036-E (tests + doctor + docs)
          -> 036-F (dogfood in this repo)
```
