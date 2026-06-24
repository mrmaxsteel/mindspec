# Spec 000: Repo + Beads Hygiene

## Goal

Ensure Beads' durable state (issues graph, config) is safely committed while runtime artifacts (sockets, locks, tmp) never leak into the repo or build contexts, unblocking all downstream dogfooding.

## Background

Beads is central to MindSpec's execution model (ADR-0002). However, Beads produces runtime artifacts (`bd.sock`, lock files, tmp directories) alongside durable state (`issues.jsonl`, config). When runtime artifacts leak into git or packaging contexts, they cause environment issues — broken builds, stale sockets, and confusing diffs — that block all other work.

This is the P0 hygiene spec because every subsequent spec (001-skeleton onward) depends on a clean Beads integration.

## Impacted Domains

- workflow: Beads is the execution tracking substrate for the mode system
- core: `doctor` must validate Beads hygiene

## ADR Touchpoints

- [ADR-0002](../../adr/ADR-0002.md): Defines Beads as passive execution substrate; Section C ("Active Workset Discipline") requires hygiene rules; Section A defines Beads responsibility boundaries

## Requirements

1. **Beads initialization**: Initialize Beads in the repo so `.beads/` exists with its durable state files
2. **Selective `.beads/` tracking**: `.gitignore` rules that commit durable state and ignore runtime artifacts
3. **Packaging excludes**: Ensure runtime Beads files are excluded from packaging/build contexts (sdist, wheel, any future Docker context)
4. **Doctor check for Beads hygiene**: `mindspec doctor` validates durable Beads state is present and no runtime artifacts are tracked in git

## Scope

### In Scope

- `beads init` (or equivalent) to bootstrap `.beads/` in the repo
- `.gitignore` updates for `.beads/` (selective rules for durable vs runtime)
- `MANIFEST.in` or `pyproject.toml` packaging excludes for `.beads/` runtime artifacts
- `src/mindspec/doctor.py` or equivalent: new Beads hygiene checks in doctor command
- Documentation of Beads file conventions (which files are durable, which are runtime)

### Out of Scope

- Beads CLI integration or wrapper commands
- Beads issue creation/management tooling
- Worktree management automation
- Modifying Beads itself

## Non-Goals

- Changing how Beads stores its data internally
- Automating Beads cleanup/compaction (future spec)
- Multi-repo Beads support

## Acceptance Criteria

- [ ] Beads is initialized in the repo: `.beads/` exists with durable state files committed
- [ ] `.gitignore` includes selective rules for `.beads/`: durable files (e.g., `issues.jsonl`, config) are trackable; runtime files (`bd.sock`, `*.lock`, `*.pid`, `tmp/`) are ignored
- [ ] `git status` shows no Beads runtime artifacts as untracked or modified after a Beads session
- [ ] Packaging (`python -m build` or equivalent) excludes `.beads/` runtime artifacts from sdist and wheel
- [ ] `mindspec doctor` reports whether `.beads/` directory exists with expected durable state
- [ ] `mindspec doctor` warns if any Beads runtime artifacts are tracked by git (checked via `git ls-files`)
- [ ] `mindspec doctor` exits non-zero if runtime artifacts are tracked

## Validation Proofs

- `git status` after running Beads: no runtime artifacts shown
- `python -m mindspec doctor`: reports Beads hygiene status (durable state present, no runtime leaks)
- `python -m build --sdist`: inspect tarball to confirm no `bd.sock`, lock, or tmp files included

## Open Questions

- (none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-11
- **Notes**: Approved via /spec-approve workflow
