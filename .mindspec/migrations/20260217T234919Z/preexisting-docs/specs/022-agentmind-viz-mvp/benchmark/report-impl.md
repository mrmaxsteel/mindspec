# Benchmark (Implementation Phase): 022-agentmind-viz-mvp

**Date**: 2026-02-14
**Phase**: Implementation (resumed from phase-1 plan/spec artifacts)
**Timeout**: 1800s per attempt
**Max Retries**: 3
**Model**: default

## Sessions

| Session | Description | Port | Events |
|---------|-------------|------|--------|
| A (no-docs) | No CLAUDE.md/.mindspec; hooks stripped; no docs/ — given plan-a.md | 4318 | 85 |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped; docs/ present — given plan-b.md | 4319 | 93 |
| C (mindspec) | Full MindSpec tooling — follows /spec-approve → plan → /plan-approve → implement | 4320 | 213 |

## Quantitative Comparison

```
Metric                         no-docs        baseline        mindspec
──────────────────────────────────────────────────────────────────────
API Calls                            0               0               0
Input Tokens                     32084           31568          125953
Output Tokens                    38371           39654           62311
Cache Read Tokens              1729854         1668796         9381143
Cache Create Tokens              79981           89315          187265
Total Tokens                     70455           71222          188264
Cost (USD)                     $1.9282         $2.1298         $7.4909
Duration                       7.3 min         7.6 min        19.2 min
Cache Hit Rate                   93.9%           93.2%           96.8%
Output/Input Ratio               1.20x           1.26x           0.49x

Per-Model Breakdown            no-docs        baseline        mindspec
──────────────────────────────────────────────────────────────────────
  claude-haiku-4-5-20251001
    Tokens In                    32053           31530          125762
    Tokens Out                    8010            4964            2728
    Cost                       $0.1391         $0.1030         $0.1394
  claude-opus-4-6
    Tokens In                       31              38             111
    Tokens Out                   30361           34690           51384
    Cost                       $1.7892         $2.0268         $6.3919
  claude-sonnet-4-5-20250929
    Tokens In                        0               0              80
    Tokens Out                       0               0            8199
    Cost                       $0.0000         $0.0000         $0.9596

```

## MindSpec Trace Summary (Session C)

```
Trace Summary: /var/folders/cw/tdy8s59d077dyky0b0qk9pxc0000gq/T/mindspec-bench-022-agentmind-viz-mvp-impl/trace-c.jsonl
  Events:     46
  Duration:   3628.3 ms
  Tokens:     0

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  bead.cli                      18  1867.5 ms        -
  command.end                    7  1760.8 ms        -
  command.start                 16          -        -
  state.transition               5          -        -

```

## Qualitative Analysis

Now I have comprehensive analysis of all three sessions. Let me synthesize the comparison.

---

### Planning Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 4/5 | Detailed 10-step plan with clear tech stack (Node.js/TypeScript), file structure, mapping tables, and verification steps. Well-organized but made a fatal architectural decision: choosing Node.js over Go violates the project's ADR-0004 (Go as implementation language). Without docs, the agent had no way to know this. |
| B (baseline) | 4/5 | Solid plan with 11 files, clear implementation order, and explicit verification steps. Correctly chose Go + `nhooyr.io/websocket` + embedded HTML. Mentions "same pattern as `internal/instruct/instruct.go:15`" for embed, showing some codebase awareness. However, planned to duplicate OTLP parsing rather than reusing it. |
| C (mindspec) | 5/5 | Structured plan with YAML frontmatter, ADR fitness analysis (ADR-0003, ADR-0004), 6 beads with explicit dependency graph, and per-bead verification checklists. The ADR citations caught the "Go implementation" and "CLI-first" constraints that guided architectural decisions. Bead decomposition maps cleanly to the implementation. |

### Architecture

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 2/5 | Chose Node.js/TypeScript — a completely separate application in `viz/` with no integration into the Go binary. Violates the single-binary distribution model. Would require users to install Node.js + npm. The backend architecture (Express + WS) is clean but wrong for this project. |
| B (baseline) | 3/5 | Correct language (Go) and CLI integration (`mindspec viz serve/replay`). But couples graph state management inside the Normalizer struct, duplicates 157 lines of OTLP parsing from `bench/collector.go`, and uses hard-coded limits. Monolithic 1008-line embedded HTML. |
| C (mindspec) | 5/5 | Clean separation: `graph.go` (state), `normalize.go` (pure function), `hub.go` (WebSocket), `live.go` (OTLP receiver), `server.go` (HTTP). Reuses existing `bench.ExtractLogEvents()` via exported wrappers (10 lines added vs 157 duplicated). Configurable `GraphConfig` with defaults. Modular frontend with vendored Three.js. |

### Code Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 3/5 | Clean TypeScript with proper types and reasonable structure, but committed `node_modules`, incomplete frontend (scaffold only), and no documentation on how to run. The backend code itself is readable and well-organized. |
| B (baseline) | 4/5 | Idiomatic Go, proper mutex usage, sensitive key stripping, graceful shutdown. Deducted for code duplication (`types.go` copying OTLP parsing) and coupling normalization to state management. Error handling is adequate. |
| C (mindspec) | 5/5 | Uses `sync.RWMutex` for read-heavy workloads, atomic counters for concurrent stats, pure normalization function, exported wrappers for code reuse. Sampling logic in LiveReceiver handles high event rates. Backpressure tracking with dropped counter. Well-structured Client with ReadPump/WritePump pattern. |

### Test Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 2/5 | Test files created for normalizer and graph-state but incomplete. No evidence tests pass. No integration tests. Only covers the backend; frontend was never implemented to test. |
| B (baseline) | 4/5 | 452 lines of tests across `normalizer_test.go` (301 lines) and `replay_test.go` (151 lines). Good coverage of API request normalization, deduplication, token/cost updates, sensitive field stripping, pruning, replay timing, and context cancellation. Tests are integration-heavy (testing combined normalizer+state). |
| C (mindspec) | 5/5 | 1,078 lines across 7 test files: `graph_test.go`, `normalize_test.go`, `hub_test.go`, `live_test.go`, `replay_test.go`, `server_test.go`, `integration_test.go`. Granular unit tests per component plus a full integration test (replay → WebSocket → client receives events). Tests hard cap eviction with configurable limits. 2.4x more test code than B. |

### Documentation

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 1/5 | No documentation. No README for the viz app. No inline comments explaining architecture. Deleted all project documentation (docs/, ADRs, ROADMAP) during neutralization. |
| B (baseline) | 3/5 | Updated `BENCHMARKING.md` with automated E2E section. Minimal inline comments. No specific viz documentation. CLI `--help` text is adequate. |
| C (mindspec) | 4/5 | Updated `BENCHMARKING.md` with comprehensive bench-run documentation. Plan persisted as `plan.md` with YAML frontmatter and ADR citations. CLI commands have descriptive long-form help text. The plan itself serves as architectural documentation. |

### Functional Completeness

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 2/5 | Backend partially implemented (OTLP receiver, normalizer, graph state, replay). Frontend is scaffold-only — no 3D rendering, no Three.js integration, no bloom/starfield, no interactive controls. The core deliverable (visual galaxy) is missing. Also built the bench-run infrastructure (shared with B/C). |
| B (baseline) | 4/5 | Complete implementation: OTLP receiver, normalizer, graph state, WebSocket hub, replay mode, and a fully functional 3D frontend with Three.js, force-directed layout, bloom, starfield, search/filter, and detail cards. CLI commands work. Deducted for lower hard caps (200/500) and no sampling/throttling for high-rate streams. |
| C (mindspec) | 5/5 | Same completeness as B plus: configurable graph limits (500/2000 default), sampling logic for high event rates, backpressure tracking (dropped counter), stats ticker, modular frontend with vendored dependencies (no CDN needed), live and replay modes, both subcommands with appropriate flags. |

### Consistency with Project Conventions

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 1/5 | Created a Node.js app instead of extending the Go binary — violates ADR-0004 (Go implementation) and ADR-0003 (CLI-first tooling). No `mindspec viz` command. Without access to docs/ or ADRs, the agent had no way to discover these constraints. This is the strongest signal of documentation value. |
| B (baseline) | 4/5 | Follows Go conventions, uses cobra commands, `embed.FS` pattern (matching existing `internal/instruct/`). Registered in `root.go`. However, duplicated code rather than reusing existing packages, and used `viz serve` naming inconsistent with the spec's `viz live` terminology. |
| C (mindspec) | 5/5 | Follows all project conventions: Go implementation, cobra subcommands, `embed.FS`, registered in `root.go`. Reuses `bench.ExtractLogEvents()` instead of duplicating. CLI naming (`viz live`, `viz replay`) matches the spec. Plan cites ADRs and conforms to plan lifecycle conventions (YAML frontmatter). |

---

### Overall Verdict

Session C (mindspec) produced the best overall result across every dimension. The most striking differentiator is **architecture**: C's clean separation of concerns (pure normalization function, dedicated graph state, separate live receiver) versus B's tightly-coupled normalizer-as-state-manager. C wrote 2.4x more tests with better granularity, reused existing OTLP parsing code instead of duplicating it, and delivered configurable limits with production-grade features (sampling, backpressure tracking). Session A's fatal choice of Node.js over Go — made because it had no access to the project's ADRs — demonstrates that documentation access directly affects architectural correctness.

### Key Differentiators

**What MindSpec provided:**
1. **ADR awareness prevented wrong-language choice.** Session A chose Node.js without any awareness of ADR-0004 (Go) or ADR-0003 (CLI-first). Session C cited both ADRs in its plan fitness analysis and chose Go accordingly. This is the single strongest argument for structured documentation.
2. **Bead decomposition mapped to clean package boundaries.** Each bead in C's plan (Graph/Normalize, Hub, Server/UI, Replay/CLI, Live, Tests) became a distinct file with clear responsibilities. B had no such decomposition and produced a more coupled architecture.
3. **Code reuse discipline.** C's plan explicitly considered existing code (e.g., `bench.Collector` patterns) and added thin exported wrappers. B duplicated 157 lines. The spec-driven workflow's emphasis on "consistency with existing patterns" drove this behavior.
4. **Higher test investment.** C's bead verification checklists (`go test ./internal/viz/... -run TestX`) produced more comprehensive testing. The spec's acceptance criteria drove explicit integration tests.

**What MindSpec did NOT provide (or provided at cost):**
1. **3.5x higher cost** ($7.49 vs $2.13 for B). Much of this is the MindSpec workflow overhead — spec/plan/approve ceremony, context pack generation, instruct emissions, mode transitions.
2. **2.5x longer duration** (19.2 min vs 7.6 min). The structured workflow adds latency to every phase.
3. **No improvement in raw output volume.** B produced 39,654 output tokens vs C's 62,311 — but B's output/input ratio (1.26x) is much more efficient than C's (0.49x), meaning C spent more tokens reading/processing context than producing code.

### Surprising Findings

1. **Session B's quality was close to C on many dimensions** despite having no MindSpec tooling. Access to `docs/` (domain docs, ADRs, context map) alone was enough for B to make the correct language choice and follow most project conventions. This suggests that well-maintained documentation provides ~80% of MindSpec's architectural guidance benefit.

2. **The no-docs session's fatal error was entirely predictable.** Without ADRs, Session A had no way to know this project mandates Go. It made a reasonable choice (Node.js for a web visualizer) that happened to be wrong for *this* project. This is the clearest evidence that documentation access — not workflow tooling per se — is the key differentiator.

3. **The bench-run infrastructure was identical across all three sessions.** Sessions A, B, and C all produced byte-identical code for the `mindspec bench run` command, N-way reporting, worktree management, qualitative analysis, and session runner. This shared code was likely already partially implemented before the benchmark started, making it the "baseline" work that all sessions completed regardless of tooling.

4. **MindSpec's cost is dominated by context loading, not code generation.** C's cache read tokens (9.38M) dwarf A's (1.73M) and B's (1.67M). The framework injects glossary, domain docs, ADRs, context packs, and mode guidance — useful for correctness but expensive. The output/input ratio (0.49x for C vs 1.26x for B) shows C spends more tokens consuming context than producing code.


## Raw Data

Telemetry and output files are in this directory:
- `session-impl-a.jsonl` — Session A (no-docs) OTEL telemetry
- `session-impl-b.jsonl` — Session B (baseline) OTEL telemetry
- `session-impl-c.jsonl` — Session C (mindspec) OTEL telemetry
- `trace-impl-c.jsonl` — Session C MindSpec trace
- `output-impl-a.txt` — Session A (no-docs) Claude output
- `output-impl-b.txt` — Session B (baseline) Claude output
- `output-impl-c.txt` — Session C (mindspec) Claude output
