---
name: ms-bead-cycle
description: Execute binary-selected steps through implementation, review, fixes and completion
---

# Bead Cycle Orchestrator

The binary selects work and enforces the gates. This skill leases actor steps,
dispatches agents, submits evidence and executes the returned commands.
Apply the repository's Bead-loop guardrails. Only the cycle runs `mindspec complete`.

## Inputs

- `spec-slug` (optional): select the spec. Ask the user when active specs are ambiguous.
- `bead-id` (optional): restrict actor dispatch to this work item.
- `prompt-path` (optional): use a staged implementation prompt.

## Loop

1. Run `mindspec next-action --apply <spec-slug> --json`.
   Read `actions`, `run`, `waiting`, `decisions`, `flags` and `status`.
   Use the returned order. The binary owns selection, dependencies and round limits.
   Never select work from `bd ready` or claim a step with `bd update`.
2. For each selected action, execute its `claim` argv (`mindspec lease <step>`).
   Capture the token. Execute the action's `brief` argv to obtain its dispatch context.
   Pass the step, subject, context, worktree and token to the actor.
   If the lease is held or blocked, follow the refusal's recovery line.
   Do not take a lease that your own still-running actor holds.
   Wait for that actor or stop it first.
   Renew a live lease with `mindspec lease <step> --renew <token>` when needed.
3. Dispatch by the returned kind:
   - `implement`: use `/ms-bead-impl` to stage and dispatch the implementation.
   - `review-slot`: use the panel prompt and isolated slot tree.
   - `fix`: use `/ms-bead-fix` for one fixer in the target worktree.
   - `change-judge`: dispatch an agent with the old and new subjects and frozen gate receipt.
     Return a `carry-over` judgement with a reason or a `re-panel` judgement with paths and slot count.

   Actor agents return evidence. They never claim, complete, merge or push lifecycle work.
   After each implementation or fix commit, run `mindspec complete <bead> --check`.
   Run this check inside the leased step before submission.
   This check writes nothing and does not run the panel gate.
   If it fails, send all
   failing gates and recovery lines to the implementer or fixer under the same lease.
   Keep the same actor. Do not submit a failed check.
   Wait for the fixed commit and run the check again.
   Submit only after the check passes so the panel reviews the fixed diff.
4. Submit the evidence with the action's `submit` argv and current token.
   Use `mindspec submit <step> --lease <token> --commit <oid>` for an implementation or fix.
   Use `mindspec submit <step> --lease <token> --artifact <path>` for a verdict or judgement.
   Keep the evidence in git. A refusal leaves the step open; follow its recovery line.
5. Execute each `run` entry's `argv` as returned. These entries include panel creation,
   work-item completion and implementation approval. Do not invent a next round or merge.
   For a panel creation entry, write the brief file before running it;
   then the configured reviewers fan out through the returned review-slot actions.
   After creation, use only `/ms-panel-run`'s dispatch steps and `/ms-panel-tally`'s report.
   Skip adapter panel creation and automatic fix or halt routing. Return to this loop.
   Lease each review-slot action before its dispatch. Pass its token to submission.
   Preserve the returned gate, subject, round and scope when calling these adapters.
6. Return each open decision to the user with its listed options.
   A REJECT decision belongs to its gate. Other eligible work can continue.
   Submit only the user's selected option with `mindspec submit <decision> --answer <option>`.
   The `halt` option stops every work item in the spec.
7. Run the loop again after each submission or command. For `wait`, report blockers and
   `retry_at`; wait for the input or lease change. For `halted`, report the durable halt.
   For `done`, return the completion result. For `indeterminate`, surface the diagnostic.
   Follow flag recovery lines. Stop on a failed merge or a dependency mismatch and return it to the user.

## Sequence

This example shows an implementation and a later fix when the binary returns them.
Each transition returns to `next-action --apply`. The returned commands control progression.

```
lease implement → /ms-bead-impl
  ↓ implementation commit SHA + flagged deviations
mindspec complete <bead> --check
  ↓ content gates PASS
mindspec submit <step> --lease <token> --commit <oid>
  ↓ next-action --apply → execute panel create from run
/ms-panel-run          (execute panel create from run, then lease review slots)
  ↓ submit review evidence → next-action --apply → open decision, if any
lease fix → /ms-bead-fix
  ↓ new implementation commit SHA + flagged deviations
mindspec complete <bead> --check
  ↓ content gates PASS
mindspec submit <step> --lease <token> --commit <oid>
  ↓ next-action --apply → execute panel create from run
/ms-panel-run          (execute the next panel create entry)
  ↓ submit review evidence → next-action --apply → execute each run entry
```

## NOT READY — dispatch triage

`/ms-bead-impl`'s dispatch ingress (`mindspec bead ready-check`, plus the staged prompt's Phase 0) can refuse a bead before any code is written. A subagent return whose first line is `NOT READY: <bead-id>` is its OWN outcome, handled distinctly from every other outcome:

- **No panel round is consumed** — nothing was implemented, so there is nothing to review.
- **Never routed to `/ms-bead-fix`** — there is no implementation to fix.
- **The bead worktree remains intact** — zero commits were made.

On a NOT READY, choose exactly one of two dispositions:

- **ACCEPT (the default)** — pause this dispatch and surface the reasons to the user.
  Return the full ordinal-numbered report. Revise the plan or spec text that the reasons quote.
  Return through `mindspec next-action --apply` and `mindspec lease` before a new dispatch.
  Reuse the current lease only while it is valid.
  Use `/ms-bead-impl` to run the ingress check and stage Phase 0 again.
  Other gates remain governed by the binary output.
- **DISAGREE / clarify** — once per bead: author a grounded, reason-keyed record (one entry for EVERY cited ordinal, each: `{ordinal, verbatim reason, concrete answer, authoritative source span}` — the verb REFUSES a partial record (an unanswered cited reason means the bead is still not ready — that is ACCEPT territory) and a paraphrased reason (the pairing must repeat the cited reason verbatim); a clarification may DISAMBIGUATE existing spec/plan/landed-code authority, but may never CREATE new normative behavior; when no span supports the answer, ACCEPT is the correct disposition, not clarify) and write it via:
  ```bash
  mindspec bead clarify <bead-id> --file <record.json>
  ```
  then re-dispatch. The cap is **categorical and durable**: ANY prior attempt record on the bead (from an earlier `bead clarify` call, even in a different session) forces the NEXT NOT READY — whether it repeats an addressed reason or raises an entirely new one — to ACCEPT. The verb itself enforces this (a second `bead clarify` on the same bead is refused), so the cap survives orchestrator restart / context loss — it is not a rule this skill has to remember to apply.

  **Cap threat boundary.** The cap stops the accidental/naive unbounded clarify↔re-dispatch loop and survives process restart. It does NOT stop a deliberate, out-of-band DELETION of the bead's readiness-attempt metadata (a manual reset) — that is a conscious operator act, analogous to `--allow-not-ready=<ids> --evidence <path>` for the mechanical floor, and it destroys the append-only audit trail (visible in the record's absence). Do not delete the record to "get another round"; if the plan genuinely needs revision, ACCEPT and revise it.

**Clarify vs. `--allow-not-ready=<ids> --evidence <path>` — never interchangeable.** `bead clarify` resolves a SEMANTIC Phase-0 concern with cited evidence (SR-1..SR-5 — implementability, decidability, helper existence, contradiction, ambiguity); it can never move a MECHANICAL MF-1..MF-4 signal (the layer boundary holds by construction — a clarification's dedicated metadata key is off the mechanical scan surface). `mindspec next --allow-not-ready=<ids> --evidence <path>` is the signal-scoped, evidence-recorded bypass of a failing mechanical floor at claim time — a deliberate "proceed anyway," not a resolution. Never substitute one for the other: clarifying a mechanical FAIL does nothing (the floor still refuses), and `--allow-not-ready=<ids> --evidence <path>` does not touch the semantic Phase-0 review at all.

## Completion

Execute `mindspec complete` only when its command appears in `run`, except for the content check.
The only exception is `mindspec complete <bead> --check` inside a leased implement or fix step.
This is the only `complete` call allowed outside `run`. It must carry `--check`.
Check `git status --porcelain` in the target worktree before completion.
Preserve pending changes and follow the CLI recovery line if completion refuses.
Never merge a bead branch with raw `git merge bead/<id>`.
Push once at the end of the spec through the orchestrator's publication path.

If a check stopped between bd-close and the actual git merge, recover it before another dispatch.
Re-run the returned completion command.

## Review discipline

Use one panel slug for all rounds. Set `--round <R>` to the next round.
Take that round from the binary's `run` entry.
Carry implementation and fixer deviations into the next brief.
Use `panel prompt` for each slot and round.
Apply `review.reproduce`: reproduce each blocking claim in an isolated slot tree.
Apply `review.no-vote`: approval counts do not resolve findings.
Apply `panel.fix-after-final` and `panel.one-fixer`: wait for verify exit 0 before one fixer.

| Pattern | Action |
|:--------|:-------|
| One family approves; another requests changes | Reproduce each blocking claim and compare the evidence |
| All slots approve | Check the binary's tally and returned commands |
| All slots request changes | Consolidate the findings for the authorised fix |
