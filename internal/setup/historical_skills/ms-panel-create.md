---
name: ms-panel-create
description: Initialise a mindspec review panel — create the directory and BRIEF.md template for 6 reviewers
---

# Review Panel Setup

Create the panel directory and BRIEF.md that the 6 reviewers will read. The BRIEF is the single artefact that gives each reviewer enough context to verdict independently.

## Inputs

- `panel-slug` (required) — e.g. `spec-050-bead2` (round 1) or `spec-050-bead2-r2` (round 2).
- `target` (required) — one of `bead <bead-id>` or `pr <pr-number>` or `commit <sha>`.
- `round` (default `1`) — the panel round.

## Steps

1. **Create the panel directory.**
   ```bash
   mkdir -p <repo>/review/<panel-slug>
   ```

2. **Compose the BRIEF.md.** Required sections:

   ```markdown
   # <panel-slug> — Round <N> Review Panel

   **Worktree**: <abs-path-to-bead-worktree>
   **Branch**: bead/<bead-id> | pr/<n>
   **Commit under review**: <sha> — <commit subject>
   **Prior round verdict** (round >= 2 only): X APPROVE, Y REQUEST_CHANGES; consolidated asks below.

   ## What the work does

   <1-paragraph plain-English summary. Don't paste the plan; summarise it.>

   ## Round-<N-1> concrete_changes_required (consolidated)  [round >= 2]

   1. ...
   2. ...

   ## Files in scope (final state at <sha>)

   - `path/to/file.py`
   - ...

   ## Shared modules reused (unchanged)

   - `app/entities/identify.py` — `IdentifyResultCase1/2/3`, `identify_via_llm`, ...

   ## Fix-author deviations (assess these explicitly)  [round >= 2]

   A. <deviation 1 — why the author diverged from the brief, and what they did instead>
   B. ...

   ## Your job

   Verify the round-<N-1> concrete_changes_required are addressed (round >= 2), or evaluate the work cold (round 1). Each ask → ADDRESSED / PARTIAL / MISSED / NEW_ISSUE.

   Verdict: APPROVE / REQUEST_CHANGES / REJECT.

   Output JSON to `<repo>/review/<panel-slug>/<your-slot>-round-<N>.json` with keys:
   `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings` (per round-<N-1> item).
   ```

3. **Pre-stage the codex prompts.** Codex CLI sessions cannot accept the BRIEF as a tool input the way Claude `Agent` calls can. Write `/tmp/codex_<panel-slug>_r{4,5,6}.md` files, each opening with:
   > You are R{N} codex on the <panel-slug> round-<round> verification panel. Read `<abs-path>/BRIEF.md`.

   followed by the slot-specific lens, the previous-round JSON path (round >= 2), and the concrete_changes_required items they personally raised in the previous round.

## Lens assignment (recommended)

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | Author-of-record — does this match what the plan asked for? |
| R2 | Claude | Codebase-pin — do all the named tests + files actually exist and pass? |
| R3 | Claude | Prompt-shape / contract stability — is the surface area for downstream beads stable? |
| R4 | Codex  | Empirical prober — run the validators, exercise edge cases by hand |
| R5 | Codex  | Schema/type correctness — Pydantic, SQLAlchemy, discriminated unions |
| R6 | Codex  | Next-bead integration — will the next bead in the chain consume this cleanly? |

Mix to taste. The point is six distinct lenses, not six clones.

## Then

Hand off to `/ms-panel-run`.
