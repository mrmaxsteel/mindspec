---
name: ms-panel-run
description: Set up a mindspec review panel (step 0 — dir + BRIEF + panel.json) then launch 6 reviewers (3 Claude Agents + 3 Codex CLI sessions) in parallel and wait for verdicts
---

# Run a 6-Reviewer Panel

Step 0 creates the panel directory, the BRIEF.md the reviewers read, and the `panel.json` state file the in-binary `mindspec complete` gate and `mindspec instruct --panel-state` consume. Then fan out three `Agent` calls (Claude) and three `codex exec` background processes (Codex) in parallel. Each writes a JSON verdict to disk. Wait for all six, then hand off to `/ms-panel-tally`.

Step 0 was previously the separate `/ms-panel-create` skill; it is folded in here so the panel directory, BRIEF, and `panel.json` are always created together — the gate's source of truth (`panel.json`) cannot be forgotten.

## Inputs

- `panel-slug` (required) — e.g. `spec-050-bead2` (round 1) or `spec-050-bead2-r2` (round 2).
- `target` (required) — one of `bead <bead-id>`, `pr <pr-number>`, or `commit <sha>`. For `/ms-spec-final-review`, `target` is the spec branch (`bead_id` null).
- `round` (default `1`) — the panel round; reviewers write to `<slot>-round-<N>.json`.
- `expected-reviewers` (default `6`) — the default panel size.

> **`<spec-dir>` / co-located reviews (spec 106 flat layout).** Panels are
> co-located under the spec they review: `<spec-dir>` is `<repo>/.mindspec/specs/<spec-slug>/`,
> so the panel dir is `<spec-dir>/reviews/<panel-slug>/` (a sibling of the spec's
> `recording/`). The in-binary `mindspec complete` gate scans
> `<spec-dir>/reviews/*/panel.json` on a flat tree — the old repo-root `review`
> directory no longer drives the gate. `<spec-slug>` is the full spec directory
> name (e.g. `106-layout-flatten`); for a `pr`/`commit` target that belongs to a
> spec, use that spec's dir.
>
> **Ad-hoc reviews (no owning spec).** A review not tied to any spec's lifecycle —
> a repo-wide review, a pre-release review, a design/proposal review, or a
> standalone bug-fix — writes its panel dir to **`<repo>/.mindspec/reviews/<panel-slug>/`**
> (a top-level sibling of `.mindspec/specs/`), never under a spec dir and never to
> the old repo-root `review/`. For an ad-hoc panel, substitute
> `.mindspec/reviews/<panel-slug>` for `<spec-dir>/reviews/<panel-slug>` everywhere
> in the steps below. These panels are stored artifacts only — they are NOT scanned
> by the `mindspec complete` gate (which is spec-lifecycle-scoped), so no `panel.json`
> of theirs ever gates a merge.

## Step 0 — register the panel

1. **Register (or re-panel) the panel.**
   ```bash
   mindspec panel create <panel-slug> --spec <spec-id> --target <ref> [--bead <bead-id>] [--round N]
   ```
   One call creates `<spec-dir>/reviews/<panel-slug>/`, writes `panel.json` — the single source of truth the pre-complete gate (ADR-0037) and `mindspec instruct --panel-state` read, with `expected_reviewers`/`approve_threshold` stamped from the configured panel defaults and `reviewed_head_sha` captured from `--target`'s live commit AT WRITE TIME — and writes (first `create`) or rewrites (re-panel) `BRIEF.md`'s machine-managed header, all in one atomic operation.

   On a re-panel (`--round N+1`), `round` and `reviewed_head_sha` co-bump in the SAME write by construction — the two fields can never drift apart, which is exactly the invariant that closes the stale-verdict bypass (lola-f4a8 class). Prior round verdict files (`<slot>-round-<K>.json`, K < N+1) and the skill-authored BRIEF body below the header are left untouched.

   Optional fields the abandon procedure (`/ms-panel-tally` § halt-recover) sets by hand, directly in `panel.json`: `"abandoned": true` plus `"abandon_reason": "<who/why>"` (required when abandoned) — a plain file edit, not something `panel create` writes.

2. **Fill in the BRIEF.md stub** `create` wrote. The machine-managed header (delimited by `<!-- mindspec:panel-header -->` … `<!-- /mindspec:panel-header -->`) already carries the slug, round, branch, reviewed commit, and the "## Your job" verdict-JSON contract (`verdict`, top-level `hard_block`, `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`, `findings`) — never edit inside it; it is machine-managed and gets rewritten wholesale on every `create`. Below it, `create` left a stub with four headings for the skill to fill:

   ```markdown
   ## Summary

   <1-paragraph plain-English summary of what this panel reviews. Don't paste the plan; summarise it.>

   ## Files in Scope

   - `path/to/file.py`
   - ...

   ## Prior-Round Asks   [round >= 2]

   1. ...
   2. ...

   ## Lens

   <per-slot lens assignment — see "Slot lens defaults" below>
   ```

   For round >= 2, also note any fix-author deviations (why the author diverged from the brief, and what they did instead) — fold them into Summary or a sub-bullet under Prior-Round Asks; there's no dedicated stub heading for them.

3. **Pre-stage the codex prompts.** Codex CLI sessions cannot accept the BRIEF as a tool input the way Claude `Agent` calls can. Write `/tmp/codex_<panel-slug>_r{4,5,6}.md` files, each opening with:
   > You are R{N} codex on the <panel-slug> round-<round> verification panel. Read `<abs-path>/BRIEF.md`.

   followed by the slot-specific lens, the previous-round JSON path (round >= 2), and the concrete_changes_required items they personally raised in the previous round.

## Launch the panel

1. **Launch Codex first** (they run 4-10 min vs Claude 1-3 min; start the slow ones first).

   For each codex slot R4, R5, R6:
   - The `/tmp/codex_<panel-slug>_r<N>.md` prompt (from step 0) tells the reviewer to read `<spec-dir>/reviews/<panel-slug>/BRIEF.md`, names the slot id (`R4 codex`, etc.) and the lens, and for round >= 2 points at the previous round's JSON.

     **Prompt MUST include this terminal instruction verbatim:**

     ```
     Your final step before terminating is to WRITE the verdict JSON to `<EXPECTED_JSON>` using a `Write` tool call (or a single-file patch). Do NOT terminate after just composing the verdict in your output — the orchestrator reads the file, not the log. If you cannot write to that exact path (sandbox or permissions), STOP and write a short error message to the same path describing why.
     ```

     Codex without this instruction reliably "finishes thinking" without writing — particularly when its working directory is outside the panel JSON path's sandbox.
   - Launch (always `cd <repo>` first — see "Working directory matters" below). Use a SINGLE source of backgrounding — the Bash tool's `run_in_background: true`. **No** trailing `&`, **no** `nohup` wrapper:
     ```bash
     cd <repo>
     codex exec --skip-git-repo-check < /tmp/codex_<panel-slug>_r<N>.md > /tmp/codex_<panel-slug>_r<N>.out 2>&1
     ```
     Tool call shape: `Bash` with the command above and `run_in_background: true`. The Bash tool backgrounds the process itself and fires a `<task-notification>` when codex actually exits.

   > **Anti-pattern: do NOT double-background codex.**
   >
   > The combination `nohup bash -c '...' &` + `run_in_background: true` puts codex into two layers of background. The shell-level `&` returns immediately, so the Bash tool's task-notification fires on bash-exit (~1 sec), not codex-exit (~5–10 min). The orchestrator then reads an empty output file and falsely concludes codex failed.
   >
   > Pick ONE source of backgrounding. Recommended: the Bash tool's `run_in_background: true` — it gives you a real `<task-notification>` when codex actually exits, plus a clean path to grep the output log via `Read` (or, for log-extraction recovery per the "Codex failure detection" section below, via `codex_verdict_extract.sh`).
   >
   > If you must shell-background instead (e.g. to capture `$!` for explicit PID tracking), drop `run_in_background: true` from the Bash tool call so the tool returns synchronously after the `&` — then poll separately. This path is harder; default to the tool-level backgrounding.

2. **Launch Claude `Agent`s second** (faster; start after Codex is already running).

   For each claude slot R1, R2, R3:
   - Compose a prompt with the BRIEF path, slot id, lens, and required JSON output path.
   - Spawn a `general-purpose` `Agent` with `run_in_background: true`.

3. **Wait for completion notifications.** Each finishes asynchronously; you receive `<task-notification>` messages. Don't poll outputs — wait for the notification.

4. **Detect codex failures.** See "Codex failure detection (deterministic)" below.

5. **Verify all six JSON files exist** at `<spec-dir>/reviews/<panel-slug>/<slot>-round-<N>.json`.

## Codex failure detection (deterministic)

A healthy codex run writes its JSON verdict to the named output path. Failure modes: codex hit its usage limit, codex tried to write but the sandbox rejected the path, or codex "finished thinking" without ever attempting a write. Detect each deterministically — the original v1 check used `"Output JSON to"` in the log as a healthy-ack signal, but that string is just codex echoing the prompt back; it appears even when the file never lands. Use three layers instead, in order.

**Pre-condition: honest task-notification timing.** This check is only reliable when the `<task-notification>` fires on *codex* exit, not on a shell-wrapper exit. That requires single-source backgrounding (see launch step 1 above) — drop `&` and `nohup`; use only the Bash tool's `run_in_background: true`. The earlier double-backgrounded pattern made the file-existence primary check race against still-running codex, producing false "completed in 1 sec, file empty" reports. With the timing honest, layer 1 below is reliable on first read.

```bash
EXPECTED_JSON="<spec-dir>/reviews/<panel-slug>/codex-<slot>-round-<N>.json"
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

Codex's default sandbox is `workspace-write [workdir, /tmp, $TMPDIR, /Users/Max/.codex/memories]`. The `workdir` is whatever directory you `cd` into before `codex exec`. If the panel JSON path (`<spec-dir>/reviews/<panel-slug>/...`) is outside that workdir, codex's write silently fails.

**Launch convention**: always `cd <repo>` first so `<spec-dir>/reviews/...` is inside the sandbox. Single-source the backgrounding via the Bash tool's `run_in_background: true` — no `&`, no `nohup`:

```bash
cd <repo>
codex exec --skip-git-repo-check < /tmp/codex_<panel-slug>_r<slot>.md > /tmp/codex_<panel-slug>_r<slot>.out 2>&1
```

If the panel runs in a worktree under a parent repo, `cd` to the worktree, not the parent — the worktree's `<spec-dir>/reviews/` subtree is where the verdict needs to land.

## Slot lens defaults

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | Author-of-record — diff matches plan §<bead>? |
| R2 | Claude | Codebase-pin — named files and tests actually exist + green? |
| R3 | Claude | Prompt-shape / contract stability — downstream surface stable? |
| R4 | Codex  | Empirical prober — run validators by hand |
| R5 | Codex  | Schema / type correctness — Pydantic, SQLAlchemy, unions |
| R6 | Codex  | Next-bead integration — will the next bead consume this cleanly? |

Mix to taste. The point is six distinct lenses, not six clones. For round >= 2, each slot inherits its previous-round lens and is told to evaluate only its own `concrete_changes_required` items as ADDRESSED / PARTIAL / MISSED / NEW_ISSUE, plus flagged fix-author deviations from the BRIEF.

## Anti-patterns

- Don't read `.out` files via `Read` while a codex session is still active — they grow and can overflow context. Wait for the completion notification, then grep targeted strings.
- Don't run all six reviewers as foreground tool calls — that serialises the panel and wastes 5-10 minutes.
- Don't reuse a codex `/tmp/codex_*.md` file across rounds — write a fresh one per round so the round number is unambiguous in the prompt.
- Don't ask Claude reviewers to also do the codex empirical-probe lens — duplication is worse than coverage gaps; trust the family split.
- Don't hand-edit `panel.json`'s `round`/`reviewed_head_sha` instead of calling `mindspec panel create --round N+1`. The verb is the only place the two co-bump atomically; the gate reads `panel.json`, and a stale or hand-desynced pair is a silent bypass.

## Then

Hand off to `/ms-panel-tally`.
