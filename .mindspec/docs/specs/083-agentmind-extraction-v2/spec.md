---
approved_at: "2026-05-19T23:32:32Z"
approved_by: user
status: Approved
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
3. Inter-process communication between mindspec and agentmind has exactly two
   normative channels:
   - **Inbound to agentmind** (telemetry from the workload under test):
     OTLP/HTTP on port 4318 only (ADR-0011). No file-watch, no shared memory,
     no env-var side-channels for data.
   - **Outbound from agentmind to mindspec consumers** (collected events
     stream): **stdout pipe with line-delimited JSON** (one
     `wire.CollectedEvent` per line, UTF-8, terminated by `\n`). The
     agentmind-side `--output <file>` flag remains a separate facility for
     writing to a file when invoked directly; mindspec consumers never depend
     on file-tailing semantics. The canonical reader is
     `client.ReadEvents(io.Reader) <-chan wire.CollectedEvent`.
4. **Graceful degradation, runtime-verified — per command class.** When the
   `agentmind` binary is absent from PATH and the configured `--bin` location,
   commands fall into three classes with distinct contracts. In every class,
   detection uses the typed sentinel `errors.Is(err, client.ErrBinaryNotFound)`
   (exported from `agentmind/client`); substring matching on error text is
   prohibited. When a warn line is emitted, it is exactly one line per process
   and reads
   `WARN: agentmind binary not found; telemetry export will drop silently`.
   Centralized emission inside `agentmind/client` via a process-level
   `sync.Once` is the mechanism that guarantees "exactly one line" across
   multiple `AutoStart` callers in the same process.
   - **Telemetry-as-output** (`mindspec record start`): MUST exit non-zero
     with a clear error message when the binary is absent. Telemetry IS the
     deliverable; a silent empty-recording is a correctness violation, not
     graceful degradation.
   - **Interactive** (`mindspec viz`, `mindspec agentmind serve`,
     `mindspec agentmind replay`): MUST exit non-zero when the binary is
     absent. A user-invoked UI command that exits 0 with no UI is a UX bug.
   - **Batch / side-effect** (`mindspec bench run`,
     `mindspec agentmind setup`): MAY exit 0 with the warn line; telemetry
     was a side-effect, not the deliverable.
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
| (new) | `client/client.go` | ~60 | Thin client API: `AutoStart`, `IsRunning`, `DefaultOTLPPort`, `DefaultUIPort`, `ReadEvents(io.Reader) <-chan wire.CollectedEvent`, and the typed sentinel `ErrBinaryNotFound`. Imports only stdlib and `agentmind/wire`. |

**Stays in `mindspec/`:**

| Path | Notes |
|------|-------|
| `internal/bench/runner.go` | Orchestrates benches. Reads NDJSON output written by agentmind subprocess; no longer parses OTLP itself. |
| `internal/bench/session.go` | Reads NDJSON via `wire.CollectedEvent` (imported from agentmind). |
| `internal/recording/collector.go` | Replaces direct OTLP parsing with: spawn agentmind via `client.AutoStart`, read its NDJSON output. |
| `cmd/mindspec/viz.go` | Thin shell. `mindspec agentmind serve` re-execs `agentmind serve` with the same flags. If agentmind binary is missing, prints the graceful-degradation warning and exits 0. |
| `cmd/mindspec/record.go`, `cmd/mindspec/bench.go` | Unchanged for end users. Internally call the new boundary. |
| `internal/agentmind/*` | **Deleted in Phase 5.** Spec failure if the directory still exists from the Phase 5 deletion commit onward. |

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
- **Install path commitment (Phase 6):** the agentmind binary is delivered
  via **documented manual download with checksum verification**. The
  agentmind GitHub release publishes prebuilt binaries for darwin-arm64,
  darwin-amd64, linux-amd64, windows-amd64 alongside a `SHA256SUMS` file
  (and detached signature if available). Mindspec documentation (README +
  Phase 6 release notes) provides the exact `curl`/`sha256sum` invocation
  to fetch the binary and verify its checksum, and instructs users to
  place it at `<mindspec-root>/bin/agentmind` (or any directory on PATH).
  This is the supported install path for v1.0.0.

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

### Deferred to follow-up spec

- **`mindspec install agentmind` subcommand.** A first-party installer that
  downloads, checksum-verifies, and places the agentmind binary in
  `<mindspec-root>/bin/` is cross-cutting (download client, checksum
  verification, platform detection, possibly signature verification) and is
  explicitly deferred to its own follow-up spec. For v1.0.0, the install
  path is documented manual download (see Scope above).

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
      `./bin/agentmind`, commands behave per their class (Hard Constraint #4):
  - `mindspec record start --spec test` exits non-zero with a clear error
    message (telemetry-as-output class).
  - `mindspec viz`, `mindspec agentmind serve`, `mindspec agentmind replay`
    each exit non-zero (interactive class).
  - `mindspec bench run <fixture>` and `mindspec agentmind setup` each exit
    0 with stderr containing exactly one line
    `WARN: agentmind binary not found; telemetry export will drop silently`
    (batch class).
- [ ] NDJSON parity with the pre-migration output is phase-scoped:
  - **Phase 2 (alias state, no subprocess yet):** byte-for-byte equality
    against a saved fixture produced under a frozen-clock test harness with
    the canonical encoding defined in `wire/event.go` — JSON object keys
    sorted lexicographically, floats formatted with a fixed precision rule
    (`strconv.FormatFloat(v, 'f', -1, 64)`), and timestamps emitted as
    UTC RFC3339Nano. Comparison via `diff`.
  - **Phase 4+ (subprocess state):** semantic equivalence under documented
    normalization. The normalization step (sort events by event time, redact
    PID/host fields, redact wall-clock timestamps to a canonical placeholder)
    is published as a tool/library inside `agentmind/wire` and named in the
    acceptance harness. The normalized streams must diff clean.
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
- **Test C — per-class degradation check (the one no v1 candidate passed):**
  Strip `agentmind` from PATH, `unset AGENTMIND_BIN`, `rm -f ./bin/agentmind`.
  Then, per command class (Hard Constraint #4):
  - **Telemetry-as-output:** `./bin/mindspec record start --spec test 2>stderr.log`
    must exit non-zero with a clear error message on stderr.
  - **Interactive:** `./bin/mindspec viz`, `./bin/mindspec agentmind serve`,
    `./bin/mindspec agentmind replay` each must exit non-zero.
  - **Batch / side-effect:** `./bin/mindspec bench run <fixture> 2>stderr.log`
    and `./bin/mindspec agentmind setup 2>stderr.log` must each exit 0 with
    `grep -q "agentmind binary not found; telemetry export will drop silently" stderr.log`
    matching exactly one line.
- **Test D — end-to-end live capture:**
  `./agentmind/bin/agentmind serve --otlp-port 4318 --ui-port 0 --output /tmp/em.ndjson &`,
  POST a synthetic OTLP log payload, then assert `/tmp/em.ndjson` is non-empty
  and contains `"name":"claude_code.api_request"`.
- **Test E — no-circular-discovery check:**
  `grep -rn 'exec\.Command.*"mindspec"\|LookPath.*"mindspec"\|StartProcess.*"mindspec"' ./agentmind/client/ ./agentmind/cmd/ ./agentmind/internal/`
  returns no match. An AST-based check that enumerates `exec.Command`,
  `exec.LookPath`, and `os.StartProcess` argument literals is an acceptable
  alternative.
- **Test F — import-boundary check:**
  `cd ./mindspec && go list -deps ./cmd/mindspec | grep mrmaxsteel/agentmind | sort -u | grep -vE '^github.com/mrmaxsteel/agentmind/(client|wire)$'`
  returns no match.
- **Test G — Phase 0 prerequisite gate (the agentmind v0.0.1 tag exists):**
  `git ls-remote --tags https://github.com/mrmaxsteel/agentmind | grep -q 'refs/tags/v0.0.1$'`
  returns success. The check is wrapped in
  `scripts/verify-agentmind-tag.sh` (also reachable via
  `make verify-agentmind-tag`), which exits 0 and prints the SHA on
  success, exits 2 when the tag is absent, and exits 3 when the repo
  itself is unreachable. The v0.0.1 tag SHA MUST be recorded in this
  spec before Phase 1 may begin. **Current state (Bead 1
  implementation):** `agentmind v0.0.1` has not yet been published
  upstream — at the time of Bead 1, the gate exits 2 with a clear
  "tag not found" message. This is the expected state during the
  parallel mindspec-side migration; the gate flips green when the
  agentmind side scaffolds and tags `v0.0.1`. The script supports
  `--record` mode (`scripts/verify-agentmind-tag.sh v0.0.1 --record`),
  which, once the tag is published, replaces the placeholder below
  with the captured SHA in this file. Placeholder until that happens:
  `agentmind v0.0.1 SHA: <TBD — record before Phase 1; populated by scripts/verify-agentmind-tag.sh --record>`.

Passing Tests A–G is the definition of "the extraction is done."

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

  **Phase 1 precondition (method-set inventory).** Go does not permit
  declaring methods on type aliases of types defined in another package, so
  Phase 2's `type CollectedEvent = wire.CollectedEvent` alias strategy
  requires that `bench.CollectedEvent`, `otlpValue`, and `otlpKeyValue`
  have no methods at the time of aliasing. Before Phase 1 may complete:
  enumerate every method currently declared on `bench.CollectedEvent`,
  `otlpValue`, and `otlpKeyValue` (via grep or AST walk on
  `internal/bench/` for `func (… *?CollectedEvent)`, `func (… *?otlpValue)`,
  `func (… *?otlpKeyValue)`) and move them into `wire/event.go` as part of
  Phase 1's public surface. If the panel/agent verifies "no methods today"
  via grep, record that finding as the precondition outcome in the
  extraction commit message and the alias strategy is unblocked. If
  methods are found and cannot be moved cleanly, fall back to full type
  duplication during Phase 2 (and delete the bench-side copies in Phase 5).
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
  `DefaultUIPort=8420`, `ReadEvents(io.Reader) <-chan wire.CollectedEvent`,
  and the typed sentinel `ErrBinaryNotFound`. `findBinary` looks up
  `agentmind` (never `mindspec`) in this exact order: `$AGENTMIND_BIN` →
  `<mindspec-root>/bin/agentmind` → `agentmind` on PATH. When none resolves,
  it returns an error that satisfies `errors.Is(err, client.ErrBinaryNotFound)`
  (wrapping with `fmt.Errorf("…: %w", client.ErrBinaryNotFound)` is
  permitted). Substring matching on error text is prohibited. The warn-line
  emission is centralized inside `agentmind/client` and guarded by a
  process-level `sync.Once` so that no matter how many `AutoStart` callers
  exist in the same process, exactly one warn line is emitted. Tag `v0.3.0`.
  In mindspec: change every consumer of `internal/agentmind.AutoStart` to
  `agentmind/client.AutoStart` (files: `internal/recording/collector.go`,
  `internal/bench/runner.go`). Each call site detects the absent-binary
  condition with `errors.Is(err, client.ErrBinaryNotFound)` and applies the
  per-class policy from Hard Constraint #4 (telemetry-as-output: propagate
  the error and exit non-zero; interactive: propagate; batch: swallow and
  return nil after the centralized warn line fires). Update
  `cmd/mindspec/viz.go`'s `agentmindServeCmd` and `agentmindReplayCmd` to
  re-exec via `client.RunStandalone(args)`; on `ErrBinaryNotFound` they
  exit non-zero per the interactive class.
- **Phase 5**: Delete `mindspec/internal/agentmind/` entirely. Delete the
  OTLP-parsing code from `mindspec/internal/bench/collector.go`, keeping only
  the type aliases until no caller uses them, then drop the file. Delete
  `mindspec/internal/viz/` (only the cobra shell-out in
  `cmd/mindspec/viz.go` remains). Record before/after `mindspec` binary size
  in the commit message.
- **Phase 6**: Cut `agentmind v1.0.0` after the Phase 3 integration test has
  been green for 7 days of nightly runs. mindspec drops the local `replace`
  directive and pins `agentmind v1.0.0`. The agentmind GitHub release
  publishes prebuilt binaries for darwin-arm64, darwin-amd64, linux-amd64,
  windows-amd64 alongside a `SHA256SUMS` file. The supported install path
  for v1.0.0 is **documented manual download with checksum verification**:
  mindspec README and release notes give users the exact `curl` and
  `sha256sum` (or `shasum -a 256`) invocation, and instruct them to place
  the verified binary at `<mindspec-root>/bin/agentmind` or any directory
  on PATH. The first-party `mindspec install agentmind` subcommand is
  deferred to a follow-up spec (see Non-Goals → Deferred to follow-up spec).

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

All previously-open questions resolved by the review panel before approval:

- **Phase 2 `replace ../agentmind` directive in CI.** Resolved: CI builds use
  a sibling-checkout helper (`scripts/checkout-agentmind.sh`) that clones the
  agentmind repo at the tag pinned in mindspec's `go.mod` to a sibling path
  before running `go test`. Implementation detail to be handled by the Phase 2
  bead; not a spec-level open question.
- **Smoke test against a real `agentmind` binary in CI.** Resolved: Test D
  (live-capture) is added to mindspec's CI matrix from Phase 4 onward,
  gated on the agentmind binary being available via the Phase 0
  prerequisite verified by Test G. Until Phase 4, unit-level testing of the
  graceful-degradation wrapper plus the Phase 3 `main_test.go` integration
  test in the agentmind repo is sufficient.
- **Wire-version-skew handling.** Resolved as a Non-Goal (see "Non-Goals"
  section): `client.Probe()` does not perform version-skew checks for v1.0.0.
  Future ADR may add a minimal "refuse to start on major mismatch" check;
  out of scope here.

## Estimated effort (mindspec side)

- Phase 2 mindspec edits (alias re-export + replace directive): half a day.
- Phase 4 mindspec edits (consumer swap + graceful-degradation wrapper +
  cobra re-exec): one day.
- Phase 5 mindspec deletions and binary-size measurement: half a day.
- Phase 6 mindspec edits (drop replace, pin tag, document manual install
  with checksum verification in README/release notes): half a day plus a
  one-week soak. (No install subcommand — deferred.)

Total mindspec-side: roughly 2.5–3 engineer-days, on top of the agentmind-side
work tracked in the agentmind repo. The v1 panel saw zero candidates that did
more than 0.5 engineer-days of work on either side; that is the size of the
gap this spec is sized against.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-19
- **Notes**: Approved via mindspec approve spec