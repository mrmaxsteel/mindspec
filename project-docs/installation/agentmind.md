# AgentMind and MindSpec

AgentMind is a **standalone companion product** — a real-time observability
dashboard for AI coding agents. It lives in its own repository at
[`github.com/mrmaxsteel/agentmind`](https://github.com/mrmaxsteel/agentmind),
with its own releases, install instructions, and documentation. It is not
part of the mindspec binary and is not installed by mindspec.

## The integration contract: OTLP only

MindSpec's relationship to AgentMind is deliberately thin (ADR-0026,
ADR-0027): mindspec **emits OpenTelemetry configuration and nothing else**.
It has no Go dependency on agentmind, never spawns an agentmind process,
and never speaks OTLP itself — a permanent CI gate
(`internal/specgate/verify_no_agentmind_dep_test.go`) fails the build if
either an agentmind package enters the Go dependency graph or an
`exec.Command`-style invocation of an `agentmind` binary appears in the
tree.

The mindspec side of the integration is one verb:

```bash
mindspec otel setup --endpoint http://localhost:4318   # Claude Code (default target)
mindspec otel setup --endpoint http://localhost:4318 --codex
mindspec otel setup --endpoint http://localhost:4318 --target env
mindspec otel status                                    # read-only: show what's configured
```

This writes the OTLP/HTTP exporter configuration into the target agent's
config; it performs zero network I/O. Anything that speaks OTLP works as
the receiver: agentmind, Honeycomb, Tempo, Jaeger,
opentelemetry-collector-contrib.

## Installing AgentMind

Follow the install instructions in the
[agentmind repository](https://github.com/mrmaxsteel/agentmind) — that
repo's releases and docs are authoritative. Nothing about the install
involves the mindspec tree: the binary does not need to live under
`<mindspec-root>/bin/`, and mindspec never looks for it.

## Historical note

MindSpec once bundled the agentmind subsystems (`bench`, `viz`,
`agentmind serve|replay|setup`) and, for one release, spawned an external
agentmind binary discovered via a lookup order. Both architectures are
gone: spec 083 extracted agentmind to its own repo, and spec 084 removed
the remaining Go-module dependency and process-spawning entirely
(see ADR-0026, ADR-0027, and ADR-0028 for the bench move). The removed
top-level verbs survive one release as hidden deprecation stubs that
print a one-line pointer and exit 2. Earlier revisions of this page
documented the spawn-and-discover architecture; consult git history if
you need it.

## See also

- [AgentMind user guide](../user/guides/agentmind.md) — what the
  companion product does and how to point an agent at it
- ADR-0026 (AgentMind extracted to standalone repo)
- ADR-0027 (mindspec is OTEL-only)
- ADR-0028 (bench rescue procedure)
