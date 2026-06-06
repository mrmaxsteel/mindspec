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

     **Prompt MUST include this terminal instruction verbatim:**

     ```
     Your final step before terminating is to WRITE the verdict JSON to `<EXPECTED_JSON>` using a `Write` tool call (or a single-file patch). Do NOT terminate after just composing the verdict in your output — the orchestrator reads the file, not the log. If you cannot write to that exact path (sandbox or permissions), STOP and write a short error message to the same path describing why.
     ```

     Codex without this instruction reliably "finishes thinking" without writing — particularly when its working directory is outside the panel JSON path's sandbox.
   - Launch (always `cd <repo>` first — see "Working directory matters" below):
     ```bash
     cd <repo>
     codex exec --skip-git-repo-check < /tmp/codex_<panel-slug>_r<N>.md > /tmp/codex_<panel-slug>_r<N>.out 2>&1 &
     ```
     Use `run_in_background: true`.

2. **Launch Claude `Agent`s second** (faster; start after Codex is already running).

   For each claude slot R1, R2, R3:
   - Compose a prompt with the BRIEF path, slot id, lens, and required JSON output path.
   - Spawn a `general-purpose` `Agent` with `run_in_background: true`.

3. **Wait for completion notifications.** Each finishes asynchronously; you receive `<task-notification>` messages. Don't poll outputs — wait for the notification.

4. **Detect codex failures.** See "Codex failure detection (deterministic)" below.

5. **Verify all six JSON files exist** at `<repo>/review/<panel-slug>/<slot>-round-<N>.json`.

## Codex failure detection (deterministic)

A healthy codex run writes its JSON verdict to the named output path. Failure modes: codex hit its usage limit, codex tried to write but the sandbox rejected the path, or codex "finished thinking" without ever attempting a write. Detect each deterministically — the original v1 check used `"Output JSON to"` in the log as a healthy-ack signal, but that string is just codex echoing the prompt back; it appears even when the file never lands. Use three layers instead, in order:

```bash
EXPECTED_JSON="<repo>/review/<panel-slug>/codex-<slot>-round-<N>.json"
OUT="/tmp/codex_<panel-slug>_r<slot>.out"

# Layer 1 (primary): did the file land?
if [ -f "$EXPECTED_JSON" ]; then
    : # healthy, nothing to do
elif grep -q "ERROR: You've hit your usage limit" "$OUT"; then
    # codex hit usage limit — sub claude
    launch_claude_sub_for_slot <slot>
elif grep -qE "apply patch|patch: completed|\+\+\+ " "$OUT"; then
    # Layer 2 (diagnostic): codex *tried* to write but file is absent → sandbox path issue
    echo "WARN: codex attempted file-write but sandbox rejected — check workdir vs $EXPECTED_JSON"
    # Layer 3 (recovery): extract verdict from log
    extract_verdict_from_log "$OUT" > "$EXPECTED_JSON" \
      || launch_claude_sub_for_slot <slot>
else
    # codex finished thinking without ever attempting a write
    # Layer 3 (recovery): try log extraction first; sub if nothing extractable
    extract_verdict_from_log "$OUT" > "$EXPECTED_JSON" \
      || launch_claude_sub_for_slot <slot>
fi
```

`extract_verdict_from_log` is the helper script shipped alongside this skill:

```bash
extract_verdict_from_log() {
    "$MINDSPEC_PLUGIN_DIR/scripts/codex_verdict_extract.sh" "$1"
}
```

(Where `$MINDSPEC_PLUGIN_DIR` is `plugins/mindspec/` resolved from wherever the plugin is embedded.) The script scans the codex `.out` log for the largest contiguous JSON object containing both `"verdict"` and `"confidence"` keys and emits it on stdout; non-zero exit means nothing extractable.

`launch_claude_sub_for_slot` spawns a `general-purpose` `Agent` with the same BRIEF + slot id + lens prompt the codex slot used, but writes its verdict JSON with `reviewer_id: "R<slot> claude-sub"` so the tally can see the family-substitution explicitly. Keep the slot name (R4 stays R4) so verdict comparability is preserved across rounds.

When deciding whether to retry codex once before substituting: don't. Empirically on lola spec-050, every codex slot that tripped the usage-limit detector stayed tripped on retry — the user's account quota refreshes hourly, not per-process. Skip straight to claude-sub.

## Working directory matters

Codex's default sandbox is `workspace-write [workdir, /tmp, $TMPDIR, /Users/Max/.codex/memories]`. The `workdir` is whatever directory you `cd` into before `codex exec`. If the panel JSON path (`<repo>/review/<panel-slug>/...`) is outside that workdir, codex's write silently fails.

**Launch convention**: always `cd <repo>` first so `<repo>/review/...` is inside the sandbox:

```bash
cd <repo>
codex exec --skip-git-repo-check < /tmp/codex_<panel-slug>_r<slot>.md > /tmp/codex_<panel-slug>_r<slot>.out 2>&1 &
```

If the panel runs in a worktree under a parent repo, `cd` to the worktree, not the parent — the worktree's `review/` subtree is where the verdict needs to land.

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
