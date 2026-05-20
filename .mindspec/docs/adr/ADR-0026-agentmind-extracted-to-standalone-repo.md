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

- **Positive:** clean import boundary; mindspec binary shrinks; the
  one-way dep from ADR-0011 is physically enforced by the module
  boundary; agentmind ships its own releases on its own cadence.
- **Negative:** users must install the `agentmind` binary out-of-band
  (until a follow-up installer ships); version-skew between mindspec and
  agentmind is now a real concern; CI must clone the sibling repo for
  cross-repo gates (Tests E/F continuity).
- **Mitigations:** documented manual install path with checksum
  verification; `go.mod` pin gives reproducible builds; CI Test E
  shallow-clone step mirrors the agentmind-side gate from mindspec's
  side.

## Notes

This ADR is drafted alongside the Phase 5 deletion bead so its text
mirrors what shipped, and finalized in the Phase 6 release bead. The
plan's `adr_citations` frontmatter lists ADR-0026 from plan-approval
time onward; the ADR's `Status` flips from Draft to Accepted only after
the deletion has merged.
