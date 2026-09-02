# Onboarding an Existing Codebase

MindSpec's gates assume the repo carries an architecture record: domains, an ownership map, ADRs to cite, docs to keep in sync. A greenfield project accumulates those from day one. An existing codebase has the architecture — it just isn't written down where a gate can check it.

Onboarding today is **agent-assisted, not automated**: MindSpec supplies the verbs, the prompts, and the validators; your coding agent does the writing; you review what it writes. There is no one-shot inference command — the record gets built the same way everything else in MindSpec gets built, with the agent authoring and a human accepting.

## The onboarding sequence

```bash
cd existing-project
mindspec init            # additive scaffold — existing files are never overwritten
mindspec setup claude    # or: codex, copilot
mindspec migrate         # emits the agent prompt to reorganize pre-existing
                         # markdown into the canonical MindSpec layout
mindspec doctor          # verify the result — re-run after every step below
```

`mindspec init` is additive by design, so running it inside a mature repo only fills in what's missing. `mindspec migrate` (and `mindspec migrate layout` for repos on an older MindSpec layout) handles docs that already exist but live in the wrong places.

Then, with your agent, build the record itself:

- **Domains** — `mindspec domain add <name>` scaffolds each bounded context with template docs and a `context-map.md` entry. Have the agent propose the decomposition from the code's actual structure; you accept or correct it.
- **Ownership** — `mindspec ownership populate` prints the agent prompt for filling each domain's `OWNERSHIP.yaml` with the path globs mapping it to the code it owns.
- **As-built ADRs** — `mindspec adr create` records the architecture decisions already embodied in the code. Writing these down is the highest-leverage step: plans must cite ADRs covering every impacted domain, so the sooner the real decisions are on record, the sooner the divergence gate is protecting them.
- **Doc-sync sources** — `mindspec source populate` prints the agent prompt for populating `source_globs` in `.mindspec/config.yaml`; `mindspec doctor --fix` scaffolds the commented config block.

Because a human reviews each artifact before it lands, the agent's guesses about your architecture never silently become the record.

## Expect doc-sync friction at first — and use it

An old codebase trips the doc-sync gate constantly at first: most code predates any doc that could have drifted with it. The escape hatches exist for exactly this — `--allow-doc-skew` on the affected verbs — and every use is journaled to the local friction log.

That journal is the point. `mindspec report` consolidates it and shows exactly where the skew concentrates, and that ranking is your onboarding roadmap:

- Overrides concentrating in one domain → that domain's docs and ADRs are what to onboard next.
- Fixes recurring in the same area → that area wants a real spec and real ADRs.
- Skew warnings drying up in a domain → its record has caught up.

The gates that never soften, in any repo: tests must pass, the panel gate stands, and ADR divergence at merge-to-main still blocks. There's no big-bang migration day — full enforcement is just the state you notice you've reached.

## Planned: a fix lane for repos you won't onboard

For repos you don't want to onboard at all — a dependency you're patching, an OSS project you're contributing to, a legacy service that gets one fix a quarter — a governed fix lane is planned as `/ms-fix-cycle` *(planned — claim `fix-cycle`, roadmap Core 6)*: discover the defect on a `fix/` branch, reproduce it in a sandbox before patching, land a one-commit minimal patch, panel-review it, and open a PR with a CI watch — with no `.mindspec/` scaffolding required and the merge click left to a human.

## Related

- [Review panels guide](review-panels.md) — the verifier every path shares
- [Autonomy guide](autonomy.md) — brownfield repos can climb the same ladder, gates permitting
- `mindspec migrate` / `mindspec migrate layout` — for repos with an older MindSpec layout or pre-existing docs to reorganize
