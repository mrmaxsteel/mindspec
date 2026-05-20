---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 084-mindspec-otel-only: Reduce MindSpec to a Pure Spec/Plan/Lifecycle Tool

## Goal

Make the user's one-sentence vision literally true: **"I should be able to
point mindspec at an OTEL collector and that's it."** Strip every byte of
code from `mindspec` that does anything observability-related beyond
*writing the OTEL endpoint into the workload's environment / settings file
and launching the workload*. After this spec ships, `mindspec` has:

- zero subprocess management of any collector;
- zero NDJSON readers;
- zero OTLP parsers;
- zero `mindspec agentmind` cobra subtree;
- zero `mindspec viz`, `mindspec bench`, or any other observability-named
  cobra command beyond `mindspec otel setup`/`status`;
- zero `github.com/mrmaxsteel/agentmind/*` Go module dependency;
- zero `internal/bench/`;
- zero `internal/agentmind/` (if any remnant survived 083);
- a **permanent CI gate** (`internal/specgate/verify_no_agentmind_dep_test.go`)
  that fails the build if any agentmind package re-enters the dep graph.

A user with their own OTEL collector running anywhere on the network sets
one env var (or runs `mindspec otel setup` once) and gets a fully working
spec/plan/lifecycle tool whose telemetry lands in *their* collector with
no further interaction.

This spec is the **smallest viable end state** (C10 spine), not the most
architecturally pure one. Where a non-controversial workaround removes a
"someday" item from the critical path — for example, deleting `internal/bench/`
outright rather than extracting it to a new repo — we take the workaround.
Extraction-to-its-own-repo is strictly slower than deletion and the user
has already said the bench subsystem is "destined for its own repo," i.e.
not mindspec's problem. We mark the deletion with a one-paragraph rescue
note (`BENCH-MOVED.md`, commit SHA citation in the deletion commit
message) and move on.

The spine pragmatism is layered with **C01-level proof discipline**: every
acceptance criterion is a negated grep, an AST check, a runtime
no-listener proof, a binary-size delta, or a help-surface diff. Symbolic
extraction (the v1-spec-083 failure mode) is impossible to land under
these gates.

## Background

- Spec 083 (PR #110, merged into mindspec `main`) moved the OTLP/HTTP
  receiver and visualization code out of mindspec into a standalone
  `agentmind` binary. It deliberately *kept* three load-bearing remnants:
  1. The `mindspec agentmind serve|replay|setup` cobra subtree (thin
     re-exec wrappers around the standalone binary).
  2. `client.AutoStart` / `client.RunStandalone` / `client.ReadEvents`
     callers in `internal/recording/collector.go`,
     `internal/bench/runner.go`, `cmd/mindspec/viz.go`.
  3. The `github.com/mrmaxsteel/agentmind/{client,wire}` Go module dep
     in `go.mod`.
- The user's post-083 vision is that *none* of those three remnants
  should exist. mindspec configures, mindspec launches. agentmind (or
  any OTEL consumer the user chooses — Honeycomb, Tempo, Jaeger, a
  local `otel/opentelemetry-collector-contrib`) is *not mindspec's
  problem*.
- Beads context: `mindspec-mm65` (the architectural pivot),
  `mindspec-r5wq` (the narrower wire/client decoupling — subsumed by
  this spec; no separate landing needed).
- agentmind v0.0.1 is published at
  `https://github.com/mrmaxsteel/agentmind`. Spec 083 added that
  dependency; this spec removes it.

### Why atomic-by-design but committed-in-sequence

Spec 083 needed phasing (across multiple PRs) because thousands of
lines of OTLP/viz code had to be ported and validated under a frozen
clock with a real binary integration test. **This spec has the opposite
risk profile**:

1. The destination already exists (agentmind v0.0.1 is live and proven).
2. No callers exist outside mindspec; the cobra subtree, the `client.*`
   call sites, and `internal/bench/` are all internal to one repo.
3. The user has rescinded the "no user-visible CLI change" contract.
4. Symbolic-extraction risk is highest in phased plans with intermediate
   states. Atomic single-PR delete (7 commits inside one PR, each
   green, but not individually mergeable) is grep-checkable: a file
   either exists in the final tree or it does not.

The right safety net is not phases-over-weeks; it is the validation
matrix in §Validation Proofs.

## Impacted Domains

- **`cmd/mindspec/viz.go`**: **Deleted**. The `mindspec agentmind`
  cobra subtree goes away in its entirety (`serve`, `replay`, plus
  the top-level `viz` alias). Users who want viz run `agentmind serve`
  / `agentmind replay` directly; the standalone binary exists and is
  already documented.
- **`cmd/mindspec/otel.go` (new)**: Provides exactly one subcommand
  surface pair: `mindspec otel setup` (writes an OTEL endpoint to
  `.claude/settings.local.json` and optionally `~/.codex/config.toml`
  with `--codex`) and `mindspec otel status` (read-only diagnostic).
  No probing, no validation, no network calls.
- **`cmd/mindspec/record.go`**: Becomes ~80 lines. Loads OTEL config,
  invokes the workload (Claude Code / Codex / `bash -c`), exits with
  whatever code the workload returned. **No subprocess management of
  any collector, no NDJSON reader, no port wrangling.**
- **`cmd/mindspec/bench.go`**: **Deleted**. `mindspec bench` is gone.
- **`cmd/mindspec/deprecated_commands.go` (new, single file)**:
  Registers `agentmind`, `viz`, `bench`, `serve`, `replay` as hidden
  cobra commands that emit a one-shot stderr message and exit 2.
  Single small file; the entire deprecation surface lives here. Time-
  boxed to one release; no feature flag.
- **`internal/recording/collector.go`**: **Deleted**. mindspec no
  longer "collects" anything. `mindspec record start` becomes a pure
  workload-launcher that (a) ensures OTEL env vars / settings are
  present and (b) execs the workload. Recording-directory bookkeeping
  (`manifest.json`, phase markers) stays — but it is filesystem
  bookkeeping, not telemetry handling.
- **`internal/recording/`**: Pared down to manifest + phase markers +
  the workload launcher. No `AutoStart`, no `ReadEvents`, no port
  wrangling.
- **`internal/bench/` (whole directory)**: **Deleted from mindspec.**
  Not extracted to a new repository inside this spec. A one-paragraph
  `BENCH-MOVED.md` rescue note at the repo root points to the
  annotated tag `pre-spec-084-bench-delete` (per HC #11 default
  option (b)) so anyone can run
  `git checkout pre-spec-084-bench-delete -- internal/bench/` and
  lift the code into its own repo whenever they want — and the
  rescue handle is reachable from `main` even if the spec-084 PR is
  squash-merged.
- **`internal/agentmind/` (if any remnant survived 083)**: **Deleted.**
  Belt-and-suspenders; spec 083 should have removed this, but the
  grep proof in §Acceptance Criteria catches anything missed.
- **`internal/otel/` (new)**: A single small package (~150 lines)
  responsible for rendering OTEL endpoint config into the formats
  different workloads need (Claude Code `.claude/settings.local.json`,
  Codex `~/.codex/config.toml`, raw `OTEL_*` env vars for everything
  else). This is the *one* legitimate observability responsibility
  mindspec retains.
- **`internal/specgate/verify_no_agentmind_dep_test.go` (new)**: A Go
  CI test that runs in `go test -short ./...` and **fails the build if
  any `github.com/mrmaxsteel/agentmind` package re-enters the
  mindspec dep graph or if any `exec.Command` / `exec.LookPath` /
  `os.StartProcess` first-arg literal contains `"agentmind"`**. Per
  C05. Implemented as a pair of helpers:
  - `TestNoAgentmindInDepGraph` — runs `go list -deps ./...` (via
    `go/build`) and asserts no `mrmaxsteel/agentmind` package appears.
  - `TestNoAgentmindExecLiteral` — AST walks every `*.go` file under
    `cmd/`, `internal/`, enumerates `exec.Command`, `exec.LookPath`,
    `os.StartProcess` call-site first-argument literals, and asserts
    none equal `"agentmind"`.
- **`go.mod`**: `require github.com/mrmaxsteel/agentmind` is
  **removed**. No `replace` directive remains. mindspec has no
  remaining import of `agentmind/wire` or `agentmind/client`.

## ADR Touchpoints

- **ADR-0011** (one-way `mindspec → agentmind` dependency over
  OTLP/HTTP): **Superseded in part**. The one-way dependency survives
  conceptually, but the dependency is no longer literal — mindspec
  doesn't depend on `agentmind` at all. The OTLP/HTTP wire shape on
  port 4318 is still how telemetry flows; mindspec just never
  participates in that flow as a producer of the receiver. Prose
  update in ADR-0011 reflects the no-Go-dep posture.
- **ADR-0026** (AgentMind extracted to standalone repo): Carried
  forward unchanged. This spec is the second half of the move ADR-0026
  envisioned.
- **ADR-0027 (new)**: "MindSpec is OTEL-config only." Records that
  mindspec emits telemetry through whatever OTEL endpoint the user
  supplies, never spawns a receiver, and treats agentmind as one of
  many possible downstream collectors (not the privileged one).
  Includes the rollback procedure (git revert of the merge commit).
- **ADR-0028 (new)**: "Bench removed from mindspec." Records the
  deletion, the rescue procedure (`git show <sha>:internal/bench/`),
  and the explicit "extraction is not mindspec's problem" stance.

## Requirements

### Non-negotiable hard constraints

1. **Zero agentmind in the Go dep graph.** `go list -deps ./... |
   grep -c 'mrmaxsteel/agentmind'` outputs exactly `0`. `go.mod` and
   `go.sum` contain no `mrmaxsteel/agentmind` entries.
2. **Zero agentmind in source — with two deliberate, allow-listed
   exceptions.** The grep is:

   ```
   grep -rn 'agentmind\|wire\.CollectedEvent\|client\.AutoStart\|client\.ReadEvents\|client\.RunStandalone' cmd/ internal/ \
     --exclude='*_specgate*.go' \
     --exclude='*verify_no_agentmind*.go' \
     --exclude='deprecated_commands*.go'
   ```

   Two — and only two — files in the post-spec tree may legitimately
   contain the literal substring `agentmind`:

   - `internal/specgate/verify_no_agentmind_dep_test.go` — by design
     enumerates the forbidden string so it can assert its absence
     in the dep graph and AST.
   - `cmd/mindspec/deprecated_commands.go` — emits the per-command
     migration table's deprecation messages, which by design name
     `agentmind` and link to the standalone repo URL.

   The grep excludes precisely those two file-name patterns so the
   acceptance criterion is self-consistent. The specgate test itself
   (Test H) AST-walks both files and verifies that the only
   occurrences of `agentmind` are inside string literals (not
   imports, not `exec.Command` first arguments) — so the two
   allow-listed files are still gated against real reintroduction.
3. **mindspec has no concept of agentmind's presence or absence.** Built
   with the `agentmind` binary completely absent from `$PATH` and from
   any `bin/` directory mindspec might once have looked in, mindspec
   **never warns, errors, or behaves differently** on account of that
   absence.
4. **`mindspec record start` is pure config + launch.** `mindspec
   record start --spec <id> -- <workload-cmd…>` exits with exactly the
   exit code of `<workload-cmd>`. Recording side-effects are pure
   filesystem operations (write manifest, create recording dir, write
   phase markers) plus *setting* the workload's environment so its own
   OTEL exporter points at the user-configured endpoint. No mindspec
   process ever reads OTLP, NDJSON, or any other telemetry format.
   **mindspec opens no TCP listener of its own** (verified at runtime
   via portable Go listener-enumeration on Linux `/proc/<pid>/net/tcp`
   and Darwin `lsof -F pn` — see Test E for the platform-specific
   implementation and the explicit CI coverage matrix).
5. **`mindspec otel setup` is the sole observability surface.** It
   accepts `--endpoint <url>`, optional `--protocol grpc|http/protobuf`,
   optional `--headers k=v,k=v`, optional `--codex`. It writes to
   `.claude/settings.local.json` and, with `--codex`,
   `~/.codex/config.toml`. It never starts, stops, restarts, queries,
   or probes any collector.

   **Explicit `--codex` contract:**
   - Target file: `~/.codex/config.toml` (override with
     `--codex-config <path>`). If the parent directory is absent,
     `mindspec otel setup --codex` creates it with mode `0700`.
   - **Merge semantics:** the target TOML is parsed; only the
     `[otel.exporter]` table (or whatever the canonical Codex
     stanza name is at implementation time — pinned in the
     `internal/otel/config.go` rendering layer) is replaced.
     Sibling top-level keys and other tables are preserved
     byte-for-byte. If the existing file fails to parse as TOML,
     exit code 1 is returned with `error: cannot merge into
     malformed TOML at <path>; back up the file and re-run` on
     stderr; the file is not modified.
   - **Secret hygiene for `--headers`:** values containing
     substrings matching `(?i)bearer|token|key|secret|password`
     are written to the target file's `[otel.exporter.headers]`
     table verbatim (the target file is the canonical store) but
     **redacted to `***` in any `mindspec otel status` output and
     in stderr error messages**. Users who wish to keep secrets
     out of `~/.codex/config.toml` may pass them via the
     `OTEL_EXPORTER_OTLP_HEADERS` env var at workload-launch time
     instead; `mindspec otel setup` documents this in `--help`.
   - **Idempotency:** re-running `mindspec otel setup
     --endpoint <same> --codex` against an unchanged
     `~/.codex/config.toml` produces a sha256-identical output
     file. This is asserted by the Acceptance Criteria sha256-
     idempotency bullet.
   - **Exit code matrix for `mindspec otel setup`:**
     - `0` — config written (or already up-to-date and re-write
       was idempotent).
     - `1` — pre-existing target TOML/JSON failed to parse;
       no modification performed.
     - `2` — invocation error (missing `--endpoint`, unknown
       `--protocol` value, etc.).
     - **No other exit codes**; `mindspec otel setup` never
       returns a network-error exit because it performs no
       network I/O.

   **Explicit `mindspec otel status` contract (read-only diagnostic):**
   - Reads (only): the current working directory's
     `.claude/settings.local.json` and the user's
     `~/.codex/config.toml` (and any path passed via
     `--codex-config`).
   - Writes (only): stdout (the report) and stderr (errors).
     `mindspec otel status` **performs zero network I/O** — this
     is asserted by Test H's AST check (no `net.Dial`, `http.Get`,
     `http.Post`, `http.Client.Do`, or `url.Parse`-followed-by-
     `Dial` call sites under `cmd/mindspec/otel.go` or
     `internal/otel/`). This closes the "doctor-by-another-name"
     hole: status is mechanically prevented from acquiring probe
     behavior.
   - Output (stable golden file under
     `cmd/mindspec/testdata/otel-status-golden.txt`): a short
     human-readable report listing:
     - configured OTLP endpoint (URL, scheme, host, port);
     - configured protocol;
     - configured headers (keys only, values redacted to `***`
       per the secret-hygiene rule above);
     - target files: which of `.claude/settings.local.json` and
       `~/.codex/config.toml` exist; for each, whether the OTEL
       stanza is present and matches the canonical mindspec
       rendering.
   - **Exit code matrix for `mindspec otel status`:**
     - `0` — config present and parseable in at least one target
       file; report printed.
     - `1` — no OTEL config found in any target file.
     - `2` — at least one target file exists but fails to parse;
       report indicates which.
     - **No other exit codes.**
6. **`mindspec --help` is observability-name-free.** None of
   `agentmind`, `bench`, `serve`, `replay`, `viz` appear in
   `mindspec --help` output. `mindspec --help` fits on one screen.
   `mindspec otel --help` lists exactly two subcommands: `setup` and
   `status`.
7. **One-shot deprecation messages on removed commands.** Invoking any
   removed top-level command (`mindspec viz`, `mindspec agentmind
   serve`, `mindspec agentmind replay`, `mindspec agentmind setup`,
   `mindspec bench …`) prints exactly one stderr line of the form:
   `command moved: install <binary> from <url> (see ADR-0027/0028)`
   and exits with code 2. This is the ONLY backwards-compatibility
   affordance. **No shim, no re-exec, no auto-install prompt, no
   feature flag, no multi-release deprecation window.** The
   deprecation messages live for exactly one mindspec release after
   this spec ships; a single-line follow-up removes them in the next
   release.
8. **Every commit builds and tests green.** Every commit in the
   migration sequence (commits 1 through N inside the single PR)
   leaves `go build ./cmd/mindspec` and `go test -short ./...`
   passing. CI enforces on each commit, not just on the merge commit.
   Mid-PR rollback requires no atomic-cutover ceremony.
9. **Permanent CI gate against re-introduction.** The specgate test
   (`internal/specgate/verify_no_agentmind_dep_test.go`) runs in every
   `go test -short` invocation and fails the build if any agentmind
   import (direct or transitive) re-enters the graph, or if any
   `exec.Command` literal targets agentmind. **This is not a one-shot
   merge-time check; it is the architecturally load-bearing invariant
   for the lifetime of the repo.**
10. **Binary-size shrinkage ≥30% is a merge gate.** `go build`
    output size is measured before and after the migration. Shrinkage
    less than 30% fails the merge. The delta is recorded in the merge
    commit message. This is C01's anti-symbolic-extraction defense:
    if real code is hiding behind renames, the binary won't shrink.

    **Pinned baseline (the numeric anchor for the gate):** measured
    on `darwin-arm64` from `main` HEAD immediately prior to spec-084
    work, `go build ./cmd/mindspec` produces a binary of
    **10,734,354 bytes (~10.24 MiB)**. The merge-time check computes
    `1 - (post_bytes / 10734354)` and fails if the result is less
    than `0.30` (post-spec-084 binary must be **≤ 7,514,047 bytes**).
    The number is pinned here so the gate is falsifiable at review
    time, not negotiable post-hoc. If a legitimate Go runtime change
    moves the baseline (e.g., a toolchain bump) before this spec
    lands, re-measure on the *same* commit that introduces the
    toolchain bump and update this number in a revision-bead PR
    before approving spec 084.

    **Calibration of the 30% floor.** The deleted code closure is:
    `internal/bench/` (~3,500 LOC), `cmd/mindspec/viz.go` (~220 LOC),
    `cmd/mindspec/bench*.go` (~200 LOC),
    `internal/recording/collector.go` (~250 LOC), plus the entire
    `github.com/mrmaxsteel/agentmind/{client,wire}` Go module and
    its transitive closure (websocket dep `nhooyr.io/websocket`, the
    `internal/viz/web/*` embed.FS UI assets that the agentmind
    binary now carries instead of mindspec). Expected shrinkage on
    this surface is 35–45%; **30% is the conservative merge floor**.
    A measured shrinkage between 30% and 35% is acceptable but
    flagged in review as suggesting incomplete dead-code elimination.
    A measured shrinkage below 30% indicates real code is still
    hiding behind renames and is a hard merge-block per HC #10.
11. **Rescue-note discipline — survives squash-merge.** The deletion
    commits (bench, viz cobra subtree, recording collector) each cite,
    in their commit message, the file paths deleted. Because this PR
    is permitted to land via squash-merge (see §Migration commits),
    parent-SHA citations in PR-fork commits do **not** survive into
    `main`'s history and would silently break six-month-old rescue
    procedures. To prevent that, this spec requires one of the
    following three SHA-preservation mechanisms; the implementer
    picks **option (b)** as the default unless they explicitly
    document choosing another:

    - **(a) Require a true merge commit (no squash).** The PR is
      merged with `git merge --no-ff`; the seven intermediate
      commits remain in `main`'s history and their parent SHAs
      survive verbatim. `BENCH-MOVED.md` cites the pre-bench-delete
      commit SHA from that history.

    - **(b) Annotated tag at the pre-delete state (default).** Before
      the bench-delete commit lands, the implementer pushes an
      annotated tag `pre-spec-084-bench-delete` pointing at the
      parent SHA. `BENCH-MOVED.md` cites the tag, not a raw SHA:
      `git show pre-spec-084-bench-delete:internal/bench/runner.go`.
      Tags are pushed to `origin` from the integration branch BEFORE
      the squash-merge so they exist in shared history independent
      of PR-fork lifecycle. This is the default because it survives
      both squash and merge-commit, and is the minimum-friction
      addition to the existing flow.

    - **(c) Inline-rescue note carrying the full directory.**
      `BENCH-MOVED.md` includes a `git format-patch` invocation
      against the squash-merge commit's parent that reconstructs
      `internal/bench/` from `main`'s reachable history. Verbose;
      acceptable only if the implementer rejects both (a) and (b).

    The deletion commit messages still cite their parent SHAs as
    review aids, but the **canonical rescue handle is the
    `pre-spec-084-bench-delete` annotated tag** unless (a) or (c)
    is chosen and documented in `BENCH-MOVED.md`.

### Per-command migration table (per C03)

Every removed command maps to its replacement (or to "no replacement,
see ADR"). This table is duplicated in the README "Telemetry" section
and the CHANGELOG release notes; it is also the source of truth for
the one-shot deprecation messages in Hard Constraint #7.

| Removed command | Replacement | Deprecation message |
|---|---|---|
| `mindspec record start --…` (old shape) | `mindspec record start --spec <id> -- <workload-cmd>` (simplified to pure config + launch; see flag enumeration below) | n/a — `record start` survives, reshaped per BRIEF task #4 |
| `mindspec bench`, `mindspec bench run`, `mindspec bench *` | No replacement in mindspec. See `BENCH-MOVED.md` for the git-history rescue procedure; future bench-repo author lifts code from cited SHA. | `command moved: 'mindspec bench' has moved out of mindspec; see BENCH-MOVED.md (or ADR-0028) for rescue procedure` |
| `mindspec agentmind serve` | `agentmind serve` (standalone binary; install from https://github.com/mrmaxsteel/agentmind/releases) | `command moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0027)` |
| `mindspec agentmind replay` | `agentmind replay` (standalone binary) | `command moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0027)` |
| `mindspec viz` (top-level alias) | `agentmind serve` (standalone binary) | `command moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0027)` |
| `mindspec agentmind setup` | `mindspec otel setup` (renamed; no backwards-compat alias) | `command renamed: use 'mindspec otel setup' (see ADR-0027 for rationale)` |
| `mindspec agentmind` (any other subcommand) | n/a — entire cobra subtree removed | `command moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0027)` |

### `record start` flag-shape enumeration (per C03)

The spec must be the source of truth for which `record start` flags
survive and which are rejected. An audit of `cmd/mindspec/record.go`
on `main` at the time of this spec (parent commit
`1eb9782 sec: validate spec/ADR/bead/domain IDs at all CLI entrypoints`)
shows `recordStartCmd` exposes exactly **one** flag:

| Flag | On main today | Post-spec-084 disposition |
|---|---|---|
| `--spec <id>` | yes (required) | **Preserved** — still required; identifies the spec the recording belongs to. |

No other flags exist on `record start` in the current codebase, so
no subprocess-management flag deprecation enumeration is required
beyond this single-row table. **If, during implementation, any
hidden flag is discovered in `record.go` that this audit missed**
(e.g., a hidden bench-coupling flag inherited from a refactor), the
implementer MUST update this table in the spec by amendment-bead
BEFORE the bench-delete commit lands, and the flag MUST either
survive (with documented rationale) or emit exit code 2 with a
stderr line `flag removed: --<name>; see ADR-0027 §record-start` —
no silent acceptance. This closes the R1:C7 / R5:C5 latent-surface
hole: the spec is the binding enumeration.

### Surface to remove — exact file inventory

**Deleted from `mindspec/`:**

| Path | Lines (approx.) | Notes |
|---|---|---|
| `cmd/mindspec/viz.go` | ~220 | Removes the entire `mindspec agentmind` cobra subtree (`serve`, `replay`, `setup`, `setup codex`) and the top-level `mindspec viz` alias. |
| `cmd/mindspec/bench.go`, `cmd/mindspec/bench_*.go` | ~200 | Removes `mindspec bench run` and siblings. |
| `internal/bench/` (whole directory: `collector.go`, `runner.go`, `session.go`, `markdown.go`, `qualitative.go`, `report.go`, `worktree.go`, plus tests) | ~3,500 | Bench subsystem leaves mindspec. Rescue via `git show <pre-delete-SHA>:internal/bench/`. |
| `internal/recording/collector.go` | ~250 | OTLP/NDJSON reader is no longer mindspec's concern. |
| `internal/recording/collector_test.go` | — | Companion test. |
| `internal/agentmind/` (if any remnant survived 083) | — | Belt-and-suspenders; the grep proof catches anything missed. |

**Modified in `mindspec/`:**

| Path | Notes |
|---|---|
| `cmd/mindspec/record.go` | Shrinks to ~80 lines. No `agentmind` subcommand reference. Loads OTEL config, sets env, execs workload, writes manifest + phase markers, exits with workload's status. |
| `cmd/mindspec/root.go` | Drops the `agentmindCmd` / `benchCmd` / `vizCmd` registrations. Adds `otelCmd`. Registers the hidden deprecated-commands group from `deprecated_commands.go`. |
| `go.mod` / `go.sum` | Drop `github.com/mrmaxsteel/agentmind`. Run `go mod tidy`. |
| `README.md` | Single section update: "Telemetry" now says "point mindspec at any OTLP/HTTP endpoint via `mindspec otel setup --endpoint …` or `OTEL_EXPORTER_OTLP_ENDPOINT=…`. Anything that speaks OTLP works — agentmind, Honeycomb, Tempo, Jaeger, opentelemetry-collector-contrib." Embeds the per-command migration table verbatim. |
| `.mindspec/docs/adr/ADR-0011.md` | Prose postscript reflecting the no-Go-dep posture. |

**Added to `mindspec/`:**

| Path | Lines (approx.) | Notes |
|---|---|---|
| `cmd/mindspec/otel.go` | ~120 | `mindspec otel setup` + `mindspec otel status` (read-only diagnostic). |
| `cmd/mindspec/deprecated_commands.go` | ~50 | One-shot exit-2 stubs for the table above. Lives for exactly one release. |
| `internal/otel/config.go` | ~150 | Render OTEL endpoint into Claude Code settings, Codex config, and raw env exports. No network calls. |
| `internal/otel/config_test.go` | ~200 | Pure-function tests for the rendering paths. sha256 idempotency assertion. |
| `internal/specgate/verify_no_agentmind_dep_test.go` | ~100 | Permanent CI gate: dep-graph + AST `exec.Command` literal check (per C05). |
| `docs/adr/ADR-0027-mindspec-otel-config-only.md` | — | The "mindspec is OTEL-config only" ADR. |
| `docs/adr/ADR-0028-bench-removed-from-mindspec.md` | — | The "bench is gone, here is the rescue procedure" ADR. |
| `BENCH-MOVED.md` (repo root) | ~30 | Pointer to the `pre-spec-084-bench-delete` annotated tag and the `git checkout <tag> -- internal/bench/` rescue command. Tag is pushed to `origin` before the spec-084 PR merges so the rescue handle survives squash-merge. |

## Scope

### In Scope

- All deletions and modifications above.
- Removal of the `agentmind` Go module dep from `go.mod` and `go.sum`.
- The single `mindspec otel setup`/`status` command pair, with its
  rendering layer in `internal/otel/`.
- The permanent CI gate at `internal/specgate/verify_no_agentmind_dep_test.go`.
- The one-release deprecation messages in `cmd/mindspec/deprecated_commands.go`
  (and the follow-up bead to delete them in the next release).
- Two new ADRs (0027, 0028) and one ADR-0011 prose postscript.
- A README rewrite of the Telemetry / Observability section including
  the per-command migration table.
- A single migration sequence (see §Migration commits) that lands in
  one PR with each commit green.

### Out of Scope

- **Extracting `internal/bench/` to a new repository.** This spec
  deletes bench from mindspec; whether and where bench reappears is
  somebody else's spec. The rescue procedure is one git command and
  is documented in `BENCH-MOVED.md` and ADR-0028.
- A first-party `mindspec install <collector>` subcommand. mindspec
  does not download, install, or version-pin any collector binary.
  (Defers the same way spec 083 deferred `mindspec install agentmind`.)
- Any change to spec/plan/lifecycle commands (`spec init`, `plan
  approve`, `impl approve`, etc.). Those are mindspec's core and are
  untouched.
- Any change to the agentmind repo. agentmind already exists at
  `v0.x`; this spec does not require any agentmind change to land.
- A control plane between mindspec and any collector. mindspec talks
  to collectors exclusively by *writing config the workload reads*.
- A `--target=shell` rendering mode for `mindspec otel setup` (Open
  Question; can be added in a point release).
- A `--validate` flag on `mindspec otel setup` that probes the
  endpoint (forbidden by Hard Constraint #4: no mindspec process speaks
  OTLP).

### Non-Goals

- No version-skew handling between mindspec and downstream collectors.
  Workloads emit OTLP; collectors consume OTLP; OTLP is the contract.
- No "graceful degradation when agentmind is missing" warn-line
  behavior. mindspec has no awareness of agentmind, so the question
  does not arise. If a user configures an endpoint and nothing is
  listening there, the workload's OTEL SDK drops events silently —
  exactly as it would for any other OTEL endpoint.
- No retention of the `mindspec agentmind setup` command name. It is
  renamed to `mindspec otel setup` (no backwards-compat alias). The
  user's vision treats agentmind as one collector of many; mindspec
  must not name it specially in the CLI.
- No multi-release feature flag. The deprecation messages exist for
  exactly one release and then go away.
- No graceful-degradation contract on `mindspec record start` when no
  endpoint is configured. If the workload's own OTEL SDK is given an
  empty endpoint, that's the workload's problem; mindspec exits with
  the workload's exit code regardless.

## Acceptance Criteria

- [ ] `grep -rn "github.com/mrmaxsteel/agentmind" .` returns no
      matches in the mindspec tree (excluding allow-listed paths:
      `.mindspec/docs/specs/083-*`, ADR-0026/0027/0028 prose, CHANGELOG).
- [ ] `grep -rn 'agentmind\|wire\.CollectedEvent\|client\.AutoStart\|client\.ReadEvents\|client\.RunStandalone' cmd/ internal/ --exclude='*_specgate*.go' --exclude='*verify_no_agentmind*.go' --exclude='deprecated_commands*.go'`
      returns zero matches. (The two allow-listed files —
      `internal/specgate/verify_no_agentmind_dep_test.go` and
      `cmd/mindspec/deprecated_commands.go` — are by-design exceptions
      per Hard Constraint #2; Test H further gates them to
      string-literal-only contexts.)
- [ ] `find internal/bench -type f` returns no results.
- [ ] `find internal/agentmind -type f` returns no results.
- [ ] `find internal/recording -name 'collector*'` returns no results.
- [ ] `find cmd/mindspec -name 'viz*.go' -o -name 'bench*.go'` returns
      no results.
- [ ] `mindspec --help` contains none of these tokens: `agentmind`,
      `bench`, `serve`, `replay`, `viz`. `mindspec otel --help` lists
      exactly two subcommands: `setup` and `status`.
- [ ] `go build ./cmd/mindspec` succeeds with `GOFLAGS=-mod=readonly`
      after `go mod tidy`, and `go test -short ./...` passes.
- [ ] `go list -m all | grep mrmaxsteel` lists only
      `github.com/mrmaxsteel/mindspec` itself.
- [ ] `go list -deps ./... | grep -c mrmaxsteel/agentmind` outputs `0`.
- [ ] With `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535` (a
      port with nothing listening), `mindspec record start --spec test
      -- echo hi` exits 0, prints `hi`, and writes the expected
      manifest + `recording/` skeleton. No mindspec stderr mentions
      OTEL, agentmind, or telemetry.
- [ ] With `mindspec otel setup --endpoint http://collector.example:4318`
      run in a fresh repo: `.claude/settings.local.json` contains the
      OTEL endpoint exactly once; **re-running with identical inputs
      yields a sha256-identical file** (per C03 sha256 idempotency).
- [ ] With `mindspec otel setup --endpoint … --codex`: Codex
      `~/.codex/config.toml` contains the matching `otel.exporter`
      stanza; sha256-idempotent on re-run.
- [ ] `mindspec viz`, `mindspec agentmind serve`, `mindspec agentmind
      replay`, `mindspec agentmind setup`, `mindspec bench run` each
      exit with code 2 and print exactly one stderr line matching the
      per-command migration table.
- [ ] **Binary-size shrinkage ≥30%.** mindspec binary size is recorded
      before and after in the final commit message; the merge is
      blocked if shrinkage is less than 30%.
- [ ] `internal/specgate/verify_no_agentmind_dep_test.go` exists,
      runs in `go test -short ./...`, and passes.
- [ ] ADR-0027 and ADR-0028 are committed and cross-referenced from
      this spec.

## Validation Proofs (Tests A–J per C01, with C02/C05 additions; K as safety belt)

These are runtime + static checks that gate the merge. Static greps
caught spec-083 candidates who left dead code in place; runtime checks
catch the rest. **Every test below either runs in CI on every commit
or runs once at merge time.** Static-only signals are insufficient.

- **Test A — no-agentmind-import check (static, CI permanent):**
  `go list -deps ./cmd/mindspec | grep -q mrmaxsteel/agentmind`
  returns no match. Implemented as part of
  `internal/specgate/verify_no_agentmind_dep_test.go` (runs in every
  `go test -short`).
- **Test B — go.mod cleanliness:**
  `grep -c "mrmaxsteel/agentmind" go.mod go.sum` returns `0` for both
  files. `go mod tidy` is a no-op after the migration.
- **Test C — help-surface check (golden-file diff):**
  `./bin/mindspec --help` is captured and asserted *not* to contain
  `agentmind`, `bench`, `serve`, `replay`, or `viz`. Golden file
  committed under `cmd/mindspec/testdata/help-golden.txt`.
- **Test D — deprecation message contract (AST-checked):**
  For each of the removed top-level invocations
  (`mindspec viz`, `mindspec agentmind serve`,
  `mindspec agentmind replay`, `mindspec agentmind setup`,
  `mindspec bench run`), exec the mindspec binary and assert exit
  code 2 and exactly one stderr line matching the documented
  per-command migration table pattern. AST-checked via a Go test in
  `cmd/mindspec/deprecated_commands_test.go`.
- **Test E — record opens no listener (portable Go runtime check,
  per C01):**
  Replaces the original lsof/dtruss design (platform-fragile;
  dtruss requires SIP-disabled hosts on modern Darwin and lsof
  flag semantics diverge between BSD and Linux).
  Implemented as a portable Go test in
  `internal/recording/no_listener_test.go` that:
  1. Spawns `mindspec record start --spec test -- bash -c 'sleep 2'`
     with the workload env containing `OTEL_EXPORTER_OTLP_ENDPOINT`.
  2. Enumerates mindspec's open TCP sockets via the platform-
     appropriate kernel interface:
     - **Linux**: parses `/proc/<mindspec-pid>/net/tcp` and
       `/proc/<mindspec-pid>/net/tcp6` and filters for state `0A`
       (LISTEN); no external tool required.
     - **Darwin**: invokes `lsof -p <mindspec-pid> -iTCP -sTCP:LISTEN
       -n -P -F pn` (the `-F` formatted output is stable across
       lsof versions and does not require dtruss / ptrace
       privileges); skipped with `t.Skip("lsof not present")` if
       lsof is absent.
     - **Other OSes**: `t.Skip("no portable listener-enumeration
       implementation on $GOOS")`.
  3. Asserts mindspec opens **zero TCP listening sockets** of its
     own across the workload's lifetime (sampled every 100ms until
     workload exit).

  **CI coverage matrix is explicit:** Test E runs on both
  `ubuntu-latest` and `macos-latest` GitHub-hosted runners; both
  must report zero listeners for the merge gate to clear. A skip
  on either runner is a CI failure (the test uses `t.Errorf` on
  unexpected skip, not `t.Skip`, when `GOOS` is `linux` or
  `darwin`). No SIP-disabled / root / ptrace privileges are
  required; Test E will not silently no-op.
- **Test F — record start exit-code propagation + clean stderr (the
  user's literal vision):**
  Start a fresh tmp repo. `export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535`
  (nothing listening). Run
  `./bin/mindspec record start --spec test -- bash -c 'echo workload;
   exit 42'`.
  Assert exit code 42 (workload's exit code propagated), stdout
  contains `workload`, stderr is empty (no telemetry-related
  warning), and the recording manifest exists on disk.
- **Test G — process tree audit (per C02 `pgrep -P <pid>`):**
  During Test E or Test F, snapshot `pgrep -P <mindspec-pid>` after
  200ms. The only child of mindspec must be the workload process
  itself (`bash` in the test). There must be **no process named
  `agentmind`**, no `otelcol`, no other collector subprocess. The
  workload may itself be the only child; nothing else.
- **Test H — no-spawn AST check (per C05):**
  AST-walk every `*.go` file under `cmd/` and `internal/`, enumerate
  `exec.Command`, `exec.LookPath`, `os.StartProcess` first-argument
  literals, and assert none equal `"agentmind"`. Runs as part of
  `internal/specgate/verify_no_agentmind_dep_test.go` (perpetual
  CI gate).
- **Test I — binary-size floor (≥30% shrinkage, merge gate):**
  Compare `go build ./cmd/mindspec` output size against the
  pre-merge baseline recorded in the spec. **Fail the merge if
  shrinkage is less than 30%.** Delta recorded in the final commit
  message.
- **Test J — point-at-a-real-collector check (MANDATORY,
  end-to-end proof of the user vision):**
  Start `otel/opentelemetry-collector-contrib` locally with the
  `loggingexporter` (pulled from the official OTel image; pinned
  by digest in `.github/workflows/spec-084-test-j.yml`).
  `mindspec otel setup --endpoint http://127.0.0.1:4318`. Run a
  workload via `mindspec record start --spec test -- <fixture
  script under internal/otel/testdata/emit-one-otlp-log.sh that
  emits one OTLP log via curl>`. Assert the collector's stdout
  contains the emitted log line. **No mindspec process touches
  OTLP at any point in this test path** — this is the literal
  proof of the user's one-sentence vision ("point mindspec at an
  OTEL collector and that's it") and the test the spec exists to
  satisfy. Promoted from optional to **mandatory** per panel
  consensus: the v1-spec-084 design treats the end-to-end check
  as the load-bearing acceptance proof, not aspirational
  documentation.

  **CI cadence:** Test J runs in `.github/workflows/spec-084-test-j.yml`
  on every PR that touches `cmd/mindspec/`, `internal/otel/`,
  `internal/recording/`, or `go.mod`. If per-PR docker-pull cost is
  prohibitive in practice (measured after first ten runs), the
  cadence may be relaxed to "nightly + on PRs touching the above
  paths" by spec-amendment bead, but it MUST run at least nightly
  on `main` and a failure MUST block the next release. It is never
  silently optional.
- **Test K — bench-rescue procedure (squash-merge resilient):**
  In a clean checkout of `main` after the spec-084 PR merges,
  `git show pre-spec-084-bench-delete:internal/bench/runner.go |
  head -n 20` returns the old file (assuming HC #11 default
  option (b) — annotated tag). The tag must be reachable from
  `origin` and not be a PR-fork-only ref. If option (a)
  (merge-commit) was chosen instead, the test uses the cited
  SHA from `BENCH-MOVED.md` directly. The test fails if neither
  the tag nor a `main`-reachable SHA can resurrect the file —
  the very failure mode this revision exists to prevent.

Passing Tests A–J (with K as safety belt) is the definition of
"mindspec is OTEL-config only and the boundary is architecturally
enforced." Test J is the load-bearing literal-vision proof and is
mandatory; K is the rescue-procedure safety belt.

## Migration commits (single PR, bench-first per C02)

Single PR, seven commits, **each green** (per Hard Constraint #8).
The PR is squash-merged or merge-commit-merged; the intermediate
commits are review aids. Commit ordering follows C02's bench-first
rationale: deleting `internal/bench/` first collapses the surface
that subsequent commits must touch by ~60%, because bench is the
dominant remaining consumer of `client.AutoStart` /
`client.ReadEvents` / `wire.CollectedEvent`.

- **Commit 1 — Add `internal/otel/`, `cmd/mindspec/otel.go`.**
  New surfaces land first. Existing `mindspec agentmind setup`
  stays in place for two commits so users can verify equivalence
  by diffing the output of the old vs. new command on the same
  flags. **The specgate test
  (`internal/specgate/verify_no_agentmind_dep_test.go`) is NOT
  landed in this commit.** Per panel consensus revision #9, the
  build-tag silencing pattern is rejected: a "permanent gate" that
  is off for five of seven commits is the spec-083 failure mode
  this spec exists to prevent. Tests A–C and Test H's draft AST
  scaffolding may be wired into ordinary test files in Commit 1
  (they assert what is true at that commit: `agentmind` IS still
  in the source and dep graph), but the specgate test itself —
  whose contract is "agentmind is never in the graph" — is
  introduced only in Commit 6 below, alongside the actual dep
  removal, so its first appearance in the repo is its permanent
  enforced form.
- **Commit 2 — Delete `internal/bench/` and `cmd/mindspec/bench*.go`.**
  `BENCH-MOVED.md` lands in the same commit at the repo root. Commit
  message cites the parent SHA explicitly for the rescue procedure.
  This is the **bench-first commit** per C02. `go mod tidy` is **not**
  run yet — the agentmind dep still appears to be used by the
  surviving callers in `internal/recording/` and `cmd/mindspec/viz.go`.
- **Commit 3 — Delete the `mindspec agentmind` cobra subtree.**
  Removes `cmd/mindspec/viz.go` and its `init()`-time registration.
  Drops `mindspec agentmind serve|replay|setup` and `mindspec viz`
  from `--help`. The agentmind binary already exists standalone;
  users who want viz run it directly.
- **Commit 4 — Delete `internal/recording/collector.go` and rewrite
  `cmd/mindspec/record.go`.**
  `mindspec record start` becomes a workload launcher with manifest
  + phase-marker bookkeeping and an OTEL-env-injection step. No
  subprocess management, no NDJSON reader. **After this commit, the
  only `client.*` callers should be gone — verified by Hard
  Constraint #2 grep.**
- **Commit 5 — Add `cmd/mindspec/deprecated_commands.go` with one-shot
  exit-2 stubs.** Registers hidden cobra commands for `agentmind`,
  `viz`, `bench`, `serve`, `replay` emitting the per-command migration
  table messages from Hard Constraint #7. Test D becomes green.
- **Commit 6 — Drop the `agentmind` Go module dep and land the
  permanent specgate test.**
  `go.mod` and `go.sum` edits, `go mod tidy`. **Add
  `internal/specgate/verify_no_agentmind_dep_test.go` in this
  commit** (its first appearance in the repo) so the test's
  initial state is its permanent enforced state: no build tags,
  no skips, runs unconditionally in `go test -short ./...` from
  this commit forward. Binary-size delta recorded in the commit
  message. Tests A, B, H all green.
- **Commit 7 — ADRs, README, CHANGELOG.**
  ADR-0027, ADR-0028, ADR-0011 prose postscript, README "Telemetry"
  section rewrite with the per-command migration table embedded.
  CHANGELOG entry names every removed/renamed command and links to
  the install instructions for agentmind. Closes the spec.

If any commit fails CI, fix forward — no atomic cutover, no
`--no-verify`.

## Lessons baked in from spec 083 and the v1 panel

- **Don't measure progress by what was moved; measure by what no
  longer exists in the mindspec tree.** Spec 083 left load-bearing
  remnants precisely because its acceptance criteria asked "did the
  move happen?" rather than "is the surface gone?" This spec inverts
  the test: every acceptance criterion is a negated grep, AST check,
  runtime no-listener proof, binary-size delta, or help-surface diff.
- **Don't extract when you can delete.** Spec 083 spent thousands of
  lines on a clean extraction of the OTLP receiver and viz UI; that
  was appropriate because those subsystems had a downstream owner
  (the agentmind binary) ready to receive them. Bench has no such
  owner yet. Extracting bench to a not-yet-existing repo is strictly
  slower than deleting it with a one-command rescue note. The user
  said "destined for its own repo" — i.e. not mindspec's concern.
  Take them at their word; delete; let the future bench-repo author
  lift the code from git history.
- **Don't preserve a name that biases the architecture.** Keeping
  `mindspec agentmind setup` would have told every future reader
  "agentmind is the special collector." Renaming to `mindspec otel
  setup` makes the user's "agentmind is one of many" stance visible
  in the CLI.
- **Make the boundary permanent, not just landed.** Spec 083's
  acceptance criteria were one-shot grep checks at merge time;
  nothing in that spec prevented an accidental re-import of
  `agentmind/client` six months later. This spec installs a
  permanent `go test`-time gate
  (`internal/specgate/verify_no_agentmind_dep_test.go`) so any
  future commit that re-introduces the dep — direct or transitive
  — fails CI immediately.
- **Binary size is a quality signal, not a vanity metric.** A
  symbolic deletion (rename without removing the import closure)
  leaves the binary nearly identical in size. C01's 30% shrinkage
  floor is the strongest single anti-symbolic-extraction defense.
  Promoted here from observation to merge gate.
- **One-shot deprecation messages over multi-release feature flags.**
  C09 proposed a three-release `MINDSPEC_OBSERVABILITY=embedded|external`
  feature flag to gradually sunset the embedded mode. That
  preserves the very code paths the spec exists to remove for
  6–8 weeks, doubles the CI matrix, and defers a decision the
  user has already made. Take the least-objectionable element of
  C09 — clear one-line stderr messages on removed commands — and
  time-box it strictly to one release. Then delete the deprecation
  file in a follow-up.

## Open Questions

- **Does `mindspec otel setup` need a `--target=shell` rendering mode?**
  Three reviewers (R1, R3, R4) cited C04's idea of a third
  rendering target that prints `export KEY=VALUE` lines on stdout
  for `eval $(mindspec otel setup --target=shell …)` workflows.
  Resolution: out of scope for v1 of this spec; can be added in a
  point release without breaking anything. The two write-targets
  the brief requires (Claude Code settings + Codex config) ship
  here.
- **Does the workload-launching contract for `mindspec record start`
  break any existing recipe?** Pre-spec, `mindspec record` had
  multiple flag shapes; post-spec, it's `mindspec record start
  --spec <id> -- <cmd…>`. Resolution: a grep of the mindspec docs +
  the `bench/v2` experiment invocations in the parent project. If
  any caller used a form this spec drops, document the new
  equivalent in the README "Telemetry" section and add to the
  per-command migration table.
- **Is there value in a `mindspec otel doctor` subcommand that does
  a one-shot OTLP probe of the configured endpoint?** Resolution:
  tempting but violates Hard Constraint #4 (no mindspec process
  speaks OTLP). If the user wants a probe, `curl -X POST
  $OTEL_EXPORTER_OTLP_ENDPOINT/v1/logs` is one line and outside
  mindspec's scope. Closed as "intentionally not added."
- **When are the deprecation messages deleted?** Hard Constraint #7
  time-boxes them to "one release." Resolution: a single-bead
  follow-up after this spec ships deletes
  `cmd/mindspec/deprecated_commands.go` and updates Test D to
  assert cobra's default "unknown command" exit. Tracked but not
  part of this spec's acceptance.

## Estimated effort

- Commit 1 (otel command + renderer + specgate skeleton): half a day.
- Commit 2 (delete bench + rescue note): an hour, mostly typing the
  commit message correctly. Bench-first per C02.
- Commit 3 (delete viz cobra subtree): an hour.
- Commit 4 (rewrite `record.go` + delete `collector.go`): half a day.
- Commit 5 (deprecated_commands.go + Test D): a couple of hours.
- Commit 6 (go.mod / go.sum / tidy + specgate enabled): an hour.
- Commit 7 (ADRs + README + CHANGELOG + migration table): half a
  day.
- Validation suite (Tests E, G, J runtime harness): half a day.

Total: **~2.5–3 engineer-days end to end**, single PR, no external
repo prerequisite. The smallness is the point; the rigor is in the
gates, not the calendar.

## Approval

- **Status**: Draft (synthesized winner — C10 spine + C01 rigor +
  C05 permanence + C02 ordering + C03 migration table + C09
  one-shot deprecation message)
- **Stance**: SYNTHESIS — smallest set of changes that makes the
  user's one-sentence vision literally true, with the strongest
  validation discipline the panel could assemble, and a permanent
  CI gate that prevents regression for the lifetime of the repo.
- **Approved By**: Panel consensus (5/6 C10 majority; R2 dissent
  on bench-disposition only — see CONSENSUS.md)
- **Approval Date**: 2026-05-19
