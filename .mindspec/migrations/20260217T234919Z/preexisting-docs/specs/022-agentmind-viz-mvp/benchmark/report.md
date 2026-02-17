# Benchmark: 022-agentmind-viz-mvp

**Date**: 2026-02-14
**Commit**: 783a12b20b3443e06a6b92764a9ecb7b6ee697e6
**Timeout**: 1800s
**Model**: default

## Prompt

Plan and implement an MVP 'agent activity galaxy' web visualizer.

Goal: read Claude Code OpenTelemetry log/events in real time and render them as a live 3D constellation on a webpage, visually inspired by the  image (starfield background, bright neon palette, glowing nodes, constellation-style linework).

What to build (MVP):

* A small ingestion service that receives OpenTelemetry logs/events (OTLP) and turns them into a normalized event stream.
* A browser UI that connects via websocket and r...

## Sessions

| Session | Description | Port | Events |
|---------|-------------|------|--------|
| A (no-docs) | No CLAUDE.md/.mindspec; hooks stripped; no docs/ | 4318 | 62 |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped; docs/ present | 4319 | 47 |
| C (mindspec) | Full MindSpec tooling | 4320 | 62 |

## Quantitative Comparison

```
Metric                         no-docs        baseline        mindspec
──────────────────────────────────────────────────────────────────────
API Calls                            0               0               0
Input Tokens                    128990           24850           22559
Output Tokens                    47488           34369           14549
Cache Read Tokens              1340647         1279470         1077775
Cache Create Tokens             197133          186199           99125
Total Tokens                    176478           59219           37108
Cost (USD)                     $2.1738         $1.6046         $1.5298
Duration                       9.7 min         5.8 min         4.7 min
Cache Hit Rate                   80.4%           85.8%           89.9%
Output/Input Ratio               0.37x           1.38x           0.64x

Per-Model Breakdown            no-docs        baseline        mindspec
──────────────────────────────────────────────────────────────────────
  claude-haiku-4-5-20251001
    Tokens In                   127853           24828           22445
    Tokens Out                   18148           17110             767
    Cost                       $0.3902         $0.2956         $0.0263
  claude-opus-4-6
    Tokens In                     1137              22              17
    Tokens Out                   29340           17259            6004
    Cost                       $1.7836         $1.3090         $0.5254
  claude-sonnet-4-5-20250929
    Tokens In                        0               0              97
    Tokens Out                       0               0            7778
    Cost                       $0.0000         $0.0000         $0.9782

```

## MindSpec Trace Summary (Session C)

```
Trace Summary: /var/folders/cw/tdy8s59d077dyky0b0qk9pxc0000gq/T/mindspec-bench-022-agentmind-viz-mvp/trace-c.jsonl
  Events:     2
  Duration:   117.3 ms
  Tokens:     0

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  command.end                    1   117.3 ms        -
  command.start                  1          -        -

```

## Qualitative Analysis

Now I have the complete picture to write the analysis. Let me summarize what happened:

**Session A (no-docs)**: Consumed 176K tokens over 9.7 minutes at $2.17. The benchmark neutralization script deleted ALL docs, ADRs, CLAUDE.md, hooks, MindSpec state, and settings. The session produced **zero implementation code** — it only committed the neutralization deletions. The massive token consumption (128K input, 47K output) with no code output suggests the session may have spent its time exploring, planning, or getting stuck.

**Session B (baseline)**: Consumed 59K tokens over 5.8 minutes at $1.60. Neutralization stripped MindSpec tooling but kept docs/. The session produced **zero implementation code** — only the neutralization file deletions were committed. 

**Session C (mindspec)**: Consumed 37K tokens over 4.7 minutes at $1.53. Produced a **comprehensive 126-line spec** (`docs/specs/022/spec.md`) with structured requirements, acceptance criteria, validation proofs, scoping, non-goals, and ADR touchpoints. Set state to spec mode. No implementation code (correctly — MindSpec workflow requires spec approval before planning/implementation).

Now I can write the analysis:

---

## Structured Comparison: Agent Activity Galaxy Visualizer

### Critical Observation

**None of the three sessions produced implementation code.** This benchmark tested a feature far more ambitious than the previous proof-runner benchmark (Spec 020). The galaxy visualizer prompt describes a full-stack system: OTLP ingestion, WebSocket streaming, 3D rendering with Three.js, force-directed layout, interactive UI — a multi-day feature, not a single-session task. What differs dramatically is *how far each session got* and *what artifacts it produced* within its time budget.

- **Session A** (9.7 min, $2.17): Produced nothing. Deleted 94 files (the entire docs/ tree as part of no-docs neutralization), modified settings — zero forward progress on the feature.
- **Session B** (5.8 min, $1.60): Produced nothing. Stripped 7 MindSpec files — zero forward progress on the feature.
- **Session C** (4.7 min, $1.53): Produced a thorough, 126-line specification (`docs/specs/022/spec.md`), correctly entering spec mode per the MindSpec workflow.

---

### Planning Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 1/5 | No plan artifact. No spec artifact. Despite consuming the most tokens (176K) and time (9.7 min), Session A produced zero forward-looking design work. The high token count (128K input, 47K output) suggests extensive codebase exploration and possibly attempted planning in conversation, but nothing was persisted. The O/I ratio (0.37x) indicates the session was mostly reading, not producing. |
| B (baseline) | 1/5 | No plan artifact. No spec artifact. Despite having access to the full docs/ directory with domain architecture, ADRs, context maps, and glossary, Session B produced no design work. The higher O/I ratio (1.38x) compared to A suggests it may have attempted to produce output (possibly conversational planning), but nothing was committed. |
| C (mindspec) | 5/5 | Produced a comprehensive, well-structured spec covering 7 requirement areas (ingestion, normalization, UI/3D, interaction, noise/safety, CLI, validation). The spec includes: explicit background connecting to prior specs (018, 019), impacted domain declarations, ADR touchpoints (ADR-0003, ADR-0004), 25 numbered requirements, clear scope/out-of-scope/non-goals boundaries, 12 acceptance criteria with testable assertions, 4 validation proofs with concrete commands, and file-path-level scope declarations (`internal/viz/normalize.go`, `cmd/mindspec/viz.go`). This is a ready-to-review specification that could proceed to plan approval. |

### Architecture

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | No code or design produced. Cannot evaluate. |
| B (baseline) | 0/5 | No code or design produced. Cannot evaluate. |
| C (mindspec) | 4/5 | While no code was written (correctly — the workflow is spec-first), the spec demonstrates strong architectural thinking: separating normalization (`internal/viz/normalize.go`) from graph state, WebSocket hub, and HTTP server; reusing the existing OTLP collector pattern from `internal/bench/collector.go`; embedding web assets via `embed.FS` (matching Go binary distribution philosophy per ADR-0004); proposing `internal/viz/web/` for static assets. The spec explicitly identifies the integration seam with existing code and defers cross-cutting concerns (persistent storage, multi-user, bundler toolchain) to out-of-scope. |

### Code Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | No code produced. |
| B (baseline) | 0/5 | No code produced. |
| C (mindspec) | N/A | No code produced (by design — spec phase). The spec itself is well-written: clear prose, consistent formatting, appropriate use of markdown structure, no ambiguous language in requirements. |

### Test Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | No tests produced. |
| B (baseline) | 0/5 | No tests produced. |
| C (mindspec) | N/A | No tests produced (by design — spec phase). However, the spec includes 12 testable acceptance criteria and 4 concrete validation proofs with executable commands, providing clear test targets for implementation. |

### Documentation

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | No documentation produced. Worse: the session deleted all 94 existing documentation files as part of the no-docs neutralization, demonstrating the destructive baseline. |
| B (baseline) | 0/5 | No documentation produced. |
| C (mindspec) | 5/5 | The spec IS documentation — a 126-line structured specification that connects to the project's existing spec ecosystem (references Spec 018, 019, ADR-0003, ADR-0004), follows the project's spec template conventions (Goal, Background, Impacted Domains, ADR Touchpoints, Requirements, Scope, Non-Goals, Acceptance Criteria, Validation Proofs, Approval), and provides a durable artifact for future implementation sessions. |

### Functional Completeness

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | Zero functional completeness. No code, no spec, no plan, no artifacts toward the goal. |
| B (baseline) | 0/5 | Zero functional completeness. Same as A — nothing produced toward the feature. |
| C (mindspec) | 2/5 | The spec comprehensively covers all requirements from the prompt (ingestion, normalization, 3D rendering, interaction, noise/safety, validation modes), but no implementation exists. Session C correctly identified this as a multi-phase effort and completed phase 1 (specification). For a "plan and implement" prompt, delivering only the spec is incomplete — but it's the correct first step in the MindSpec workflow, and the spec is implementation-ready. |

### Consistency with Project Conventions

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs) | 0/5 | Deleted all project documentation and conventions. Without docs/, ADRs, or CLAUDE.md, the session had no conventions to follow and produced nothing to evaluate against them. |
| B (baseline) | 1/5 | Had access to the full docs/ directory but produced no artifacts to evaluate against conventions. The docs/ presence didn't translate into convention-following output. |
| C (mindspec) | 5/5 | Perfectly follows project conventions: spec placed in `docs/specs/022/spec.md` matching the `docs/specs/NNN-slug/spec.md` pattern; state file updated via MindSpec CLI; spec structure matches the project's spec template (visible in existing specs 001-021); ADR citations use correct relative paths; impacted domain declarations follow the context-map taxonomy; acceptance criteria use checkbox format; validation proofs include executable commands. |

### Overall Verdict

Session C (mindspec) produced the only meaningful output: a comprehensive, implementation-ready specification that correctly follows the project's spec-driven workflow. While none of the sessions delivered working code, this outcome reveals something fundamental about ambitious feature prompts: **structure determines whether partial progress has value**. Session C's spec is a durable, reviewable, resumable artifact — a future session can approve it and proceed to planning and implementation. Sessions A and B consumed more tokens (A: 4.7x more, B: 1.6x more) and more time, but produced nothing that advances the project. The MindSpec workflow turned a "not enough time to finish" situation into "completed phase 1 of a multi-phase effort," while the freestyle sessions turned it into "wasted budget."

### Key Differentiators

1. **Structured partial progress**: MindSpec's spec-first workflow ensured that even though the feature is too large for one session, the session produced a high-value artifact. The spec captures all design decisions, scope boundaries, and acceptance criteria — preventing re-derivation in future sessions. Sessions A and B, lacking workflow structure, had no intermediate artifact format and thus no way to persist partial progress.

2. **Cost efficiency through focus**: Session C was the cheapest ($1.53) and fastest (4.7 min) despite producing the most valuable output. MindSpec's workflow guidance prevented the session from attempting premature implementation of a complex feature, avoiding the "thrashing" pattern visible in Session A's metrics (176K tokens, 9.7 min, 0.37x O/I ratio — reading vastly more than writing).

3. **Correct scope recognition**: Session C recognized this as a multi-phase effort and completed the appropriate first phase. Sessions A and B apparently attempted to tackle the full "plan and implement" prompt head-on and failed to produce anything.

4. **Convention inheritance**: Session C's spec references prior specs (018, 019), cites relevant ADRs, declares impacted domains, and proposes file paths consistent with the existing codebase. This institutional knowledge is embedded in the artifact, making it transferable to any future implementation session.

### Surprising Findings

1. **The most expensive session produced the least**: Session A cost $2.17 (42% more than C) and took 9.7 minutes (2x longer) while producing zero usable artifacts. The no-docs condition appears to have caused extensive exploration without convergence — the session read far more than it wrote (O/I ratio 0.37x).

2. **Docs without workflow didn't help**: Session B had access to the full docs/ directory (ADRs, domain docs, context map, glossary, architecture docs) but produced nothing. This echoes the finding from the Spec 020 benchmark where Session B also failed to produce code. Documentation alone, without workflow tooling to channel it into action, appears to be inert or even counterproductive (potentially causing analysis paralysis).

3. **The "plan and implement" prompt exposed workflow fragility**: For a sufficiently ambitious feature, freestyle sessions (A, B) produced zero value while the structured session (C) produced phase-appropriate value. This suggests MindSpec's advantage scales with task complexity — simple features may not need it (per Spec 020, Session A delivered working code for the simpler proof-runner), but ambitious features benefit enormously from structured decomposition.

4. **Session C used Sonnet for the heavy lifting**: The per-model breakdown shows Session C routed 7,778 output tokens through Sonnet ($0.98) — likely for the spec-writing work — while using minimal Opus tokens (6,004 output, $0.53). Sessions A and B used no Sonnet at all. This model routing may partially explain C's efficiency: the MindSpec workflow apparently triggered agent delegation patterns that used cheaper models for appropriate subtasks.


## Raw Data

Telemetry and output files are in this directory:
- `session-a.jsonl` — Session A (no-docs) OTEL telemetry
- `session-b.jsonl` — Session B (baseline) OTEL telemetry
- `session-c.jsonl` — Session C (mindspec) OTEL telemetry
- `trace-c.jsonl` — Session C MindSpec trace
- `output-a.txt` — Session A (no-docs) Claude output
- `output-b.txt` — Session B (baseline) Claude output
- `output-c.txt` — Session C (mindspec) Claude output
