# ADR-0045: The binary owns sequence and rules; skills are harness adapters

- **Date**: 2026-10-18
- **Status**: Accepted
- **Domain(s)**: core, workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Amends**: [ADR-0040](ADR-0040-orchestration-layering-ratchet.md) §1 (the L4 definition, the default answer to "where does this rule live?", and the open-ended staging ratchet); [ADR-0044](ADR-0044-panel-review-conduct.md) (where reviewer-conduct text is placed)
- **Related**: [ADR-0032](ADR-0032-adr-semantic-gates.md), [ADR-0037](ADR-0037-panel-gate-enforced-contract.md), [ADR-0041](ADR-0041-gate-before-mutate.md)

*Written in ASD-STE100 Simplified Technical English.*

**Status note.** `Accepted` records the direction that the operator decided (Deciders). When this ADR was added (spec 129), no step of the Implementation order had shipped. Spec 129 moves only one rule: the override-coverage rule moves from the `ms-bead-impl` skill into `mindspec bead ready-check`. ADR-0032 (tri-state amendment) makes an `Accepted` ADR count as coverage at impl approve. Thus this ADR sets a limit: a bead can cite ADR-0045 in `adr_citations` only for a rule that the bead itself moves out of a skill into a gate, a config key or a binary template. It cannot cite ADR-0045 for code that this ADR does not yet govern.

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
   - *Relation to ADR-0040 §3.* §3 puts agent integration at the artifact and CLI contract, "never at the prompt-format level". This decision agrees with §3. A CLI verb renders each template, so a harness still integrates through the CLI. The binary owns the prompt content. It does not own a harness prompt format.
4. **A skill contains only harness mechanics:** how this harness starts an agent, waits, and returns a result through `submit`. A skill does not contain:
   - a command sequence;
   - a rule ("must", "never", "only after");
   - a judgement prompt;
   - rationale or history.

   Put a command sequence in `next-action`. Put a rule in a gate or in config. Put a judgement prompt in a binary template. Put rationale in an ADR or in the docs.
5. **Staging is temporary.** A new rule can start in a skill (ADR-0040 §1). It must show the ID of its target gate or config key, and a removal date. `doctor` reports a staged rule that is past its date. *Transition:* this marking duty applies to a rule that is staged in a skill after this ADR is accepted. Skill content that already existed is grandfathered until Implementation order step 7 removes it. The `doctor` date check starts when the rule table (step 1) exists, because the rule table holds the target IDs. This replaces two parts of ADR-0040 §1. The first is the open-ended staging area. The second is the default answer "start it at L4 if its correctness depends on judgment". Judgement now goes into a binary template (Decision 3), and a rule never goes into a skill.
6. **A skill is a stub that loads one canonical source.** `mindspec setup` installs stubs, not full copies. `doctor` reports a skill that differs from the canonical source.

### Relation to ADR-0044

ADR-0044 calls reviewer conduct "L4 skill-layer doctrine". It puts role-local conduct fragments in the five panel skills. This ADR changes only where that text is placed, not what it says. When the binary-owned `panel brief` template ships (Decision 3), the conduct fragments move into that template, and the skills cite nothing. Until then, the fragments in the skills are grandfathered under the Decision 5 transition. Their target is the `panel brief` template (specs 130–131). They are removed at Implementation order step 7. ADR-0044 stays the normative home of the conduct itself.

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

1. Write the rule table. Give each rule an ID, a scope, a prohibition, a replacement, and `enforced_by` (spec 130).
2. Move each rule that a skill states into a gate or config key (specs 129–133).
3. Move the judgement prompts into binary templates. Add the renderers and the override path (specs 130, 131 and 133).
4. Add `next-action` (spec 133).
5. Make `instruct` render from the rule table and `next-action` (unassigned).
6. Change `setup` to install stubs. Add the `doctor` drift checks for stubs and overrides (unassigned).
7. Delete the sequences, rules, prompts and rationale from the skills. Keep only the harness adapters (unassigned).

## Verification (2026-10-18, `main` at `e2325e36`)

The Context claims above were checked against the tree when this ADR was added by spec 129:

- *N−1 versus the gate*: `.claude/skills/ms-bead-cycle/SKILL.md` ("≥ N−1 APPROVE → merge terminal → exit") versus `.claude/skills/ms-panel-tally/SKILL.md` ("a floor, not a sufficient condition") versus `internal/panel/gate.go`'s leg (9.5): any unresolved REQUEST_CHANGES blocks.
- *Two selection rules*: `ms-bead-cycle` picks the lowest-numbered eligible bead; `ms-spec-autopilot` picks the lowest-depth bead.
- *Follow-up bead*: `ms-bead-cycle` says a dissenting verdict's changes may be "filed as a follow-up bead before merge"; the gate's unresolved-verdict leg does not accept that as resolution.
- *Drift*: the installed `.claude/skills/ms-bead-cycle/SKILL.md` was last changed 2026-07-07; `plugins/mindspec/skills/ms-bead-cycle/SKILL.md` on 2026-07-24; the two files differ.
- *History machinery*: `internal/setup/skills.go`'s `previouslyShippedSkills` matches installed skills against byte-exact embedded `historical_skills/` snapshots (not content hashes — the wording above was corrected accordingly).
- *Conflicts stated (spec-129 round-1 panel)*: ADR-0044's Related line says that conduct "is L4 skill-layer doctrine", and its Context says that it "ships as skill/ADR text". Decision 4 forbids rules and rationale in skills. The reconciliation is in "Relation to ADR-0044" above. ADR-0040 §1 holds the default answer "start it at L4" and an open-ended staging area. Decision 5 replaces both. The Amends line names them, and ADR-0040 §1 carries a back-pointer.
- *Not yet in the tree*: `next-action`, `submit`, `bead prompt`, `panel brief`, `.mindspec/prompts/`, and the rule table. The Implementation order above is the plan for them; none is claimed as shipped.

## Amendment (Spec 130)

- **Implementation status (R13.5).** Implementation order step 1 (the rule table) and the step-3 `panel brief` template move to spec 131. The staged-rule marker is `<!-- staged-rule: target=next-action; remove-by=2027-06-30 (provisional) -->`. It names no spec or ADR, because it ships to consumers; the spec-131 rule table may revise it. Refute and abandon procedures are binary verbs; skills invoke them and never edit the panel snapshot or append coverage manifests.

## Amendment (Spec 131)

Implementation order step 1 is implemented by one embedded YAML rule table. Each row has `id`, `scope`, `prohibition`, `replacement` and `enforced_by`. A row can also have `gates` and `target`. `mindspec rules` lists the rows. `mindspec rules show <id>` prints one row. Unknown ids and scopes are refused.

Implementation order step 5 starts with the `## Rules` section in `mindspec instruct`. Every mode includes orchestrator rules. Implement mode also includes implementer rules. The table is the source of this text. `next-action` remains outside this amendment.

The staged-rule marker is `<!-- staged-rule: id=<rule-id>; remove-by=YYYY-MM-DD -->`. The `ms-bead-impl` marker uses `id=sequence.bead-impl-prompt`. Its table row has `target: next-action`. The marker parser retains invalid markers for diagnosis. A marker names the rule id; it does not repeat the planned target.
