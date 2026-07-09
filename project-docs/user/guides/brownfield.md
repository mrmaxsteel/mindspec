# Onboarding an Existing Codebase

MindSpec's gates assume the repo carries an architecture record: domains, an ownership map, ADRs to cite, docs to keep in sync. A greenfield project accumulates those from day one. An existing codebase has the architecture — it just isn't written down where a gate can check it.

There are three entry paths, and they compose: you can start on the lightest one today and let the repo earn its way to full onboarding.

| Path | What it needs | What you get |
|:-----|:--------------|:-------------|
| **Full onboarding** (`mindspec onboard --infer`) | willingness to review inferred docs | the complete lifecycle, gates at full strength |
| **Brownfield mode** (`mode: brownfield`) | an onboarded skeleton | full lifecycle; doc-sync warns instead of blocks while docs catch up |
| **The fix lane** (`/ms-fix-cycle`) | nothing — no `.mindspec/` at all | governed bug/security fixes with panel review and a human merge |

## Full onboarding: `mindspec onboard --infer`

Reverse-onboarding: instead of you writing the architecture record before the agent can work, the agent generates its first draft *as* work — and the record is marked as inferred until you promote it.

```bash
cd existing-project
mindspec onboard --infer
mindspec setup claude        # or codex, copilot
mindspec doctor              # verify the result
```

The inference pass authors, with validation after each step:

- **`context-map.md` and a domain decomposition** — bounded contexts inferred from the code's actual structure, each scaffolded under `.mindspec/domains/`
- **`OWNERSHIP.yaml` manifests** — the path globs mapping each domain to the code it owns
- **As-built ADRs** — the architecture decisions already embodied in the code, written down as ADRs with `Status: Proposed` and a `Provenance: inferred` marker
- **`source_globs`** — the doc-sync source classification in `config.yaml`

### Why "Proposed" matters

Inferred ADRs are deliberately second-class, and the lifecycle is built around that:

- At **plan approval** and **bead completion**, a Proposed ADR counts as *provisional* coverage — work can proceed, with an advisory warning.
- At **implementation approval** (the spec → main gate), Proposed-only coverage is a hard error.

That hard error is the **promotion ceremony**: before inferred architecture can gate a merge to main, a human has to read the ADR and either accept it (it becomes the record) or correct it. The agent's guesses about your architecture never silently become authoritative. The `Provenance: inferred` marker survives promotion, so you can always tell which decisions were excavated rather than made.

## Brownfield mode: softening doc-sync while docs catch up

An old codebase fails doc-sync constantly at first — most code predates any doc that could have drifted with it. The `mode: brownfield` profile in `.mindspec/config.yaml` turns the doc-sync gate's errors into warnings **while still recording every skew to the friction journal**.

The distinction matters: brownfield mode doesn't disable the check, it defers the enforcement. `mindspec report` keeps score of exactly where the skew concentrates — and that ranking is your onboarding roadmap. When a domain stops accumulating skew reports, flip it back to full enforcement.

The gates that never soften, in any mode: tests must pass, the panel gate stands, and ADR divergence at merge-to-main still blocks.

## The fix lane: `/ms-fix-cycle` on repos with no MindSpec at all

For repos you don't want to onboard — a dependency you're patching, an OSS project you're contributing to, a legacy service that gets one fix a quarter — the fix lane runs a governed fix without any `.mindspec/` scaffolding:

1. **Discover** — identify the bug or vulnerability, on a `fix/<description>` branch (never main).
2. **Reproduce before patching** — write `repro.sh` demonstrating the defect in a sandbox. No reproduction, no patch: validation comes before the fix, not after.
3. **Patch** — one commit, minimal diff, no new dependencies, tests pass, plus `verify.sh` proving the defect is gone.
4. **Panel** — a review panel judges the patch (the panel machinery doesn't require a bead or a spec). The empirical-prober lens *executes* `repro.sh` and `verify.sh` — the evidence is run, not read.
5. **PR + CI watch** — the lane opens the PR and watches checks. **The merge click is the one mandatory human gate.**

The panel is registered before any patching starts, so the fix lane can't fail open — there is no path where a patch reaches a PR without its verifier already on record.

### Severity routing

Not every fix deserves the same ceremony, so the lane routes by blast radius:

- **T1 — localized** (single function/file, clear repro): straight through the fix lane, tracked as a lightweight `hotfix` bead so the work leaves a trace without a spec.
- **T2 — cross-cutting** (touches multiple domains or changes behavior): a mini spec citing whatever ADRs exist — inferred ones included. If the fix keeps wanting to be bigger, that's a spec telling you it exists.
- **T0 — critical / actively exploited**: the fix lane at maximum urgency, with a security-lens panel (exploitability probing, patch completeness, regression risk) empowered to hard-block, and redaction-first handling of the details.

**Scope boundary:** the fix lane is for repos you own or are explicitly engaged to work on. It is a repair tool, not a scanning tool.

## Progressive onboarding: friction as the compass

The three paths are designed to converge. Every fix, override, and doc-skew warning feeds the friction journal, and `mindspec report` aggregates it:

- Overrides concentrating in one domain → that domain's docs and ADRs are what to onboard next.
- T2 fixes recurring in the same area → that area wants a real spec and real ADRs.
- Doc-skew warnings drying up → flip that domain out of brownfield leniency.

Each fix leaves the repo slightly more onboarded than it found it. There's no big-bang migration day — full enforcement is just the state you notice you've reached.

## Related

- [Review panels guide](review-panels.md) — the verifier all three paths share
- [Autonomy guide](autonomy.md) — brownfield repos can climb the same ladder, gates permitting
- `mindspec migrate` / `mindspec migrate layout` — for repos with an older MindSpec layout or pre-existing docs to reorganize
