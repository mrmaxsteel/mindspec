---
adr_citations:
    - id: ADR-0002
      sections:
        - Decision
        - Summary
    - id: ADR-0003
      sections:
        - Decision
        - Consequences
    - id: ADR-0005
      sections:
        - Decision
        - Alternatives Considered
        - Consequences
    - id: ADR-0013
      sections:
        - Decision
        - Summary
approved_at: "2026-02-19T16:49:06Z"
approved_by: user
bead_ids:
    - mindspec-i1b8
    - mindspec-6s2i
    - mindspec-to0s
    - mindspec-4aei
    - mindspec-a9u4
    - mindspec-f96z
last_updated: 2026-02-19T00:00:00Z
spec_id: 038-beads-native-multi-spec-state
status: Approved
version: "0.2"
work_chunks:
    - depends_on: []
      id: 1
      scope: .mindspec/docs/adr/, .mindspec/docs/core/{USAGE,CONVENTIONS,MODES}.md
      title: ADR supersession and canonical state contract
      verify:
        - Superseding ADR clearly defines lifecycle source of truth as per-spec molecule state
        - ADR-0007 is formally withdrawn
        - state.json end-state role documented (cursor only)
        - Docs no longer describe .mindspec/state.json as canonical lifecycle state
    - depends_on:
        - 1
      id: 2
      scope: spec-init path, spec frontmatter contract, lazy backfill logic for existing specs
      title: Spec metadata binding and lazy backfill
      verify:
        - Each spec has durable spec↔molecule binding in spec artifact metadata
        - Legacy specs are lazily backfilled on first access via molecule convention match
        - doctor warns on unbound specs
    - depends_on:
        - 2
      id: 3
      scope: workflow state resolution package and molecule step mapping logic
      title: Per-spec mode derivation engine
      verify:
        - Mode(spec) is derived from molecule steps, not global activeSpec/activeBead
        - Resolver can enumerate multiple active specs deterministically
        - Active-spec predicate is explicitly implemented (terminal/active edge cases are deterministic)
        - Resolver uses bounded Beads round-trips (documented and guarded by tests)
    - depends_on:
        - 3
      id: 4
      scope: mindspec instruct/approve/next/complete command surfaces and target selection behavior
      title: Command targeting and ambiguity handling
      verify:
        - Commands accept explicit --spec targeting
        - Untargeted commands refuse to guess when multiple active specs exist
        - Single-active-spec auto-select behavior is consistent, tied to the explicit active-spec predicate, and documented
    - depends_on:
        - 4
      id: 5
      scope: state compatibility behavior, regression tests, parallel-same-worktree scenarios
      title: Compatibility layer and multi-spec test matrix
      verify:
        - Pre-change repositories continue to operate through migration period
        - Automated tests cover at least two active specs in one worktree
        - No command cross-targets another spec without explicit instruction
        - Mode resolution uses bounded Beads round-trips per command path (guarded by tests)
    - depends_on:
        - 5
      id: 6
      scope: repo dogfood scenario, guides, doctor/validate updates, rollout notes, SessionStart latency measurement
      title: Dogfood, docs finalization, and rollout validation
      verify:
        - Dogfood run demonstrates independent progression of two active specs in one worktree
        - User-facing docs describe canonical state and targeting rules
        - SessionStart path latency for mindspec instruct is measured and documented for 1-spec and multi-spec scenarios
        - Quality gates pass
---

# Plan: Spec 038 — Beads-Native Multi-Spec State

**Spec**: [spec.md](spec.md)

## Context

This plan transitions MindSpec from a global singular lifecycle pointer to a per-spec derived lifecycle model based on Beads molecule state. The target architecture is:

- Beads molecule + step statuses are the canonical workflow progression signal.
- Spec artifacts carry durable binding to lifecycle molecules.
- Commands operate on explicit target specs (or deterministic single-target defaults).
- `.mindspec/state.json` is retained as a non-canonical "last focused spec" cursor for UX convenience (e.g., default `--spec` when unambiguous). It is no longer the lifecycle source of truth. End state: the file persists indefinitely as an optional cursor but is never consulted for mode derivation or lifecycle progression.

## ADR Fitness

| ADR | Verdict | Notes |
|-----|---------|-------|
| ADR-0002 | Conform | Beads remains the work graph and execution substrate. |
| ADR-0003 | Conform | MindSpec continues to own orchestration semantics and UX. |
| ADR-0013 | Conform | Formula/molecule lifecycle remains core mechanism. |
| ADR-0005 | Diverge (Supersede Required) | Current decision makes `.mindspec/state.json` primary lifecycle truth; this work makes per-spec molecule state canonical. |
| ADR-0007 (Proposed) | Withdraw | Retains Beads shared-state constraint insight (valid), but the per-worktree state approach is superseded by per-spec derived-state model that works inside one worktree without requiring worktree isolation. 038-A should formally withdraw ADR-0007 since it was never accepted. |

## Bead 038-A: ADR supersession and canonical state contract

**Scope**: Define authoritative lifecycle state semantics and update core docs.

**Steps**:
1. Draft a superseding ADR that replaces ADR-0005's "state.json as primary lifecycle source" rule.
2. Specify canonical lifecycle source as per-spec molecule step state + spec metadata binding.
3. Define the end-state role for `.mindspec/state.json`: non-canonical "last focused spec" cursor for UX convenience only; never consulted for mode derivation or lifecycle progression.
4. Formally withdraw ADR-0007 (Proposed, never Accepted) — its per-worktree state approach is superseded by per-spec derived state.
5. Update core docs (`USAGE`, `CONVENTIONS`, `MODES`) to remove single-active-state assumptions.
6. Add migration notes for existing repositories.

**Verification**:
- [ ] New ADR explicitly supersedes ADR-0005 lifecycle-source clauses.
- [ ] ADR-0007 is marked as Withdrawn with reference to the superseding ADR.
- [ ] No core docs claim `.mindspec/state.json` is the canonical mode source.
- [ ] `state.json` end-state role is documented (cursor only, not lifecycle truth).
- [ ] Canonical targeting/ambiguity behavior is documented.

**Depends on**: nothing

## Bead 038-B: Spec metadata binding and migration/backfill strategy

**Scope**: Ensure every spec can resolve to a lifecycle molecule without global state coupling.

**Steps**:
1. Define spec frontmatter keys for molecule binding and optional step ID mapping.
2. Update `spec-init` path to populate binding at creation time.
3. Implement lazy backfill for existing specs: on first access, resolve the spec's molecule by convention (title match or molecule search), write binding into frontmatter, and proceed. `mindspec doctor` emits a warning for unbound specs so users can backfill proactively if desired.
4. Add validation for binding presence/consistency.
5. Document fallback/error behavior when binding is missing or molecule cannot be resolved.

**Verification**:
- [ ] New specs include molecule binding metadata immediately after `spec-init`.
- [ ] Legacy specs can be bound without manual brittle edits.
- [ ] Validation reports missing or inconsistent binding clearly.

**Depends on**: 038-A

## Bead 038-C: Per-spec mode derivation engine

**Scope**: Build the resolver that computes `mode(spec_id)` from molecule state.

**Steps**:
1. Implement resolver inputs: target spec ID, molecule binding, step status fetches.
2. Encode deterministic mapping from step states to `spec/plan/implement/review/idle`.
3. Define and implement explicit active-spec predicate:
   - inactive if no molecule binding
   - inactive if `review` step is `closed`
   - otherwise active if any lifecycle/implementation step in that spec's molecule is not `closed`
   - if step mapping is partial/missing, fall back to molecule-child status scan with deterministic behavior
4. Add active-spec discovery for listing candidate specs.
5. Add deterministic tie-breaking/sorting for output stability.
6. Add focused tests for terminal molecules, partially mapped molecules, and missing-step edge states.

**Verification**:
- [ ] Resolver returns correct mode for each lifecycle phase.
- [ ] Multiple active specs are discoverable and stable in order.
- [ ] Active-spec predicate is unambiguous and test-covered for terminal vs non-terminal molecules.
- [ ] Resolver behavior is test-covered for partial/inconsistent molecule state.
- [ ] Resolver uses bounded Beads round-trips (documented and guarded by tests).

**Depends on**: 038-B

## Bead 038-D: Command targeting and ambiguity handling

**Scope**: Apply per-spec resolver to primary workflow commands.

**Steps**:
1. Add/standardize `--spec` targeting on `instruct`, `approve`, `next`, and `complete` flows where needed.
2. Implement command behavior contract:
   - one active spec (per explicit predicate): deterministic auto-select
   - many active specs: fail with explicit selection guidance
3. Ensure all transitions act only on targeted spec molecule steps.
4. Update CLI help and error messages to explain targeting.
5. Add command-level regression tests for ambiguity and cross-target safety.

**Verification**:
- [ ] Ambiguous untargeted commands refuse to guess.
- [ ] Targeted commands mutate only the targeted spec's lifecycle.
- [ ] Help text clearly explains targeting behavior.

**Depends on**: 038-C

## Bead 038-E: Compatibility layer and multi-spec test matrix

**Scope**: Preserve backward compatibility while enforcing new behavior.

**Steps**:
1. Keep compatibility reads from legacy state fields where necessary during migration.
2. Gate writes to legacy canonical fields or convert them to non-canonical cursor usage.
3. Add integration tests for two active specs in same worktree across approve/next/complete/instruct.
4. Add migration tests for repos created under old model.
5. Add guardrails preventing accidental cross-spec progression.

**Verification**:
- [ ] Existing repos remain functional through migration path.
- [ ] Same-worktree multi-spec scenarios pass integration tests.
- [ ] Cross-spec mutation without explicit target is prevented.

**Depends on**: 038-D

## Bead 038-F: Dogfood, docs finalization, and rollout validation

**Scope**: Prove behavior in this repository and finalize operational docs.

**Steps**:
1. Run dogfood scenario with at least two active specs in one worktree.
2. Capture proof outputs for instruct/approve flows per spec.
3. Update user guides and quickstarts to show new targeting model.
4. Ensure doctor/validation messaging aligns with new canonical state semantics.
5. Complete final QA gates and release notes.

**Verification**:
- [ ] Dogfood evidence shows independent per-spec progression in one worktree.
- [ ] Guides are consistent with command behavior and canonical state model.
- [ ] `make test` and relevant quality gates pass.

**Depends on**: 038-E

## Dependency Graph

```text
038-A (ADR supersession + state contract)
  → 038-B (spec↔molecule metadata + lazy backfill)
    → 038-C (per-spec mode resolver + active predicate)
      → 038-D (command targeting + ambiguity handling)
        → 038-E (compat + multi-spec test matrix + call-count guardrails)
          → 038-F (dogfood + docs + latency measurement + rollout validation)
```
