# Benchmark: 027-spec-recording

**Date**: 2026-02-16
**Commit**: 52f249c466036d8e692e7fa97909c11dc0c9cd9e
**Timeout**: 1800s
**Model**: default

## Prompt

# Spec 027-spec-recording: Automatic Per-Spec Agent Telemetry Recording

## Goal

Automatically capture agent telemetry for the full lifecycle of every spec — from `spec-init` through `impl-approve` — so any feature's development journey can be replayed in AgentMind. Recording is zero-friction: it starts when you create a spec and stops when implementation is approved.

## Background

ADR-0009 established AgentMind as the embedded real-time agent visualization system, and the bench system al...

## Sessions

| Session | Description | Port | Events |
|---------|-------------|------|--------|
| A (no-docs) | No CLAUDE.md/.mindspec; hooks stripped; no docs/ | 4318 | 0 |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped; docs/ present | 4319 | 0 |
| C (mindspec) | Full MindSpec tooling | 4320 | 0 |

## Quantitative Comparison

```
(telemetry sent to external OTLP endpoint — no local session data)

```

## MindSpec Trace Summary (Session C)

```
Trace Summary: /var/folders/cw/tdy8s59d077dyky0b0qk9pxc0000gq/T/mindspec-bench-027-spec-recording/trace-c.jsonl
  Events:     51
  Duration:   4915.5 ms
  Tokens:     2208

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  bead.cli                      15  2093.3 ms        -
  command.end                   12  2822.2 ms        -
  command.start                 18          -        -
  instruct.render                2          -     2208
  state.transition               4          -        -

```

## Qualitative Analysis

_(skipped)_

## Raw Data

Telemetry and output files are in this directory:
- `session-a.jsonl` — Session A (no-docs) OTEL telemetry
- `session-b.jsonl` — Session B (baseline) OTEL telemetry
- `session-c.jsonl` — Session C (mindspec) OTEL telemetry
- `trace-c.jsonl` — Session C MindSpec trace
- `output-a.txt` — Session A (no-docs) Claude output
- `output-b.txt` — Session B (baseline) Claude output
- `output-c.txt` — Session C (mindspec) Claude output
