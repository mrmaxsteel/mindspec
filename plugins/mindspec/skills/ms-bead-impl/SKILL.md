---
name: ms-bead-impl
description: Stage a pre-built implementation prompt for a claimed bead (Phase A), then dispatch a general-purpose subagent to land one PR-sized commit (Phase B)
---

# Bead Implementation Dispatch

Land one PR-sized implementation for a single claimed bead. You orchestrate; the subagent codes. Two phases: **A** stages a structured prompt, **B** dispatches the subagent with it. Phase A was previously the separate `/ms-bead-prep` skill; it is folded in here so prompt-staging is part of dispatch, not ad-hoc improvisation.

**Why staged prompts matter.** On lola spec-050, pre-staged prompts were the single biggest quality lever on impl-subagent output. A subagent given a structured prompt with explicit files, helpers, ACs, and boundaries produced a passing-first-round implementation; the same subagent given a one-line "implement bead 1" produced rework on round 2. The pattern is documented in `plugins/mindspec/FINDINGS.md` (item 3).

Include the standard guardrails (AGENTS.md § Bead-loop guardrails) in every subagent prompt — do not re-transcribe them here.

## Inputs

- `bead-id` (required) — the `bd` id, e.g. `lola-8gbp.3`.
- `prompt-path` (optional) — pre-staged implementation prompt at `<spec-dir>/reviews/prep/bead<N>_impl_prompt.md`. If supplied, skip Phase A and read it; if absent, run Phase A to stage one.

> `<spec-dir>` is the spec's flat directory `<repo>/.mindspec/specs/<spec-slug>/`; prep prompts and panel artifacts are co-located under `<spec-dir>/reviews/` (spec 106 flat layout).

## Ingress — readiness re-check (EVERY dispatch path)

Run this FIRST, unconditionally, before the Phase-A-skip decision below — on every dispatch path, no exceptions:

```bash
mindspec bead ready-check <bead-id>
```

This is not part of Phase A and it is not optional. It runs whether Phase A stages a fresh prompt, a `prompt-path` was supplied (which skips Phase A entirely), or the documented manual `bd update --claim --status in_progress` / `git worktree add` fallback was used instead of `mindspec next` (step 0's belt-and-braces path). None of those paths may bypass this check — a bead claimed before spec 124 shipped, or a plan/spec hand-edited after the claim, is caught here at dispatch time even if it slipped past `mindspec next`'s own gate.

- **FAIL, no override marker** → STOP. Do not stage a prompt (skip Phase A entirely), do not dispatch a subagent (skip Phase B entirely). Surface the per-signal report and its `recovery:` lines to the user/orchestrator; hand off per `/ms-bead-cycle`'s NOT-READY routing.
- **FAIL, WITH the durable override marker** — `bd show <bead-id>` reveals a `mindspec_readiness_override` metadata key (written by `mindspec next --allow-not-ready`) whose recorded `signals` array names the signal set the operator deliberately bypassed at claim time. The marker's PRESENCE alone is NOT authority to dispatch — it covers only the signals it RECORDS. Compare the fresh ready-check report's currently-failing signal IDs against the marker's recorded `signals` array, and proceed to dispatch ONLY if EVERY currently-failing signal is present in that recorded set — then print a warning naming every overridden signal. If ANY currently-failing signal is missing from the recorded set (a NEW failure introduced after the override was recorded — e.g. the plan or spec hand-edited after the claim), the stale marker does not cover it: STOP and treat this exactly like the FAIL-no-override branch above (NOT READY routing), naming each uncovered signal. A deliberately force-claimed bead still gets a coherent path to implementation — but only for the failures the operator actually overrode.
- **PASS** → proceed normally (Phase A or the supplied `prompt-path`, then Phase B).
- **Re-dispatch** — if `bd show <bead-id>` reveals a `mindspec_readiness_attempt` metadata key (written by `mindspec bead clarify`), read the record and inject its entries, keyed by ordinal, into the staged prompt's `## Phase 0` / `## Readiness Clarification` sections below. The injection iterates the record's `report` array: EVERY originally-cited reason is rendered, never a subset. For EACH report ordinal you MUST render BOTH the original cited NOT-READY reason (verbatim from the record's `report` array — the `{ordinal, signal, reason}` entry) AND its paired clarification (from the record's `clarifications` array — the `{ordinal, reason, answer, span}` entry); a report ordinal with NO paired clarification MUST still be rendered, with `clarification: (none recorded)`, so the fresh Phase 0 sees the unaddressed reason and re-reports NOT READY for it — the anti-browbeat rule cannot be evaded by clarifying only some reasons. Pairing the original reason with its answer is load-bearing: the re-dispatched Phase 0 cannot apply the anti-browbeat rule (judging whether the clarification genuinely RESOLVES the reason) if it never sees the reason it is supposed to resolve. If the stored record is malformed (an empty `report` array, a clarification citing an ordinal absent from `report`, or a pairing that cannot be established), STOP — fail closed to the NOT-READY routing rather than dispatching with a partial injection.

## Phase A — stage the prompt

Skip this phase if `prompt-path` was supplied. Otherwise compose a structured prompt and stage it at `<spec-dir>/reviews/prep/bead<N>_impl_prompt.md`.

1. **Resolve spec context.**
   ```bash
   bd show <bead-id>
   ```
   Capture: parent epic id, plan path (`.mindspec/specs/<spec-slug>/plan.md`), spec path, declared Rs/ACs, declared `Depends on:` beads. Status must be `open` or `in_progress`.

2. **Extract the plan section.** Grep the bead's `## Bead N — ...` header out of `plan.md` and read through to the next `## Bead` header (or EOF). Capture:
   - Files in scope (look for "Files:" or "Touches:" lines).
   - Tests to add (from the plan's AC list — usually `AC<n>` references).
   - Public API the bead introduces (function names, class names, exported constants).

3. **Read the spec section (if the bead claims specific Rs/ACs).** For each Rn / ACn the bead claims, grep `spec.md` for the matching definition and quote the success criteria verbatim.

4. **Walk the dependency chain.** For each `Depends on:` bead the plan declares:
   - Find the merged helper module(s) — usually under the same package as the bead's primary file (use `git log --oneline bead/<dep-id>` or read the merged commit to locate paths).
   - Read each helper file and extract its public API: function signatures (with types), exported class names + public method signatures, key module-level constants.
   - Note import paths the new bead should use to consume these (no reimplementation).

5. **Compose the prompt.** Write to `<spec-dir>/reviews/prep/bead<N>_impl_prompt.md` with this skeleton:

   ```markdown
   # Bead <N> implementation prompt — <spec-slug>

   ## Identity
   - Bead id: <bead-id>
   - Branch: bead/<bead-id>
   - Worktree: <abs path>
   - Spec branch: spec/<spec-slug>

   ## Read first
   - `.mindspec/specs/<spec-slug>/spec.md` §R<n> + §AC<n>
   - `.mindspec/specs/<spec-slug>/plan.md` §Bead <N>

   ## Phase 0 — readiness review (before any edit)

   Before touching any file, evaluate these five signals against the spec/plan material quoted above (and the `## Readiness Clarification` entries below, if this is a re-dispatch):

   - **SR-1** — the plan steps and ACs are implementable without inventing unstated behavior.
   - **SR-2** — each claimed AC is decidable: a concrete pass/fail check exists or is directly constructible.
   - **SR-3** — every helper this prompt says to reuse actually exists at the stated import path with the stated shape.
   - **SR-4** — the bead does not contradict the spec or a sibling bead's landed work.
   - **SR-5** — no ambiguity remains that would force you to choose between materially different implementations.

   If all five PASS, print exactly one line — `Phase 0: READY` — and proceed to implement. Make zero commits until Phase 0 passes.

   If ANY signal FAILS, make **zero commits** and return a report whose first line is EXACTLY `NOT READY: <bead-id>`, followed by reasons numbered by **ordinal** (1, 2, … — each unique within the report), each tagged with its signal ID (one of `SR-1`, `SR-2`, `SR-3`, `SR-4`, `SR-5`) and quoting the offending or missing span **verbatim**, plus the concrete unblocking question.

   **Clarification-handling rule (anti-browbeat, R8c)**: on a re-dispatch, each `## Readiness Clarification` entry below pairs, per ordinal, the ORIGINAL cited NOT-READY reason with the clarification that answers it. Judge whether the clarification actually **resolves** its paired original reason against the cited source span — a bare "it's ready" never suffices, and an entry must never be read as license to invent new normative behavior. Any cited reason ordinal that lacks a mapped, span-grounded clarification, OR whose clarification does not genuinely resolve the paired original reason, MUST be **re-reported NOT READY** — do not let a plausible-sounding but ungrounded clarification talk you into READY.

   ## Readiness Clarification

   (Populated by the dispatch ingress on re-dispatch, from the bead's `mindspec_readiness_attempt` metadata — one PAIRED entry per ordinal in the record's `report` array (ALL originally-cited reasons, never a subset), rendering the original reason ALONGSIDE its clarification:
   `- Ordinal <n> [<signal>]: reason: <original verbatim reason> — clarification: <answer> (span: <span>)`
   and, for a report ordinal carrying no recorded clarification:
   `- Ordinal <n> [<signal>]: reason: <original verbatim reason> — clarification: (none recorded)`.
   Empty on a first dispatch.)

   ## Files in scope
   - `<path>` — <one-line purpose>
   - ...

   ## Shared helpers to REUSE (do not reimplement)
   - `from <module> import <name>` — <signature> — landed in bead <M>
   - ...

   ## Tests to add
   - `<test path>::<test name>` — covers AC<n>
   - ...

   ## Guardrails
   Include the standard guardrails (AGENTS.md § Bead-loop guardrails) — the
   subagent prompt fences (no `mindspec complete`, no `git push`, no scope
   creep, no reimplementing helpers, exactly ONE commit ending
   `Deviations: <list or "none">`, tests must PASS, report SHA + counts).

   ## Expected commit shape
   ```
   impl(<spec-slug>, bead <N>): <2-5 word summary>

   - <change 1>
   - <change 2>

   Deviations: <list any prompt items you couldn't address literally, or "none">
   ```

   ## Report back with
   - Commit SHA on `bead/<bead-id>`.
   - Test counts: passed / failed / skipped (run the bead's test scope, not the full repo suite).
   - Flagged deviations from this prompt (one line each + reason).
   ```

6. **Verify file landed.**
   ```bash
   ls -l <spec-dir>/reviews/prep/bead<N>_impl_prompt.md
   ```

## Phase B — dispatch the subagent

1. **Confirm the bead worktree.** The cycle's Step 0 (`/ms-bead-cycle`) already claimed the bead and created `<spec-worktree>/.worktrees/worktree-<bead-id>/` on branch `bead/<bead-id>`. Verify it exists; if it is missing, re-run `mindspec next --spec <slug>` (auto-recovers the worktree for an already-claimed bead).

2. **Load the prompt.** Read the staged `<spec-dir>/reviews/prep/bead<N>_impl_prompt.md` (from `prompt-path` or Phase A).

3. **Dispatch.** Spawn a `general-purpose` `Agent` with the prompt as `prompt`. Run in background if you have other parallel work; otherwise foreground.
   - The subagent makes exactly ONE commit on the bead branch ending `Deviations: <list or "none">`.
   - It does NOT merge, push, or complete the bead (guardrails fences).

4. **On return, capture:**
   - Commit SHA → record in conversation.
   - Test summary (pass/fail/skip) → record.
   - Any flagged deviations → carry into the round-1 BRIEF (these become "fix-author deviation (A/B/...)" sections).

## Anti-patterns

- Don't paraphrase the spec/plan in the prompt — quote verbatim so the subagent reads canonical text, not your summary.
- Don't list helpers without import paths. "There's a helper in bead 2" is worse than no mention; the subagent will reimplement it.
- Don't omit the guardrails pointer. Subagents scope-creep without explicit fences.
- Don't ask the subagent to "iterate" or "fix issues as they come up" — that's the panel's job, not the impl agent's.
- Don't bake test results into the prompt ("the tests pass" before they're written). Specify expected test names; let the subagent run them.

## Then

Hand off to `/ms-panel-run` step 0 to set up the round-1 review panel.
