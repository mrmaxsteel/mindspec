---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 083-agentmind-extraction-v2: AgentMind Extraction (v2)

## Goal

Move AgentMind — the OTLP receiver, OTLP parser, visualization server, and embedded
web UI — out of the `mindspec` monorepo into a standalone
`github.com/mrmaxsteel/agentmind` repository with its own binary. The mindspec
side must keep building and passing tests on every commit, the user-visible CLI
surface must not change, and graceful degradation when the `agentmind` binary is
absent must be reliable and runtime-verified.

This is v2 of an earlier attempt (`SPEC_AGENTMIND_SEPARATION.md`, used in session
4/5 benches). A 10-reviewer panel (5 claude + 5 codex) unanimously voted SKIP on
all 10 candidate implementations of v1. Every implementation was a "symbolic
extraction": agents moved the 135-line process-spawn shim, left ~3,000 lines of
real AgentMind code in mindspec, and introduced a circular-binary-discovery bug
where `agentmind/client` invoked the `mindspec` binary to perform the work it
was supposed to take over. v2 makes the surface-to-move explicit, requires a
runnable `agentmind` binary, and adds runtime checks that fail when the
boundary is only symbolic.

## Background

- ADR-0011 established the one-way `mindspec → agentmind` dependency via
  OTLP/HTTP on port 4318. This spec executes the physical separation that ADR
  authorized.
- The v1 spec failed because it did not enumerate the surface to move, did not
  require a buildable standalone binary, did not define the binary-lookup order,
  and did not require deletion of the old code. v2 closes each of those gaps.
- Source reference document: `../bench/v2/specs/SPEC_AGENTMIND_EXTRACTION_V2.md`
  (lives outside this repo; this spec is the in-repo, MindSpec-format
  counterpart and is authoritative for the mindspec side of the work).

## Impacted Domains

- `internal/agentmind`: Deleted in Phase 5. Auto-start / find-binary logic is
  replaced by `github.com/mrmaxsteel/agentmind/client`.
- `internal/bench`: `collector.go` loses its OTLP parser. `CollectedEvent` and
  sibling types become type aliases of `wire.CollectedEvent`. `runner.go` and
  `session.go` read NDJSON written by the agentmind subprocess instead of
  parsing OTLP directly.
- `internal/recording`: `collector.go` stops parsing OTLP; spawns the
  `agentmind` binary via `client.AutoStart` and consumes its NDJSON output.
- `internal/viz`: Removed from mindspec; lives in agentmind. Only the
  cobra-level shell-out remains.
- `cmd/mindspec/viz.go`: `mindspec agentmind serve` / `mindspec agentmind replay`
  become thin re-exec wrappers around the `agentmind` binary.
- `cmd/mindspec/record.go`, `cmd/mindspec/bench.go`: Unchanged user-facing
  behavior; internally wired to the new boundary with the graceful-degradation
  contract.
- `go.mod`: Adds `require github.com/mrmaxsteel/agentmind`. Local development
  uses `replace ... => ../agentmind`; release branch pins a tagged version.

## ADR Touchpoints

- [ADR-0011](../../adr/ADR-0011.md): One-way `mindspec → agentmind` dependency
  over OTLP/HTTP:4318. This spec is the physical realization of that ADR.
- ADR-0026 (new, to be authored as part of this work): "AgentMind extracted to
  standalone repo." Records the move, the deferred decisions (UI-port
  discovery, version-skew handling, setup ownership), and the rollback
  procedure (`git revert <merge-sha>` in mindspec + drop the `require` line).

## Requirements

### Non-negotiable hard constraints

1. mindspec imports only `github.com/mrmaxsteel/agentmind/client` and
   `github.com/mrmaxsteel/agentmind/wire`. No `internal/*`, no other agentmind
   subpackage.
2. agentmind never imports `github.com/mrmaxsteel/mindspec/*`. Verified by
   `go list -m -json all` in agentmind — any mindspec dep fails CI.
3. Inter-process communication between mindspec and agentmind is OTLP/HTTP on
   port 4318 only (ADR-0011). No file-watch, no shared memory, no env-var
   side-channels for data.
4. **Graceful degradation, runtime-verified.** When the `agentmind` binary is
   absent from PATH and the configured `--bin` location, every mindspec command
   that previously auto-started agentmind must succeed with exit code 0 and a
   single stderr line:
   `WARN: agentmind binary not found; telemetry export will drop silently`.
   Specifically: `mindspec record start`, `mindspec bench run`, and
   `mindspec spec-init` must all complete with exit code 0 in this state.
5. CLI surface unchanged on the user side. `mindspec record`, `mindspec bench`,
   and `mindspec viz` (alias `mindspec agentmind`) behave identically as
   observed from the user perspective. `mindspec agentmind serve` and
   `mindspec agentmind replay` continue to exist as thin shells that re-exec
   the standalone `agentmind` binary with the same flags.
6. No atomic cutover. Every commit on the mindspec side leaves
   `go build ./cmd/mindspec` and `go test -short ./...` passing.
7. No circular binary discovery. The `agentmind` binary must not invoke any
   `mindspec` binary at runtime. Symmetrically, mindspec looks up `agentmind`
   by name only — never `mindspec` to satisfy its agentmind path.
8. Wire-protocol contract: every type that crosses the OTLP boundary
   (currently `bench.CollectedEvent` and the OTLP request shapes it parses) is
   published at `github.com/mrmaxsteel/agentmind/wire` with explicit Go module
   SemVer tags. v0.x allowed; breaking changes increment the minor version.

### Surface to move — exact file inventory

**Moves to `agentmind/`:**

| From (mindspec) | To (agentmind) | Lines | Notes |
|-----------------|----------------|-------|-------|
| `internal/agentmind/autostart.go` | `client/autostart.go` | 113 | Process spawn helpers. **Rename target binary from `mindspec` to `agentmind`**. |
| `internal/agentmind/sysproc_unix.go` | `client/sysproc_unix.go` | 12 | Build-tagged platform helper. |
| `internal/agentmind/sysproc_windows.go` | `client/sysproc_windows.go` | 10 | Build-tagged platform helper. |
| `internal/bench/collector.go` | `internal/otlp/collector.go` | 363 | OTLP HTTP receiver. Stays internal to agentmind. |
| types from `internal/bench/collector.go` (`CollectedEvent`, `otlpValue`, `otlpKeyValue`) | `wire/event.go` | ~80 | The cross-boundary types. Become the public wire API. |
| `internal/viz/server.go` | `internal/viz/server.go` | 278 | Web server + embed.FS for web UI. |
| `internal/viz/hub.go` | `internal/viz/hub.go` | — | WebSocket broadcast hub. |
| `internal/viz/graph.go` | `internal/viz/graph.go` | 475 | Graph state machine. |
| `internal/viz/normalize.go` | `internal/viz/normalize.go` | — | Imports `wire.CollectedEvent` instead of `bench.CollectedEvent`. |
| `internal/viz/live.go` | `internal/viz/live.go` | 306 | OTLP → graph live pipeline. |
| `internal/viz/replay.go` | `internal/viz/replay.go` | 200 | NDJSON replay. |
| `internal/viz/codex_fallback.go` | `internal/viz/codex_fallback.go` | 464 | Codex JSONL converter. |
| `internal/viz/run.go` | `internal/viz/run.go` | — | `RunLiveOpts` entrypoint. |
| `internal/viz/web/*` (embed.FS assets) | `internal/viz/web/*` | — | UI bundle. |
| (new) | `cmd/agentmind/main.go` | ~150 | Standalone binary with `serve`, `replay`, `setup` subcommands. Mirrors current `mindspec agentmind` cobra tree. |
| (new) | `cmd/agentmind/serve.go` | ~60 | `serve` subcommand handler — calls `viz.RunLiveOpts`. |
| (new) | `cmd/agentmind/replay.go` | ~80 | `replay` subcommand handler. |
| (new) | `client/client.go` | ~40 | Thin client API: `AutoStart`, `IsRunning`, `DefaultOTLPPort`, `DefaultUIPort`. Imports only stdlib and `agentmind/wire`. |

**Stays in `mindspec/`:**

| Path | Notes |
|------|-------|
| `internal/bench/runner.go` | Orchestrates benches. Reads NDJSON output written by agentmind subprocess; no longer parses OTLP itself. |
| `internal/bench/session.go` | Reads NDJSON via `wire.CollectedEvent` (imported from agentmind). |
| `internal/recording/collector.go` | Replaces direct OTLP parsing with: spawn agentmind via `client.AutoStart`, read its NDJSON output. |
| `cmd/mindspec/viz.go` | Thin shell. `mindspec agentmind serve` re-execs `agentmind serve` with the same flags. If agentmind binary is missing, prints the graceful-degradation warning and exits 0. |
| `cmd/mindspec/record.go`, `cmd/mindspec/bench.go` | Unchanged for end users. Internally call the new boundary. |
| `internal/agentmind/*` | **Deleted** after the migration. Spec failure if the directory still exists at the end of any extraction commit beyond Phase 1. |

**Becomes shared between both repos:**

| Module | Owner | Consumers |
|--------|-------|-----------|
| `github.com/mrmaxsteel/agentmind/wire` | agentmind | mindspec (read NDJSON, deserialize events) + agentmind internal |

## Scope

### In Scope

- All file moves and deletions in the inventory above.
- mindspec go.mod changes: add `require github.com/mrmaxsteel/agentmind`, plus
  a local `replace ... => ../agentmind` directive used during phases 0–5,
  removed in Phase 6 in favor of a pinned tag.
- Rewiring `cmd/mindspec/viz.go`, `internal/recording/collector.go`, and
  `internal/bench/runner.go` to call `agentmind/client.AutoStart`.
- The graceful-degradation wrapper around every `client.AutoStart` caller.
- ADR-0026 (new): records the extraction, deferred decisions, rollback.

### Out of Scope

- The `github.com/mrmaxsteel/agentmind` repository scaffolding itself (Phase 0
  in the source document) is a prerequisite, performed in that repo. This spec
  treats agentmind v0.x as a sequenced dependency and tracks only the mindspec
  side. The acceptance criteria depend on it existing.
- Recording-directory ownership: phase markers and `manifest.json` management
  stay in `mindspec/internal/recording`. The agentmind binary writes only to
  the `--output` path it is given.
- `mindspec agentmind setup` (writes to `.claude/settings.local.json`) stays
  in mindspec for this spec because it knows mindspec's `.claude/` layout.
  Whether to move it later is a deferred ADR.

## Non-Goals

- No sidecar / Docker mode. ADR-0011 requires auto-start; the standalone
  binary satisfies that with less operational overhead than containers.
- No HTTP control plane. The launch contract remains CLI flags. A control API
  can come later; it is not required to ship a working extraction.
- No `agentmind → mindspec` callbacks. ADR-0011's "one-way dependency" is
  non-negotiable. If agentmind ever needs mindspec state, that state moves
  into the OTLP/NDJSON contract or it does not exist on the agentmind side.
- No daemon-mode UI-port discovery. Port 8420 stays hardcoded. Collisions are
  a future ADR.
- No version-skew runtime handling (`client.Probe()` semantics). Future ADR.

## Acceptance Criteria

- [ ] `find internal/agentmind` returns no results in the mindspec tree.
- [ ] `grep -r 'http.HandleFunc.*"/v1/logs"' .` returns no results in the
      mindspec tree.
- [ ] `go list -deps ./cmd/mindspec | grep mrmaxsteel/agentmind | sort -u`
      returns only `github.com/mrmaxsteel/agentmind/client` and
      `github.com/mrmaxsteel/agentmind/wire` — no `internal/*`.
- [ ] mindspec `go build ./cmd/mindspec` and `go test -short ./...` pass on
      every commit produced by the migration.
- [ ] With `agentmind` binary absent from PATH, `AGENTMIND_BIN` unset, and no
      `./bin/agentmind`: `mindspec record start --spec test`,
      `mindspec bench run <fixture>`, and `mindspec spec-init` each exit 0
      with stderr containing exactly one line
      `WARN: agentmind binary not found; telemetry export will drop silently`.
- [ ] With `agentmind` binary present, `mindspec bench run` produces NDJSON
      output byte-for-byte equal to the pre-migration output for the same
      fixture (compared via `diff`).
- [ ] `mindspec agentmind serve --help` and `mindspec agentmind replay --help`
      print the same usage text as before the extraction.
- [ ] mindspec binary size shrinks; the before/after sizes are recorded in
      the commit message for the deletion phase.
- [ ] ADR-0026 is committed and referenced from this spec's ADR Touchpoints.
- [ ] Release branch of mindspec has `require github.com/mrmaxsteel/agentmind
      v1.0.0` and no `replace` directive for agentmind.

## Validation Proofs

These are runtime tests the implementation bench must run instead of only
`go build` and `go test -short`. Static checks proved blind to symbolic
extractions in the v1 panel; runtime checks are not.

- **Test A — standalone-binary check (precondition):**
  `test -x ./agentmind/bin/agentmind && ./agentmind/bin/agentmind --version | grep -q "^agentmind"`.
- **Test B — no-mindspec-dep check (precondition):**
  `cd ./agentmind && go list -m -json all | jq -r '.Path' | grep -q '^github.com/mrmaxsteel/mindspec'` returns no match.
- **Test C — graceful-degradation check (the one no v1 candidate passed):**
  Strip `agentmind` from PATH, `unset AGENTMIND_BIN`, `rm -f ./bin/agentmind`.
  Run `./bin/mindspec record start --spec test 2>stderr.log`. Assert
  `grep -q "agentmind binary not found; telemetry export will drop silently" stderr.log`
  and `rc == 0`. Repeat for `mindspec bench run` and `mindspec spec-init`.
- **Test D — end-to-end live capture:**
  `./agentmind/bin/agentmind serve --otlp-port 4318 --ui-port 0 --output /tmp/em.ndjson &`,
  POST a synthetic OTLP log payload, then assert `/tmp/em.ndjson` is non-empty
  and contains `"name":"claude_code.api_request"`.
- **Test E — no-circular-discovery check:**
  `grep -rn '"mindspec"' ./agentmind/client/ ./agentmind/cmd/ ./agentmind/internal/ | grep -vE 'README|comment|//'`
  returns no match.
- **Test F — import-boundary check:**
  `cd ./mindspec && go list -deps ./cmd/mindspec | grep mrmaxsteel/agentmind | sort -u | grep -vE '^github.com/mrmaxsteel/agentmind/(client|wire)$'`
  returns no match.

Passing Tests A–F is the definition of "the extraction is done."

## Migration phases (mindspec side)

Each phase is one or more commits. Every commit leaves mindspec building,
tests passing, and `mindspec record` / `bench` / `viz` functional. CI runs
`go build ./cmd/mindspec && go test -short ./...` after every commit; failure
aborts the migration.

- **Phase 0** *(prerequisite, agentmind repo)*: scaffold the agentmind repo
  (`go.mod`, package skeletons, CI). Tag `v0.0.1`. No mindspec changes.
- **Phase 1**: agentmind publishes `wire/event.go` with `CollectedEvent`,
  `otlpValue`, `otlpKeyValue`, `flattenAttributes`, `parseOTLPTimestamp`,
  plus round-trip tests for every event shape currently emitted (log events,
  metric events, the seven attribute key sets in the codex fallback). Tag
  `v0.1.0`. No mindspec changes yet.
- **Phase 2**: agentmind absorbs the OTLP collector
  (`internal/otlp/collector.go` + tests). In mindspec, change
  `internal/bench/collector.go` to re-export via type aliases
  (`type CollectedEvent = wire.CollectedEvent`, etc.) imported from
  `agentmind/wire`. Add the local `replace ... => ../agentmind` directive.
  Verify NDJSON output is byte-for-byte identical against a saved fixture.
- **Phase 3**: agentmind moves `internal/viz/*` and `internal/viz/web/*`,
  adds `cmd/agentmind/{main,serve,replay}.go`, and ships
  `cmd/agentmind/main_test.go` integration test (build binary, run `serve`,
  POST OTLP, read NDJSON, assert event recorded, stop cleanly). Tag `v0.2.0`.
  Still no consumer change in mindspec — the new binary exists in parallel.
- **Phase 4**: agentmind adds `client/client.go` exporting `AutoStart`,
  `IsRunning`, `WaitForPort`, `Probe`, `DefaultOTLPPort=4318`,
  `DefaultUIPort=8420`. `findBinary` looks up `agentmind` (never `mindspec`)
  in this exact order: `$AGENTMIND_BIN` → `<mindspec-root>/bin/agentmind` →
  `agentmind` on PATH. Returns an error matching the literal text
  `"agentmind binary not found"` when none resolves. Tag `v0.3.0`.
  In mindspec: change every consumer of `internal/agentmind.AutoStart` to
  `agentmind/client.AutoStart` (files: `internal/recording/collector.go`,
  `internal/bench/runner.go`). Wrap each call site with the
  graceful-degradation contract: errors matching `"agentmind binary not found"`
  print the warn line and return nil, not the error. Update
  `cmd/mindspec/viz.go`'s `agentmindServeCmd` and `agentmindReplayCmd` to
  re-exec via `client.RunStandalone(args)`.
- **Phase 5**: Delete `mindspec/internal/agentmind/` entirely. Delete the
  OTLP-parsing code from `mindspec/internal/bench/collector.go`, keeping only
  the type aliases until no caller uses them, then drop the file. Delete
  `mindspec/internal/viz/` (only the cobra shell-out in
  `cmd/mindspec/viz.go` remains). Record before/after `mindspec` binary size
  in the commit message.
- **Phase 6**: Cut `agentmind v1.0.0` after the Phase 3 integration test has
  been green for 7 days of nightly runs. mindspec drops the local `replace`
  directive and pins `agentmind v1.0.0`. agentmind GitHub release publishes
  prebuilt binaries for darwin-arm64, darwin-amd64, linux-amd64,
  windows-amd64. A `mindspec install agentmind` subcommand (or a documented
  manual download) places them in `<mindspec-root>/bin/`.

## Lessons baked in from sessions 4 and 5

- **Don't trust diff-stat as a quality signal.** The v1 bench's "winning" arm
  scored highest on boundary_soundness because the diff *looked* like a clean
  import swap. The 10-reviewer panel showed it was a symbolic-only
  extraction. Phase 3's `main_test.go` integration test is the runtime check
  that makes the next bench detect this directly.
- **Define `findBinary` in the spec.** Five of six v1 candidates broke
  graceful degradation by failing to find any binary; four had
  circular-binary-discovery (looking for `mindspec`). Phase 4 defines the
  exact lookup order and the exact error text above.
- **Require deletion of the old code.** Every v1 candidate left
  `mindspec/internal/agentmind/` in place as dead code. Phase 5 makes
  deletion a hard requirement and a grep-checkable acceptance criterion.
- **Require a wire-types module.** Without `agentmind/wire`, the type
  crossing the boundary (`CollectedEvent`) becomes either a copy in
  agentmind that drifts or an import back to mindspec (banned). Phase 1
  publishes it explicitly.

## Open Questions

- [ ] Should the in-repo `mindspec install agentmind` subcommand (Phase 6) be
      part of this spec, or split into its own follow-up spec? It is small
      but cross-cutting (download, checksum, platform detection).
- [ ] During Phase 2, the local `replace` directive points at `../agentmind`.
      Is that path acceptable in CI, or do we need a sibling-checkout helper?
- [ ] Does mindspec need a smoke test that runs against a real `agentmind`
      binary in CI, or is unit-level testing of the graceful-degradation
      wrapper sufficient until Phase 6?
- [ ] Wire-version-skew handling (`client.Probe()` semantics) is deferred —
      confirm that's acceptable for v1.0.0 or whether a minimal "refuse to
      start on major mismatch" check belongs in this spec's Phase 4.

## Estimated effort (mindspec side)

- Phase 2 mindspec edits (alias re-export + replace directive): half a day.
- Phase 4 mindspec edits (consumer swap + graceful-degradation wrapper +
  cobra re-exec): one day.
- Phase 5 mindspec deletions and binary-size measurement: half a day.
- Phase 6 mindspec edits (drop replace, pin tag, optional install
  subcommand): half a day plus a one-week soak.

Total mindspec-side: roughly 2.5–3 engineer-days, on top of the agentmind-side
work tracked in the agentmind repo. The v1 panel saw zero candidates that did
more than 0.5 engineer-days of work on either side; that is the size of the
gap this spec is sized against.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
