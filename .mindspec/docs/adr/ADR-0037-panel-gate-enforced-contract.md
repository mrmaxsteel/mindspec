# ADR-0037: Panel Gate as Enforced Contract — panel.json Convention, Thresholds, and the Trust Boundary

- **Date**: 2026-06-11
- **Status**: Accepted
- **Domain(s)**: workflow, execution
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0023](ADR-0023.md) (beads as single state authority), [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) (artifact paths filtered from the dirty-tree rule), [ADR-0030](ADR-0030-executor-boundary.md) (hook-side git budget), [ADR-0035](ADR-0035-agent-error-contract.md) (block-message protocol)

---

## Context

Before spec 093, "the panel must approve before merge" was prose in
skill files. Prose under pressure fails: the lola-f4a8 incident
(spec-050 shipped without its AC8c cost-projection artifact; first
prod cron burned $417) happened because a stale APPROVE was honored
and the missing-artifact rule lived only in a skill an agent could
skim past. Spec 093 turns the rule into a machine gate: a PreToolUse
hook on `mindspec complete` that reads panel state from the
filesystem and blocks premature merges.

That gate needs a stable, recorded contract — what registers a panel,
how rounds and verdicts are derived, what passes, what blocks, and
(critically) what the gate is and is NOT defending against — because
every one of those rules looks like a bug to a future editor who
encounters it without this record.

## Decision

### 1. Registration: `panel.json` is the panel's identity

A panel is **registered** iff `review/<slug>/panel.json` exists.
Schema (one source of truth: `internal/panel.Panel`):

```json
{
  "bead_id": "<id> | null",
  "spec": "<spec-id>",
  "target": "<bead|pr|commit|branch ref>",
  "round": 1,
  "expected_reviewers": 6,
  "reviewed_head_sha": "<git rev-parse of target at fan-out>",
  "abandoned": false,
  "abandon_reason": "<who/why — required when abandoned>"
}
```

`bead_id` is null for non-bead targets (final-review/PR panels),
which are outside v1 hook enforcement (surfaced via
`mindspec instruct --panel-state` only). On every re-panel, `round`
and `reviewed_head_sha` are bumped **in the same write** by
/ms-panel-run step 0 — the two fields move together by construction.

**Registered-panels-only scope**: the gate covers panels that opt in
by writing panel.json. Legacy review dirs (BRIEF.md, no panel.json)
and solo/harness/CI flows with no panel at all are structurally
unaffected — see the fail-open rule below. Externally-orchestrated
flows that never route through Claude Code hooks (codex sessions,
raw-shell agents) get the advisory tally inside `mindspec complete`
only; hard enforcement exists at the hook layer alone. This is an
honest limitation, not an oversight.

### 2. Round derivation: filenames over panel.json

The latest round is **max(N) over `*-round-<N>.json` filenames** —
never `panel.json.round`, which can lag (reviewers write files
independently of step 0). A disagreement in either direction is a
reported round-mismatch that blocks: never tally a round below the
filename max, and never read a previous round's APPROVEs as the
bumped round's outcome.

### 3. Threshold: `expected_reviewers − 1`, defined once

The approval threshold is **N − 1** where N = `expected_reviewers`
(one dissent tolerated): 5-of-6 for the default panel. The rule lives
in exactly one place (`internal/panel.Panel.ApproveThreshold`); no
consumer hardcodes a second copy of 6 (or 5). Scaling guidance for
humans choosing N (ceil(5N/6)) stays in the README.

A malformed verdict JSON counts as **missing** and is named in the
block; any REJECT or `"hard_block": true` blocks regardless of vote
count (the lola-f4a8 artifact-gate rule, mechanized).

### 4. Staleness: `reviewed_head_sha` must match the live ref

An APPROVE attaches to a commit, not a bead. If the target ref no
longer points at `reviewed_head_sha`, the verdicts are stale and the
gate **blocks** (not warns — a Warn here is the same
prose-under-pressure failure the gate exists to close): bump the
round and re-panel. Exception: if the bead branch no longer exists,
the merge already landed (completion deletes the branch) — pass
through with a warning to `mindspec complete`'s own idempotent
handling, because re-running complete is the documented
partial-failure recovery and false positives are the pinned bug
class.

### 5. Dirty tree: uncommitted user edits block

When a registered panel matches the bead, uncommitted **user-authored**
changes in the bead worktree block: `mindspec complete` would
auto-commit them past review (CommitAll runs after the hook fires),
making a post-approval "tiny touch-up" the cheapest stale-verdict
bypass. ADR-0025 artifact paths (`.beads/issues.jsonl`) are filtered
out first — artifact dirt is designed-for and never blocks at
complete time.

### 6. Fail-open without a panel, fail-closed with one

No panel.json referencing the bead → silent pass. A present
registration with missing/malformed verdicts, a stale SHA, or a
sub-threshold tally → block. This asymmetry is deliberate:
solo/non-panel/harness flows must be structurally unaffected (the
test harness completes beads constantly with no panels), while a
panel, once registered, is a commitment.

### 7. Escape hatches: env-only skip, audited abandonment, config toggle

- **Skip (`MINDSPEC_SKIP_PANEL=1`)** is read via `os.Getenv` ONLY —
  the command string is never consulted. **Spawn topology is why**:
  a PreToolUse hook inherits Claude Code's process environment, which
  the agent cannot alter, so the env channel is the single
  agent-proof input the gate has. Contrast the git pre-commit hook,
  which git spawns as a *child of the prefixed command* — an
  env-prefix on the command line (`MINDSPEC_ALLOW_MAIN=1 git commit`)
  legitimately reaches it, so that hook may keep printing its
  paste-able incantation. **Do not harmonize this asymmetry in either
  direction.** The panel gate's block output never prints a
  paste-able skip prefix (a blocked LLM's highest-probability next
  action is pasting a suggested prefix); the variable is documented
  for humans in /ms-panel-tally § Escape hatch only. Every use is
  audited: a Warn at skip time plus `panel_gate_skipped: true` +
  timestamp on bead metadata via `bead.MergeMetadata`.
- **Abandonment** (`"abandoned": true` + mandatory `abandon_reason`
  with who/why in panel.json) is the legitimate exit for a panel that
  should not conclude. It is a plain repo-file edit and therefore
  agent-performable; it is legitimate precisely because it is always
  audited, never silent: the gate passes with a Warn naming the
  reason, and completion writes `panel_abandoned: true` + timestamp +
  the reason to bead metadata.
- **Config toggle** (`enforcement.panel_gate: false`, default true)
  is the persistent opt-out, mirroring the pre-commit hook toggle.

### 8. Trust boundary: anti-footgun, not anti-adversary

**Every gate input is an agent-writable repo artifact**: panel.json
(including `round`, `expected_reviewers`, `reviewed_head_sha`,
`abandoned`), the verdict JSONs, and equally the
`enforcement.panel_gate` config toggle (a git-visible repo file, like
abandonment). An agent that *wants* to forge a 6/6 APPROVE can write
six JSON files. The gate's purpose is to stop the **accidental**
premature merge — the orchestrator that lost panel context to
compaction, the eager loop that runs `mindspec complete` while
verdicts are still landing — not a motivated adversary, which no
repo-file mechanism can stop.

**A future panel or editor must not "fix" perceived forgeability at
this layer.** Signing verdicts, hashing panel.json, or moving state
out of the repo would buy nothing (the agent controls the process
that would verify) and would break the property that makes the system
debuggable: panel state is plain reviewable files. The one
agent-proof input is the env channel (above), and the audit writes
make every legitimate bypass visible after the fact. Forgery, unlike
a footgun, leaves deliberate fingerprints in git history — that is
the actual deterrent, and it is enough.

Per ADR-0023, panel.json and verdict JSONs are **review artifacts,
not workflow state**: the gate reads them to decide whether a
state-changing command may proceed and writes nothing to them; bead
lifecycle state stays derived from bd statuses.

## Decision Details

- `internal/panel` is the single reader: `Scan(roots...)` (fs-only
  glob of `review/*/panel.json`, deduped) and `Tally(dir)`
  (registration + filename-derived latest round + verdict tally +
  malformed-as-missing names + hard-block slots). The hook, the
  complete-side advisory tally, and `instruct --panel-state` all call
  the same `Tally` — the "gate would PASS/BLOCK" line can never
  disagree with the gate.
- `internal/panel` makes zero git/bd/subprocess calls. Staleness and
  dirty-tree checks are the hook's own git work, capped at two
  subprocesses on the matched path (ADR-0030 discipline); the
  non-match path does zero work beyond string matching.
- Block messages follow the hook Emit protocol (stderr + exit 2) and
  end with an actionable next step plus the raw-merge fence: only
  `mindspec complete` merges bead branches — raw `git merge bead/<id>`
  skips bd closure, worktree cleanup, and this gate (git runs no
  commit hook for automatic merge commits, so the gate cannot catch
  it; the fence is prose by necessity).

## Consequences

### Positive

- The stale-verdict rule, the artifact-gate HARD block, and the
  panel-approval precondition stop being skimmable prose; the failure
  class that produced lola-f4a8 gets a machine gate.
- One tally implementation serves the hook, complete's advisory line,
  and --panel-state — no decision drift between surfaces.
- Solo and harness flows pay nothing: no panel.json, no gate.

### Negative / Tradeoffs

- The contract is honest about its limits: an agent can forge inputs,
  disable the toggle, or abandon a panel. Each path is audited, not
  prevented — accepting this keeps panel state as plain files.
- A registered panel makes recovery reruns stricter; the missing-ref
  and missing-worktree pass-throughs exist precisely to keep the
  documented partial-failure recovery (`mindspec complete` rerun)
  unblocked.
- Skills and docs must never print the skip variable in block output;
  this is a standing editorial constraint (HC-7) enforced by test.

## Alternatives Considered

### 1. Keep the panel-approval rule in skill prose

Rejected: this is the status quo that failed under pressure
(lola-f4a8). Prose cannot block a command.

### 2. Tamper-resistant panel state (signatures, out-of-repo store)

Rejected per the trust boundary above: the agent controls the
verifying process, so tamper-resistance is theater; it would also
make panel state opaque to review. Anti-footgun is the design goal.

### 3. Command-string escape hatch (`MINDSPEC_SKIP_PANEL=1 mindspec complete ...`)

Rejected: the command string is agent-writable, and a blocked LLM's
most likely next action is pasting a suggested prefix. The PreToolUse
spawn topology makes `os.Getenv` agent-proof — the only input that
is — so the hatch is env-only, and the block message tells the agent
a human must set it.

### 4. Warn (not block) on reviewed_head_sha mismatch

Rejected: a warning on staleness reproduces the exact bypass class
the gate exists for — the stale APPROVE honored at merge time.

## Validation / Rollout

1. Spec 093 Bead 2 lands `internal/panel` (schema, Scan, Tally) with
   unit tests for every tally shape, plus this ADR.
2. Bead 3 fixes the settings-merge identity machinery so the
   PreToolUse entry can install without clobbering user hooks.
3. Bead 4 ships the hook gate (decision matrix, staleness,
   dirty-tree, hatches + audits) with table-driven `hook.Run` tests
   and one LLM-harness scenario (`panel_gate_blocks_premature_complete`).
4. Beads 5-6 wire `--panel-state` and the panel.json writer
   (/ms-panel-run step 0) so every surface speaks this convention.
