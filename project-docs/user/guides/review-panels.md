# Review Panels

A panel reviews the work with separate reviewer slots. The binary computes the decision.
Each slot has a family, model and lens. The implementer cannot approve its own work.

## Artifacts and verdicts

A spec panel uses `.mindspec/specs/<spec-slug>/reviews/<panel-slug>/`.
An adhoc panel uses `.mindspec/reviews/<panel-slug>/` on a flat layout.
Other layouts use `review/<panel-slug>/`.

The directory holds `panel.json`, `BRIEF.md`, `dispositions.jsonl` and submitted verdict files.
`panel.json` records the roster, round, reviewed SHA and subject digest.
Every new round uses verdict schema v2. Historical rounds keep their file-based rules.

Run `mindspec panel schema` for the JSON Schema.
A verdict has `schema: 2`, `slot`, `round`, `reviewed_sha`, `status`, `verdict`, `model`, `rationale`, `concrete_changes_required` and `commands_run`.
Status is `draft` or `final`. Verdict is `APPROVE`, `REQUEST_CHANGES`, `REJECT` or `VOID`.
VOID requires `void_reason`. Other verdicts require a rationale.
Only REQUEST_CHANGES has change items. Each item has `text` and a `basis` of `executed`, `traced` or `hypothesis`.
An executed item also names its zero-based `commands_run` index in `command`.
`hard_block` is an optional boolean. `findings` and `inputs_read` are optional string arrays for evidence.

Harness reviewers use `mindspec panel submit --file <absolute-attempt-path>/verdict.json`.
The slot prompt gives the path. Submit validates the bytes and writes the panel verdict file.
Process reviewers use the output channel in their prompt. Launch submits their verdicts.
A verdict file without a submission does not count. The latest submission replaces earlier submissions for that slot and round.
The ledger retains all submissions. A replacement in flight keeps the slot out of the gate.

## Reviewers and lenses

Zero config uses one harness family for every slot. No tool runner ships.
Defaults are 4 reviewers for bead panels and 6 for spec, plan, final and adhoc panels.
A one-family panel is valid. A panel needs at least two reviewers.

Code review lenses are author-of-record, empirical-prober, codebase-pin, adversarial, contract-stability and integration.
Spec and plan approval use falsifiability, scope-honesty, contradiction and feasibility.
Set focus text in `panel.lenses`. Set the ordered lens list and acceptance question under `panel.gates.<gate>`.
`mindspec config resolve --gate bead --json` previews a new roster and each value's source.
For an existing panel, use `mindspec panel verify <slug> --json`.

## Runners

Configure `panel.runners` in `.mindspec/config.yaml`:

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

Replace `review-cli` with a command on your PATH. Match its output to the slot prompt.
A process runner needs `argv`, `prompt` and `output`. Placeholders occupy whole argv elements.
A harness runner uses the adapter's agent tool. It accepts an optional timeout.
Set `on_quota` on a process runner to select a substitute family.
A non-zero exit or timeout must match a configured signal in stderr before quota substitution occurs.
`panel.substitution` and `claude_sub_on_quota` are retired; use `on_quota` on a runner.

Use `.mindspec/config.local.yaml` for machine-local `panel.runners` and `models` only.
The local file merges after the project file. Runners replace the same family or append a new family.
Do not track the local file. Run `mindspec init` to add ignore entries for local config and review scratch.
A reviewer family without a runner refuses creation. A model-only reviewer runs on the harness with its model and produces a warning.
When the effective model equals the family, the adapter uses its default model and retains the slot model as the verdict label.
If the agent tool cannot run an explicit model, the adapter reports the slot.

The top-level `runner` selects the orchestration adapter:

- `claude-code-skills`: dispatch through the panel skills.
- `claude-code-workflow`: run `/ms-panel` once for the round.
- `external`: use another orchestrator with the same CLI contract.

## Dispatch and isolation

```bash
mindspec panel create <slug> --spec <id> --gate bead --target bead/<id> --bead <id>
mindspec panel verify <slug> --json
# Harness slot:
mindspec panel checkout <slug> --slot R1 --dispatch
mindspec panel prompt <slug> --slot R1
# Dispatch the prompt through the harness, then submit the named attempt file.
mindspec panel submit --file <absolute-slot-dir>/attempt-1/verdict.json
# Process slot:
mindspec panel launch <slug> R2
mindspec panel recover <slug> R2 --reason '<why>'
```

Slot clones and scratch live under `.mindspec-scratch` by default.
The slot contains `slot.json`, `tree/` and `attempt-<k>/` directories.
Reviewers run tests in their slot tree. They keep scratch in the slot directory.
A harness redispatch runs checkout with `--dispatch`, reads a new prompt and dispatches it once.
Add `--replace` after a final verdict other than VOID. Old attempt files cannot submit for a new attempt.
An agent that returns with no submission gets one redispatch. Report a second failure.
Use the latest verify status to choose the next action. Wait while a slot runs.
Run verify after each process or agent ends. Only dispatch a fixer when verify exits 0.
Use `panel checkout <slug> --fixer` for fixer scratch. The fixer works in the target worktree.
Use `panel checkout <slug> --prune` to remove the panel's slot directories.

## Slot status and exit codes

Verify reports one status per slot:

| Status | Meaning |
|:-------|:--------|
| `final` | Latest final submission is complete and its file matches |
| `damaged` | Counted final submission has a missing or changed file |
| `void` | Latest submitted verdict is VOID |
| `draft` | Latest submission is a draft |
| `running` | Current attempt is pending or running |
| `quota_wall` | Process attempt matched a configured quota signal |
| `launch_failed` | Attempt failed or exceeded its timeout |
| `malformed` | Process output failed verdict validation |
| `unsubmitted` | File has no matching latest submission |
| `missing` | Slot has no verdict or attempt |

Exit 0 means all slots are final. It does not mean the gate passes.
Exit 1 means a slot is running or missing, a harness draft is pending, or a quota handoff is pending.
Exit 2 applies in every other case. `--json` includes the effective family, model, lens, kind, annotations and `handoff_pending`.
On a status-read error, verify prints a text preview and exits 2, also with `--json`.

## Decision and recovery

Run `mindspec panel tally <slug>` for the decision and findings.
PASS requires a fresh, intact panel with every roster slot in, no REJECT, no hard block, no INVALID and no unresolved REQUEST_CHANGES.
Approval counts do not discharge findings. Reproduce blocking claims before a fix dispatch or refutation.
A missing required measurement artifact stays open until the artifact exists.

- Fix the findings, then use `panel create <slug> --round <N+1>` for a new review.
- Refute a finding with `mindspec panel refute <slug> --slot R1 --round N --finding 'ccr[0]' --ground disproven --reason '<why>' --evidence <reference> --by '<name>'`.
- Defer a finding with `mindspec panel defer <slug> --slot R1 --round N --finding 'ccr[0]' --bead <follow-up> --by '<name>'`. Spec approval and adhoc panels cannot defer.
- Use `mindspec panel rebind <slug> --reason '<who>: <why>'` when a passing panel's changed paths need no new review.
- Use `mindspec panel abandon <slug> --reason '<why>' --by '<name>'` to record an exit from the panel flow.

At the first v2 round, a carried slot without a family needs a runner that matches its old model.
If none exists, use `panel create <slug> --round <N+1> --reviewers <n>` for a fresh roster.
Process runners need a Unix host. Use harness runners on Windows.

Only a human can set `MINDSPEC_SKIP_PANEL=1` in the session environment before launch.
Completion records the audited skip. An agent cannot set it on a command line.

See the [autonomy guide](autonomy.md) for gate authority and loop budgets.
