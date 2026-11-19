# mindspec — autonomous spec-driven implementation, with multi-reviewer panels

A skills package that takes an approved MindSpec plan through implementation and panel review.
The binary owns work selection, leases, receipts, panels and decisions. Skills dispatch the work.
Zero config uses one harness family. Add process runners for other review tools.

## What this plugin replaces

| Without | With |
|:--------|:-----|
| "Implement bead. Eyeball diff. Merge." | "Implement → configured panel → fix → re-panel → merge" |
| Single-reviewer LLM gating | Independent slots with configured models and lenses |
| Manual "what's the next ready bead?" | `/ms-bead-cycle` executes the actions from `mindspec next-action --apply` |
| Manual fix-up after review | `/ms-bead-fix` consolidates verdicts and dispatches a fix subagent |
| Manual per-bead orchestration | `/ms-bead-cycle` runs the whole pick → impl → review → merge loop |
| Manual per-spec orchestration | `/ms-spec-autopilot` cycles every bead until the spec is done |

## Skills

Twelve skills total: four lifecycle gates + eight plugin skills. This is the
post-thin-down inventory — orchestration only; operational
knowledge (recovery recipes, gate semantics) lives in CLI point-of-use output
and hook messages, not in skill prose. Following the Anthropic agent-skills
pattern, each skill is a thin, composable orchestration unit.

### Spec lifecycle (defined in `internal/setup/claude.go::lifecycleSkillFiles()`)

| Skill | Purpose |
|:------|:--------|
| `/ms-spec-create` | Create a new specification (enters Spec Mode) |
| `/ms-spec-approve` | Approve spec → Plan Mode |
| `/ms-plan-approve` | Approve plan → Implementation Mode |
| `/ms-impl-approve` | Approve implementation → Idle |

These are the existing mindspec gating skills. The new skills in this plugin assume they're already wired. (Spec status is no longer a skill — run `mindspec state show` / `mindspec instruct` directly.)

### Bead lifecycle (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-impl` | Phase A stages the impl prompt (plan section + dep helper signatures); Phase B dispatches the implementation subagent |
| `/ms-bead-fix` | Dispatch a fix-up subagent with the consolidated change list |

### Review panel (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-panel-run` | Step 0 writes the panel dir + BRIEF + `panel.json`; then dispatch the roster from `panel verify --json` and collect submissions |
| `/ms-panel-tally` | Single decision authority: decision matrix (fresh, intact, complete, no veto, no unresolved REQUEST_CHANGES), artifact gates, consolidation, halt-recovery, escape hatch |

### Orchestrators (new)

| Skill | Purpose |
|:------|:--------|
| `/ms-bead-cycle` | Execute returned actions: lease → dispatch → submit → run commands |
| `/ms-spec-autopilot` | Whole spec: execute the binary workflow until its status is `done` |
| `/ms-spec-final-review` | Final panel of the whole spec branch vs main, before `/ms-impl-approve` |

## The autonomous loop

`/ms-spec-autopilot` uses `/ms-bead-cycle` to execute one binary workflow:

1. Run `mindspec next-action --apply <spec> --json`.
2. Lease each returned actor step with `mindspec lease`.
3. Dispatch the actor with its brief, subject and token.
4. Submit committed evidence with `mindspec submit`.
5. Execute each returned `run` entry's `argv`.
6. Return open decisions to the user. Repeat until the binary reports `done`.

The binary selects work, applies dependencies and authorises each panel round.
The adapters preserve the returned gate, subject and scope.
The `run` entries include panel creation, completion and final approval.
Use `mindspec next-action <spec> --json` for a read that writes nothing.

A REJECT opens a local `fix` or `halt` decision. Other eligible work can continue.
The round allowance counts all registrations for the gate. At the limit, the
user chooses `continue:<n>` or `halt`. A halt stops every work item in the spec.
The policy defaults are `panel.max_rounds: 5` and `panel.lease_ttl: 2h`.
Plan approval pins the policy. Later config edits do not change that spec.

People who work without MindSpec can use bd and git in the same store.
Run `bd ready --exclude-label mindspec` to hide MindSpec work.
MindSpec keeps external items and respects their blocking links.

## Panel defaults

Bead panels have 4 reviewers. Spec, plan, final and adhoc panels have 6.
Zero config runs all slots on the harness. A one-family panel is valid.
Each reviewer has a separate lens. Configure the roster and runners for the work and budget.
The gate needs a fresh, intact panel with every slot in, no veto and no unresolved finding.

## Why these patterns

The plugin's mechanics are an explicit composition of the Anthropic
"Building Effective Agents" patterns — naming them is the quickest way to
see why it works:

| Pattern | Where the plugin uses it |
|:--------|:-------------------------|
| Parallelization | the configured panel — independent review slots and one binary decision |
| Orchestrator-workers | `/ms-spec-autopilot` + `/ms-bead-cycle` dispatching to specialist subagents |
| Evaluator-optimizer | the impl → panel → fix → re-panel iterative loop |
| Prompt chaining | the spec → plan → bead lifecycle |
| Routing | **not used** — could classify bead type (migration / logic / docs) and pick different impl + panel shapes; deliberately out of scope |

## Configuration

Use `bd` for issue tracking and `mindspec` for the lifecycle.
The host needs an agent tool for harness slots. No named reviewer CLI is required.
Configure each process tool in `panel.runners` before you add its family to the reviewers.
Process runners need a Unix host. Use harness slots on Windows.

```yaml
panel:
  runners:
    - family: local-review
      kind: process
      argv: [review-cli, "{prompt_file}"]
      prompt: file
      output: stdout
      quota_signals: ["quota limit reached"]
      on_quota: harness
  reviewers:
    - {family: harness, count: 2}
    - {family: local-review, count: 2}
```

Replace `review-cli` with your local tool. Match its output to `panel prompt`.
`on_quota` selects the substitute family when stderr matches a configured quota signal.
The slot keeps its id. Verify reports the effective runner and a pending harness handoff.
The adapter dispatches a harness substitute through `panel checkout --slot <slot> --dispatch` and `panel prompt`.
Reviewers use `panel submit` or their configured process output channel.
`panel.substitution` and `claude_sub_on_quota` are retired; use `on_quota` on a runner.
Machine-local runner and model settings belong in `.mindspec/config.local.yaml`.

## Disk layout

```
<repo>/
  .mindspec/specs/<spec-id>/                # flat layout
    spec.md
    plan.md
    reviews/                                # panel artifacts co-located with the spec they review
      prep/                                 # pre-staged impl prompts (optional)
        bead<N>_impl_prompt.md
      <panel-slug>/                         # one per panel round
        BRIEF.md
        R1-round-1.json
        ...
        R6-round-2.json                # round 2 after fix-up
```

## Integration with mindspec core

As of 2026-06, the plugin's SKILL.md files are embedded into the `mindspec`
binary via `plugins/mindspec/embed.go` (a `//go:embed skills/*/SKILL.md`
block) and merged into `internal/setup/claude.go::skillFiles()` alongside
the 4 lifecycle gate skills. Every `mindspec setup <agent>` user gets the
full autonomous-loop skill set by default — no opt-in copy step. Setup
refreshes a mindspec-shipped skill file in place when its content
byte-matches a previously-shipped version, and removes retired skill dirs on
the same provenance check; a user-modified file is left untouched with a
notice.

- **Lifecycle gate skills** are defined inline in
  `internal/setup/claude.go::lifecycleSkillFiles()` (the 4 spec lifecycle
  transitions). They win on key collision — they are the canonical
  authority.
- **Plugin skills** live here as on-disk SKILL.md files and are embedded
  via `pluginmindspec.SkillFiles()`. Editing the SKILL.md on this branch
  + rebuilding the binary is the iteration path; the on-disk copy under
  `plugins/mindspec/skills/` is the source of truth.

## Decisions and recovery

Return open decisions to the user with their listed options.
Report `wait`, `halted` and `indeterminate` states from the binary.
Return failed merges and dependency mismatches before more dispatches.
Follow the printed recovery lines. Keep pending work and committed evidence.
