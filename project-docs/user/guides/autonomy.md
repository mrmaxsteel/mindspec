# Governed autonomy

MindSpec has one workflow for every newly approved plan. The binary selects the
next steps. An orchestrator executes them. You can also execute the returned
commands yourself. `/ms-spec-autopilot` uses `/ms-bead-cycle` for this workflow.

## Configure the policy

Set defaults before plan approval:

```yaml
panel:
  reviewers: [{family: harness, count: 4}]
  max_rounds: 5
  lease_ttl: 2h
```

Plan approval pins the round allowance and lease duration for that spec.
Later config edits do not change its policy. Panel rounds count across every
registration for a gate, including abandoned registrations. Redispatch of a slot
uses no extra round.

Run `mindspec setup <agent>` after an upgrade. Setup refreshes unchanged shipped
skills. It keeps user edits and reports them. Setup adds required Beads types
and keeps existing custom types. Plans approved before enrolment was available
finish through the existing lifecycle commands. They are not enrolled by a read
or by a repeated plan approval.

## Execute the workflow

1. Inspect with `mindspec next-action <spec> --json`. This command writes nothing.
2. Run `mindspec next-action --apply <spec> --json` to create authorised steps.
3. Read `actions`, `run`, `waiting`, `decisions`, `flags` and `status`.
4. Execute each action's `claim` argv. `mindspec lease <step>` prints a token once.
5. Dispatch an actor with the returned brief, subject, worktree and token.
6. Submit the committed evidence with the action's `submit` argv.
7. Execute each `run` entry's `argv` in order.
8. Read the next action again after each submission or command.

For an implementation or fix, submit its commit:

```bash
mindspec submit <step> --lease <token> --commit <oid>
```

For a review or change judgement, submit its artifact:

```bash
mindspec submit <step> --lease <token> --artifact <path>
```

Keep the evidence in git. The receipt binds its path, holding commit and blob.
A separate evidence worktree can keep review files together. Evidence can also be
committed on the subject branch. A commit that changes only directories registered
to the gate does not move its subject. A commit that changes another path does.
The same receipt can be submitted again after an interruption.
A different result for an already closed step is refused.

The `run` entries include panel creation, work-item completion and final approval.
Use those commands as returned. Only `panel create` opens a panel round.
Actor agents do not close work with bd or merge their branches.
The orchestrator runs completion after the review gate passes.

## Handle decisions and waiting

A REJECT opens a decision for its gate. Return its `fix` and `halt` options to the
user. Other eligible work can continue. At the round limit, return the
`continue:<n>` and `halt` options. Submit the user's selected option:

```bash
mindspec submit <decision> --answer <option>
```

A `halt` answer stops every work item in the spec. The halt is durable and has
no resume command. Read commands remain available. Implementation approval
prints the policy and audit counts before final publication.

For `wait`, report blockers and `retry_at`. Wait for the missing input or lease
expiry. Renew a live lease with `mindspec lease <step> --renew <token>`.
For `indeterminate`, surface the diagnostic. Follow each refusal's recovery line.
Return a failed merge or dependency mismatch to the user before more dispatches.
A missing artifact, stale subject or invalid token leaves the step open.

After a passing gate's subject changes, an actor judges the change. It can record
`carry-over` with a reason or request `re-panel` with paths and a slot count.
The binary checks the judgement and returns the authorised next commands.

## Share the Beads store

People who do not use MindSpec need only bd and git. No MindSpec setup is required
for their work. To hide MindSpec items, run:

```bash
bd ready --exclude-label mindspec
```

MindSpec labels its own items and prefixes their titles with `[mindspec]`.
It preserves other items and their links. An external blocker holds the affected
step. Follow the printed recovery to close the blocker or remove its link with bd.
An open external child can also prevent a parent close.
