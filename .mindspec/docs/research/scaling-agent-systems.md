# Scaling Agent Systems: Decomposition Research Reference

## Citation

Kim, S., Shen, Y., Saphra, N., & Rush, A. M. (2025). *Towards a Science of Scaling Agent Systems*. arXiv:2512.08296.

Evaluated 180 configurations of multi-agent systems across 5 architectures and 4 benchmarks.

## Key Findings

### 1. Sequential dependency chains degrade performance

Tasks with strict sequential interdependence — where each step modifies state the next step depends on — universally degrade under multi-agent coordination. The coordination overhead fragments reasoning capacity without decomposition benefit.

- **Impact**: -39% to -70% on PlanCraft benchmark
- **Implication for plans**: Minimize serial dependency chains between beads. Deep chains (A->B->C->D) compound coordination overhead at each link.

### 2. Optimal redundancy R ~ 0.41; R > 0.50 hurts

Some overlap between work units is healthy for context continuity, but too much means agents duplicate work rather than divide it.

- **Metric**: R_scope = |paths referenced by >1 bead| / |total unique paths|
- **Optimal**: R ~ 0.41 (moderate overlap provides shared context)
- **Degradation**: R > 0.50 correlates with negative returns (r=-0.136, p=0.004)
- **Implication for plans**: Beads that share most of the same files should probably be merged. Beads with zero shared files may lack the context continuity needed for coherent implementation.

### 3. Agent count sweet spot: 3-4

Turn scaling follows a power law with exponent 1.724 (super-linear). Per-agent reasoning capacity becomes prohibitively thin beyond 3-4 agents under fixed token budgets.

- **Optimal**: 3-4 parallel agents (beads)
- **Practical target**: 3-5 beads per plan; >6 needs explicit justification
- **Implication for plans**: Over-decomposing creates coordination overhead that exceeds the benefit of parallelism.

### 4. Capability saturation at ~45% single-agent baseline

When one agent can already handle a task at ~45%+ baseline performance, splitting it across multiple agents yields negative returns.

- **Effect size**: beta = -0.404, p < 0.001
- **Implication for plans**: Trivial tasks (rename a variable, update a config value) don't justify a separate agent session. Fold them into adjacent beads.

### 5. Tool-heavy tasks suffer fragmentation penalty

Token budgets fragment across agents, leaving insufficient capacity per agent for complex tool reasoning.

- **Penalty**: 6.3x efficiency loss under multi-agent fragmentation (beta = -0.267, p < 0.001)
- **Implication for plans**: Operations requiring many tools or commands in concert (CI setup, build config, multi-file refactors) should stay in a single bead.

## Threshold Summary

| Metric | Threshold | Signal |
|--------|-----------|--------|
| Scope redundancy (R_scope) | > 0.50 | High overlap — consider merging beads |
| Scope redundancy (R_scope) | < 0.15 (with >2 beads) | Low overlap — beads may lack shared context |
| Dependency chain depth | > 3 | Deep serial chain — coordination overhead grows super-linearly |
| Parallelism ratio | < 0.25 | Most beads serial — check for false dependencies |
| Bead count | > 6 | Too fine-grained — 3-5 is optimal |

## Decision Framework

When decomposing a spec into beads, apply these checks in order:

1. **Can each bead be completed independently?** If a bead requires reading the output of another bead, that's a real dependency. If it just needs the same source files, that's shared scope (not a dependency).

2. **Are beads touching the same files?** If 3+ beads modify the same files, they likely belong in one bead. Shared file access is the strongest signal of insufficient decomposition.

3. **Is any bead trivial?** A bead with 1-2 trivial steps (rename, config update) creates agent session overhead with no decomposition benefit. Fold it into an adjacent bead.

4. **Is the total count reasonable?** 3-5 beads is the sweet spot. More than 6 needs justification — coordination overhead grows super-linearly with agent count.

5. **Are dependency chains shallow?** Chains deeper than 3 are a red flag. Look for false dependencies (beads that could actually run in parallel) or restructure the decomposition.

6. **Are tool-heavy operations grouped?** Multi-tool operations (CI setup, build config, multi-file refactors) should stay in one bead rather than being split across beads that each need full tool context.
