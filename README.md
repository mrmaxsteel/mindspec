# MindSpec

[![Release](https://img.shields.io/github/v/release/mrmaxsteel/mindspec)](https://github.com/mrmaxsteel/mindspec/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**An opinionated loop-engineering framework for AI coding agents.**

Spec-driven development, research-backed planning, architecture guardrails, and adversarial review panels — so agents can build resilient software semi-autonomously, with the engineering discipline enforced by code instead of prose.

---

> "The new default is that you are not writing the code directly 99% of the time, you are orchestrating agents who do and acting as oversight." — Andrej Karpathy, on why "vibe coding" grew up into *agentic engineering*

MindSpec is a framework for that oversight. AI coding agents are phenomenal executors and unreliable engineers: left to themselves they drift from intent, steamroll architecture decisions, skip documentation, and let a "small feature" become a three-subsystem refactor. The usual answer is to watch them more closely. MindSpec's answer is to engineer the loop instead.

Prompt engineering optimizes one instruction; context engineering one window; harness engineering one run. **Loop engineering** — Addy Osmani's name for "replacing yourself as the person who prompts the agent" by designing "the system that does it instead" — builds the system that decides what to work on, whether the result is acceptable, and when to stop. MindSpec is that system: a CLI plus a set of agent skills that wrap your coding agent in a gated lifecycle — **spec → plan → implement → review** — where **every gate is a validation in a Go binary that exits non-zero**, not an instruction the model might ignore.

An unattended loop cannot be argued with, so its guardrails must not be arguable either. That is the core opinion, and everything else follows from it — and it's the conclusion production-scale loops have converged on independently: Stripe ships [over 1,300 machine-written pull requests a week](https://www.infoq.com/news/2026/03/stripe-autonomous-coding-agents/) — every one human-reviewed, none human-written — on the rule that anything deterministic never goes to a probabilistic model. Reliability comes from the quality of the constraints, not the size of the model.

```
 idea ─▶ SPEC ══gate══▶ PLAN ══gate══▶ IMPLEMENT ═══════▶ FINAL REVIEW ══gate══▶ main
          │              │              │
      adversarially   decomposition    the bead loop, per work item:
      grilled until   checked against    • fresh agent, isolated git worktree
      falsifiable     published          • exactly one commit, tests must pass
                      research           • panel size and runners from config
                                         • merge gate enforced in the binary
```

**[Quickstart](#quickstart) · [Governed autonomy](#governed-autonomy) · [Documentation](#documentation)**

## What you get

- **Specs that survive contact with an agent** — every spec is interrogated by an adversarial grill agent until each requirement is concrete, falsifiable, and grounded in the actual repo. Vague verbs, contradictions, and untestable claims don't make it past the first gate.
- **Research-backed planning** — decomposition quality is among the strongest predictors of agent success. Plans are validated against published thresholds for bead count, scope overlap, and dependency-chain depth (see [Planning](#planning-is-research-backed)).
- **A work graph, not a chat history** — work items live in [Beads](https://github.com/gastownhall/beads), a git-native issue tracker. Each bead is a self-contained work packet a fresh agent can pick up with zero session history.
- **Maker ≠ verifier** — the implementing agent makes exactly one commit and cannot merge its own work. A panel with configured reviewers and distinct lenses verifies the work. The binary computes the decision.
- **Architecture that can't be steamrolled** — plans must cite ADRs covering every impacted domain. If the diff touches a domain whose decisions weren't cited, the merge gate blocks until a human approves a superseding ADR.
- **Docs that can't rot** — a bead cannot close while source and documentation have drifted apart. Doc-sync is a gate, not a convention.
- **Governed autonomy** — use the same binary workflow interactively or through an orchestrator. The binary selects steps, checks evidence and returns decisions to you ([Autonomy](#governed-autonomy)).
- **Every escape hatch audited** — overrides like `--override-adr` and `--allow-doc-skew` exist, but each use is journaled to a redacted, local friction log. Where overrides concentrate is where the system improves next.

## The loop

Every phase transition is a gate. The binary checks the evidence at each gate.

**Spec** — define what "done" looks like: problem statement, ≥3 falsifiable acceptance criteria each paired with a runnable proof, impacted domains, ADR touchpoints. The `ms-spec-grill` skill then interrogates the draft one question at a time, hunting synonym-dodges ("support", "handle", "improve"), non-falsifiable claims, cross-requirement contradictions, and repo claims the tree doesn't back up. No code allowed.

**Plan** — decompose the spec into beads: independently completable work items with per-bead acceptance criteria. The plan validator checks decomposition against published research thresholds and requires ADR citations covering every impacted domain. If the plan needs to deviate from a cited ADR, it stops and escalates — you approve a superseding ADR or reject the divergence.

**Implement** — run `mindspec next-action --apply <spec> --json`. Lease each returned actor step with `mindspec lease`, dispatch the agent, and submit its committed evidence with `mindspec submit`. Execute each returned `run` command. A review panel checks each work item before `mindspec complete` merges it. Discovered work becomes a new work item.

**Review** — after the last bead merges, a final panel reviews the cumulative spec branch against main: scope drift, inter-bead coherence, release readiness. Approval merges spec → main via PR, and the lifecycle returns to idle.

State never lives in the context window. The lifecycle phase is derived from the beads graph and git — which means a crashed or restarted agent loses nothing, and a fresh agent can always reconstruct exactly where the work stands.

## Beads: a work graph your agent can actually use

MindSpec tracks work in [Beads](https://github.com/gastownhall/beads) — a git-native issue tracker that lives in your repo and needs no external service. In its creator's words:

> "…a magical 4-dimensional graph-based git-backed fairy-dusted issue-tracker database, designed to let coding agents track all your work and never get lost again." — Steve Yegge, announcing Beads

This is not an incidental choice; it's what makes the loop possible.

Each bead is a **self-contained work packet**: requirements, per-bead acceptance criteria, impacted domains, cited ADRs, dependency edges, and completion evidence. A fresh agent picking up a bead needs no session history and no tribal knowledge — `mindspec context bead <id>` assembles a deterministic, token-budgeted context pack (spec, plan section, cited ADR decisions, domain docs, file paths — with a SHA-256 provenance record of every input) and the agent gets exactly what the plan intended it to see.

The session-freshness gate refuses automatic bead selection from a stale, compacted, or already-claimed session. A claim by explicit bead ID warns and continues. Clear the context before automatic selection.

## Review panels

Every bead merge — and, on the higher autonomy rungs, every gate — is judged by a review panel:

- **Configured reviewers**, launched in parallel. Zero config uses one harness family for every slot. Process reviewers need a configured runner.
- **Distinct lenses**: author-of-record, empirical prober, codebase pin, adversarial, contract stability and integration. Spec and plan reviews use falsifiability, scope honesty, contradiction and feasibility.
- **No unresolved findings** — every REQUEST_CHANGES finding must be fixed, refuted with evidence, or deferred to a follow-up bead before the panel passes.
- **Artifact hard gates** — a finding that names a missing measurement artifact (a benchmark, a cost projection, a regression baseline) blocks regardless of the vote count. Agents can't vote evidence into existence.
- **The decision matrix is a pure function in the binary** — identical facts produce an identical decision, whether the panel was launched by a human, a skill, or an unattended loop. `mindspec panel create | checkout | prompt | launch | submit | recover | verify | tally | refute | defer | rebind | abandon` are the agent-neutral verbs; any orchestrator that can run a CLI can run a panel. `mindspec panel disposition validate | append | check | query` is the durable review-telemetry store behind them.
- **A REJECT opens a local decision.** Choose `fix` or `halt` for that gate. Other eligible work can continue. A `halt` answer stops every work item in the spec.

Verdicts persist as JSON alongside the spec, so every merge carries its review history in the repo.

## Planning is research-backed

Task decomposition quality is one of the strongest predictors of agent execution success. MindSpec's plan gate encodes thresholds derived from *Towards a Science of Scaling Agent Systems* (Kim et al., 2025 — [arXiv:2512.08296](https://arxiv.org/abs/2512.08296), v3 2026: 260 configurations across six agentic benchmarks, five architectures, and three LLM families):

| Signal | Threshold | Why |
|:-------|:----------|:----|
| Beads per plan | 3–5 (>6 needs justification) | Coordination overhead grows super-linearly with agent count |
| Scope overlap between beads | ~0.41 optimal; >0.50 flagged | Moderate overlap gives shared context; high overlap means duplicated work |
| Dependency chain depth | ≤3 | Serial chains degraded performance 39–70% in benchmarked tasks |
| Tool-heavy operations | kept in one bead | Fragmenting tool context across agents costs ~6× efficiency |
| Trivial beads | folded into neighbors | A rename doesn't justify an agent session |

The full decision framework lives in [project-docs/research/scaling-agent-systems.md](project-docs/research/scaling-agent-systems.md). Your agent doesn't need to remember any of this — the validator applies it to every plan.

## Governed autonomy

MindSpec has one workflow for each newly approved plan. Run its commands yourself,
or use `/ms-spec-autopilot` to dispatch agents. The binary selects the same steps in both cases.
Plan approval enrols the spec and pins its round allowance and lease duration.

```yaml
panel:
  reviewers: [{family: harness, count: 4}]
  max_rounds: 5
  lease_ttl: 2h
```

Use `mindspec next-action <spec> --json` to inspect the next steps without writing.
Add `--apply` to create the authorised steps. Lease actors, dispatch them, submit
committed evidence, and execute each `run` entry. Repeat until the status is `done`.
The binary returns waiting steps, flags and decisions with the actions.

Round limits count all panel registrations for the gate, including abandoned panels.
At the limit, the user chooses `continue:<n>` or `halt`. A REJECT returns a local
`fix` or `halt` decision. A halt stops every work item and cannot be resumed.
Implementation approval prints an audit of overrides, decisions and review changes.

People who work without MindSpec can use bd and git in the same store.
Run `bd ready --exclude-label mindspec` to show only work outside MindSpec.
MindSpec preserves those items and respects their blocking links.

See the [autonomy guide](project-docs/user/guides/autonomy.md) for the commands and recovery flow.

## Architecture guardrails

MindSpec borrows bounded contexts from domain-driven design and makes them operational:

- **Domains** (`mindspec domain add|list|show`) are bounded contexts with their own docs and an `OWNERSHIP.yaml` manifest mapping them to code paths. A `context-map.md` records the relationships between them.
- **Specs declare impacted domains**, and context packs expand through the context map so the agent sees neighboring contexts it will touch.
- **ADRs** (`mindspec adr create|list|show`) are the architecture's memory: auto-numbered, domain-tagged, with a governed superseding workflow. Plans must cite ADRs covering every impacted domain; the divergence gate blocks any merge whose diff touches a domain with uncited decisions. Deviating means creating a superseding ADR — a human decision with an audit trail, not a silent drift.
- **Where a rule lives is itself governed**: enforcement ratchets downward from agent skills → mode-selected guidance → declared config → in-binary gates. A rule that proves load-bearing (or gameable) in a prompt ratchets down into the binary — never casually back up.

## Quickstart

### Install

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/mrmaxsteel/mindspec/main/install.ps1 | iex
```

Also available from [GitHub Releases](https://github.com/mrmaxsteel/mindspec/releases) (cosign-signed from v0.8.0 — see [SECURITY.md](SECURITY.md#verifying-releases)) or from source: `make build`. Upgrade by re-running the installer with `--force`.

You'll also need [Beads](https://github.com/gastownhall/beads) (`bd`) and git. `mindspec doctor` checks the full setup.

### New project

```bash
cd your-project
mindspec init            # scaffold .mindspec/ + AGENTS.md
mindspec setup claude    # or: codex, copilot — hooks, skills, gates
```

Then tell your agent what to build. The SessionStart hook runs `mindspec instruct`, which emits mode-appropriate guidance derived from the current state — the agent knows where it is in the lifecycle without you explaining anything:

1. **Explore** (optional) — "I have an idea about X"; the agent assesses feasibility, you decide go/no-go
2. **Spec** — the agent drafts, the grill interrogates, you approve
3. **Plan** — the agent decomposes, the validator checks the research thresholds, you approve
4. **Implement** — the bead loop runs: fresh agent per bead, panel per merge
5. **Review** — final panel over the whole branch, you approve, spec merges to main

Use `/ms-spec-autopilot` to execute the returned workflow for the whole spec. Each open decision returns to you.

### Existing codebase

```bash
cd existing-project
mindspec onboard --infer   # reverse-onboard: inferred context map, domains,
                           # ownership manifests, and as-built ADRs (Status: Proposed)
mindspec setup claude
```

Inferred ADRs are marked as such and count as provisional coverage — promoting them to Accepted is an explicit ceremony, so the agent's guesses about your architecture never silently become the record. A `mode: brownfield` profile softens doc-sync from error to warning while the friction journal keeps score of the actual skew.

For repos you don't want to onboard at all, `/ms-fix-cycle` runs a governed fix lane — discover, reproduce in a sandbox, patch with one commit, panel-review, PR with CI watch — with no `.mindspec/` required and the merge click left to a human.

## Works with your agent

**Claude Code is the first-class integration** — `mindspec setup claude` installs the hooks, the `ms-*` skill family, and the gates. **OpenAI Codex CLI** and **GitHub Copilot** are supported the same way (`mindspec setup codex|copilot`). Codex is one supported harness. A process reviewer needs a configured runner.

Portability is a design principle, not an aspiration: agents integrate at the **artifact + CLI contract** level — beads, spec files, `panel.json`, and the `mindspec` verbs — never at the prompt-format level. Orchestration runners are adapters selected by one config key (`runner:`), so wiring up another agent (opencode, pi, your in-house harness) means writing an adapter behind existing contracts, not forking the framework. Contributions welcome.

MindSpec is CLI-first and works standalone: every gate, validator, and panel verb is a testable command, which is also what makes the unattended rungs of the ladder trustworthy.

## Built with itself

MindSpec is fully self-hosted: **every feature since day one has shipped through its own lifecycle** — 100+ specs, 40+ architecture decision records, and, since the panel gate landed, a review-verdict trail carried with every merge — all in this repo. The `.mindspec/` directory here isn't a demo; it's the actual development history.

It is also continuously tested against real agents: a behavioral harness runs live LLM sessions through every lifecycle phase and scores them on forward progress, retries, and wasted turns. The harness's failure taxonomy (skipped gates, shortcut closes, commits to main) doubles as the live monitor set for unattended loops — failures observed in testing become halt conditions in production.

## Design principles

1. **Guardrails over guidance** — rules that matter live in the binary and exit non-zero; prompts are advice
2. **Maker ≠ verifier** — the implementing agent never judges or merges its own work
3. **Spec-anchored** — all code traces to a versioned spec with falsifiable acceptance criteria
4. **Fresh context per work item** — enforced, because state belongs on disk, not in the window
5. **Evidence over assertion** — beads close on proof; missing artifacts block regardless of votes
6. **Docs-first** — doc-sync is a gate; documentation debt can't accumulate silently
7. **Human gates for divergence** — architecture deviations require a superseding ADR; halting is the default
8. **Scope discipline** — discovered work becomes new beads, never scope creep
9. **Deterministic context** — token-budgeted, provenance-hashed context packs, not "go read the repo"
10. **Portability by contract** — integrate at artifacts and CLI verbs, never at prompt formats

## Documentation

| Goal | Guide |
|:-----|:------|
| Full workflow with Claude Code | [Claude Code guide](project-docs/user/guides/claude-code.md) |
| Full workflow with Codex | [Codex guide](project-docs/user/guides/codex.md) |
| Full workflow with Copilot | [Copilot guide](project-docs/user/guides/copilot.md) |
| Running the governed workflow | [Autonomy guide](project-docs/user/guides/autonomy.md) |
| Onboarding an existing codebase | [Brownfield guide](project-docs/user/guides/brownfield.md) |
| Configuring and running review panels | [Review panels guide](project-docs/user/guides/review-panels.md) |
| Contributing to MindSpec | [CONTRIBUTING.md](CONTRIBUTING.md) |
| The decomposition research behind the plan gate | [scaling-agent-systems.md](project-docs/research/scaling-agent-systems.md) |
| Workflow state machine (allowed transitions) | [WORKFLOW-STATE-MACHINE.md](.mindspec/core/WORKFLOW-STATE-MACHINE.md) |
| Complete command reference | [USAGE.md](.mindspec/core/USAGE.md) |
| Observability (OTEL + AgentMind) | [AgentMind guide](project-docs/user/guides/agentmind.md) — `mindspec otel setup --endpoint <url>` points telemetry at any OTLP/HTTP receiver |
| Release history | [CHANGELOG.md](CHANGELOG.md) |

## Project structure

```
your-project/
├── .mindspec/
│   ├── config.yaml          # panels, models, runner, policy defaults
│   ├── specs/               # versioned specs + plans + panel verdicts
│   ├── adr/                 # Architecture Decision Records
│   ├── domains/             # bounded contexts + OWNERSHIP.yaml manifests
│   ├── context-map.md       # relationships between domains
│   ├── reviews/             # ad-hoc review panels
│   └── core/                # reference docs (USAGE, MODES, state machine)
├── .beads/                  # the work graph (committed, git-native)
├── .claude/                 # agent integration (or .agents/, .github/)
├── AGENTS.md                # cross-agent conventions
└── CLAUDE.md                # Claude Code entry point
```

## Requirements

- Go 1.23+ (building from source only)
- [Beads](https://github.com/gastownhall/beads) CLI (`bd`)
- Git (worktree support)
- Git 2.43 or later is needed for a spec or plan panel whose target is a commit.
- A coding agent — Claude Code, Codex CLI, or Copilot for the integrated workflow; the CLI works standalone

## Building

```bash
make build      # Build to ./bin/mindspec
make test       # Run the short test suite
```

## License

MIT — see [LICENSE](LICENSE).
