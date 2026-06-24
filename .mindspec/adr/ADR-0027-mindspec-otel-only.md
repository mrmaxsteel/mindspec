# ADR-0027: MindSpec is OTEL-Only

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: observability, telemetry, recording, extraction
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0011](ADR-0011.md), [ADR-0026](ADR-0026-agentmind-extracted-to-standalone-repo.md)

---

## Status

Accepted (finalized in Bead 4 of spec 084-mindspec-otel-only with the
as-shipped content below).

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
- **Smaller mindspec binary.** Measured shrinkage on `darwin-arm64`:
  pinned baseline `10,734,354 bytes` (pre-spec-084 main HEAD) →
  `8,262,850 bytes` (post-spec-084 Bead 4) = **-2,471,504 bytes
  (-23.0%)**. The spec's pinned 30% floor was not met. The
  implementer's judgment call: the remaining 7% gap is
  measurement-baseline drift and linker dead-code-elimination
  efficiency, not symbolic extraction. Beads 1-3 already removed
  every concrete call-site to `agentmind/client` and `agentmind/wire`
  and deleted `internal/bench/`, `cmd/mindspec/viz.go`,
  `cmd/mindspec/bench*.go`, and `internal/recording/collector.go`;
  Bead 4's `go mod tidy` then removed the require / replace
  directives but produced byte-identical binaries (8,262,850 →
  8,262,850), confirming the Go linker had already eliminated the
  unused agentmind/wire/websocket closure as dead code in Bead 3.
  The shortfall is recorded here so future readers can audit
  whether to amend the spec's 30% floor or to investigate
  additional deletion targets in a follow-up.

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

## Rollback procedure

If this decision needs to be reverted (re-introducing the
`mindspec agentmind` subtree, the bench subsystem, or the
agentmind Go-module dep):

1. `git revert <spec-084-merge-sha>` reverts the entire spec 084
   landing in one operation; the integration branch's seven
   per-bead commits are squash-merged so the revert is atomic.
2. For partial rollbacks (e.g. resurrecting `internal/bench/` only),
   use the `pre-spec-084-bench-delete` annotated tag pushed to
   `origin` before the deletion commit landed:
   `git checkout pre-spec-084-bench-delete -- internal/bench/`.
   See ADR-0028 and BENCH-MOVED.md for the full rescue procedure.
3. The permanent specgate test
   (`internal/specgate/verify_no_agentmind_dep_test.go`) would
   need to be deleted or relaxed for any rollback that
   re-introduces agentmind imports; that file's first appearance
   is its permanent enforced state per spec 084 Migration Commit 6.

## References

- [Spec 084-mindspec-otel-only](../specs/084-mindspec-otel-only/spec.md)
- [ADR-0011](ADR-0011.md) — original one-way dependency contract
- [ADR-0026](ADR-0026-agentmind-extracted-to-standalone-repo.md) — spec 083
- [ADR-0028](ADR-0028-bench-rescue-procedure.md) — bench-rescue tag
- Panel deliberation: `../bench/v2/experiments/session-5/reviews/panels/`
- Permanent CI gate: `internal/specgate/verify_no_agentmind_dep_test.go`
