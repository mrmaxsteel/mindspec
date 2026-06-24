# Spec 040-beads-config-management: Beads Configuration Management

## Goal

Ensure `mindspec init` (and `mindspec migrate apply`) writes a `.beads/config.yaml` with mindspec-appropriate defaults, so beads behavior is correctly configured for all mindspec-managed projects without manual setup.

## Background

MindSpec depends on beads for issue tracking, gates, and worktree management. Several beads behaviors need project-level configuration:

- **Custom types**: MindSpec uses `gate` type beads for spec/plan approval gates.
- **Custom statuses**: Gates use `resolved` status (distinct from `closed`).
- **Events export**: The trace/bench system (Spec 018) consumes beads event streams.
- **Sync branch**: Team projects need a consistent sync branch across clones.
- **Issue prefix**: Should match the project name for readable issue IDs.

Today, this configuration is set manually per-repo. This is error-prone — a missing `types.custom: gate` means gate creation fails silently or falls back to generic types, and missing `status.custom: resolved` means gate resolution doesn't work as expected.

The `config.yaml` file is the right layer because it's:
- Checked into version control (portable across clones)
- Human-readable and editable
- Overridable per-clone via env vars or CLI flags

Database-stored config (`bd config set`) is for per-clone preferences and shouldn't be the source of truth for project-wide settings.

## Impacted Domains

- **core**: `mindspec init` and `mindspec migrate apply` gain config-writing responsibility.
- **workflow**: beads integration becomes self-configuring rather than manual.

## ADR Touchpoints

- [ADR-0005](../../adr/ADR-0005.md): explicit state — config.yaml makes beads configuration explicit and versioned rather than implicit per-clone database state.
- [ADR-0012](../../adr/ADR-0012.md): deterministic stages — init/migrate produces a deterministic config artifact.

## Requirements

1. `mindspec init` must write `.beads/config.yaml` with mindspec defaults when initializing a new project. If beads is not yet initialized, `mindspec init` must run `beads init` first (or prompt the user).
2. The generated config must include:
   - `issue-prefix` derived from the project directory name (matching current `beads init` behavior).
   - `types.custom: "gate"` — required for spec/plan approval gates.
   - `status.custom: "resolved"` — required for gate resolution.
   - `events-export: true` — enables trace/bench event stream.
   - `sync-branch: "beads-sync"` — default sync branch for team projects.
3. If `.beads/config.yaml` already exists, `mindspec init` must merge mindspec-required keys without overwriting user customizations. Specifically: append to `types.custom` and `status.custom` if they already have values (comma-separated), set other keys only if absent.
4. `mindspec migrate apply` must perform the same config merge when onboarding an existing project that already has `.beads/`.
5. `mindspec doctor` must check that `.beads/config.yaml` contains the required mindspec keys and warn if any are missing or misconfigured.
6. Config values must also be written to the beads database via `bd config set` for keys that have dual storage (types.custom, status.custom), ensuring consistency between file and database.

## Scope

- IN: config.yaml generation/merge in init and migrate, doctor check for config health.
- OUT: beads config.yaml schema changes (that's a beads concern), new config keys beyond what's listed above.

## Acceptance Criteria

1. Running `mindspec init` in an empty directory produces a `.beads/config.yaml` containing all required keys from Requirement 2.
2. Running `mindspec init` in a directory with an existing `.beads/config.yaml` that has `types.custom: "molecule"` results in `types.custom: "molecule,gate"` — existing values are preserved.
3. `mindspec doctor` reports a warning when `types.custom` in config.yaml does not include `gate`.
4. `mindspec doctor` reports a warning when `status.custom` in config.yaml does not include `resolved`.
5. After `mindspec init`, `bd config list` shows `types.custom = gate` and `status.custom = resolved` in database config.

## Approval

- **Status**: DRAFT (backlog)
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Becomes critical when onboarding external projects to mindspec. Good candidate for the Explore Mode workflow once 041 is implemented.
