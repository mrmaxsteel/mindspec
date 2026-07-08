# Changelog

All notable changes to MindSpec are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### Changed
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

### Added
- **Codex symlink protection in `mindspec setup`** — all managed-doc writes for
  claude, codex, and copilot now route through one shared `safeio`-backed
  helper; `mindspec setup codex` refuses to write through a symlinked
  `AGENTS.md` (previously only claude/copilot were protected), with refusal and
  per-agent full-content-equality tests. (Spec 107)
- **`AGENTS.md § Bead-loop guardrails (mindspec)`** — the canonical orchestrator
  fences section that `CLAUDE.md` and the `ms-*` skills reference now exists
  (the reference was previously dangling). (Spec 107)

### Changed
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

[Unreleased]: https://github.com/mrmaxsteel/mindspec/compare/v0.10.0...HEAD
[0.10.0]: https://github.com/mrmaxsteel/mindspec/compare/v0.9.0...v0.10.0
