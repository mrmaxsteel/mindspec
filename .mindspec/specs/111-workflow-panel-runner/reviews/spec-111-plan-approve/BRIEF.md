# spec-111-plan-approve — Round 1 (plan review, 9 reviewers, three families)

**Under review**: `.mindspec/specs/111-workflow-panel-runner/plan.md` @ **3ca2cd2c** (686 lines) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner`. Read the APPROVED `spec.md` beside it — the plan is judged against that contract.
**Panel**: 9 reviewers — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5 (codex). Pass = **>=8 APPROVE, no REJECT**.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; leave `git status` clean.

## What the plan does

Spec 111 ships the `/ms-panel` WORKFLOW runner — panel fan-out as a deterministic `.claude/workflows/ms-panel.js` script (Workflow-tool orchestration) behind spec 110's `mindspec panel create|verify|tally` verb contracts, selected by spec 109's (currently inert) `runner:` config key. 3 beads: B1 OWNERSHIP claim for `.claude/workflows/**` + validate test (R9); B2 the workflow runner artifact itself — ALLOWED_CLI allowlist (EXACTLY four commands, statically tested), codex slot driving, parse-failure re-prompt with verdict-VALUE fidelity, quota-wall claude-sub vs MISSING semantics, runner NEVER runs `mindspec complete` (R1–R8); B3 skill re-pointing + runner-selection via 109's `runner:` key (R6–R7).

## MANDATORY cross-check (all slots, O3 especially)

`review/spec-111-approve/consolidated-round-2.md` (top level in the worktree) lists the spec-approval panel's carry-forwards explicitly deferred TO THIS PLAN: #1 AC4 exact-four ALLOWED_CLI static check; #2 verdict-VALUE fidelity in the parse-failure re-prompt Manual proof; #3 naming the `mindspec ${VERB}` dynamic-construction indirection in R5's falsified clause; #4 the `../../adr/` relative-link convention. The plan claims all four are folded (B2's TestMsPanelWorkflow_AllowedCLIExactSet, the Manual e2e's value-fidelity diff, the three-form indirection grep, §655). Verify against the file.

## Context

- **109 is MERGED** (this branch sits on post-109 main): `runner:` key + resolvers exist, INERT.
- **110 is PLAN-APPROVED, NOT merged** (beads fbel.1–.5; fbel.1 is in review right now): the `panel create|verify|tally` verb + artifact contracts 111 builds behind. 110's plan: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.mindspec/specs/110-panel-verbs-parser-parity/plan.md` (READ-ONLY).
- **112 is PLAN-APPROVED, NOT merged** (beads lma4.1–.3; lma4.1 implementing now): per-gate `gates:` config. NOTE: 111's SPEC predates 112 and never cites it — check the plan doesn't smuggle in 112-consumption as scope creep; the plan's Risks/Sequencing (§606) should pin the boundary.
- Known advisory: `validate plan` WARN decomposition-scope-redundancy R=0.04 (3 beads, disjoint packages).
- Quality-bar precedent: the 110 plan panel's round-1 killers were false-green Verifications (un-anchored greps, `! cmd | grep` forms, installed-binary self-checks, tautological contract tables, unrunnable e2e). The 111 author was briefed to avoid all of these — verify they actually are avoided, especially on the JS-artifact structural greps where grep-on-JS is the prime false-green risk.

## Slot lenses

| Slot | Lens |
|:-----|:-----|
| F1 | Adversarial — attack the central fences: is the exact-four ALLOWED_CLI test unfakeable (AST-ish parse vs grep)? can the runner reach `mindspec complete` through any indirection the greps miss? is MISSING-not-substituted actually enforced by the described mechanism? |
| F2 | DAG / merge-signal — 3 beads' dep edges real, doc-sync files disjoint, the §606 upstream claims (110/112 surfaces, rebase points) accurate against those specs' actual plans. |
| F3 | Falsification — every Verification command: does it FAIL if the step was skipped/wrong? JS structural greps, workflow-schema checks, the Manual e2e's observables. |
| O1 | Implementability — the Workflow-script mechanics (agent()/parallel() semantics, schema-forced verdicts, codex driving from a workflow, `.claude/workflows/` install/embed story): will a Sonnet implementer succeed from these steps alone? |
| O2 | ADR / process conformance — citations genuine + Accepted + domain-intersecting (check deliberate omissions), steps ≤7, plan-approve gates pass, frontmatter/work_chunks shape. |
| O3 | Spec coverage — R1–R9/ACs ↔ beads exact; ALL FOUR carry-forwards verified against consolidated-round-2.md; no scope creep (especially no un-specced 112-consumption); Non-Goals respected. |
| G1 | Test-runnability — every Verification command runnable as written on macOS and proving its claim; the Manual live-agents e2e's setup completeness. |
| G2 | Security / robustness — ALLOWED_CLI enforcement strength, prompt-injection surface of workflow-composed reviewer prompts, codex `.codex.log` parsing, path handling. |
| G3 | Downstream contract — what the loop-engineering supervisor (L5) and the surviving skills consume from the runner's result shape; degraded modes (codex walled, workflow tool absent); stability of the runner-selection contract via `runner:`. |

## Your job

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<slot>-round-1.json` in this dir. Keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence`, `rationale` (<=160 words), `concrete_changes_required` (empty if APPROVE), `findings`.
