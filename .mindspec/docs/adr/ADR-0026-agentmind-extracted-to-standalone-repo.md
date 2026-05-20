# ADR-0026: AgentMind Extracted to Standalone Repo

- **Date**: 2026-05-19
- **Status**: Accepted
- **Domain(s)**: observability, telemetry, recording, bench, extraction
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0011](ADR-0011.md) (one-way `mindspec → agentmind` dependency via OTLP/HTTP:4318)

---

## Context

[ADR-0011](ADR-0011.md) established the one-way `mindspec → agentmind`
dependency boundary, with OTLP/HTTP:4318 as the inbound IPC channel. At the
time, agentmind code lived inside mindspec under `internal/agentmind/` and
`internal/viz/`. Spec 083 (`agentmind-extraction-v2`) is the physical
realization of ADR-0011: agentmind moves to its own repo
(`github.com/mrmaxsteel/agentmind`), and mindspec consumes it as an external
Go module via two narrow surfaces: `client` and `wire`.

## Decision

- **Repo location:** AgentMind lives at `github.com/mrmaxsteel/agentmind`.
- **Import surface from mindspec:** `mindspec` imports only
  `github.com/mrmaxsteel/agentmind/client` (process lifecycle: `AutoStart`,
  `RunStandalone`, `ReadEvents`, typed sentinel `ErrBinaryNotFound`) and
  `github.com/mrmaxsteel/agentmind/wire` (canonical NDJSON
  encoder/decoder, normalization tool, types).
- **IPC channels:** OTLP/HTTP:4318 is the inbound channel (mindspec writers
  → agentmind ingestor); NDJSON-over-stdout is the outbound channel
  (agentmind subprocess → mindspec readers via `client.ReadEvents` on the
  subprocess stdout pipe — never via file-tailing the `--output` path).
- **Per-class degradation contract** when the agentmind binary is absent:
  - *telemetry-as-output* (`mindspec record start`): non-zero exit.
  - *batch* (`mindspec bench run`, `mindspec agentmind setup`): exit 0 with
    exactly one centralized `sync.Once` warn line emitted from
    `agentmind/client`.
  - *interactive* (`mindspec viz`, `agentmind serve`, `agentmind replay`):
    non-zero exit.
- **Binary-not-found detection:** `errors.Is(err, client.ErrBinaryNotFound)`
  at every call site. Substring-matching on error message text is
  prohibited and enforced by a unit-test assertion at each rewired call
  site.

## Deferred decisions

- **UI-port discovery:** port 8420 stays hardcoded for v1.0.0. A follow-up
  spec may add discovery once we have evidence of port conflicts in the
  field.
- **Version-skew handling:** no `client.Probe()` / capability-negotiation
  is shipped at v1.0.0. mindspec pins a specific agentmind tag in
  `go.mod`; any incompatibility surfaces as a build error, not a runtime
  surprise.
- **`mindspec agentmind setup` ownership:** stays in mindspec for now. A
  follow-up spec may move it into agentmind once a first-party installer
  is shipped.
- **First-party `mindspec install agentmind` subcommand:** deferred. v1.0.0
  documents a manual `curl` + `sha256sum` install path in the README.

## Rollback procedure

1. `git revert <mindspec-merge-sha>` on the spec 083 merge commit.
2. Drop the `require github.com/mrmaxsteel/agentmind` line from `go.mod`.
3. `go mod tidy`.
4. Verify `go build ./cmd/mindspec && go test -short ./...` passes.

After rollback, mindspec returns to the pre-extraction state. agentmind
remains live at its own repo but is no longer consumed by mindspec.

## Consequences

- **Positive:** clean import boundary; mindspec binary shrinks
  measurably (Phase 5 measurement: 14,323,266 → 10,750,866 bytes,
  -3,572,400 bytes = -24.94% on darwin-arm64); the one-way dep from
  ADR-0011 is physically enforced by the module boundary; agentmind
  ships its own releases on its own cadence.
- **Negative:** users must install the `agentmind` binary out-of-band
  (until a follow-up installer ships); version-skew between mindspec and
  agentmind is now a real concern; CI must clone the sibling repo for
  cross-repo gates (Tests E/F continuity).
- **Mitigations:** documented manual install path with checksum
  verification; `go.mod` pin gives reproducible builds; CI Test E
  shallow-clone step mirrors the agentmind-side gate from mindspec's
  side.

## As-shipped surface (Phase 5)

After Bead 5 (`mindspec-6oxg.6`) merged:

- `mindspec/internal/agentmind/` — **deleted entirely** (8 files).
  Lockfile contract relocated to `internal/recording/lockfile.go`
  (mindspec owns the lockfile; the standalone agentmind binary
  honors it). AutoStart / IsRunning / Token / Probe / findBinary
  are now owned by `github.com/mrmaxsteel/agentmind/client`.
- `mindspec/internal/viz/` — **deleted entirely** (web/{app.js,
  index.html, style.css} + server.go + hub.go + graph.go +
  normalize.go + live.go + replay.go + codex_fallback.go + run.go
  + tests). AgentMind owns the viz code now.
- `mindspec/internal/bench/collector.go` — **OTLP parser deleted**.
  The Collector type, handleLogs/handleMetrics HTTP handlers,
  extractLogEvents/extractMetricEvents parsers, and helpers
  (flattenAttributes, parseOTLPTimestamp) are gone. The file is
  reduced to type-alias re-exports of `wire.CollectedEvent`,
  `wire.OTLPValue`, `wire.OTLPKeyValue` for the surviving
  in-mindspec callers.
- `mindspec bench collect` and `mindspec record collect` subcommands
  — **removed**. They were deprecated aliases that already told
  users to use `mindspec agentmind serve --output <path>`.
- `cmd/mindspec/viz.go` `agentmind setup codex --session` path
  rewired to re-exec `agentmind setup codex --session …` via
  `client.RunStandalone` (interactive-class degradation: exits
  non-zero if binary absent).

Acceptance criteria green after this bead:

- `find internal/agentmind`  → no results.
- `find internal/viz`        → no results.
- `grep -rn 'http.HandleFunc.*"/v1/logs"' --include="*.go" .` →
  no results.
- Test E (no-circular-discovery, against the agentmind sibling):
  no matches.
- Test F (import-boundary): mindspec's only agentmind deps are
  `client` and `wire`.

## Notes

This ADR was drafted in Bead 5 alongside the Phase 5 deletion so its
text mirrors what shipped, and is finalized in Bead 7 once Bead 6
(release) merges. The plan's `adr_citations` frontmatter lists ADR-0026
from plan-approval time onward; the ADR's `Status` is Accepted now that
the deletion has shipped.
