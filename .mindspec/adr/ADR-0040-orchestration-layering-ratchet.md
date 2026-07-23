# ADR-0040: Orchestration Layering Ratchet — Gates, Config, Instruct, Skills; and the Portability Principle

- **Date**: 2026-07-07
- **Status**: Accepted
- **Domain(s)**: core, workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (the decision matrix this ADR names as L1, mechanized), [ADR-0034](ADR-0034-ceremony-collapse.md) (the ceremony-collapse precedent this ADR cites as already-shipped ratcheting), [ADR-0030](ADR-0030-executor-boundary.md) (the `Executor` boundary cited as a healthy interface born with a real second consumer), [ADR-0032](ADR-0032-adr-semantic-gates.md) (the Accepted-vs-Proposed distinction this codification ADR lands under), [ADR-0038](ADR-0038-friction-reporter.md) (the friction journal that supplies evidence for a rule ratcheting down), [ADR-0039](ADR-0039-flat-layout-v2.md) (the artifact layout the L1/L2 contract in this ADR rides on), [ADR-0036](ADR-0036-ownership-discovery.md) (the ZFC guidance stack — schema block, populate prompt, doctor nudge — the spec 123 amendment below extends verbatim to `models:` and `commands:`), [ADR-0035](ADR-0035-agent-error-contract.md) (the recovery-line contract every new finding this amendment's checks introduce must carry)

---

## Context

MindSpec has four surfaces that steer agent behavior, and by spec 109 they had
blurred into an undeclared continuum rather than a designed layering:

- **In-binary gates** — `internal/panel/gate.go`, `mindspec complete`'s panel
  check, doc-sync and ADR-divergence checks — exit non-zero and cannot be
  talked past.
- **`.mindspec/config.yaml`** — a thin, mostly-unused settings surface
  (`enforcement.panel_gate`, worktree and merge-strategy knobs) with no
  orchestration vocabulary at all: no panel mix, no model tiering, no loop
  governance.
- **`mindspec instruct`** — mode-selected prose advice, generated per lifecycle
  state, that already partially *describes* what the gates do.
- **`ms-*` skill files** — the largest surface by word count, mixing genuine
  judgment calls (how to phrase a panel BRIEF, how to grill a spec) with
  *re-stated gate logic* that has no business living in prose at all.

The failure mode this ADR names is concrete, not hypothetical. Before spec 093,
"the panel must approve before merge" lived only in skill prose; the lola-f4a8
incident (spec-050 shipped without its AC8c cost-projection artifact because a
stale APPROVE was honored) happened because a rule that should have been a gate
was instead a sentence an agent could skim past — the incident that motivated
ADR-0037. By the 107/108 delivery runs the same pattern had recurred one layer
up: the 6-reviewer panel mix and the N−1 threshold lived only in `SKILL.md`
prose plus a hand-typed `panel.json`; model tiering (impl=opus, review=fable)
lived in an operator memory file, not the repo; the codex-quota-wall
substitution policy was improvised live. None of these were gate-worthy
invariants — they are *policy*, legitimately variable across operators and
runs — but they also weren't declared anywhere machine-readable, so every run
re-derived them from memory or prose under time pressure.

The 2026-07-02 loop-engineering research
(`project-docs/research/loop-engineering-adaptation.md`) frames the missing
piece as the "loop head" and is emphatic about the invariant any config
substrate underneath it must preserve: **the decision matrix stays in the
binary** (as ADR-0037 did for the bead-merge gate — "identical decision over
identical facts"); config may only select *who holds the authority*, never
*what the evidence is*. Spec 109's config substrate (the `panel:` / `models:` /
`loop:` blocks) is built directly on top of the layering this ADR now names, so
the layering has to be written down before the substrate can cite it as
architectural cover.

This ADR is a **codification**, not a proposal: the layering it names already
governs the shipped codebase (see the precedent chain in Decision §2). Per
ADR-0032's Accepted-vs-Proposed distinction, a codification ADR that documents
an architecture already in production lands **Accepted** on arrival — matching
how ADR-0037 and ADR-0034 themselves landed — rather than Proposed awaiting
validation.

## Decision

### 1. Four layers, one-directional ratchet

MindSpec's behavior-steering surfaces are, in order of enforcement strength:

- **L1 — in-binary gates.** Deterministic, unarguable invariants that live in
  Go and exit non-zero on violation, with any bypass recorded through a
  journaled escape hatch (never a silent skip). Example: `internal/panel`'s
  `PanelGateDecision`, `mindspec complete`'s panel/doc-sync/ADR-divergence
  checks. An L1 rule cannot be talked past by an agent, only bypassed through
  an audited hatch.
- **L2 — declared config.** Machine-readable, parsed, defaulted, and validated
  policy — `.mindspec/config.yaml` — consumed *by* gates and by orchestration,
  but not itself gate logic. L2 is the **middle rung**: it is how a prose rule
  becomes machine-readable *without* becoming code. A config value can be
  wrong or misconfigured, but it cannot silently drift from itself the way two
  copies of the same rule in two prose files can drift from each other — it
  has exactly one parsed representation.
- **L3 — mode-selected `instruct` advice.** Generated, not authored-by-hand,
  prose that tells an agent what to do next given the current lifecycle phase.
  Where `instruct` output *describes* a gate, it is generated from the gate's
  own definition rather than hand-copied, so the description cannot drift from
  the gate it describes.
- **L4 — judgment-kernel skills.** `ms-*` `SKILL.md` files. This is where
  genuine judgment lives (grilling a spec, phrasing a panel BRIEF, deciding
  how to decompose a plan into beads) and it is also the **staging area** for
  procedure that has not yet proven itself: a new rule is *drafted* in a skill
  before it has earned a home lower in the stack.

**The ratchet is one-directional.** A rule that starts in a skill (L4) and
proves itself **load-bearing** (the run breaks without it) or **gameable** (an
agent under pressure can skim past it, restate it wrong, or "forget" it under
compaction) ratchets **down** — into declared config (L2) if it is legitimate
per-run policy, or into a binary gate (L1) if it is an invariant that must
never be violated. A rule never moves back up casually: a gate is not
relaxed into a config knob, and a config knob is not relaxed into skill
prose, merely because it would be more convenient to edit as text. Moving up
is not forbidden outright — an invariant can, in principle, be found to be
over-strict and relaxed — but it requires the same deliberate ADR-level
scrutiny as the original ratchet down, not an incidental refactor. The
direction of least resistance is **never casually up**.

This gives MindSpec a default answer to a question that previously had none:
**"where does this rule live?"** — start it at L4 if its correctness depends on
judgment; move it to L2 the moment it is a checkable fact about a run
(a count, a threshold, an enum, a boolean); move it to L1 the moment violating
it must be structurally impossible rather than merely discouraged.

### 2. Precedent chain — already-shipped evidence

This layering is a codification because the ratchet has already fired three
times in the shipped codebase:

- **Spec 102: the PreToolUse-hook → in-binary panel-gate move.** The panel
  approval check started as a `PreToolUse` hook heuristically string-matching
  shell commands (an L3/L4-adjacent surface, per ADR-0037's original text) and
  was relocated to the authoritative in-binary gate inside `mindspec complete`
  (ADR-0037's 2026-06-14 and 2026-06-17 amendments) — a rule ratcheting
  **down** into L1 from a best-effort heuristic, with the heuristic
  predecessor retired once the mechanized gate was proven sufficient.
- **ADR-0037's mechanized decision matrix.** "The panel must approve before
  merge" started as skill prose (pre-093) and became `internal/panel`'s single
  `Tally`/`PanelGateDecision` implementation, called identically by every
  consuming surface — a rule ratcheting from L4 prose into L1 code after the
  lola-f4a8 incident proved prose insufficient under pressure.
- **ADR-0034's ceremony collapse.** The spec → plan → impl → complete
  progression was represented by hand-scaffolded "ceremony" beads (a
  procedural convention living partly in skill practice); it collapsed into
  `mindspec_phase` metadata derived and read by the binary — a rule ratcheting
  from ad hoc procedure into a declared, mechanically-derived state field.

Spec 109's own `panel:` / `models:` / `loop:` config blocks are the next turn
of the same ratchet, applied prospectively: they take exactly the policy that
lived only in `SKILL.md` prose and operator memory during the 107/108 runs
(the panel mix, the threshold, model tiering, the quota-substitution rule) and
give it a single declared, parsed, defaulted, validated home at L2 — without
promoting any of it to L1, because none of it is a violate-never invariant;
it is legitimate per-run policy that an operator or orchestrator may
legitimately set differently across runs.

### 3. The portability principle

Agent integration happens at the **artifact + CLI contract** level — the
`panel.json` and verdict-JSON schemas, the `mindspec` CLI surface (today's
lifecycle verbs, the future `mindspec panel` verbs), `mindspec instruct`
output, and beads state — and **never** at the prompt-format level.
Orchestration runners (Claude Code skills/workflows today; opencode,
codex-cli, copilot-cli, or others later) are **adapters** behind those
contracts, not the contracts themselves. A runner may render `instruct`
output as a slash-command skill, a workflow step, or a human-read terminal
message; what it may never do is require the *contract* — the schema a gate
reads, the exit code a gate returns — to change shape to suit one runner's
prompting idiom. This is the config expression of Requirement 10's `runner:`
selector in spec 109: `runner` names which adapter is in front of the
contracts, and the contracts themselves are runner-agnostic by construction.

**In-repo precedent, both directions.** `internal/setup`'s per-agent trio
(`claude.go` / `codex.go` / `copilot.go`, each installing that runner's own
integration surface against the same underlying skill/hook artifacts) and the
`Executor` interface (ADR-0030 — extended with three new methods only once a
second real caller needed them) are healthy interfaces: each was born, or
grew, with a **real second consumer already in hand**, so its shape reflects
actual variation rather than speculative generality. Contrast
`internal/harness.Agent` (`internal/harness/agent.go`): an interface
abstracting "a coding agent" ahead of having more than one disciplined
consumer risks accreting methods that only one implementation ever uses —
abstraction ahead of a second implementation is rot, not portability. The
portability principle this ADR states is deliberately narrow: contracts are
justified by an existing or concretely-planned second consumer (a second
runner adapter, a second panel-writing verb), not by anticipating one.

**Capability-tier / degraded-modes note.** Runners are not uniform: Claude
Code has a `SessionStart` hook and subagent spawning; codex today has neither.
The contract this ADR names therefore defines **degraded modes**, not a
single required feature set — a runner without hook support falls back to a
human running `mindspec instruct` directly; a runner without a panel
mechanism falls back to panel-less operation under
`enforcement.panel_gate: false` (ADR-0037 §7's config toggle). A contract that
silently assumed every runner has Claude Code's full surface would not be
portable; naming the degraded path for each missing capability is what makes
the artifact + CLI contract the actual integration seam rather than an
aspirational one.

> **Amendment (2026-07-23, spec 123 — the consumer-identity clause):**
> §1's layering doctrine was written for rules mindspec enforces ON
> ITSELF (which surface holds "the panel must approve," "the threshold
> is n-1"). Spec 123 (GH #211) found the identical failure mode one
> layer outward, in content mindspec GENERATES **INTO** a consuming
> repo: `init`'s starter `AGENTS.md` and `setup codex`'s managed block
> hardcoded mindspec-the-framework's OWN repo facts — its title ("# AGENTS.md
> — MindSpec Project") and its OWN Go build (`make build`/`make test`) —
> directly into prose, with no declared home and no override, so every
> onboarded consumer's agent read instructions to run commands that do
> not exist in their repo. This is the exact L1-vs-hardcoded-prose
> failure this ADR already names (§1's "where does this rule live?"),
> just aimed at generated artifacts instead of binary behavior. This
> amendment states the **consumer-identity clause** the ADR Touchpoints
> promised: it extends the ratchet's own logic — hardcoded prose facts
> drift; declared config has exactly one parsed representation —
> outward, to content mindspec writes into a consumer's tree.
>
> **The clause.** A managed or scaffolded artifact mindspec generates
> into a consuming repo (a starter doc, an appended managed block, a
> commented config schema) may carry only framework-generic guidance or
> the consumer's own L2-declared values — never mindspec-the-framework's
> own repo facts:
>
>   1. **Framework-generic guidance** — content that is true of every
>      consumer equally (e.g. "run `mindspec instruct` for
>      mode-appropriate operating guidance"), or
>   2. **Values sourced from the consumer's own L2 declared config**
>      (`.mindspec/config.yaml`) — never mindspec-the-framework's own
>      repo facts baked in as if they were universal.
>
> A repo-specific fact that generated content needs (the consumer's
> build/test commands were the #211 case; the per-phase model protocol,
> #210's `models:`, is the sibling case) either **gets an L2 home** —
> this spec gives build/test its own declared key (`commands:`, beside
> `models:`) — **or is omitted entirely**, never guessed (the ADR-0036
> ZFC posture, applied to generated content the same way it already
> applies to doctor's advisory nudges): an unset key renders NO
> placeholder that could read as runnable, just a clean omission plus a
> doctor nudge naming the populate path.
>
> **The honesty half.** A declared-but-inert L2 key (`models:` today;
> any future key with no wired enforcement) must SAY it is inert
> **everywhere it is surfaced** — the schema block scaffolded by
> `doctor --fix`, the ZFC populate-prompt command, `mindspec config
> show`'s own annotation — and each surface must name **today's
> authoritative consumer** (for `models:`, the orchestration skills, not
> the mindspec binary). A declared key that goes quiet about its own
> inertness is exactly the kind of undeclared-continuum drift §1 exists
> to prevent, just at the config layer instead of the gate layer:
> nothing stops an operator from assuming a declared key does something
> it does not, absent this standing honesty requirement.
>
> **Extends ADR-0036, doesn't replace it.** The `source_globs:` pattern
> ADR-0036 already ships (commented schema scaffold, ZFC populate
> prompt, doctor nudge, framework proposes no values) is the concrete
> mechanism this clause generalizes to every future L2 surface that
> backs generated content — spec 123 applies it a second and third time
> (`models:`, `commands:`) verbatim, per this amendment's own logic:
> reuse the ratchet's shipped shape rather than inventing a parallel
> one.
>
> This is an amendment, not a supersession: no existing clause is
> displaced, and §1's four-layer ratchet is unchanged for in-binary
> behavior — this only states that the SAME "prose drifts, declared
> config doesn't" logic governs content the binary writes outward, too.

## Consequences

### Positive

- Every future "where should this rule live?" question has a default answer
  instead of an ad hoc one, and the answer is directional: skills draft,
  config declares, gates enforce — never the reverse by default.
- The config substrate this ADR licenses (spec 109's `panel:` / `models:` /
  `loop:` blocks) has a named architectural home instead of being an isolated
  schema addition; later specs (110, 111) can cite ADR-0040 as `core`/
  `workflow` coverage the moment they touch the same layers.
- The portability principle gives spec 111's non-default `runner:` adapters
  (and any future runner) a concrete boundary to implement against, and
  explains why `internal/harness.Agent`-style interface sprawl is a named
  anti-pattern to avoid repeating in the adapter layer.

### Negative / Tradeoffs

- The ratchet is a discipline, not a mechanism: nothing in the binary
  currently *detects* a load-bearing or gameable skill rule and forces its
  demotion — that judgment stays human (or agent-with-review), same as every
  other architectural call in this repo. This ADR names the standard; it does
  not automate applying it.
- "Never casually up" is deliberately not "never up" — an invariant later
  found to be over-strict can still be relaxed, but only through the same
  ADR-level scrutiny as its original ratchet down, which is friction by
  design, not an oversight.
- The degraded-modes note commits future runner adapters to explicitly
  documenting what they cannot do, rather than silently omitting the
  unsupported surface — more upfront honesty, at the cost of a slightly
  longer adapter checklist.

## Alternatives Considered

### 1. Leave the four surfaces as an undeclared continuum

Rejected: this is the status quo that produced the 107/108 friction — the
same rule (panel mix, threshold) re-derived from prose and memory on every
run because no layer below L4 was declared to hold it. Naming the layers and
the ratchet direction is what turns "we should really config-ify that
someday" into a standing default.

### 2. Automate the ratchet (detect load-bearing/gameable rules mechanically)

Rejected for this ADR's scope: no such detector exists, and building one is a
substantially larger undertaking than codifying the direction of travel.
Nothing here forecloses a future mechanized detector; it would slot in as
evidence feeding the same human/agent-reviewed promotion decision.

### 3. A single contract that assumes Claude Code's full runner surface

Rejected per the portability principle: a contract that requires
`SessionStart` hooks or subagent spawning would not be portable to codex or
future runners lacking them. Degraded modes are named explicitly instead of
being an implicit gap discovered later.

### 4. Build the adapter interface (`runner:` dispatch) ahead of a second real consumer

Rejected per the `internal/harness.Agent` counter-example cited above: an
adapter interface designed before spec 111's concrete second runner exists
would be speculative and likely to need reshaping once that consumer is real.
Spec 109 declares and validates the `runner:` key; the adapter interface
itself is deferred to spec 111, which supplies the second consumer that
justifies its shape.
