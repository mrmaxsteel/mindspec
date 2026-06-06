---
name: ms-panel-run
description: Launch a 6-reviewer mindspec panel — 3 Claude Agents + 3 Codex CLI sessions in parallel — and wait for verdicts
---

# Run a 6-Reviewer Panel

Fan out three `Agent` calls (Claude) and three `codex exec` background processes (Codex) in parallel. Each writes a JSON verdict to disk. Wait for all six, then hand off to `/ms-panel-tally`.

## Inputs

- `panel-slug` (required) — e.g. `spec-050-bead2`.
- `round` (default `1`) — round number; reviewers write to `<slot>-round-<N>.json`.

Prerequisite: `/ms-panel-create` has already created `<repo>/review/<panel-slug>/BRIEF.md`.

## Steps

1. **Launch Codex first** (they run 4-10 min vs Claude 1-3 min; start the slow ones first).

   For each codex slot R4, R5, R6:
   - Write `/tmp/codex_<panel-slug>_r<N>.md` containing the slot-specific prompt. Each prompt should:
     - Tell the reviewer to read `<repo>/review/<panel-slug>/BRIEF.md`.
     - Name the slot id (`R4 codex`, etc.) and the lens.
     - For round >= 2, point at the previous round's JSON (`<repo>/review/<panel-slug>/codex-<slot>-round-<N-1>.json`) and list the `concrete_changes_required` the slot raised.
     - End: "Output JSON to `<repo>/review/<panel-slug>/codex-<slot>-round-<N>.json`. ≤200 words."
   - Launch:
     ```bash
     nohup bash -c 'codex exec --skip-git-repo-check < /tmp/codex_<panel-slug>_r<N>.md' > /tmp/codex_<panel-slug>_r<N>.out 2>&1 &
     ```
     Use `run_in_background: true`.

2. **Launch Claude `Agent`s second** (faster; start after Codex is already running).

   For each claude slot R1, R2, R3:
   - Compose a prompt with the BRIEF path, slot id, lens, and required JSON output path.
   - Spawn a `general-purpose` `Agent` with `run_in_background: true`.

3. **Wait for completion notifications.** Each finishes asynchronously; you receive `<task-notification>` messages. Don't poll outputs — wait for the notification.

4. **Detect codex failures.** Codex can hit usage limits or get killed mid-stream. If a codex JSON is missing after the process completes (exit 0 but no JSON written), check the `.out` file:
   - Empty output / just the prompt echoed → usage limit. Retry once.
   - Retry also fails → substitute a Claude `Agent` in the same slot with `reviewer_id: "R<N> claude-sub"`.

5. **Verify all six JSON files exist** at `<repo>/review/<panel-slug>/<slot>-round-<N>.json`.

## Slot lens defaults

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | Author-of-record — diff matches plan §<bead>? |
| R2 | Claude | Codebase-pin — named files and tests actually exist + green? |
| R3 | Claude | Prompt-shape / contract stability — downstream surface stable? |
| R4 | Codex  | Empirical prober — run validators by hand |
| R5 | Codex  | Schema / type correctness — Pydantic, SQLAlchemy, unions |
| R6 | Codex  | Next-bead integration — will the next bead consume this cleanly? |

For round >= 2, each slot inherits its previous-round lens and is told to evaluate only its own `concrete_changes_required` items as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE, plus flagged fix-author deviations from the BRIEF.

## Anti-patterns

- Don't read `.out` files via `Read` while a codex session is still active — they grow and can overflow context. Wait for the completion notification, then grep targeted strings.
- Don't run all six reviewers as foreground tool calls — that serialises the panel and wastes 5-10 minutes.
- Don't reuse a codex `/tmp/codex_*.md` file across rounds — write a fresh one per round so the round number is unambiguous in the prompt.
- Don't ask Claude reviewers to also do the codex empirical-probe lens — duplication is worse than coverage gaps; trust the family split.

## Then

Hand off to `/ms-panel-tally`.
