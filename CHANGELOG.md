# Changelog

All notable changes to MindSpec are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.11.0] - 2026-07-16

The review panel becomes a first-class, in-binary lifecycle: `mindspec panel
create | verify | tally` verbs over a documented artifact contract, a
deterministic workflow runner behind them, and per-gate reviewer configuration
— capped by a governance arc that makes reviewer findings un-ignorable and a
security fix that stops hostile panel artifacts from forging terminal or
agent-transcript lines.

**One deliberate gate-behavior change:** a panel carrying an unresolved
REQUEST_CHANGES no longer passes on the approve count alone (see Changed,
Spec 114). Everything else is backward-compatible: absent the new config
blocks, every gate and resolver behaves exactly as v0.10.0.

### Security
- **Panel messages can no longer forge terminal or transcript lines** — closed
  a terminal/prompt-injection hole (`mindspec-fl91`): hostile bytes (NUL,
  ESC/CSI sequences, embedded newlines forging a `recovery:` line) planted in
  agent-writable panel artifacts — `panel.json` fields (bead ID, abandon
  reason, recorded SHA, refutation slots), verdict filenames and verdict
  strings, the panel directory name, stale-worktree paths, and in-progress-bead
  commit subjects — previously reached `panel verify`/`panel tally` output,
  `mindspec complete`'s gate messages, and the SessionStart transcript block
  verbatim, where a forged line becomes a forged instruction to the
  orchestrating agent. Every attacker-influenceable field is now escaped at the
  gate's construction boundary via the new stdlib-only `internal/termsafe`
  escaper (a single implementation home: printable ASCII passes through
  unchanged, everything else renders as a single-line quoted literal), with the
  sibling render sites that bypass the gate message covered too. Clean panels
  — all-printable-ASCII values, i.e. every real one — render byte-identically,
  and no gate decision changes. (Spec 116)

### Added
- **Orchestration config substrate (ADR-0040)** — `.mindspec/config.yaml`
  gains `panel:` (reviewer mix, approve-threshold expression,
  quota-substitution policy), `models:` (phase→model tiering), and `loop:`
  (governance skeleton: gate authority, halt conditions, budget, context
  policy) blocks, plus a top-level `runner:` adapter selector — all parsed,
  defaulted, validated, and surfaced by the new read-only `mindspec config
  show` (which quote-escapes any control bytes or embedded newlines in a
  config-controlled value before it reaches stdout). `panel:` seeds a fresh
  `panel.json`'s creation-time defaults; the gate's Allow/Block decision
  stays a pure function of the recorded `panel.json`. `models:`, `loop:`,
  and `runner:` are declared and validated now; in-binary enforcement is
  deferred to later specs. (Spec 109)
- **Panel lifecycle verbs: `mindspec panel create | verify | tally`** — the
  panel lifecycle that lived in skill prose and a hand-typed `panel.json` is
  now three agent-neutral commands. `create` writes the panel directory, BRIEF
  stub, and `panel.json` in one operation, stamping the config-resolved
  reviewer count/threshold and co-bumping `round` with a freshly captured
  `reviewed_head_sha` by construction (a re-panel can never leave a stale SHA
  misdirecting reviewers). `verify` prints a read-only completeness/staleness
  report with the **same** PASS/BLOCK the complete-gate computes; `tally`
  renders the verdict table and decision from the binary — exit 0 on Allow,
  non-zero on Block with a recovery line — so `panel.PanelGateDecision` is the
  single decision home everywhere. The verdict-file/slot schema is documented
  as the agent-neutral contract other runners target. Additionally,
  `mindspec spec approve` now validates the spec with the same canonical
  parsers the downstream gates consume, so a mis-formatted
  `## Impacted Domains` entry or a dangling ADR-Touchpoint link fails cheaply
  at spec-approve instead of detonating two gates later. (Spec 110)
- **`/ms-panel` workflow panel runner** — a tracked Claude Code dynamic
  workflow (`.claude/workflows/ms-panel.js`, embedded in the plugin and
  installed by `mindspec setup` on the Claude target) runs a full review panel
  behind the panel verbs: registration via `panel create`, reviewer fan-out per
  the configured mix (codex reviewers run behind wrapper agents that persist
  codex's raw stdout as a `.codex.log` audit artifact and write the verdict
  file themselves), a semantics-preserving parse-retry ladder that can never
  launder a rendered verdict into a substitute's, deterministic `claude-sub`
  substitution on a quota wall, and the verify + tally output returned
  verbatim as one compact result. The workflow is constrained to an explicit
  command allowlist and never runs `mindspec complete`. Selected by the 109
  `runner:` key; the skills-driven launch path remains the default. (Spec 111)
- **Per-gate panel config** — the `panel:` block gains a `gates:` map
  (`spec_approve`, `plan_approve`, `bead`, `final_review`, `adhoc`) and a
  generalized reviewer entry `{model, lens, count}` — open model vocabulary
  (never an enum, so a brand-new model id loads on day one; unknown ids draw a
  warning, never an error), first-class lenses with a deterministic round-robin
  default, and legacy `{family, count}` entries still accepted. A model-level
  `substitution.substitutes` map declares the quota-wall stand-in policy with
  the slot-id-preserving `reviewer_id` convention. Gate-scoped resolvers feed
  panel creation, and `mindspec config show --gate <name> [--json]` prints a
  gate's resolved slots/threshold as a documented, additive-only machine
  contract. With `gates:` absent, behavior is identical to v0.10.0. (Spec 112)
- **Codex symlink protection in `mindspec setup`** — all managed-doc writes for
  claude, codex, and copilot now route through one shared `safeio`-backed
  helper; `mindspec setup codex` refuses to write through a symlinked
  `AGENTS.md` (previously only claude/copilot were protected), with refusal and
  per-agent full-content-equality tests. (Spec 107)
- **`AGENTS.md § Bead-loop guardrails (mindspec)`** — the canonical orchestrator
  fences section that `CLAUDE.md` and the `ms-*` skills reference now exists
  (the reference was previously dangling). (Spec 107)
- **MIT LICENSE** — the repository now ships a LICENSE file (MIT) at the repo
  root. (PR #191)
- **Docs** — new autonomy, brownfield-onboarding, and review-panels guides,
  plus a CONTRIBUTING.md. (PR #172 follow-up)

### Changed
- **The panel gate blocks on any unresolved REQUEST_CHANGES** — a reviewer's
  REQUEST_CHANGES can no longer be silently out-voted by the approve count:
  `mindspec complete` now passes only when every latest-round verdict is an
  APPROVE or an explicitly refuted REQUEST_CHANGES (the approve threshold
  remains a necessary floor on genuine approvals). The escape is a per-slot
  **audited refutation** — a `refutations` entry (slot, round, reason,
  evidence) recorded on `panel.json` — following the abandonment precedent:
  legitimate precisely because always audited, never silent. An applied
  refutation always leaves a durable `panel_refuted` audit on bead metadata —
  across retries, escape hatches, and even a later-removed panel — and never
  counts toward the approval floor, so refutation cannot buy past a
  sub-threshold panel. ADR-0037 amended. (Spec 114)
- **`mindspec impl approve` refuses to finalize an unsettled spec** — the one
  remaining lifecycle verb that could merge a bead branch un-gated now REFUSES
  (no epic close, no phase write, no merge, no push) when any closed bead under
  the spec's epic was closed via raw `bd close` without `mindspec complete`, or
  carries an unsettled durable refutation obligation — naming the bead and,
  best-effort, the unresolved REQUEST_CHANGES slot(s). Recovery converges on
  the one gate home: `mindspec complete <bead>`, which tolerates an
  already-closed bead and re-runs the full layered gate. The gate fails closed
  on infrastructure errors; a spec whose beads all went through `complete`
  finalizes exactly as today. (Spec 115)
- **YAML frontmatter is the single source of approval truth** — the `## Approval`
  prose scan is gone; spec status comes only from the frontmatter `status:`
  field (case-insensitive). Hand-rolled frontmatter fence scanners across
  approve/validate/contextpack were consolidated onto the canonical
  `internal/frontmatter.Parse`; one deliberate tightening: a space-padded
  `---` fence is now treated as no-frontmatter everywhere. (Spec 108)
- **Validation gates are cheaper** — `OWNERSHIP.yaml` manifests load once per
  gate run (was once per changed-file × domain, including per-domain `git show`
  fan-out at refs); cited ADRs parse at most once per validation run; doctor's
  dead-manifest check walks the tree once per check (was once per domain). All
  proven behavior-identical by golden-diagnostics and seam-count tests. (Spec 108)
- **`internal/trace/**` and `.golangci.yml` are now workflow-owned** — claimed in
  OWNERSHIP.yaml; the dead `trace.Event.MarshalJSON` no-op marshaler was removed
  (NDJSON output golden-proven identical) and three stale lint carve-outs
  referencing deleted code were dropped. (Spec 108)
- **`mindspec complete` is cheaper** — the children query is a single
  comma-joined `bd list --parent` call via the new exported
  `phase.FetchChildren` (was one call per status, ~5), and the immutable
  spec→epic lookup is resolved once per run (was four throwaway lookups); the
  post-close children read stays fresh. (Spec 107)
- **Dead-code sweep (−271 lines)** — ~25 confirmed-dead functions/clusters
  removed across `internal/{hook,gitutil,layout,doctor,validate,contextpack,
  next,recording,harness,panel}`, `cmd/mindspec`, and `plugins/mindspec`
  (verified unreachable even from tests via `deadcode -test`), including the
  no-op `SetUsageTemplate` in `cmd/mindspec/hook.go` and the dead flags on the
  deprecated `state set`; the hidden `spec-init` alias now reuses
  `spec create`'s `RunE` instead of a byte-identical copy. (Spec 107)

### Fixed
- **Panel-verb & workflow follow-up wave** — `panel verify`/`panel tally` now
  tell the truth about a **non-bead** panel's staleness: a spec-approve or
  final-review panel previously always reported PASS-advisory (it could never
  Block, even with zero verdicts or a REJECT on file) and rendered a malformed
  `bead/` message fragment; staleness now resolves from the panel's own
  recorded target, through the same unchanged decision function. Also in the
  wave: the `ms-panel.js` shell-safety regex rejects a bare `$` (closing
  `$HOME`-style variable-expansion survival in workflow inputs); `panel create`
  gains `--gate <name>`, stamping the recorded gate identity and that gate's
  per-gate creation-time defaults (the writer side spec 112 deferred); and the
  empty-string-model config ambiguity is reconciled (resolves to the family)
  and pinned by test. (Spec 113)
- **Protected-main `impl approve` finalize** — resolves the v0.10.0 Known Issue
  (`mindspec-wu7t`): on a branch-protected `main`, the finalize flow now lands
  the beads JSONL sync on a from-main branch when the spec branch is already
  merged, is retry-idempotent, and orders its pushes, so the committed
  `.beads/issues.jsonl` can no longer be left out of sync with the
  source-of-truth Dolt store. (PR #174)
- **`complete` close-verify reads committed state** — the post-close
  verification now reads bd's committed state (`bd show --as-of HEAD`) instead
  of the uncommitted working set, so a close that never landed in a Dolt commit
  can no longer pass verification. (PR #173)
- **Headless spec-grill guard** — the grill auto-chained by `ms-spec-create` no
  longer stalls a headless/non-interactive run: the skill gains a three-mode
  disposition (interactive grill; self-answer under an explicit non-interactive
  instruction; a blocking defer with an audit marker as the backstop). (PR #176)

## [0.10.0] - 2026-06-24

A large release headlined by the **flattened `.mindspec/` layout**, plus ten
specs' worth of lifecycle, gate, and CLI work merged since v0.9.0.

**Upgrading is non-breaking:** the binary alone changes nothing on disk — a
multi-tier resolver (flat → canonical → legacy) reads every layout, first-exists
-wins — and the new lifecycle gates engage only within the mindspec spec workflow
(and only once you register a panel), so existing layouts and non-panel usage
keep working as before. Flattening an existing project is opt-in via
`mindspec migrate layout` (clean tree + no unmerged pre-flatten branch;
`--allow-branch`/`--force` scope the precondition; `--abort` rolls back a
pre-publish run; forward-only after publish; run `mindspec doctor` afterward to
confirm links).

### Added
- **Flattened `.mindspec/` layout** — `specs/`, `adr/`, `domains/`, `core/`, and
  `context-map.md` are now top-level children of `.mindspec/`; panel reviews are
  co-located under `<spec-dir>/reviews/`; repo dogfood docs moved to a top-level
  `project-docs/` tree.
- **`mindspec migrate layout`** — an opt-in transactional mover (two commits per
  move: a pure `git mv`, then a link-rewrite) with `--abort` pre-publish
  rollback, forward-only publish, a root-doc 404 link-check, and
  `--allow-branch`/`--force` precondition escapes.
- **`ms-spec-grill` skill** — an agent skill that interrogates a draft spec one
  question at a time, grounded in the live domains, ADRs, and code, to drive out
  vague language, synonym-dodges, non-falsifiable claims, cross-requirement
  contradictions, and unprobed edge cases; backed by a tracked `bench/grill/`
  eval and auto-chained by `ms-spec-create`.
- **`mindspec next <bead-id>`** — claim a named bead (full or short-form ID)
  instead of the first ready item.
- **`mindspec release <bead>`** — cleanly reverse a wrong claim (remove the
  worktree, then re-open the bead); refuses a dirty worktree without `--force`.
- **`mindspec version`** subcommand, matching `--version` byte-for-byte.
- **`doctor` layout detection**, plus bd schema-drift and
  multiple-`bd`-on-PATH checks.
- **`spec-orchestrator` agent** (`.claude/agents/`) for multi-bead / multi-spec
  autonomous runs.
- **`MS_HARNESS_MODEL`** override for the LLM test harness.

### Changed
- **Panel gate enforced inside `mindspec complete`** — the ADR-0037 review-panel
  gate now runs inside `complete <id>`, keyed off the bead ID, so it covers every
  invocation form (wrapped, quoted, aliased); the PreToolUse hook remains only as
  a defense-in-depth backstop. A bead with no registered panel still completes —
  the gate fails closed only once a panel exists.
- **Ownership/ADR gates resolve domains by file path** — `## Impacted Domains`
  entries that are file paths now resolve to their owning domain by glob-matching
  the `OWNERSHIP.yaml` manifests, at one shared source consumed by the bead-time
  divergence gate and the plan-time coverage/citation gates; a correct manifest is
  no longer rejected or forced onto `--override-adr`. Zero/multi-owner cases still
  error clearly.
- **`mindspec spec create`** branches from `origin/<default-branch>` after a fetch
  (default branch detected, not hardcoded), falling back to local `HEAD` with a
  warning when offline.
- **`plan approve`** auto-fills a missing `version` to `"1"` instead of hard-
  blocking (`status`/`spec_id` stay required).
- Claim failures now surface bd's real stderr (so a stale-binary schema error is
  legible).

### Fixed
- **`complete` close-verify** — after `bd close`, `complete` forces a Dolt commit
  and verifies the bead persisted as closed; it can no longer print `closed` +
  exit 0 on a close that didn't land, and keeps the worktree on failure.
- **`bd close` bypass guard** — a lifecycle floor detects and blocks using
  `bd close` to skip the panel/merge gates, steering callers to `mindspec complete`.
- **Fresh-repo merge safety** — bootstrap provisions the beads JSONL merge driver
  (portable, cross-worktree) from commit 0.
- **ADR numbering & worktree** — `adr.NextID` parses slugged `ADR-NNNN-slug.md`
  filenames (no colliding low IDs); `adr create` writes into the invoking
  worktree.
- **Git argument safety** — `internal/gitutil` rejects hostile `-`-prefixed
  ref/branch operands at its boundary and fast-fails with `GIT_TERMINAL_PROMPT=0`
  so a slow/auth-prompting origin can't hang `spec create`.
- **Phase cache** now counts `blocked` and custom-status children, matching the
  state-advance path.
- **`migrate layout` hardening** — precondition scoping (unrelated stale branches
  no longer false-block), a wider repo-root 404 link-check, and
  resume-before-clean-tree-precondition so a crashed run can recover.
- **Harness** — a Sonnet full-suite audit fixed three LLM-test failures and
  trimmed a slow scenario's turn count.

### Removed
- The `.mindspec/docs/` wrapper directory, the vestigial `glossary.md` and
  `policies.yml`, and the retired PreToolUse heuristic complete-matcher (now
  redundant with the in-binary gate) and harness prose path-scraper.

### Governance
- **ADR-0039** (Flat `.mindspec/` Layout v2) — **Accepted**.
- **ADR-0037** (panel gate) amended — in-binary enforcement is authoritative and
  the review location moved to `<spec-dir>/reviews/`.
- **ADR-0032** (ADR semantic gates) amended — path-like Impacted-Domains entries
  are normalized to their owning domain rather than rejected.
- **DOCS-LAYOUT.md** updated to the flat layout.

### Known issues
- On a branch-protected `main`, `mindspec impl approve` can momentarily leave the
  committed `.beads/issues.jsonl` out of sync with the source-of-truth Dolt store,
  because the lifecycle's finalize commit cannot land directly on protected
  `main`. A one-time manual `.beads` sync resolves it (the post-merge hook is then
  idempotent). Tracked as `mindspec-wu7t`. Normal feature work and non-protected
  repositories are unaffected.

## [0.9.0] - 2026-06-13 and earlier

Release notes for v0.9.0 and prior are on the
[GitHub Releases](https://github.com/mrmaxsteel/mindspec/releases) page.

[Unreleased]: https://github.com/mrmaxsteel/mindspec/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/mrmaxsteel/mindspec/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/mrmaxsteel/mindspec/compare/v0.9.0...v0.10.0
