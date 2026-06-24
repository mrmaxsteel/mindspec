---
approved_at: "2026-04-21T22:13:34Z"
approved_by: user
status: Approved
---
# Spec 082-beads-artifact-handling: Treat .beads/issues.jsonl as a build artifact, not user work

## Goal

Stop the beads JSONL export from blocking mindspec's workflow guards, without sacrificing durability in projects that lack a Dolt remote. Make the mindspec ↔ beads integration work correctly out of the box: a fresh mindspec project, a new worktree, or a session that started with an ad-hoc `bd create` should all cooperate with `mindspec next` / `approve` / `complete` without manual stash/branch dances.

## Background

`.beads/issues.jsonl` is a deterministic projection of Dolt, rewritten by `bd` after every mutation (`export.auto=true` default, throttled to 60s) and by the pre-commit hook (`cmd/bd/hooks.go:exportJSONLForCommit`). `bd.FindBeadsDir` follows the worktree redirect file, so every export — whether initiated from main or from a bead worktree — lands at `<main>/.beads/issues.jsonl`. That file is tracked in git.

The friction pattern has now been reproduced in at least two mindspec-using projects (this repo and Lola):

1. Agent or user runs `bd create` (or any mutation) from any worktree.
2. bd auto-exports → `<main>/.beads/issues.jsonl` is dirty.
3. Next session on main: `mindspec next` refuses to claim work because the workspace is dirty (guard in `internal/next/`).
4. Workaround dance: stash → `mindspec next` → pop. In projects that accumulated a stray root-level `issues.jsonl` from the v1.0.2 GIT_DIR-pollution bug (upstream GH#3311, fix d0f0ad6f, not yet in a released tag), the pop also conflicts because the bead branch has a different file at a different path. The user ends up hand-authoring a `chore/` branch for something that is not user intent.

Three conclusions inform this spec:

- **The JSONL is not user intent.** It is a build artifact produced deterministically from Dolt. Treating its diff as "uncommitted work to protect" is a category error — the data is recoverable at any time via `bd export`.
- **Silencing auto-export is not safe in projects without a Dolt remote.** Without bd federation, the JSONL committed to git is the only off-machine copy of issue state. Setting `export.auto=false` widens a data-loss window and also short-circuits the pre-commit export path (`exportJSONLForCommit` gates on `export.auto` too). We therefore cannot solve this at the beads config layer alone — the fix belongs in mindspec.
- **Fresh mindspec projects inherit none of the hard-won config.** The `.beads/config.yaml` keys that mindspec's runtime relies on — `types.custom: gate`, `status.custom: resolved`, `issue-prefix` — are hand-authored in this repo today. A user running `mindspec init` followed by `bd init` gets bd's defaults, which do not match what mindspec expects. `mindspec setup claude/copilot` chains `bd setup claude/codex` but never edits `.beads/config.yaml`.

## Impacted Domains

- **workflow**: dirty-tree guard policy in `mindspec next`; artifact-vs-user-work classification
- **execution**: executor commits should carry a fresh JSONL matching Dolt at commit time
- **bootstrap**: `mindspec init`, `mindspec setup`, `mindspec doctor` should establish and enforce the mindspec-required beads config

## ADR Touchpoints

- [ADR-0023](../../adr/ADR-0023.md) (Beads as single state authority): reinforces this spec — Dolt remains authoritative, JSONL is a projection whose purpose is durability and grep-ability
- [ADR-0006](../../adr/ADR-0006.md) (Protected main + PR-based merging): the new dirty-tree policy must continue to respect main protection; auto-handling the JSONL does not open a back door to direct main writes
- **Candidate new ADR**: "JSONL as build artifact" — record the explicit stance that `.beads/issues.jsonl` is co-managed by mindspec and beads, not user-edited. Cross-cutting enough to deserve its own ADR under the ADR-0023 lineage.

## Requirements

### R1: Treat `.beads/issues.jsonl` as a build artifact in the dirty-tree guard

The dirty-tree guard in `mindspec next` (currently in `internal/next/`) must distinguish build-artifact dirt from user-work dirt.

1. Snapshot the set of dirty paths (staged + unstaged) via `git status --porcelain`.
2. If `.beads/issues.jsonl` appears in the dirty set:
   - Run `bd export -o .beads/issues.jsonl` to normalize the diff to current Dolt state (defensive against stale throttled exports).
   - Re-check `git status`. The JSONL may still be dirty (legitimate issue changes) or may have gone back to clean (throttled auto-export was writing stale content).
3. If the **only** remaining dirty path is `.beads/issues.jsonl`, proceed with the bead claim. The guard's job is to prevent losing user code/docs — it must not block on auto-generated files.
4. If other paths are also dirty, preserve current behavior (abort with "uncommitted changes"). The guard's error message should explicitly note that `.beads/issues.jsonl` is auto-handled and need not be stashed.
5. The JSONL diff must not be silently discarded. When `mindspec next` creates the bead worktree, the bead worktree reads through main's `.beads/` via the redirect file, so the new issues are visible inside the worktree without an explicit carry. The first `mindspec complete` on the bead branch will commit the JSONL as part of the bead's work via R2.

### R2: Run `bd export` at executor commit points

All executor-driven commits must include a fresh JSONL reflecting Dolt at commit time. In `internal/executor/mindspec_executor.go`:

1. `CommitAll` (or whichever helper wraps `git add -A && git commit`) must, before staging, run `bd export -o <main>/.beads/issues.jsonl`. Use `bd.FindBeadsDir()`-equivalent path resolution so the export always writes to the main repo's `.beads/`, regardless of which worktree the executor is running in.
2. If `bd export` fails, surface the error — do not silently fall through to staging stale state. Hook failures should be treated like any other pre-commit hook failure.
3. Applies to each executor path that ends in a git commit: `approve spec`, `approve plan`, `approve impl`, and `complete`.
4. The call must be idempotent and cheap — `bd export` against unchanged Dolt state is a byte-identical rewrite, so re-running it is safe.

This also closes the durability gap opened by R1: every mindspec-driven commit that lands on a PR-merged branch carries the point-in-time JSONL, making `git push` the off-machine durability guarantee even in projects without a Dolt remote.

### R3: Bootstrap mindspec-required beads config

Introduce `ensureBeadsConfig(root string) (*ConfigResult, error)` in `internal/bead/config.go` (or equivalent). It must:

1. Read `.beads/config.yaml` if present; create an empty one with a brief header comment if not.
2. Idempotently merge the mindspec-required keys:
   - `issue-prefix: "<project>"` — default to the repo directory name; do **not** overwrite an existing value
   - `types.custom: "gate"`
   - `status.custom: "resolved"`
   - `export.git-add: false` (belt-and-braces until bd v1.0.3+ ships the GH#3311 fix)
3. Preserve any user-authored keys and comments verbatim — structural-preserving YAML edit (e.g. `yaml.v3` Node-based mutation), not a full rewrite via `yaml.Marshal`.
4. Return a `ConfigResult` describing what was added, what was already correct, and any user-authored values left alone. Callers use this to print a summary.

Wire the helper into three call sites:

- `cmd/mindspec/init.go` → extend `bootstrap.Run` so a fresh `mindspec init` leaves `.beads/config.yaml` mindspec-ready (or leaves an informative no-op if `bd init` has not yet run).
- `cmd/mindspec/setup.go` (claude and copilot variants) → call after `chainBeadsSetup` so projects that ran `bd init` before `mindspec setup` still get the config patched.
- `cmd/mindspec/doctor.go` → in read-only mode, detect drift. With `--fix`, call `ensureBeadsConfig`.

**Intentional non-change:** `export.auto` is left at bd's default (`true`). Leaving auto-export on preserves durability — the JSONL stays fresh, the pre-commit hook keeps working, and R1 + R2 together make the resulting main-worktree dirt a non-blocker. Projects that want to disable auto-export for independent reasons can do so manually; mindspec does not mandate it either way.

### R4: Doctor checks for beads ↔ mindspec integration health

In `internal/doctor/beads.go`, add these checks. All are warnings (non-fatal) unless flagged otherwise.

1. **Config drift**: mindspec-required keys from R3 are missing or have unexpected values. Offer remediation in the message.
2. **Stray root-level `issues.jsonl`**: if `<root>/issues.jsonl` is tracked on any branch reachable from `HEAD`, warn. This is v1.0.2 GIT_DIR-pollution leakage. Suggest `git rm --cached issues.jsonl` on the affected branches. (Cleanup across branches is out of scope for this spec — see Out of Scope.)
3. **Durability risk**: if bd reports no configured Dolt remote AND `export.auto: false`, warn that ad-hoc `bd create` sessions outside mindspec's approve/complete flows will not refresh the JSONL — recommend either configuring a Dolt remote or reverting `export.auto` to `true`.
4. **bd version floor**: if `bd --version` reports < v1.0.2, warn. Earlier versions lack redirect fixes that mindspec's worktree model relies on.

## Scope

### In Scope

- `internal/next/` (or wherever the dirty-tree guard lives) — refactor to distinguish artifact dirt from user dirt
- `internal/executor/mindspec_executor.go` — insert `bd export` call before `git add -A` in commit helpers
- `internal/bead/config.go` (new file) — `ensureBeadsConfig` helper with structural YAML merge
- `cmd/mindspec/init.go`, `cmd/mindspec/setup.go`, `cmd/mindspec/doctor.go` — wire `ensureBeadsConfig` and new doctor checks
- `internal/doctor/beads.go` — four new checks (R4)
- Tests for all of the above: unit tests for each helper, plus at least one LLM harness scenario that exercises the dirty-tree guard path end-to-end
- Domain doc update: `workflow/` or `execution/` overview — add a short section describing the JSONL-as-artifact stance and which commands refresh it
- New ADR: "JSONL as build artifact" stance, if the approval conversation decides it is worth capturing

### Out of Scope

- Configuring Dolt remotes (projects manage that independently of mindspec)
- Cleaning up pre-existing stray root-level `issues.jsonl` from historical bead/spec branches in user projects — a doctor `--fix` could handle this in a later spec
- Contributing changes upstream to beads (GH#3311 is already fixed there; no further upstream work required for this spec)
- Changes to the Executor interface signatures
- Changes to beads' redirect mechanism or auto-export behavior
- Changes to bd's config defaults

## Non-Goals

- Hiding `.beads/issues.jsonl` diffs from `git status` / `git diff` — the file remains visible and inspectable at all times
- Force-flipping `export.auto` or `export.git-add` in user config beyond what R3 mandates
- Replacing Dolt as the source of truth for beads state
- Supporting beads versions older than v1.0.2

## Acceptance Criteria

- [ ] `mindspec next` succeeds when the only dirty path is `.beads/issues.jsonl`, and the new bead worktree sees an up-to-date JSONL (verified by `bd list` inside the worktree returning the newly-filed issue).
- [ ] `mindspec next` still refuses to proceed when any non-`.beads/issues.jsonl` path is dirty (user-work protection preserved).
- [ ] After `mindspec approve spec`, `approve plan`, `complete`, and `approve impl`, `git show --stat HEAD` includes `.beads/issues.jsonl` whenever Dolt state changed during that operation; the committed JSONL is byte-identical to a fresh `bd export` run at commit time.
- [ ] A fresh `mindspec init` in an empty git repo, followed by `bd init`, produces a `.beads/config.yaml` containing `types.custom: gate`, `status.custom: resolved`, `export.git-add: false`, and an `issue-prefix` derived from the directory name. Running `mindspec init` a second time is a no-op on the config file.
- [ ] `mindspec doctor` on a repo whose `.beads/config.yaml` is missing mindspec keys reports each missing key; `mindspec doctor --fix` patches them; re-running reports clean.
- [ ] `mindspec doctor` on a repo with a tracked `issues.jsonl` at the repo root warns and suggests remediation.
- [ ] An LLM harness scenario (new or extended) exercises: issue filed via `bd create` from main → `mindspec next` proceeds without stash or branch intervention → the resulting bead's first commit carries the filed issue's JSONL line.
- [ ] No regression on existing harness scenarios that exercise `approve` / `complete` flows.
- [ ] Domain doc (`workflow/` or `execution/`) contains a short section explaining the JSONL-as-artifact policy and which commands refresh it.

## Validation Proofs

- `cd <scratch> && git init && mindspec init && bd init && cat .beads/config.yaml`: Expected to contain `types.custom: gate`, `status.custom: resolved`, `export.git-add: false`, and a derived `issue-prefix`.
- From a clean main worktree: `bd create --title="spec-082 smoke" --type=task` then `mindspec next --spec=<active>`: Expected to claim the next ready bead without error, without stash, without a chore branch.
- Inside the new bead worktree: `bd list | grep "spec-082 smoke"`: Expected to show the issue.
- After `mindspec complete <bead>`: `git show --stat HEAD | grep .beads/issues.jsonl`: Expected to show the JSONL in the commit.
- `mindspec doctor` on a scratch repo with bd-default config: Expected to report each missing mindspec key.

## Open Questions

- [x] **Handling of existing user-authored `export.git-add: true`**: `ensureBeadsConfig` must **not** silently overwrite it. If the file has a user-authored value different from the mindspec-required value, emit a warning naming the key and current value, leave it alone, and require an explicit `--force` flag (on `mindspec doctor --fix` and on the helper's API) to replace it. Rationale: respect user authorship; `export.git-add: false` is belt-and-braces, not load-bearing.
- [x] **Bead worktree JSONL carry vs. redirect**: the redirect suffices for reads — `bd` calls inside a bead worktree transparently see main's `.beads/issues.jsonl` via `FollowRedirect`. No explicit copy-forward is required. Plan must include a harness test that verifies: file an issue from main, run `mindspec next`, inside the new bead worktree `bd list` returns the newly-filed issue. If that test fails, the plan introduces an explicit carry; if it passes, the redirect is load-bearing and documented as such.
- [x] **Dedicated ADR for "JSONL as build artifact"**: yes. The stance cuts across workflow, bootstrap, doctor, and executor layers, and future specs will want to reference it by number rather than by prose. The plan will draft a short ADR under the ADR-0023 lineage and include its creation as a discrete step.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-04-21
- **Notes**: Approved via mindspec approve spec