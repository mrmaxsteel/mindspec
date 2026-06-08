---
name: ms-panel-tally
description: Read all 6 verdict JSONs from a mindspec panel round and consolidate concrete_changes_required for the fix subagent
---

# Tally Panel Verdicts

Read `<repo>/review/<panel-slug>/*-round-<N>.json`, summarise verdicts, consolidate the convergent `concrete_changes_required` list, and decide whether the panel passes.

## Inputs

- `panel-slug` (required).
- `round` (required) — the round just completed.

## Steps

1. **Load all six verdicts.**
   ```bash
   cd <repo>/review/<panel-slug>
   for f in *-round-<N>.json; do
     python3 -c "import json; d=json.load(open('$f')); print(f, d['verdict'], d.get('confidence'))"
   done
   ```

2. **Tabulate.** Report:
   - Per-slot: `verdict`, `confidence`, one-line rationale snippet.
   - Aggregate: APPROVE count / REQUEST_CHANGES count / REJECT count.
   - Family split: of the APPROVEs, how many Claude vs how many Codex? Family asymmetry matters.

3. **Decision:**

   | Condition | Action |
   |:----------|:-------|
   | Any verdict's `concrete_changes_required` references a missing measurement artifact, drift report, cost projection, or regression baseline | **HARD block**. Halt the cycle; orchestrator must commission the measurement run before merge can proceed. Not satisfiable by PR-body fixes. |
   | Any REJECT | Halt the cycle; ask the user. REJECTs usually mean the brief or plan needs work. |
   | ≥5/6 APPROVE AND no HARD-block flags | Panel passes. Hand off to `/ms-bead-merge`. |
   | 3-4 APPROVE with mixed families | Fix-up needed. Hand off to `/ms-bead-fix`. |
   | ≤2 APPROVE | Significant rework. Hand off to `/ms-bead-fix`, but flag to the user. |

   The HARD-block check exists because round-2 fix-up subagents can flip a REQUEST_CHANGES to APPROVE by editing the PR body to name the artifact's intended landing path — without producing the artifact. This is a real failure mode (lola-f4a8: spec-050 shipped without the AC8c cost projection artifact; first prod Mon cron burned $417 because the missing measurement would have caught the no-cap prefilter blow-up). Body-precision fixes are necessary but not sufficient for evidence-bearing gates.

4. **Consolidate `concrete_changes_required`.** This is the input to `/ms-bead-fix`. Process:

   a. Collect every `concrete_changes_required` item across the four REQUEST_CHANGES verdicts.

   b. Dedupe semantically — multiple reviewers often flag the same defect differently ("enforce Case-3 invariants" / "reject malformed Case-3 payloads" / "reuse Bead-1 case models"). Group by defect, list distinct asks under each group.

   c. Rank by criticality:
      - **Code defects** (functional bugs, broken contracts) — must fix
      - **Test coverage gaps** — must fix
      - **Refactors / sharing** (e.g. "reuse the shared model") — fix if it changes behaviour, defer if pure style
      - **Documentation / prose** — fix if user-facing, defer otherwise

   d. Write the consolidated list to `<repo>/review/<panel-slug>/consolidated-round-<N>.md` for the fix subagent to read.

5. **Report to the orchestrator** (`/ms-bead-cycle`):
   ```
   Panel <slug> round <N>: <A> APPROVE, <R> REQUEST_CHANGES, <X> REJECT
   Family split (APPROVEs): <claude>/3 claude, <codex>/3 codex
   Decision: <merge | fix | halt>
   Consolidated changes: <path-to-md>
   ```

## Anti-patterns

- Don't auto-merge on 4/6 APPROVE. The threshold is 5/6, and you should still note family asymmetry.
- Don't pass raw verdict JSONs to the fix subagent — dedupe first. Six verdicts × ~3 items each = ~18 lines of duplicated asks otherwise.
- Don't ignore `confidence`. A 0.96 REQUEST_CHANGES from one slot should outweigh a 0.70 APPROVE from another. Note this in the report.
- Don't drop a REQUEST_CHANGES because "only one reviewer flagged it". The whole point of mixed families is that a single empirically-grounded objection can be load-bearing — verify the claim before discarding.

## Then

Decision-dependent:
- `merge` → `/ms-bead-merge`
- `fix` → `/ms-bead-fix`
- `halt` → return to user
