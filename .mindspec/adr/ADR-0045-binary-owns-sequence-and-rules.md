# ADR-0045: The binary owns sequence and rules; skills are harness adapters

- **Date**: 2026-10-18
- **Status**: Accepted
- **Domain(s)**: core, workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Amends**: [ADR-0040](ADR-0040-orchestration-layering-ratchet.md) §1 (the L4 definition)
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md), [ADR-0041](ADR-0041-gate-before-mutate.md), [ADR-0044](ADR-0044-panel-review-conduct.md)

*Written in ASD-STE100 Simplified Technical English.*

---

## Context

MindSpec has three channels that tell an agent what to do:

1. **The binary.** Lifecycle verbs and gates (ADR-0040 L1, L2).
2. **`mindspec instruct`.** Guidance that the binary generates for each phase (L3).
3. **Skills.** `SKILL.md` files that are installed into each repository (L4).

ADR-0040 lets a rule start in a skill and move down into the binary. It does not say which content a skill keeps at the end. Thus, the skills now hold three types of content: judgement prompts, command sequences, and rules.

This causes these problems:

- **Skills contradict the binary.** Examples: the bead-cycle skill says that N−1 approvals are sufficient, but the gate requires all findings to be resolved. Two skills select the next bead with different rules. One skill tells the agent to file a follow-up bead, but the gate does not accept a follow-up bead.
- **Skills drift.** Installed copies are older than the source. A repository that edits a skill keeps a frozen fork. Upgrades do not change it.
- **Skills cost context.** Each dispatch loads long skill text. Most of that text repeats rules and sequences that the binary can own.
- **Rules in prose are advice.** An agent can skip a rule that no tool enforces.

## Decision

1. **The binary owns state, sequence and rules.**
   - State: the bead graph and the panel store.
   - Sequence: a `mindspec next-action` verb returns the next step from a closed set, with its rendered brief.
   - Rules: gates (L1) and declared config (L2).
2. **`mindspec instruct` is the only guidance channel.** The binary generates it from the same rule table that the gates and `doctor` use. A rule has one home.
3. **The binary owns the judgement prompts.** A judgement prompt is a prompt for work that needs a model, for example: grill a spec, review with a lens, draft a plan, implement a bead. The binary keeps each prompt as a versioned template. A verb renders it with the current state (for example `bead prompt`, `panel brief`). The binary also validates the output contract of each prompt through `submit`.
   - A repository can override a template. It puts the override in `.mindspec/prompts/`, and config declares it.
   - `doctor` reports each override that is older than the shipped template.
4. **A skill contains only harness mechanics:** how this harness starts an agent, waits, and returns a result through `submit`. A skill does not contain:
   - a command sequence;
   - a rule ("must", "never", "only after");
   - a judgement prompt;
   - rationale or history.

   Put a command sequence in `next-action`. Put a rule in a gate or in config. Put a judgement prompt in a binary template. Put rationale in an ADR or in the docs.
5. **Staging is temporary.** A new rule can start in a skill (ADR-0040 §1). It must show the ID of its target gate or config key, and a removal date. `doctor` reports a staged rule that is past its date.
6. **A skill is a stub that loads one canonical source.** `mindspec setup` installs stubs, not full copies. `doctor` reports a skill that differs from the canonical source.

## Consequences

**Good:**
- **One home for each rule,** so a skill cannot contradict a gate.
- **Upgrades change behaviour,** because sequence and rules are in the binary.
- **Skills become very small,** so each dispatch uses less context.
- **History machinery goes:** the historical skill snapshots and their byte-exact snapshot provenance become unnecessary.
- **Both operation modes use the same verbs.** A deterministic runner and an agentic orchestrator both call `next-action`.

**Bad:**
- **More of the binary must change before skills can shrink.** A skill rule can go only after its gate exists.
- **Harness work increases.** Each harness needs its own adapter skill.
- **A repository override can become old.** `doctor` reports it, but it does not prevent it.
- **A prompt change needs a release.** A user who wants to change a prompt quickly must use an override.

## Implementation order

1. Write the rule table. Give each rule an ID, a scope, a prohibition, a replacement, and `enforced_by`.
2. Move each rule that a skill states into a gate or config key (specs 129–133).
3. Move the judgement prompts into binary templates. Add the renderers and the override path (specs 130, 131 and 133).
4. Add `next-action` (spec 133).
5. Make `instruct` render from the rule table and `next-action`.
6. Change `setup` to install stubs. Add the `doctor` drift checks for stubs and overrides.
7. Delete the sequences, rules, prompts and rationale from the skills. Keep only the harness adapters.

## Verification (2026-10-18, `main` at `e2325e36`)

The Context claims above were checked against the tree when this ADR was added by spec 129:

- *N−1 versus the gate*: `.claude/skills/ms-bead-cycle/SKILL.md` ("≥ N−1 APPROVE → merge terminal → exit") versus `.claude/skills/ms-panel-tally/SKILL.md` ("a floor, not a sufficient condition") versus `internal/panel/gate.go`'s leg (9.5): any unresolved REQUEST_CHANGES blocks.
- *Two selection rules*: `ms-bead-cycle` picks the lowest-numbered eligible bead; `ms-spec-autopilot` picks the lowest-depth bead.
- *Follow-up bead*: `ms-bead-cycle` says a dissenting verdict's changes may be "filed as a follow-up bead before merge"; the gate's unresolved-verdict leg does not accept that as resolution.
- *Drift*: the installed `.claude/skills/ms-bead-cycle/SKILL.md` was last changed 2026-07-07; `plugins/mindspec/skills/ms-bead-cycle/SKILL.md` on 2026-07-24; the two files differ.
- *History machinery*: `internal/setup/skills.go`'s `previouslyShippedSkills` matches installed skills against byte-exact embedded `historical_skills/` snapshots (not content hashes — the wording above was corrected accordingly).
- *Not yet in the tree*: `next-action`, `submit`, `bead prompt`, `panel brief`, `.mindspec/prompts/`, and the rule table. The Implementation order above is the plan for them; none is claimed as shipped.
