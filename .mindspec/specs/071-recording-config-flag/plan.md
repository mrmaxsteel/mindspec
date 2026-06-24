---
approved_at: "2026-03-04T18:21:40Z"
approved_by: user
spec_id: 071-recording-config-flag
status: Approved
version: "1"
---
# Plan: 071-recording-config-flag

## ADR Fitness

No ADRs are relevant to this work.

## Testing Strategy

Unit tests verify config default and YAML parsing. Manual validation via `mindspec spec create` confirms no recording artifacts.

## Bead 1: Add recording config flag and guard all entry points

**Steps**
1. Add `Recording` struct (`Enabled bool`) to `Config` in `internal/config/config.go`, default `false`
2. Add `IsEnabled(root) bool` to `internal/recording/` that checks config
3. Guard `StartRecording`, `StopRecording`, `UpdatePhase`, `AddBeadToPhase` in `recording.go`
4. Guard `EmitMarker` in `markers.go`
5. Guard `HealthCheck`, `RestartIfDead` in `health.go`
6. Guard `EnsureOTLP` in `bootstrap.go`
7. Update `recordStatusCmd` in `cmd/mindspec/record.go` to show "recording disabled"
8. Add config tests in `internal/config/config_test.go`

**Verification**
- [ ] `make test` passes
- [ ] `make build` succeeds

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| Default config creates no recording/ dir | Bead 1 step 3 (StartRecording guard) |
| record status shows "recording disabled" | Bead 1 step 7 |
| enabled: true restores behavior | Bead 1 step 2 (IsEnabled reads config) |
| Existing tests pass | Bead 1 verification |
