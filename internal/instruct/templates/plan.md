# MindSpec — Plan Mode

**Active Spec**: `{{.ActiveSpec}}`
{{- if .SpecGoal}}
**Goal**: {{.SpecGoal}}
{{- end}}

## MindSpec Lifecycle

```
idle ── spec ──── >>> plan ── implement ── review ── idle
```

| Phase | Command | What happens |
|-------|---------|--------------|
| idle → spec | `mindspec spec create <slug>` | Creates branch + worktree + spec template |
| spec → plan | `mindspec spec approve <id>` | Validates spec, auto-commits |
| plan → impl | `mindspec plan approve <id>` | Validates plan, auto-creates beads, auto-claims first bead |
| per bead | `mindspec next` | Claims next bead, creates bead worktree |
| bead done | `mindspec complete "msg"` | Auto-commits, closes bead, merges bead→spec, removes worktree |
| review → idle | `mindspec impl approve <id>` | Merges spec→main, removes all worktrees + branches |

### Git rules
- You should not need any raw git commands — all git operations are handled by mindspec
- Raw git is available for repair/recovery but the happy path never requires it

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

### Bead section format

Each bead must use this exact structure (the validator parses these markers):

```markdown
## Bead 1: Short Title

**Steps**
1. First implementation step
2. Second step
3. Third step

**Verification**
- [ ] `go test ./internal/foo/...` passes
- [ ] `mindspec validate plan <id>` passes

**Depends on**
None (or: Bead 1)
```

Required plan sections:
- `## ADR Fitness` — evaluate whether each relevant ADR remains the best choice; if no ADRs are relevant, explain why (this section is **required** even when no ADRs apply)
- `## Testing Strategy` — declare the overall test approach (unit, integration, e2e) and shared test infrastructure
- `## Provenance` — map each spec acceptance criterion to the bead verification steps that satisfy it (output provenance)

## Decomposition Heuristics

Apply these research-backed questions while decomposing the spec into beads (see `.mindspec/docs/research/scaling-agent-systems.md` for full context).

1. **Independence test**: Can this bead be completed without reading the output of another bead? If yes, don't add a dependency edge. Shared source files are not dependencies — only state produced by one bead and consumed by another is. *Why: sequential dependency chains degrade multi-agent performance by -39% to -70% due to coordination overhead.*

2. **Merge signal**: Do 3+ beads touch the same files? They should probably be one bead. High file overlap (R_scope > 0.50) means agents duplicate context rather than divide work. *Why: redundancy above R≈0.50 correlates with negative returns (r=-0.136, p=0.004).*

3. **Trivial-work test**: Does this bead have only 1-2 trivial steps (rename a variable, update a config value)? It doesn't justify a separate agent session — fold it into an adjacent bead. *Why: when a single agent can already handle a task (~45% baseline), splitting it yields negative returns (β=-0.404, p<0.001).*

4. **Target count**: Is the total bead count between 3 and 5? More than 6 needs justification — coordination overhead grows super-linearly with agent count. *Why: turn scaling follows a power law with exponent 1.724; per-agent reasoning thins beyond 3-4 agents.*

5. **Serial chain limit**: Is the longest dependency chain deeper than 3 (A→B→C→D)? That's a red flag. Look for false dependencies or restructure so beads can run in parallel. *Why: each link in a serial chain compounds coordination overhead without decomposition benefit.*

6. **Tool grouping**: Does an operation require many tools or commands in concert (CI setup, build config, multi-file refactors)? Keep it in a single bead rather than splitting across beads that each need the full tool context. *Why: tool-heavy tasks suffer a 6.3x efficiency penalty under multi-agent fragmentation (β=-0.267, p<0.001).*

## Human Gates

- **Plan approval**: Run `mindspec plan approve <id>` when the plan is ready
- **ADR divergence**: If a better design would diverge from an accepted ADR, **stop planning**. Present: (1) which ADR, (2) why it should be superseded, (3) the proposed alternative. Wait for human approval before proceeding. Use `mindspec adr create --supersedes <ADR-NNNN>` to create the superseding ADR once approved.

## Next Action
{{- if .PlanApproved}}

Plan is approved. Run `mindspec next` to claim the next bead.
{{- else}}

Complete the plan at `.mindspec/docs/specs/{{.ActiveSpec}}/plan.md`, then run `mindspec plan approve {{.ActiveSpec}}`. This will approve the plan AND automatically claim the first bead.
{{- end}}
