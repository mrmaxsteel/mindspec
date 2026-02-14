# Benchmark: 020-mindspec-proof

**Date**: 2026-02-14
**Commit**: 8bb8c16a43c95a2d3b9d112983b6f4debf415de8
**Timeout**: 1800s
**Model**: default

## Prompt

Plan and implement: mindspec proof — a command that parses the Validation Proofs section from a spec's plan.md, executes each proof command, captures output, and reports pass/fail with evidence. Follow existing patterns in internal/validate/. Include tests.

## Sessions

| Session | Description | Port | Events |
|---------|-------------|------|--------|
| A (no-docs)  | No CLAUDE.md/.mindspec; hooks stripped; no docs/ | 4318 | 54 |
| B (baseline) | No CLAUDE.md/.mindspec; hooks stripped; docs/ present | 4319 | 37 |
| C (mindspec) | Full MindSpec tooling | 4320 | 70 |

## Quantitative Comparison

### C vs A (mindspec vs no-docs)

```
Metric                        mindspec         no-docs           Delta
────────────────────────────────────────────────────────────────────
API Calls                            0               0              +0
Input Tokens                     33419           31313           +2106
Output Tokens                    36016           19941          +16075
Cache Read Tokens              2869653         1438650        +1431003
Cache Create Tokens             140724          108300          +32424
Total Tokens                     69435           51254          +18181
Cost (USD)                     $2.0955         $1.1805        +$0.9151
Duration                       6.0 min         3.4 min        +2.6 min
Cache Hit Rate                   94.3%           91.2%
Output/Input Ratio               1.08x           0.64x

Per-Model Breakdown           mindspec         no-docs           Delta
────────────────────────────────────────────────────────────────────
  claude-haiku-4-5-20251001
    Tokens In                    31060           31293            -233
    Tokens Out                   16136            9790           +6346
    Cost                       $0.3216         $0.2177        +$0.1039
  claude-opus-4-6
    Tokens In                     2359              20           +2339
    Tokens Out                   19880           10151           +9729
    Cost                       $1.7739         $0.9627        +$0.8112
```

### C vs B (mindspec vs baseline)

```
Metric                        mindspec        baseline           Delta
────────────────────────────────────────────────────────────────────
API Calls                            0               0              +0
Input Tokens                     33419           18892          +14527
Output Tokens                    36016           26041           +9975
Cache Read Tokens              2869653         1706951        +1162702
Cache Create Tokens             140724          156798          -16074
Total Tokens                     69435           44933          +24502
Cost (USD)                     $2.0955         $1.8136        +$0.2819
Duration                       6.0 min         5.9 min          +9.5 s
Cache Hit Rate                   94.3%           90.7%
Output/Input Ratio               1.08x           1.38x

Per-Model Breakdown           mindspec        baseline           Delta
────────────────────────────────────────────────────────────────────
  claude-haiku-4-5-20251001
    Tokens In                    31060           16012          +15048
    Tokens Out                   16136            9176           +6960
    Cost                       $0.3216         $0.1913        +$0.1303
  claude-opus-4-6
    Tokens In                     2359            2880            -521
    Tokens Out                   19880           16865           +3015
    Cost                       $1.7739         $1.6223        +$0.1516
```

### A vs B (no-docs vs baseline)

```
Metric                         no-docs        baseline           Delta
────────────────────────────────────────────────────────────────────
API Calls                            0               0              +0
Input Tokens                     31313           18892          +12421
Output Tokens                    19941           26041           -6100
Cache Read Tokens              1438650         1706951         -268301
Cache Create Tokens             108300          156798          -48498
Total Tokens                     51254           44933           +6321
Cost (USD)                     $1.1805         $1.8136        -$0.6332
Duration                       3.4 min         5.9 min        -2.4 min
Cache Hit Rate                   91.2%           90.7%
Output/Input Ratio               0.64x           1.38x

Per-Model Breakdown            no-docs        baseline           Delta
────────────────────────────────────────────────────────────────────
  claude-haiku-4-5-20251001
    Tokens In                    31293           16012          +15281
    Tokens Out                    9790            9176            +614
    Cost                       $0.2177         $0.1913        +$0.0264
  claude-opus-4-6
    Tokens In                       20            2880           -2860
    Tokens Out                   10151           16865           -6714
    Cost                       $0.9627         $1.6223        -$0.6596
```

## MindSpec Trace Summary (Session C)

```
Trace Summary: /tmp/mindspec-bench-020-mindspec-proof/trace-c.jsonl
  Events:     13
  Duration:   948.5 ms
  Tokens:     0

  Event                      Count   Duration   Tokens
  -----------------------------------------------------
  command.end                    6   948.5 ms        -
  command.start                  7          -        -
```

## Qualitative Analysis

You've hit your limit · resets 11am (Europe/London)
(qualitative analysis failed)

## Raw Data

Telemetry and output files are in `/tmp/mindspec-bench-020-mindspec-proof/`:
- `session-a.jsonl` — Session A (no-docs) OTEL telemetry
- `session-b.jsonl` — Session B (baseline) OTEL telemetry
- `session-c.jsonl` — Session C (mindspec) OTEL telemetry
- `trace-c.jsonl` — Session C MindSpec trace
- `output-a.txt` — Session A Claude output
- `output-b.txt` — Session B Claude output
- `output-c.txt` — Session C Claude output
