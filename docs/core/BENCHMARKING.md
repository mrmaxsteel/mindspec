# Benchmarking MindSpec vs Freestyle Claude Code

This guide walks through running an A/B comparison between a MindSpec-assisted session and a freestyle Claude Code session, then producing a quantitative report.

## What You'll Measure

| Metric | Source |
|:-------|:-------|
| Input/output/cache tokens | Claude Code OTel telemetry |
| Estimated cost (USD) | Claude Code OTel telemetry |
| Wall-clock duration | Event timestamps |
| API call count | Claude Code OTel telemetry |
| Cache hit rate | Derived from token counts |
| Context pack token overhead | MindSpec `--trace` (Session A only) |

## Prerequisites

- Two VSCode windows (or terminals with `claude` CLI)
- MindSpec binary built: `make build`
- A feature spec written down (same description for both sessions)
- Same repo, same commit — use git worktrees to isolate

## Step 1: Prepare the Worktrees

From your main repo:

```bash
# Create two worktrees on the same commit
git worktree add ../bench-mindspec -b bench-mindspec
git worktree add ../bench-baseline -b bench-baseline
```

### Neutralize MindSpec in the baseline worktree

The baseline worktree inherits `CLAUDE.md`, `.mindspec/`, and `.claude/` from the repo — these would cause Claude Code to follow MindSpec workflows automatically. Strip them out:

```bash
cd ../bench-baseline

# Remove MindSpec project instructions (Claude Code reads this automatically)
rm -f CLAUDE.md

# Remove MindSpec state (prevents hooks from emitting guidance)
rm -rf .mindspec/

# Remove project-level Claude Code hooks and rules
rm -rf .claude/
```

These deletions are on the `bench-baseline` branch and won't affect `main`. Without these files, Claude Code operates as a vanilla agent with no MindSpec awareness.

Open each worktree in a separate VSCode window.

## Step 2: Print the Setup Instructions

```bash
mindspec bench setup
```

This prints environment variable blocks for both sessions. You can also configure manually — see below.

## Step 3: Start the Collectors

Open two terminals (these run in the foreground and capture telemetry):

```bash
# Terminal 1 — collects Session A (MindSpec)
mindspec bench collect --port 4318 --output /tmp/bench-session-a.jsonl

# Terminal 2 — collects Session B (Baseline)
mindspec bench collect --port 4319 --output /tmp/bench-session-b.jsonl
```

Each collector is a lightweight HTTP server that receives OTel events from Claude Code and writes them as NDJSON.

## Step 4: Configure the Sessions

### Session A — MindSpec

In the VSCode terminal for `../bench-mindspec`, set these environment variables **before** starting Claude Code:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export MINDSPEC_TRACE=/tmp/mindspec-bench-a-trace.jsonl
```

Then start Claude Code normally. The `MINDSPEC_TRACE` variable also enables MindSpec's internal tracing (context pack sizes, glossary matches, bead CLI timing).

### Session B — Baseline (no MindSpec)

In the VSCode terminal for `../bench-baseline`:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4319
```

Then start Claude Code normally. No MindSpec commands — just give Claude the feature description directly.

## Step 5: Run the Experiment

Give both sessions the **exact same feature description**. For example:

> "Implement a `widget list` command that reads widgets from a YAML file and prints them in a table. Include tests."

### Session A workflow (MindSpec):
1. `/spec-init` → write the spec
2. `/spec-approve` → plan
3. Write the plan → `/plan-approve`
4. `mindspec next` → implement each bead
5. `mindspec complete` when done

### Session B workflow (freestyle):
1. Paste the feature description
2. Let Claude implement it however it wants
3. Iterate until satisfied

**Important:** Try to achieve roughly equivalent quality in both sessions. The comparison is most meaningful when both produce working, tested code.

## Step 6: Stop the Collectors

When both sessions are complete, press `Ctrl-C` in each collector terminal. The collector prints a summary:

```
Collected 47 events → /tmp/bench-session-a.jsonl
```

## Step 7: Generate the Report

```bash
mindspec bench report \
  /tmp/bench-session-a.jsonl \
  /tmp/bench-session-b.jsonl \
  --labels "mindspec,baseline"
```

Output:

```
Metric                        mindspec        baseline           Delta
────────────────────────────────────────────────────────────────────
API Calls                           12              18              -6
Input Tokens                     45000           82000          -37000
Output Tokens                     8500           14000           -5500
Cache Read Tokens                12000            3000           +9000
Cache Create Tokens               2000            1500            +500
Total Tokens                     53500           96000          -42500
Cost (USD)                     $0.3200         $0.6800        -$0.3600
Duration                       8.2 min        14.5 min        -6.3 min
Cache Hit Rate                   20.3%            3.5%
Output/Input Ratio               0.19x           0.17x
```

For machine-readable output:

```bash
mindspec bench report /tmp/bench-session-a.jsonl /tmp/bench-session-b.jsonl \
  --labels "mindspec,baseline" --format json > /tmp/bench-report.json
```

## Step 8: Inspect MindSpec Trace (Optional)

Session A also produces a MindSpec-side trace showing where tokens were spent:

```bash
mindspec trace summary /tmp/mindspec-bench-a-trace.jsonl
```

```
Trace Summary: /tmp/mindspec-bench-a-trace.jsonl
  Events:     34
  Duration:   4823.1 ms
  Tokens:     3142

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  bead.cli                      12   3200.0 ms        -
  command.end                    8    4823.1 ms        -
  command.start                  8          -        -
  contextpack.build              2    120.3 ms     2400
  glossary.match                 1      1.2 ms       42
  instruct.render                2      5.1 ms      700
  state.transition               3          -        -
```

You can also inspect individual events:

```bash
# See context pack token breakdown
jq 'select(.event=="contextpack.build") | .data' /tmp/mindspec-bench-a-trace.jsonl

# See all bead CLI calls sorted by duration
jq -r 'select(.event=="bead.cli") | "\(.data.dur_ms | tostring | .[:6])ms \(.data.op) \(.data.args | join(" "))"' \
  /tmp/mindspec-bench-a-trace.jsonl | sort -rn
```

## Cleanup

```bash
# Remove worktrees
git worktree remove ../bench-mindspec
git worktree remove ../bench-baseline

# Remove trace files
rm /tmp/bench-session-a.jsonl /tmp/bench-session-b.jsonl /tmp/mindspec-bench-a-trace.jsonl
```

## Tips

- **Same complexity:** Pick a feature that's complex enough to show differentiation (at least 2-3 files, with tests) but not so large that session variance dominates.
- **Same model:** Ensure both sessions use the same Claude model. Check with `/model` in Claude Code.
- **Warm cache:** If running multiple experiments, the second run may benefit from prompt caching. Consider running a throwaway warm-up session first, or comparing cache rates as part of the analysis.
- **Quality:** The report only measures cost and time. Assess quality separately — review the code both sessions produced, check test coverage, architectural cleanliness, etc.
