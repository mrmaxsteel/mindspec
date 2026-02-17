# Spec 033-security-hardening-sast-findings: Security Hardening for SAST Findings

## Goal

Harden MindSpec's local security posture by addressing the validated SAST findings in AgentMind, bench collection, and recording lifecycle paths so users can run telemetry and visualization workflows with safer defaults and bounded risk.

## Background

A deep SAST and manual security review identified several high-confidence issues:

- AgentMind and collector listeners bind to all interfaces and websocket upgrade accepts cross-origin connections.
- Replay/log/metrics handlers read request bodies without size limits.
- HTTP servers are missing timeout hardening (`ReadHeaderTimeout`, etc.).
- Recording stop paths trust manifest PID values without process identity validation.
- Spec-path flows rely on user-supplied/loaded spec IDs in multiple places and need consistent validation + containment checks.

These findings impact the default "local-first" trust model and create avoidable DoS and local process-safety risks.

## Impacted Domains

- core: AgentMind HTTP/WS server behavior, OTLP ingestion safety, timeout and body-limit guardrails.
- workflow: recording and approval lifecycle process control, spec ID/path validation in CLI flows.

## ADR Touchpoints

- [ADR-0009](../../adr/ADR-0009.md): establishes AgentMind OTLP/WS architecture that this spec hardens.
- [ADR-0011](../../adr/ADR-0011.md): defines AgentMind as unified collector, so security controls must be applied centrally there.

## Requirements

1. AgentMind and collector servers must bind to loopback by default (`127.0.0.1`) with explicit opt-in for non-loopback listening.
2. Websocket upgrades must enforce origin checks with a local-origin allowlist by default.
3. Replay/log/metrics handlers must apply strict request-size limits and return `413` for oversized payloads.
4. All HTTP servers in viz/live/collector paths must configure timeout hardening and max header size.
5. Recording stop flows must verify target PID/process identity before signaling and fail closed on mismatch.
6. All spec-driven filesystem paths in affected commands must use centralized spec ID validation and path-containment checks under project root.
7. Security regression tests must be added for origin rejection, body limits, and PID/spec-path safeguards.

## Scope

### In Scope
- `internal/viz/server.go`, `internal/viz/live.go`, `internal/bench/collector.go`
- `cmd/mindspec/viz.go` flag/config surface for listen behavior
- `internal/recording/*.go` stop/restart/manifest handling paths
- `cmd/mindspec/record.go`, `internal/approve/impl.go` stop-recording call paths
- shared validation helpers for spec ID + safe path resolution

### Out of Scope
- Websocket library migration or frontend architecture changes unrelated to security controls.
- Broad file-permission normalization across the repository (tracked separately).
- Security hardening work inside `beads/` module.

## Non-Goals

- Implementing authentication/authorization for remote multi-user deployment.
- Redesigning telemetry schema or AgentMind visualization behavior.
- Refactoring unrelated benchmark or recording features.

## Acceptance Criteria

- [ ] `agentmind serve` defaults to loopback listeners; remote exposure requires explicit user configuration.
- [ ] Cross-origin websocket upgrade attempts are rejected by default.
- [ ] Oversized POST bodies to replay/log/metrics endpoints are rejected with `413`, without unbounded memory growth.
- [ ] Viz/live/collector servers enforce timeout fields and max header size.
- [ ] Recording stop logic refuses to signal PIDs that do not match expected process identity.
- [ ] Invalid/traversal-like spec IDs are rejected before any recording path/file operation.
- [ ] New regression tests cover all above controls and pass in CI.

## Validation Proofs

- `go test ./internal/viz/... ./internal/bench/... ./internal/recording/... ./internal/approve/... ./cmd/mindspec/...`: security regression tests pass.
- `./bin/mindspec agentmind serve --ui-port 8420 --otlp-port 4318`: server starts bound to loopback by default.
- `curl -i -X POST http://127.0.0.1:8420/api/replay --data-binary @/tmp/oversized.ndjson`: returns `413 Payload Too Large`.
- `curl -i -H 'Origin: https://evil.example' -H 'Connection: Upgrade' -H 'Upgrade: websocket' http://127.0.0.1:8420/ws`: websocket upgrade is rejected.

## Open Questions

- [x] Should non-loopback listening remain possible? Yes, but only via explicit opt-in CLI/config.
- [x] How should PID verification behave when process metadata cannot be inspected? Fail closed and require explicit restart instead of signaling.

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
