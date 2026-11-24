---
name: ms-bead-impl
description: Give an implement step to one fresh agent with the brief that the binary prints
---

# Implementation Dispatch

The binary prints the implement brief: the work item, the readiness review and the checks.
This skill leases the implement step, starts one fresh agent with that brief and submits its commit.
<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=2027-06-30 -->

## Inputs

- `spec-slug` (optional): the spec. Ask the user when more than one spec is active.
- `panel-slug` (optional): the panel, when the user starts this skill for a panel.

## Panel

When the user starts this skill for a panel, run `mindspec panel tally <panel-slug>`.

- It exits non-zero and prints no `decision:` line: report the error to the user and stop.
- It prints a `Next` block: run the lines of that block, then stop. Do not run `next-action --apply`.
- It prints no `Next` block: go to the loop.

## Loop

1. Run `mindspec next-action --apply <spec-slug> --json`.
   If it refuses, run the commands that the refusal and `mindspec instruct` print.
2. For each entry in `actions`, run its `claim` argv (`mindspec lease <step>`) and keep the token.
   Run its `brief` argv. Start a fresh agent with that output as its prompt.
   If the `brief` argv exits non-zero, start no agent and follow the `mindspec instruct` rules.
   Renew a long lease with `mindspec lease <step> --renew <token>`.
3. When the agent returns, run the action's `submit` argv with the token and the evidence:
   `--commit <oid>` for an implement or fix step; `--record <path>` for a lens, verify or
   change-judge step, at the record path that its brief names; `--artifact <path>` for a review slot.
   If `submit` refuses, run its recovery line.
4. Run each `run` entry's `argv` as printed.
5. For each entry in `decisions`, follow the `mindspec instruct` rules.
6. Read `status`:
   - `work`: go to step 1.
   - `wait`: report `waiting` and `retry_at` to the user, wait, then go to step 1.
   - `done`: report the result to the user and stop.
   - any other status: report it and each flag to the user, and stop.

Run the `repair` line of each flag. Follow the orchestrator rules that `mindspec instruct` prints.
