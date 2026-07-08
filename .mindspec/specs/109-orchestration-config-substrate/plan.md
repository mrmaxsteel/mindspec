---
adr_citations:
    - ADR-0037
    - ADR-0035
    - ADR-0034
    - ADR-0039
approved_at: "2026-07-02T21:57:34Z"
approved_by: user
bead_ids:
    - mindspec-dov0.1
    - mindspec-dov0.2
    - mindspec-dov0.3
    - mindspec-dov0.4
spec_id: 109-orchestration-config-substrate
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - .mindspec/adr/ADR-0040-orchestration-layering-ratchet.md
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/config/config.go
        - internal/config/config_test.go
        - .mindspec/domains/core/interfaces.md
    - depends_on:
        - 1
      id: 3
      key_file_paths:
        - internal/panel/panel.go
        - internal/panel/gate.go
        - internal/panel/panel_test.go
        - .mindspec/domains/workflow/architecture.md
    - depends_on:
        - 2
        - 3
      id: 4
      key_file_paths:
        - cmd/mindspec/config.go
        - cmd/mindspec/config_test.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_advisory_test.go
        - .mindspec/domains/workflow/interfaces.md
---
# Plan: 109-orchestration-config-substrate

## ADR Fitness

The impacted domains are **core** (`internal/config`) and **workflow**
(`internal/panel`, `cmd/mindspec`). The four cited ADRs are the minimal set that
both **covers** those two domains and **genuinely constrains** the implementation;
each was evaluated and remains the correct architectural choice for this work
(adherence, no divergence):

- **ADR-0037 — Panel Gate as Enforced Contract** (Domains: workflow, execution;
  covers workflow). The load-bearing constraint on Beads 3–4. §3 pins the N−1
  threshold to a **single home**, `internal/panel.Panel.ApproveThreshold`, and
  the spec-099 relocation pins "identical decision over identical facts." This
  plan **adheres**: the gate decision stays a pure function of the recorded
  `panel.json` (`PanelGateDecision` gains no `config` input), threshold
  interpretation stays inside `ApproveThreshold()`, and `internal/panel` stays a
  config-free leaf. The one change is an honest **amendment note** appended to
  ADR-0037 (Bead 1) recording the new optional `approve_threshold` field as an
  **extension** of §3's threshold rule — an optional recorded in-range integer
  overrides the `N−1` default, still single-homed in `ApproveThreshold()` and
  `N−1` when absent; §§6/8 (fail-open/closed, trust boundary) are unchanged. The
  amendment is NOT a supersede and NOT a claim of "semantically unchanged"; the
  rule is genuinely extended, so no `mindspec adr create --supersedes` flow is
  triggered. ADR-0037 remains the best design: the alternative (letting config
  re-derive the threshold at decision time) was rejected in the spec's Open
  Questions because it breaks "identical decision over identical facts."
- **ADR-0035 — Agent Error Contract** (Domains: workflow, execution, core;
  covers **core** and workflow). Constrains Bead 2's new `Load` refusals and Bead
  4's `config show`: every validation error path (the `panel_skip` / non-`halt`
  `on_reject` / enum / threshold-bounds / reviewer-floor refusals) is a
  guard-style failure carrying a recovery line, never a panic. Sound as-is.
- **ADR-0034 — Ceremony Collapse** (Domain: workflow; covers workflow).
  Constrains Bead 2's `loop.gate_authority` map — its four keys
  (`spec_approve` / `plan_approve` / `bead_merge` / `impl_approve`) name exactly
  the single-bead-lifecycle gates this ADR collapsed, each defaulting to `human`.
  Sound as-is.
- **ADR-0039 — Flat `.mindspec/` Layout v2** (Domains: core, workflow, execution,
  context-system; covers **core** and workflow). `config.yaml` and the
  panel-review directories both live under the flat layout this ADR fixed; the
  `panel:` defaults feed the same layout-aware `panel.json` the gate reads. Sound
  as-is.

**Coverage check.** core is covered by ADR-0035 and ADR-0039; workflow is covered
by all four. Every citation's `Domain(s)` line intersects the impacted set
{core, workflow}, so none is architecturally irrelevant.

**Deliberately not cited.** The spec's ADR Touchpoints also names ADR-0036,
ADR-0032, ADR-0023, and ADR-0038. Each intersects workflow (so citing them would
not be *irrelevant*), but none adds an implementation constraint beyond the cited
set — they are architectural context, not code constraints — so they are omitted
to keep `adr_citations` load-bearing.

**ADR-0040 is created, not cited.** The spec's central deliverable is a new
Accepted **ADR-0040** (the layering ratchet + portability principle), landed by
Bead 1. It is a **codification** ADR — it documents a layering that already
governs the codebase (spec 102's in-binary gate, ADR-0037's mechanized matrix,
ADR-0034's ceremony collapse) — so it lands **Accepted**, not Proposed (spec Open
Question, resolved), and needs no supersede flow. It is **not** listed in this
plan's `adr_citations` because it does not exist on disk at `mindspec validate
plan` / `mindspec plan approve` time (Bead 1 creates it); citing it would fail the
`adr-cite-missing` gate. **The mechanical complete-time ADR-divergence coverage
for the later source beads is supplied entirely by ADR-0035 and ADR-0039 (both
carry `core, workflow`); ADR-0040 is the in-tree architectural anchor landed by
Bead 1 — citable by later specs, but not load-bearing for this plan's
`adr_citations` and not required to satisfy any coverage gate here.** Bead 1
landing first ensures that anchor is present before any source bead lands (see
Testing Strategy → dependency shape).

## Testing Strategy

**Approach.** Pure unit tests, table-driven, added to each touched package's
existing `_test.go` file (`internal/config/config_test.go`,
`internal/panel/panel_test.go`, `cmd/mindspec/config_test.go`). No integration or
e2e layer is needed: the substrate is parse / default / validate / render logic
with no new I/O surface. `renderConfig(*config.Config) (string, error)` is a
testable pure function so `mindspec config show` is exercised without spawning a
process (spec R9).

**Per-test proof discipline.** Every new test is verified with an anchored
PASS-line grep so a reviewer sees the specific test pass, not a bare package
green: `go test ./PKG -v -run 'TestName$' | grep -q -- '--- PASS: TestName'`.
The `$` anchor on `-run` prevents a prefix from sweeping in a sibling test.

**Leaf invariant.** Bead 3 must keep `internal/panel` import-clean of
`internal/config`; verified with `go list -deps ./internal/panel | grep -q
internal/config` returning **non-zero** (config absent from the dependency
closure). The config default reaches the panel package only as plain `int` values
passed by the caller.

**Regression.** The full suite `go test ./...` is run once at plan time and again
before `/ms-impl-approve`. It is expected green **except** the pre-existing
`internal/next` test-isolation signature (`TestRun_IdleNoBeads` shared-cwd leak,
tracked as `z4ps`) which is unrelated to this spec and is excluded from the
per-bead gate; this spec introduces no new shared-state test. Any git-touching
test is invoked with `GIT_TERMINAL_PROMPT=0` so a missing ref fast-fails instead
of prompting. No new external dependency, no network access.

**Dependency shape (decomposition).** Four beads. Bead 1 (docs) has no deps.
Beads 2 (`internal/config`) and 3 (`internal/panel`) depend only on Bead 1 — a
package-disjoint pair that runs in parallel after the ADR anchor lands; Bead 3
stays a pure leaf (no dependency on Bead 2). Bead 4 (`cmd/mindspec` +
`internal/complete`, the workflow-side consumers) depends on **both** Bead 2 and
Bead 3: `renderConfig` reads the new `config.Panel/Models/Loop/Runner` fields
Bead 2 adds (a **real compile-time dependency**, not merely an ordering wish),
and the two caller-side note surfaces call `panel.ReviewerCountNote` (Bead 3)
with `cfg.PanelExpectedReviewers()` (Bead 2). Bead 1 is transitively guaranteed
before Bead 4 via either parent. Longest chain is 3 nodes (1 → 2 → 4 and
1 → 3 → 4), at the advisory `max_chain_depth` threshold, not over it; parallelism
ratio is 0.25, at the advisory floor. Beads 2 and 3 are genuinely independent
(disjoint packages, no shared produced state), so no false edge is added between
them — grouping both caller-side renderings into Bead 4 (rather than splitting the
complete-gate wiring into Bead 3) keeps the leaf helper's definition and its two
consumers cleanly separated and preserves the 2‖3 parallelism.

## Bead 1: ADR-0040 (Accepted) + the ADR-0037 amendment note

**Steps**
1. Create `.mindspec/adr/ADR-0040-orchestration-layering-ratchet.md` with the
   standard ADR header, including `- **Status**: Accepted` and
   `- **Domain(s)**: core, workflow` (so the later source beads have an additional
   covering ADR present on the branch).
2. Write the Decision codifying the **four layers and their one-directional
   ratchet**: (L1) in-binary gates (exit non-zero, journaled escape hatches);
   (L2) declared config (the middle rung — how a prose rule becomes
   machine-readable without becoming code); (L3) mode-selected `instruct` advice,
   generated from gate definitions where it describes gates; (L4) skills as
   judgment kernels and the staging area for procedure — a load-bearing/gameable
   skill rule ratchets **down** into config or a binary gate, **never casually
   up**. State the default answer to "where does this rule live?".
3. Cite the precedent chain as already-shipped evidence: spec 102's
   PreToolUse-hook → in-binary panel gate move, the decision matrix mechanized in
   ADR-0037, and the ceremony collapse in ADR-0034.
4. Add the **portability principle**: agent integration happens at the
   **artifact + CLI contract** level (the `panel.json` / verdict-JSON schemas,
   the future `mindspec panel` verbs, `mindspec instruct` output, beads state) and
   **never** at the prompt-format level; orchestration runners are **adapters**
   behind those contracts. Cite the in-repo precedent both directions
   (`internal/setup`'s per-agent trio and the `Executor` / ADR-0030 boundary as
   healthy interfaces born with a real second consumer, vs. a harness `Agent`
   interface accreting dead methods as rot), and include the **capability-tier /
   degraded-modes** note (no SessionStart hook in codex, no subagent spawning in
   some runners → human-run `instruct`, panel-less operation under
   `enforcement.panel_gate`).
5. Append an **amendment note** to
   `.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` recording the one new
   optional schema field `approve_threshold` as an **extension** of §3's threshold
   rule (an optional recorded in-range integer overrides the `N−1` default, still
   single-homed in `ApproveThreshold()` and `N−1` when absent; §§6/8 unchanged;
   an absent field is byte-identical `N−1`) — worded as an extension, not
   "semantically unchanged".
6. Run the content-anchor greps to confirm every required label/phrase is present.

**Verification**
- [ ] `F=.mindspec/adr/ADR-0040-orchestration-layering-ratchet.md; grep -Eq '^- \*\*Status\*\*: Accepted' "$F" && grep -q 'L1' "$F" && grep -q 'L2' "$F" && grep -q 'L3' "$F" && grep -q 'L4' "$F" && grep -q 'never casually up' "$F" && grep -q 'artifact + CLI contract' "$F"` exits `0`. The Status anchor uses a bolding-tolerant regex because the repo header format is `- **Status**: Accepted` (the literal `Status: Accepted` substring is absent from that line). The spec AC's `grep -q 'Status: Accepted'` is the same intent — a bolding-tolerant "Status is Accepted" check — and is interpreted that way; the approved spec is **not** edited (forward-only lifecycle, ADR-0023).
- [ ] `grep -qiE 'Domain\(s\).*\bcore\b.*\bworkflow\b' .mindspec/adr/ADR-0040-orchestration-layering-ratchet.md` exits `0` (Domain line covers both impacted domains)
- [ ] `grep -q 'approve_threshold' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` exits `0` (amendment note present)
- [ ] `git show --name-only --format= HEAD | grep -qE '\.go$'` returns **non-zero** — the bead commit touches no Go source, so `mindspec complete`'s doc-sync gate is **vacuous** (no domain source changed → no domain doc required; the `.mindspec/adr/**` edits are doc-classified and carry themselves)
- [ ] `go build ./...` exits `0` (tree integrity; ADR docs do not affect the build)

**Acceptance Criteria**
- [ ] ADR-0040 lands Accepted (`- **Status**: Accepted`, matched bolding-tolerantly) and carries all four layer labels (L1–L4), the ratchet-direction phrase `never casually up`, and the portability phrase `artifact + CLI contract` (spec AC "ADR-0040 lands Accepted…" — R1 content falsifications made runnable)
- [ ] The ADR-0037 amendment note recording the optional `approve_threshold` field is present (spec Validation Proof `grep -q 'approve_threshold' …ADR-0037…`)

**Depends on**
None

## Bead 2: internal/config — panel/models/loop/runner schema, defaults, refusals, resolvers

**Steps**
1. Add the new schema to `internal/config`: a `Panel` block (`reviewers` list of
   `{family, count}`, `approve_threshold` string, `substitution.claude_sub_on_quota`
   bool), a `Models map[string]string` keyed by phase (free-form string values),
   a `Loop` block (`enabled` bool; `gate_authority` map with the four keys
   `spec_approve`/`plan_approve`/`bead_merge`/`impl_approve`; `halt` with
   `max_rounds_per_bead`/`panel_deadlock_rounds`/`max_consecutive_impl_failures`/
   `on_reject`; `budget` with `max_beads_per_wake`/`token_budget`; `context` with
   `controller_handoff`; `handoff_log` string), and a top-level `Runner string`.
2. Extend `DefaultConfig()` with today's-practice defaults: reviewers
   `[{claude,3},{codex,3}]`, `approve_threshold "n-1"`, `claude_sub_on_quota true`;
   empty `models`; `loop.enabled false`, all four `gate_authority` values `human`,
   `halt` `3`/`2`/`2` + `on_reject "halt"`, `budget` `0`/`0`, `context.controller_handoff
   "per-spec"`, `handoff_log "AUTOPILOT-LOG.md"`; `runner "claude-code-skills"`.
3. Extend `loadUncached`'s backfill so an absent/empty new block resolves to its
   default (round-trip), leaving every pre-existing field's backfill untouched.
4. Add the `Load` validations/refusals, each returning an error with a recovery
   line (ADR-0035): a `panel_skip` key under `loop.gate_authority`; a
   `loop.halt.on_reject` other than `halt`; a `gate_authority` value not in
   `{panel, human}`; a `controller_handoff` not in `{per-spec, at-usage-threshold}`;
   an unknown `runner`; a `panel.approve_threshold` that is neither `"n-1"` nor an
   integer in `[1, sum(reviewers.count)]`; a `reviewers[].count < 1`; and a
   `sum(reviewers.count) < 2`.
5. Add resolvers `(*Config).PanelExpectedReviewers() int` (sum of counts, default
   `6`) and `(*Config).PanelApproveThresholdExpr() string` (the **raw** expression,
   default `"n-1"` — it does NOT resolve to an int; resolution stays single-homed
   in `internal/panel`).
6. Add tests `TestLoad_ZeroConfigPanelModelsLoopDefaults`,
   `TestPanelExpectedReviewers_SumsReviewerCounts`, `TestLoad_RefusesUnweakenableKnobs`,
   `TestLoad_PopulatedConfigRoundTrips` to `internal/config/config_test.go`.
7. Document the new config schema surface + the two resolvers in
   `.mindspec/domains/core/interfaces.md` (doc-sync for the core source edit).

**Verification**
- [ ] `go test ./internal/config -v -run 'TestLoad_ZeroConfigPanelModelsLoopDefaults$' | grep -q -- '--- PASS: TestLoad_ZeroConfigPanelModelsLoopDefaults'`
- [ ] `go test ./internal/config -v -run 'TestPanelExpectedReviewers_SumsReviewerCounts$' | grep -q -- '--- PASS: TestPanelExpectedReviewers_SumsReviewerCounts'`
- [ ] `go test ./internal/config -v -run 'TestLoad_RefusesUnweakenableKnobs$' | grep -q -- '--- PASS: TestLoad_RefusesUnweakenableKnobs'`
- [ ] `go test ./internal/config -v -run 'TestLoad_PopulatedConfigRoundTrips$' | grep -q -- '--- PASS: TestLoad_PopulatedConfigRoundTrips'`
- [ ] `go test ./internal/config` exits `0` (whole package green, existing tests included)
- [ ] `git show --name-only HEAD | grep -qxF '.mindspec/domains/core/interfaces.md'` (doc-sync: the core source edit carries a core domain-doc edit in the same commit)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] Absent config file yields 3+3 reviewers, `"n-1"` threshold, `claude_sub_on_quota=true`, empty `models`, `loop.enabled=false` with all four `gate_authority` `human`, `halt` 3/2/2 + `on_reject=halt`, `budget` 0/0, `context.controller_handoff="per-spec"`, `handoff_log="AUTOPILOT-LOG.md"`, `runner="claude-code-skills"`, every pre-existing field unchanged (spec AC1)
- [ ] `PanelExpectedReviewers()` returns 6 for 3+3 and the correct sum for custom counts; `PanelApproveThresholdExpr()` returns the raw expression unresolved (spec AC2)
- [ ] `Load` returns an error for each of: `panel_skip` under `gate_authority`, non-`halt` `on_reject`, out-of-enum `gate_authority`, out-of-enum `controller_handoff`, unknown `runner`, `approve_threshold` of `0`/negative, `approve_threshold` exceeding the reviewer count, `reviewers[].count` below 1, and `reviewers` summing below 2 (spec AC3)
- [ ] A populated config (all five `models` phases, custom `reviewers` e.g. 2+2 with `approve_threshold: "3"`, a full valid `loop:` block, `runner: claude-code-workflow`) round-trips through `Load` unchanged (spec AC7)

**Depends on**
Bead 1

## Bead 3: internal/panel — recorded approve_threshold interpreter + leaf-safe advisory + gate pin

**Steps**
1. Add an optional field `ApproveThresholdExpr string \`json:"approve_threshold,omitempty"\``
   to `panel.Panel`.
2. Make `ApproveThreshold() int` the sole interpreter of that expression: an
   absent/empty field → `ExpectedReviewers − 1` (byte-identical to today and to
   every legacy `panel.json`); `"n-1"` (case-insensitive) → `ExpectedReviewers − 1`;
   an integer string in `[1, ExpectedReviewers]` → that integer; an integer outside
   `[1, ExpectedReviewers]` (`0`, negative, `> N`) or any other unparseable value →
   `ExpectedReviewers − 1` fallback (so a recorded `0` never yields `0`). No second
   interpreter is introduced.
3. Add the pure, config-free helper `panel.ReviewerCountNote(recorded, configDefault int) string`
   — empty string when equal, a non-empty advisory when they differ.
4. Add `TestApproveThreshold_InterpretsRecordedExpr` and
   `TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision` to
   `internal/panel/panel_test.go`. The second test asserts (a) `PanelGateDecision`
   over fixed `GateFacts` returns the same `Allow`/`Block` regardless of any config
   default, (b) `ReviewerCountNote` returns `""` on match and non-empty on
   mismatch, and (c) a panel whose resolved threshold is `0` (`ExpectedReviewers=1`,
   absent field — the legacy N=1 edge) still returns `Block`, pinning the
   `threshold > 0` gate-side guard at `gate.go:257`.
5. Document the recorded-threshold extension and the reaffirmed config-free leaf
   invariant in `.mindspec/domains/workflow/architecture.md` (doc-sync for the
   workflow source edit).

**Verification**
- [ ] `go test ./internal/panel -v -run 'TestApproveThreshold_InterpretsRecordedExpr$' | grep -q -- '--- PASS: TestApproveThreshold_InterpretsRecordedExpr'`
- [ ] `go test ./internal/panel -v -run 'TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision$' | grep -q -- '--- PASS: TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision'`
- [ ] `go test ./internal/panel` exits `0` (whole package green, existing tests included)
- [ ] `go list -deps ./internal/panel | grep -q internal/config` returns **non-zero** (leaf invariant: `internal/panel` imports no `internal/config`)
- [ ] `grep -q 'threshold > 0' internal/panel/gate.go` exits `0` (the third-defense clause is not deleted)
- [ ] `git show --name-only HEAD | grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] Absent field and `"n-1"` both yield `N−1`; an in-range integer (`1..N`) yields that integer; an out-of-range integer (`0`, negative, `> N`) and any other unparseable value each fall back to `N−1` — in particular a recorded `0` never yields `0` (spec AC4)
- [ ] `PanelGateDecision` over fixed `GateFacts` returns the same decision regardless of config default; `ReviewerCountNote` returns `""` when counts match and a non-empty advisory when they differ; a resolved-`0`-threshold panel still returns `Block`, pinning the `threshold > 0` guard (spec AC5)
- [ ] `internal/panel` still imports no `internal/config` (spec Validation Proof `go list -deps … | grep -q internal/config` returns non-zero)

**Depends on**
Bead 1

## Bead 4: cmd/mindspec + internal/complete — read-only `config show` + renderConfig + caller-side ReviewerCountNote rendering

This bead lands both **caller-side** surfaces R8 names for `panel.ReviewerCountNote`
(defined in Bead 3): `mindspec config show` and the in-binary complete-gate
advisory. Both are the *workflow-side callers*; neither is `PanelGateDecision`
(which stays a config-free leaf), and neither changes any `Allow`/`Block`. Both
packages touched (`cmd/mindspec`, `internal/complete`) are the **workflow** domain.

**Steps**
1. Add `cmd/mindspec/config.go`: a `config` command with a read-only `show`
   subcommand that resolves the repo root, calls `config.Load(root)`, prints
   `renderConfig(cfg)` to stdout, exits `0`, and writes nothing; register the
   command (with its `show` subcommand) on the root command.
2. Implement `renderConfig(*config.Config) (string, error)` rendering the
   `panel:` (reviewers, `approve_threshold`), `models:`, `loop:`, and top-level
   `runner:` keys alongside the pre-existing config, and annotating the
   `models`/`loop`/`runner` blocks as **"declared, not yet enforced"** so a reader
   is never misled that a configured `gate_authority` or `runner` changes behavior
   in this release. `renderConfig` stays **pure** over `*config.Config` (no panel
   scan, no fs) so it is exercised without a process (R9).
3. Caller-side note in `config show` (R8): after `renderConfig`, the `show`
   command handler scans registered panels (`panel.Scan` / `panel.ForBead` over the
   repo's review roots) and, for a registered panel whose recorded
   `expected_reviewers` differs from `cfg.PanelExpectedReviewers()`, appends
   `panel.ReviewerCountNote(recorded, cfg.PanelExpectedReviewers())` (an empty
   string appends nothing — the common no-panel case). The scan/append lives in the
   command handler, NOT in `renderConfig`; the command still writes no file.
4. Caller-side note in the complete gate (R8): in `internal/complete`'s panel
   advisory path (`panel_advisory.go` — the live `panelGate` surface reached from
   `complete.go`, which already holds the matched `panelReg` and a loaded `cfg`),
   compute `panel.ReviewerCountNote(panelReg.Panel.ExpectedReviewers, cfg.PanelExpectedReviewers())`
   and print it to the advisory writer when non-empty — **advisory only**;
   `panel.PanelGateDecision` is not touched and the gate's `Allow`/`Block` is
   unchanged (a legitimately smaller substituted quorum is surfaced, never
   false-blocked).
5. Add tests: `TestConfigShow_EmitsPanelModelsLoop` (asserts `renderConfig(DefaultConfig())`
   output contains the panel reviewers, `approve_threshold: n-1`, an empty `models`
   block, `loop` with `enabled: false`, `runner: claude-code-skills`, and the
   "declared, not yet enforced" annotation) and `TestConfigShow_ReviewerCountNoteWhenPanelDiffers`
   (a registered panel with a differing `expected_reviewers` → output contains the
   note) in `cmd/mindspec/config_test.go`; and `TestPanelAdvisory_ReviewerCountNote`
   (a recorded `expected_reviewers` differing from the config default → the advisory
   writer receives the note AND the gate decision is unchanged) in
   `internal/complete/panel_advisory_test.go`, using the existing `panelScanFn` /
   `panelTallyFn` / `panelAdvisoryOut` seams.
6. Document the read-only `config` / `config show` CLI surface **and** the
   complete-time reviewer-count advisory in `.mindspec/domains/workflow/interfaces.md`
   (doc-sync for the workflow source edits in both `cmd/mindspec` and
   `internal/complete`; a different workflow doc than Bead 3's `architecture.md`).

**Verification**
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShow_EmitsPanelModelsLoop$' | grep -q -- '--- PASS: TestConfigShow_EmitsPanelModelsLoop'`
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShow_ReviewerCountNoteWhenPanelDiffers$' | grep -q -- '--- PASS: TestConfigShow_ReviewerCountNoteWhenPanelDiffers'`
- [ ] `go test ./internal/complete -v -run 'TestPanelAdvisory_ReviewerCountNote$' | grep -q -- '--- PASS: TestPanelAdvisory_ReviewerCountNote'`
- [ ] `go test ./cmd/mindspec ./internal/complete` exits `0` (both packages green, existing tests included — proving the complete-gate `Allow`/`Block` is unchanged)
- [ ] `go build ./... && go run ./cmd/mindspec config show >/dev/null && [ -z "$(git status --porcelain)" ]` exits `0` (surfacing command mutates no file — read-only, even when a panel is scanned)
- [ ] `git show --name-only HEAD | grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync)

**Acceptance Criteria**
- [ ] `renderConfig(DefaultConfig())` output contains the `panel` reviewers, `approve_threshold: n-1`, an empty `models` block, `loop` with `enabled: false`, `runner: claude-code-skills`, and the "declared, not yet enforced" annotation on the `models`/`loop`/`runner` blocks (spec AC6)
- [ ] `mindspec config show` on a repo with no `.mindspec/config.yaml` prints the effective defaults, exits `0`, and leaves `git status` clean (spec Validation Proof — read-only)
- [ ] Both caller-side surfaces (`config show` and the complete-gate advisory) render `panel.ReviewerCountNote` — empty when the recorded `expected_reviewers` equals the config default, a non-empty advisory when they differ — with no change to any `Allow`/`Block` (spec R8 caller-side note requirement)

**Depends on**
Bead 2, Bead 3

## Provenance

| Acceptance Criterion (spec) | Verified By |
|---------------------------|-------------|
| AC1 — zero-config panel/models/loop/runner defaults (`TestLoad_ZeroConfigPanelModelsLoopDefaults`) | Bead 2 verification (PASS-line grep) |
| AC2 — resolvers `PanelExpectedReviewers` / raw `PanelApproveThresholdExpr` (`TestPanelExpectedReviewers_SumsReviewerCounts`) | Bead 2 verification (PASS-line grep) |
| AC3 — `Load` refuses the un-weakenable knobs + threshold/reviewer floors (`TestLoad_RefusesUnweakenableKnobs`) | Bead 2 verification (PASS-line grep) |
| AC4 — recorded `approve_threshold` interpreter, single home, N−1 fallbacks (`TestApproveThreshold_InterpretsRecordedExpr`) | Bead 3 verification (PASS-line grep) |
| AC5 — decision-invariance + `ReviewerCountNote` + resolved-0 Blocks pin (`TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision`) | Bead 3 verification (PASS-line grep) + `grep -q 'threshold > 0' gate.go` |
| AC6 — `config show` emits panel/models/loop/runner + not-yet-enforced annotation (`TestConfigShow_EmitsPanelModelsLoop`) | Bead 4 verification (PASS-line grep) |
| AC7 — populated config round-trips through `Load` (`TestLoad_PopulatedConfigRoundTrips`) | Bead 2 verification (PASS-line grep) |
| AC8 — ADR-0040 lands Accepted with L1–L4, `never casually up`, `artifact + CLI contract` | Bead 1 verification (content-anchor grep) |
| AC9 — tree builds + touched packages green (`go build ./...` + `go test ./internal/config/... ./internal/panel/... ./cmd/mindspec/... ./internal/complete/...`) | Beads 2/3/4 `go build ./...` + per-package `go test` (incl. `./internal/complete` in Bead 4); full `go test ./...` regression at plan time and pre-`/ms-impl-approve` (Testing Strategy) |
| R8 — caller-side `ReviewerCountNote` rendering (`config show` + complete-gate advisory), no `Allow`/`Block` change | Bead 4 verification (`TestConfigShow_ReviewerCountNoteWhenPanelDiffers` + `TestPanelAdvisory_ReviewerCountNote` PASS greps; `go test ./cmd/mindspec ./internal/complete` green proving decision unchanged) |
| Leaf invariant — `internal/panel` imports no `internal/config` | Bead 3 verification (`go list -deps … | grep -q internal/config` non-zero) |
| ADR-0037 amendment note present | Bead 1 verification (`grep -q 'approve_threshold' …ADR-0037…`) |
