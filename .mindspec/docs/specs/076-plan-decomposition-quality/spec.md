---
approved_at: "2026-03-08T16:08:52Z"
approved_by: user
status: Approved
---
# Spec 076-plan-decomposition-quality: Research-Informed Plan Decomposition Guidance

## Goal

Improve plan quality at authoring time by embedding research-backed decomposition heuristics into the plan-mode instruct template, and add a deterministic validator as a feedback loop. The primary lever is **guidance that shapes how the agent decomposes work before writing** — the validator confirms the guidance was followed.

Based on Kim et al. (2025) "Towards a Science of Scaling Agent Systems" (arXiv:2512.08296).

## Background

### Research basis

Kim et al. evaluated 180 configurations of multi-agent systems across 5 architectures and 4 benchmarks. Their key findings relevant to plan decomposition:

1. **Sequential dependency chains degrade performance -39% to -70%** (PlanCraft benchmark). Tasks with strict sequential interdependence — where each step modifies state the next step depends on — universally degrade under multi-agent coordination. The coordination overhead fragments reasoning capacity without decomposition benefit.

2. **Optimal redundancy R≈0.41; R>0.50 hurts** (r=-0.136, p=0.004). Some overlap between work units is healthy for context continuity, but too much is wasteful. High redundancy means agents duplicate work rather than divide it.

3. **3-4 parallel agents is the sweet spot**. Turn scaling follows a power law with exponent 1.724 (super-linear). Per-agent reasoning capacity becomes prohibitively thin beyond 3-4 agents under fixed token budgets.

4. **Capability saturation at ~45% single-agent baseline**. When one agent can already handle a task, splitting it across multiple agents yields negative returns (β=-0.404, p<0.001). Over-decomposing trivial work creates coordination overhead with no benefit.

5. **Tool-heavy tasks suffer 6.3x efficiency penalty** under multi-agent fragmentation (β=-0.267, p<0.001). Token budgets fragment, leaving insufficient capacity per agent for complex tool reasoning.

### Applicability to mindspec

In mindspec, each bead is executed by a separate agent session (analogous to an independent agent in the paper). The plan's bead decomposition directly determines the multi-agent topology. Plans with deep serial chains, excessive bead counts, or high file overlap between beads match the paper's degradation patterns.

### Two levers: guidance and validation

**Guidance (primary)**: The plan-mode instruct template is what the agent reads *before* writing the plan. Embedding concrete decomposition heuristics here directly shapes how work is broken down. This is the high-leverage change — it prevents bad decomposition rather than catching it after the fact.

**Validation (secondary)**: A `checkDecompositionQuality()` function computes metrics from the parsed bead sections and emits warnings when the structure correlates with known degradation patterns. This is the feedback loop — it catches cases where the agent ignored or misapplied the guidance.

### Current state

The plan-mode instruct template (`internal/instruct/templates/plan.md`) specifies bead format (3-7 steps, verification, dependencies) but provides no guidance on *how to decompose* — when to split vs. merge, when dependencies are warranted, or how many beads are appropriate.

`ValidatePlan()` in `internal/validate/plan.go` checks individual bead quality but performs no cross-bead analysis.

## Impacted Domains

- instruct: Plan-mode template gains decomposition heuristics section (primary change)
- validate: New decomposition quality checks as feedback loop (secondary change)

## ADR Touchpoints

- [ADR-0016](../../adr/ADR-0016.md): Bead Creation Timing — decomposition quality directly affects the beads created at plan approval
- [ADR-0003](../../adr/ADR-0003.md): Centralized Agent Instruction Emission — the instruct template is the mechanism for delivering decomposition guidance to agents

## Requirements

### Documentation

1. A research reference document (`.mindspec/docs/research/scaling-agent-systems.md`) must be created summarizing the paper's findings relevant to plan decomposition, including full citation, key metrics, thresholds, and the decision framework. This serves as the canonical reference for the heuristics and thresholds used in the instruct template and validator.

### Guidance (instruct template)

2. The plan-mode instruct template must include a `## Decomposition Heuristics` section with concrete, actionable rules the agent applies while writing the plan
3. The heuristics must include these research-backed rules:
   - **Independence test**: "Can this bead be completed without reading the output of another bead? If yes, don't add a dependency edge."
   - **Merge signal**: "If 3+ beads touch the same files, they should probably be one bead."
   - **Trivial-work test**: "If a bead has 1-2 trivial steps (rename a variable, update a config value), it doesn't justify a separate agent session. Fold it into an adjacent bead."
   - **Target count**: "3-5 beads is the sweet spot. More than 6 needs justification — coordination overhead grows super-linearly."
   - **Serial chain limit**: "Dependency chains deeper than 3 (A→B→C→D) are a red flag. Look for false dependencies or restructure so beads can run in parallel."
   - **Tool grouping**: "Operations requiring many tools or commands in concert (CI setup, build config, multi-file refactors) should stay in a single bead rather than being split across beads that each need the full tool context."
4. The heuristics must be framed as design-time questions the agent asks itself while decomposing, not post-hoc rules to check against
5. Each heuristic must include a brief "why" referencing the research finding that motivates it
6. The instruct template must reference the research doc (`.mindspec/docs/research/scaling-agent-systems.md`) so agents can consult the full context if needed

### Validation (feedback loop)

7. `ParseBeadSections()` must capture raw step lines (`StepLines []string`) in addition to the existing `StepsCount`, so file paths can be extracted from step text
8. A new `ExtractPathRefs(text string) []string` function must extract file/package path references from arbitrary text using regex (matching patterns like `internal/foo/bar.go`, `cmd/mindspec/root.go`, `./internal/foo/...`, `go test ./pkg/...`)
9. `checkDecompositionQuality()` must compute and warn on:
   - **Scope redundancy (R_scope)**: `|paths referenced by >1 bead| / |total unique paths|`. Warn if R > 0.50 ("high bead overlap — consider merging beads that share most files") or R < 0.15 with >2 beads ("low overlap — beads may lack shared context")
   - **Dependency chain depth**: Longest path in the bead dependency DAG. Warn if depth > 3 ("deep serial chain — coordination overhead grows super-linearly")
   - **Parallelism ratio**: `beads with zero inbound deps / total beads`. Warn if < 0.25 ("most beads are serial — check for false dependencies")
   - **Bead count**: Warn if > 6 ("plan has N beads — consider whether decomposition is too fine-grained; 3-5 is optimal")
10. Dependency parsing must reuse the existing `bead\s+(\d+)` regex pattern from `internal/approve/plan.go` to build an adjacency list from `DependsOn` text
11. All decomposition checks must be warnings (not errors) — they are advisory signals, not hard gates
12. All warnings must include the computed metric value and the threshold, so the agent can make an informed decision about whether to restructure

## Scope

### In Scope
- `.mindspec/docs/research/scaling-agent-systems.md` — research reference doc with citation, key findings, metrics, and thresholds
- `internal/instruct/templates/plan.md` — add `## Decomposition Heuristics` section with research-backed rules (primary deliverable)
- `internal/validate/plan.go` — extend `ParseBeadSections()` to capture `StepLines`, add `ExtractPathRefs()`, add `checkDecompositionQuality()`
- `internal/validate/plan_test.go` — unit tests for path extraction, R_scope calculation, dependency graph analysis, and threshold warnings

### Out of Scope
- Runtime metrics (actual agent performance, token usage) — this spec uses static analysis only
- Changing the bead section format — the existing `## Bead N:` / `**Steps**` / `**Depends on**` format is sufficient
- Hard errors for any decomposition metric — all checks are warnings
- Changes to `internal/approve/plan.go` — bead creation logic is unchanged
- P_SA (single-agent baseline) estimation — would require historical data, not in scope

## Non-Goals

- Predicting actual agent performance — we provide directional guidance and structural warnings, not performance predictions
- Enforcing a maximum bead count — teams may have legitimate reasons for larger plans
- Changing existing plans retroactively — checks skip already-approved plans (consistent with existing `isApproved` pattern)
- Replacing human judgment — the heuristics and warnings inform the agent's decomposition decisions, they don't override them

## Acceptance Criteria

### Documentation
- [ ] `.mindspec/docs/research/scaling-agent-systems.md` exists with full citation, key findings, metric definitions, and threshold rationale

### Guidance
- [ ] Plan-mode instruct template includes a `## Decomposition Heuristics` section
- [ ] Each heuristic is framed as a question the agent asks during decomposition
- [ ] Each heuristic includes a "why" grounded in the research
- [ ] `./bin/mindspec instruct` in plan mode emits the decomposition heuristics

### Validation
- [ ] `ParseBeadSections()` returns `StepLines []string` for each bead section
- [ ] `ExtractPathRefs()` correctly extracts Go file paths, package paths, and test paths from arbitrary text
- [ ] `mindspec validate plan` warns when R_scope > 0.50 (high overlap)
- [ ] `mindspec validate plan` warns when R_scope < 0.15 with >2 beads (low overlap)
- [ ] `mindspec validate plan` warns when dependency chain depth > 3
- [ ] `mindspec validate plan` warns when parallelism ratio < 0.25
- [ ] `mindspec validate plan` warns when bead count > 6
- [ ] All warnings include the computed metric value and the research-backed threshold
- [ ] Already-approved plans skip decomposition checks (consistent with existing pattern)
- [ ] `go test ./internal/validate/...` passes with new test cases

## Validation Proofs

- `./bin/mindspec instruct` in plan mode: output contains "Decomposition Heuristics" section with all six rules
- `go test ./internal/validate/ -run TestExtractPathRefs`: path extraction from various text patterns
- `go test ./internal/validate/ -run TestDecompositionQuality`: all threshold warnings fire correctly
- `go test ./internal/validate/ -run TestDecompositionQuality_NoWarnings`: a well-structured plan produces no warnings
- `mindspec validate plan 074-self-contained-beads`: existing approved plan skips decomposition checks

## Open Questions

- [x] Should decomposition checks be errors or warnings? **Resolved**: Warnings only. The thresholds are empirical correlates, not absolute rules. A 7-bead plan with legitimate need should not be blocked.
- [x] Should we compute R_scope from step text only, or also verification text? **Resolved**: Both. Steps describe what to change, verification describes what to test — both reference file paths that indicate scope overlap.
- [x] What path patterns to extract? **Resolved**: Go-specific patterns for now (`*.go`, `./internal/...`, `go test ./...`). The regex can be extended for other languages later without API changes.
- [x] Which lever matters more — guidance or validation? **Resolved**: Guidance is primary (shapes decomposition at authoring time). Validation is the feedback loop (catches what guidance missed). The instruct template update is the higher-leverage change.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-08
- **Notes**: Approved via mindspec approve spec