# MindSpec

**An opinionated loop-engineering framework for AI coding agents.**

Spec-driven development, research-backed planning, architecture guardrails, and adversarial review panels — so agents can build resilient software semi-autonomously, with the engineering discipline enforced by code instead of prose.

---

AI coding agents are phenomenal executors and unreliable engineers. Left to themselves they drift from intent, steamroll architecture decisions, skip documentation, and let a "small feature" become a three-subsystem refactor. The usual answer is to watch them more closely. MindSpec's answer is to engineer the loop instead.

> "Loop engineering means replacing yourself as the person who prompts the agent. You design the system that does it instead." — Addy Osmani

Prompt engineering optimizes a single instruction. Harness engineering optimizes a single session. **Loop engineering** builds the system that decides what to work on, whether the result is acceptable, and when to stop. MindSpec is that system: a CLI plus a set of agent skills that wrap your coding agent in a gated lifecycle — **spec → plan → implement → review** — where every gate is a validation in a Go binary that exits non-zero, not an instruction the model might ignore.

An unattended loop cannot be argued with, so its guardrails must not be arguable either. That is the core opinion, and everything else follows from it.

```
 idea ─▶ SPEC ══gate══▶ PLAN ══gate══▶ IMPLEMENT ═══════▶ FINAL REVIEW ══gate══▶ main
          │              │              │
      adversarially   decomposition    the bead loop, per work item:
      grilled until   checked against    • fresh agent, isolated git worktree
      falsifiable     published          • exactly one commit, tests must pass
                      research           • 6-reviewer panel, two model families
                                         • merge gate enforced in the binary
```

## What you get

- **Specs that survive contact with an agent** — every spec is interrogated by an adversarial grill agent until each requirement is concrete, falsifiable, and grounded in the actual repo. Vague verbs, contradictions, and untestable claims don't make it past the first gate.
- **Research-backed planning** — decomposition quality is the #1 predictor of agent success. Plans are validated against published thresholds for bead count, scope overlap, and dependency-chain depth (see [Planning](#planning-is-research-backed)).
- **A work graph, not a chat history** — work items live in [Beads](https://github.com/gastownhall/beads), a git-native issue tracker. Each bead is a self-contained work packet a fresh agent can pick up with zero session history.
- **Maker ≠ verifier** — the implementing agent makes exactly one commit and cannot merge its own work. A six-reviewer panel with six distinct lenses across two model families is the verifier; the decision matrix is a pure function in the binary.
- **Architecture that can't be steamrolled** — plans must cite ADRs covering every impacted domain. If the diff touches a domain whose decisions weren't cited, the merge gate blocks until a human approves a superseding ADR.
- **Docs that can't rot** — a bead cannot close while source and documentation have drifted apart. Doc-sync is a gate, not a convention.
- **An autonomy ladder, not an autonomy switch** — run interactively, run a supervised autopilot, or grant a governed unattended loop with named halt conditions, budget ceilings, and a handoff log. You choose the rung per project ([The autonomy ladder](#the-autonomy-ladder)).
- **Every escape hatch audited** — overrides like `--override-adr` and `--allow-doc-skew` exist, but each use is journaled to a redacted, local friction log. Where overrides concentrate is where the system improves next.

## The loop

Every phase transition is a gate. Who holds each gate — you or a review panel — is configuration; what the gate checks is not.

**Spec** — define what "done" looks like: problem statement, ≥3 falsifiable acceptance criteria each paired with a runnable proof, impacted domains, ADR touchpoints. The `ms-spec-grill` skill then interrogates the draft one question at a time, hunting synonym-dodges ("support", "handle", "improve"), non-falsifiable claims, cross-requirement contradictions, and repo claims the tree doesn't back up. No code allowed.

**Plan** — decompose the spec into beads: independently completable work items with per-bead acceptance criteria. The plan validator checks decomposition against published research thresholds and requires ADR citations covering every impacted domain. If the plan needs to deviate from a cited ADR, it stops and escalates — you approve a superseding ADR or reject the divergence.

**Implement** — the bead loop. `mindspec next` claims the next ready bead and creates an isolated git worktree; a fresh agent implements it with a deterministic, token-budgeted context pack; it makes exactly one commit; a review panel judges the diff; `mindspec complete` runs the doc-sync, ADR-divergence, and panel gates before merging bead → spec branch. Discovered work becomes new beads — never scope creep in the current one.

**Review** — after the last bead merges, a final panel reviews the cumulative spec branch against main: scope drift, inter-bead coherence, release readiness. Approval merges spec → main via PR, and the lifecycle returns to idle.

State never lives in the context window. The lifecycle phase is derived from the beads graph and git — which means a crashed or restarted agent loses nothing, and a fresh agent can always reconstruct exactly where the work stands.

## Beads: a work graph your agent can actually use

MindSpec tracks work in [Beads](https://github.com/gastownhall/beads) — a git-native issue tracker that lives in your repo and needs no external service. This is not an incidental choice; it's what makes the loop possible.

Each bead is a **self-contained work packet**: requirements, per-bead acceptance criteria, impacted domains, cited ADRs, dependency edges, and completion evidence. A fresh agent picking up a bead needs no session history and no tribal knowledge — `mindspec context bead <id>` assembles a deterministic, token-budgeted context pack (spec, plan section, cited ADR decisions, domain docs, file paths — with a SHA-256 provenance record of every input) and the agent gets exactly what the plan intended it to see.

Fresh context per work item isn't a suggestion, it's enforced: the session-freshness gate hard-errors if an agent tries to claim a bead from a stale, compacted, or already-claimed session. Context quality degrades as sessions age; MindSpec makes the fresh start mandatory rather than hopeful.

## Review panels

Every bead merge — and, on the higher autonomy rungs, every gate — is judged by a review panel:

- **Six reviewers, two model families**, launched in parallel. Cross-family review catches what any single model family systematically misses.
- **Six distinct lenses, not six clones**: author-of-record (does the diff match the plan?), codebase pin (do the files and tests actually exist and pass?), contract stability, empirical prober (runs the validators by hand), schema correctness, and next-bead integration.
- **N−1 approval threshold** — one dissent is tolerated; two is a fix round.
- **Artifact hard gates** — a finding that names a missing measurement artifact (a benchmark, a cost projection, a regression baseline) blocks regardless of the vote count. Agents can't vote evidence into existence.
- **The decision matrix is a pure function in the binary** — identical facts produce an identical decision, whether the panel was launched by a human, a skill, or an unattended loop. `mindspec panel create | verify | tally` are the agent-neutral verbs; any orchestrator that can run a CLI can run a panel.
- **A REJECT halts the track.** Auto-fixing a rejection is the definition of verification debt, so the loop never does it.

Verdicts persist as JSON alongside the spec, so every merge carries its review history in the repo.

## Planning is research-backed

Task decomposition quality is the strongest predictor of agent execution success. MindSpec's plan gate encodes the thresholds from *Towards a Science of Scaling Agent Systems* (Kim, Shen, Saphra & Rush, 2025 — [arXiv:2512.08296](https://arxiv.org/abs/2512.08296); 180 multi-agent configurations across 5 architectures and 4 benchmarks):

| Signal | Threshold | Why |
|:-------|:----------|:----|
| Beads per plan | 3–5 (>6 needs justification) | Coordination overhead grows super-linearly with agent count |
| Scope overlap between beads | ~0.41 optimal; >0.50 flagged | Moderate overlap gives shared context; high overlap means duplicated work |
| Dependency chain depth | ≤3 | Serial chains degrade multi-agent performance by 39–70% |
| Tool-heavy operations | kept in one bead | Fragmenting tool context across agents costs ~6× efficiency |
| Trivial beads | folded into neighbors | A rename doesn't justify an agent session |

The full decision framework lives in [project-docs/research/scaling-agent-systems.md](project-docs/research/scaling-agent-systems.md). Your agent doesn't need to remember any of this — the validator applies it to every plan.

## The autonomy ladder

Autonomy in MindSpec is a ladder you climb deliberately, not a switch you flip and hope.

| Level | Trigger | Gate authority |
|:------|:--------|:---------------|
| **0 — Interactive** | you | you approve every gate |
| **1 — Assisted loop** | you start `/ms-spec-autopilot` | you approve spec and plan; panels verify every bead |
| **2 — Governed loop** | you grant a scoped run | panels hold every gate, under named halt conditions and a handoff log |
| **3 — Scheduled loop** | cron / heartbeat | as level 2, plus budget ceilings per wake |
| **4 — Fleet** | a queue of specs | as level 3, parallel across specs (beads stay serial within a spec, by design) |

The governance profile lives in `.mindspec/config.yaml`, and it selects **who holds each gate — never what the evidence is**:

```yaml
loop:
  enabled: true
  gate_authority:
    spec_approve:  panel      # panel | human
    plan_approve:  panel
    bead_merge:    panel
    impl_approve:  panel
    panel_skip:    human      # not delegable — a loop never self-authorizes skipping its verifier
  panel_quorum: {reviewers: 6, approve_threshold: n-1}
  halt:
    max_rounds_per_bead: 3
    max_consecutive_impl_failures: 2
    panel_deadlock_rounds: 2
    on_reject: halt           # a REJECT always stops the track
  budget: {max_beads_per_wake: 4}
  handoff_log: .mindspec/loop/AUTOPILOT-LOG.md
```

Some things deliberately stay human at every level: skipping a panel, waving through a rejection, and accepting missing evidence. Halting is the default for anything not explicitly delegated. The handoff log records every panel-substituted decision with its vote, so your review moves from per-decision to per-batch — it doesn't disappear. `mindspec loop status` is the supervisor's poll surface: open panels, rounds consumed, budget spent, escape hatches used, halt state.

> "A loop running unattended is also a loop making mistakes unattended." — Addy Osmani, on verification debt. The governance profile exists so that debt is bounded, visible, and yours to review — not compounding silently.

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

When you're ready to loosen your grip, `/ms-spec-autopilot` runs the whole bead loop for a spec (level 1), and the `loop:` profile takes you up the ladder from there.

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

**Claude Code is the first-class integration** — `mindspec setup claude` installs the hooks, the `ms-*` skill family, and the gates. **OpenAI Codex CLI** and **GitHub Copilot** are supported the same way (`mindspec setup codex|copilot`), and Codex additionally serves as the second model family on review panels.

Portability is a design principle, not an aspiration: agents integrate at the **artifact + CLI contract** level — beads, spec files, `panel.json`, and the `mindspec` verbs — never at the prompt-format level. Orchestration runners are adapters selected by one config key (`runner:`), so wiring up another agent (opencode, pi, your in-house harness) means writing an adapter behind existing contracts, not forking the framework. Contributions welcome.

MindSpec is CLI-first and works standalone: every gate, validator, and panel verb is a testable command, which is also what makes the unattended rungs of the ladder trustworthy.

## Built with itself

MindSpec is fully self-hosted: **every feature since day one has shipped through its own lifecycle** — 100+ specs, 40 architecture decision records, and a review panel verdict trail for every merge, all in this repo. The `.mindspec/` directory here isn't a demo; it's the actual development history.

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
| The decomposition research behind the plan gate | [scaling-agent-systems.md](project-docs/research/scaling-agent-systems.md) |
| Loop engineering: design notes and the maturity ladder | [loop-engineering-adaptation.md](project-docs/research/loop-engineering-adaptation.md) |
| Workflow state machine (allowed transitions) | [WORKFLOW-STATE-MACHINE.md](.mindspec/core/WORKFLOW-STATE-MACHINE.md) |
| Complete command reference | [USAGE.md](.mindspec/core/USAGE.md) |
| Observability (OTEL) | `mindspec otel setup --endpoint <url>` points telemetry at any OTLP/HTTP receiver, e.g. [AgentMind](https://github.com/mrmaxsteel/agentmind) |
| Release history | [CHANGELOG.md](CHANGELOG.md) |

## Project structure

```
your-project/
├── .mindspec/
│   ├── config.yaml          # panels, models, runner, loop governance
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
- A coding agent — Claude Code, Codex CLI, or Copilot for the integrated workflow; the CLI works standalone

## Building

```bash
make build      # Build to ./bin/mindspec
make test       # Run all tests
```

## License

MIT
