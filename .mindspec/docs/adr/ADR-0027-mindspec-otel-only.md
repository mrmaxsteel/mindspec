# ADR-0027: MindSpec is OTEL-Only

- **Date**: 2026-05-20
- **Status**: Proposed
- **Domain(s)**: observability, telemetry, recording, extraction
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0011](ADR-0011.md), [ADR-0026](ADR-0026-agentmind-extracted-to-standalone-repo.md)

---

## Status

Proposed (drafted at plan-approval time for spec 084-mindspec-otel-only;
finalized in Bead 4 with as-shipped content).

## Context

After spec 083 extracted the OTLP receiver out of mindspec into the standalone
`github.com/mrmaxsteel/agentmind` repository, mindspec still retained several
observability-coupling surfaces: the `mindspec agentmind serve|replay|viz`
cobra subtree, the `client.AutoStart`/`RunStandalone`/`ReadEvents` callers in
`internal/recording/collector.go`, `internal/bench/runner.go`, and
`cmd/mindspec/viz.go`, the `internal/bench/` subsystem itself, and the
`github.com/mrmaxsteel/agentmind/{client,wire}` Go module dependency in
`go.mod`.

The user's stated vision after the spec 083 merge: *"I should be able to
point mindspec at an OTEL collector and that's it — no other interaction
between mindspec and agentmind."*

This is the architectural decision to realize that vision.

## Decision

MindSpec's only relationship to observability is **emitting OTEL
configuration to downstream tools** (`claude`, `codex`) so they emit OTEL
telemetry to a user-supplied endpoint. Specifically:

1. **No subprocess management of any collector.** No `client.AutoStart`,
   no `RunStandalone`, no `ReadEvents`. Removed entirely.
2. **No NDJSON readers.** Mindspec never deserializes
   `wire.CollectedEvent`. The agentmind/wire and agentmind/client Go module
   dependencies are dropped.
3. **No `mindspec agentmind` cobra subtree.** No `serve`, no `replay`, no
   `viz` alias.
4. **No `internal/bench/`.** Bench is destined for its own repository; this
   spec deletes it from mindspec with a documented git-rescue procedure
   (BENCH-MOVED.md) for users who need to recover the historical code.
5. **One legitimate observability surface remains**:
   `mindspec otel setup [--codex] [--target=claude|codex|env]` writes OTEL
   endpoint configuration to `.claude/settings.local.json`,
   `.codex/config.toml`, or stdout `export` lines. `mindspec otel status`
   reports the currently configured endpoint (read-only). These are pure
   configuration; no network calls, no subprocess management.
6. **Migration is communicated, not gradual.** Removed cobra commands return
   exit 2 with a per-command stderr migration hint (see the per-command
   migration table in spec 084). No feature flag, no `embedded`/`external`
   mode toggle.

This decision supersedes parts of [ADR-0011](ADR-0011.md): the one-way
dependency remains, but it is now mediated by user-configured OTEL
endpoints rather than by mindspec spawning the agentmind binary.

## Consequences

**Positive:**
- Mindspec users who don't want agentmind aren't forced to install it.
- Agentmind users who don't use mindspec aren't tied to its release schedule.
- Bench can iterate independently in its own repository.
- Standard OTEL semantics throughout — mindspec stops being a special
  telemetry-orchestrator.
- Smaller mindspec binary (target: ≥30% additional shrinkage on top of spec
  083's -3.4 MB).

**Negative:**
- Users with scripts that invoke `mindspec agentmind serve` get exit 2 with
  migration text; no transition window.
- Users of `mindspec bench` lose the integrated workflow until the bench
  extraction repo ships.
- `mindspec record start` becomes "launch workload with OTEL env vars";
  users wanting persisted local replay data must run their own collector
  with file output.

## Alternatives considered

- **Feature flag (`MINDSPEC_OBSERVABILITY=embedded|external`)**: rejected
  (panel candidate C09). Two code paths across two releases adds carrying
  cost without real user-disruption benefit.
- **Vendor agentmind/wire into mindspec privately** (panel candidate C05):
  rejected because the wire types are no longer used by mindspec at all
  post-extraction.
- **Keep `internal/bench/` and decouple it from agentmind/client** (panel
  candidate C07): rejected because bench's eventual extraction to its own
  repo is already on the roadmap; carrying it through this spec just to
  delete it later is churn.
- **Multi-spec roadmap** (panel candidate C06): rejected because the 4-bead
  decomposition can deliver the full vision in a single PR with
  panel-reviewed gates.

## References

- [Spec 084-mindspec-otel-only](../specs/084-mindspec-otel-only/spec.md)
- [ADR-0011](ADR-0011.md) — original one-way dependency contract
- [ADR-0026](ADR-0026-agentmind-extracted-to-standalone-repo.md) — spec 083
- Panel deliberation: `../bench/v2/experiments/session-5/reviews/panels/`
