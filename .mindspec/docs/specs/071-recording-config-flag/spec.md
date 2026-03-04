---
approved_at: "2026-03-04T18:20:10Z"
approved_by: user
status: Approved
---
# Spec 071-recording-config-flag: Recording Config Flag

## Goal

Add a config flag to enable/disable spec recording, defaulting to **off**. Recording artifacts (manifest.json, events.ndjson) cause merge conflicts during multi-branch workflows and are not needed while the core mindspec lifecycle is being stabilized.

## Background

Recording (Spec 027) is deeply wired into the lifecycle — `specinit`, `approve spec/plan`, `next`, `complete`, and `impl approve` all call into `internal/recording/`. These calls create `docs/specs/<id>/recording/` artifacts that persist across branches and cause merge conflicts when specs are worked on across sessions or branches.

At this stage, the recording artifacts add friction without providing essential value. The recording infrastructure should remain in place but be gated behind a config flag so it can be re-enabled when the AgentMind visualization pipeline is ready to consume it.

## Impacted Domains

- `internal/recording`: All functions become no-ops when recording is disabled
- `internal/specinit`: `StartRecording()` call gated by config
- `internal/approve`: Phase marker emissions gated by config
- `internal/complete`: Bead completion markers gated by config
- `cmd/mindspec/next.go`: Bead start markers gated by config
- `cmd/mindspec/record.go`: `record health` hook becomes no-op when disabled

## ADR Touchpoints

- None — this is a configuration-only change with no architectural implications.

## Requirements

1. Add a `recording.enabled` config key (boolean), defaulting to `false`
2. All recording entry points become no-ops when `recording.enabled` is false
3. `mindspec record status` shows "recording disabled" when the flag is off
4. No recording artifacts are created when the flag is off
5. Setting `recording.enabled: true` restores current behavior with no code changes needed
6. The OTLP bootstrap (`EnsureOTLP`) is also skipped when recording is disabled

## Scope

### In Scope
- Config flag in `.mindspec/config.yaml` (existing config system at `internal/config/config.go`)
- Guard checks at each recording call site
- `record status` output when disabled

### Out of Scope
- Removing the recording code itself
- Changing the recording artifact format
- AgentMind collector changes

## Non-Goals

- This spec does not remove or deprecate the recording system — it only gates it
- No changes to the AgentMind visualization pipeline

## Acceptance Criteria

- [ ] Fresh `mindspec spec create` with default config creates NO `recording/` directory
- [ ] `mindspec record status` prints "recording disabled" when flag is off
- [ ] Setting `recording.enabled: true` in config restores full recording behavior
- [ ] No merge conflicts from recording artifacts in normal multi-spec workflows
- [ ] All existing tests pass (recording tests may need config setup)

## Validation Proofs

- `mindspec spec create test-no-recording && ls docs/specs/test-no-recording/`: No `recording/` directory
- `mindspec record status`: Shows "recording disabled"
- `grep -r "recording" .mindspec/config.yaml`: Shows `enabled: false`

## Open Questions

None — config system already exists at `internal/config/config.go` with `.mindspec/config.yaml`.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-04
- **Notes**: Approved via mindspec approve spec