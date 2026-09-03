# Workflow Domain — Overview

## What This Domain Owns

The **workflow** domain owns the spec-driven development lifecycle — the "what" layer that decides which operations should happen:

- **Mode system** — Spec/Plan/Implement/Review mode enforcement and transitions
- **Spec lifecycle** — spec creation, approval gates, status tracking
- **Plan lifecycle** — plan decomposition, bead creation, plan approval gates
- **Beads integration** — adapter layer between MindSpec and the Beads work graph
- **Phase derivation** — determining lifecycle phase from beads epic/child statuses (ADR-0023)
- **Validation gates** — human-in-the-loop approval, ADR compliance checks, doc-sync enforcement

## Boundaries

Workflow does **not** own:
- Git operations, worktree lifecycle, or filesystem operations (execution domain)
- CLI infrastructure or project health checks (core)
- Glossary parsing, context pack assembly, or provenance tracking (context-system)

Workflow **delegates** all git and worktree operations to the `Executor` interface (execution domain). Workflow packages MUST NOT import `internal/gitutil/` directly.

Workflow **uses** context packs (from context-system) to provide mode-appropriate context during planning and implementation.

## Key Packages

| Package | Purpose |
|:--------|:--------|
| `internal/approve/` | Spec, plan, and impl approval enforcement |
| `internal/complete/` | Bead close-out orchestration |
| `internal/next/` | Work selection, claiming, worktree dispatch |
| `internal/spec/` | Spec creation (worktree-first flow) |
| `internal/cleanup/` | Post-lifecycle worktree/branch cleanup |
| `internal/phase/` | Phase derivation from beads (ADR-0023) |
| `internal/resolve/` | Target spec resolution and prefix matching |
| `internal/state/` | Mode definitions, worktree path conventions |
| `internal/bead/` | Beads CLI adapter (bd commands) |
| `internal/validate/` | Spec/plan validation gates |

## Current State

Mode system is implemented. Beads is the single state store (ADR-0023) — no filesystem state files. Phase is derived from epic/child statuses. All git operations go through the Executor interface (Spec 077).

## JSONL as Artifact

`.beads/issues.jsonl` is a **build artifact**, not user authorship (ADR-0025). It is a deterministic projection of Dolt, rewritten by `bd export` and by bd's pre-commit hook after every mutation. Workflow guards must not treat its diff as user work:

- **`mindspec next`** classifies dirty paths. If the only dirty path is `.beads/issues.jsonl`, the guard runs `bd export` from the main repo root to normalize the diff against stale throttled exports, then re-checks. User-authored dirt still blocks — the guard's purpose is to protect user code, not to enforce hygiene on derived files (`internal/next/guard.go`, citing ADR-0025).
- **Executor commits** (`approve spec`, `approve plan`, `approve impl`, `complete`) refresh the JSONL via `bd export` before `git add -A`, so every mindspec-driven commit carries current beads state. In projects without a Dolt remote, this makes `git push` the off-machine durability guarantee.

Adding a future artifact (e.g. `.beads/events.jsonl`) is a one-line change to the classifier's path list; the broader artifact policy (ADR-0025) does not change.

## Cleanup notes

- **2026-09-03 (spec 128, lifecycle-verb-state-honesty):** A verb's
  reported mode can no longer disagree with, or fabricate, its real
  state. `internal/resolve` distinguishes a pre-epic DRAFT from a genuine
  bd/discovery failure (R1); `internal/instruct`/`internal/hook` render or
  fail-closed on the new `state.ModeIndeterminate` sentinel instead of
  emitting `idle.md`/a wrong mode for an undiscovered failure (R2/R2.3);
  `mindspec repair phase` gained an operator-facing backward-move guard
  and `spec approve`'s existing-epic backfill gained replay safety (R3/
  R3.6); `mindspec complete`'s content gates now run against a
  prospective snapshot BEFORE any complete-authored commit, closing the
  #186 false-panel-staleness wedge by construction (R4, OQ-1(a));
  `internal/next`'s worktree-creating claim guard was narrowed to a
  derived (today empty) entanglement set (R5, #179); bead selection
  became deterministic and every filter drop loud, with bd's stderr
  surfaced end-to-end (R6, #187/#146); and idle/review guidance now names
  the `mindspec impl adopt` exit for a frozen-contract/externally-merged
  predecessor (R7, #221). Every verb's success/guidance ordering is now
  MECHANICALLY total-classified against the registered cobra command tree
  (AC-14) rather than hoped current via an in-diff obligation. See
  `architecture.md` § Lifecycle verb state honesty and `interfaces.md`'s
  matching section for the full surface; the core domain owns the
  `mindspec_phase` write lock this all routes through, and the execution
  domain owns the prospective-snapshot git primitives R4 orchestrates.
- **2026-07-02 (spec 107 wave 1, mindspec-oexu.2):** Unified the three `internal/setup` managed-doc writers. `ensureClaudeMD`, `ensureAgentsMD`, and `ensureCopilotInstructions` now route through one shared `ensureManagedDoc` helper whose every create/update/append goes through `safeio.WriteFileNoSymlink` / `safeio.OpenAppendNoSymlink`. This closes the symlink-safety gap in `codex.go` (its managed `AGENTS.md` writes previously used bare `os.WriteFile`/`os.OpenFile`, which followed a planted symlink). The now-dead `hasManagedBlock` helper was removed (its presence check is folded into `ensureManagedDoc`), and `chainBeadsSetup`/`chainBeadsSetupCodex` were folded into one agent-parameterized `chainBeadsSetup`.
- **2026-07-02 (spec 108 wave 2, mindspec-wpjv.2):** Consolidated the workflow-owned YAML-frontmatter scanners onto the canonical `internal/frontmatter` package and unified the approval-status source of truth. In `internal/approve`, the two near-duplicate mutate-rewrite scanners (`updatePlanApproval`, `writeBeadIDsToFrontmatter`) now share one `mutateFrontmatterFile` helper built on `frontmatter.Parse` — write output is byte-identical for well-formed inputs (golden-locked). The reader fence-scans in `approve/spec.go` (`splitFrontmatter`), `approve/impl.go` (`readPlanBeadIDs`), and `internal/validate/plan.go` (`parsePlanFrontmatter`) now call `frontmatter.Parse`, dropping the redundant manual `#`-comment filtering and adopting the canonical `TrimRight("\r\n")` fence strictness (a space-padded `---` now reads as no-frontmatter everywhere — the one deliberate behavior tightening). Separately, `internal/validate/state.go`'s prose-scanning `readSpecApprovalStatus` was deleted; the two `CrossValidate` callers now read the declared spec status from the YAML frontmatter `status:` field via `validate.SpecStatusAt` (case-insensitive), so the frontmatter — not the `## Approval` prose — decides when the two disagree (the ZFC correctness fix).
- **2026-07-09 (spec 110 bead 3, mindspec-fbel.3):** Ratcheted `internal/instruct`'s `PanelStateEntry.verdict()` onto the single panel-gate decision home (ADR-0040). It used to be a SEPARATE, independently-computed reproduction of the `mindspec complete` panel gate's decision matrix (Spec 093 Reqs 11/12) — it never called `panel.PanelGateDecision`, so `instruct --panel-state`'s "gate would PASS/BLOCK" line and the real gate's outcome could drift apart. `verdict()` now builds `panel.GateFacts` and returns `mapGateAction(panel.PanelGateDecision(facts).Action)`; `gatherPanelState` sources a bead panel's tally + staleness facts through `panel.ResolveGateFacts` (the same fact-gatherer `internal/complete`'s panel gate uses), adapting the pre-existing `BranchSHAResolver` into the `panel.GateIO` seam with `Worktree` always reporting absent (instruct has never done dirty-tree detection — a read-only snapshot). A non-bead panel (final-review/PR; `BeadID` null) builds `GateFacts` with no `bead/<id>` rev-parse, keeping the staleness leg inert exactly as the old `if p.IsBead()` guard did. `instruct --panel-state` and `mindspec complete`'s panel gate now render the ONE decision function; the second copy is gone.
