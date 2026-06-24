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

### Planning Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 3/5 | No explicit plan artifact. Jumped straight into implementation. The lack of docs/ meant no spec to reference, so planning was implicit — inferred from the prompt alone. The approach (adding to `internal/validate/`) was reasonable but not deliberate. |
| B (baseline) | 2/5 | Produced a plan but never implemented it. The output ("The plan is ready for your review") suggests the session entered Claude's plan mode and waited for approval that never came. Having docs/ may have actually caused analysis paralysis — the session spent 5.9 minutes and $1.81 without writing a single line of code. |
| C (mindspec) | 5/5 | Clear architectural decomposition into parse/run/report. Created a dedicated `internal/proof/` package (not bolted onto existing validate/). The plan included dry-run mode, evidence file persistence, failure inversion logic, and content matching — features that directly address real proof-runner use cases visible in the existing specs. |

### Architecture

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 3/5 | Placed code in `internal/validate/proof.go` — a single file in an existing package. Reasonable choice given no docs to suggest otherwise, but creates a monolithic file mixing parsing, execution, and reporting. CLI wiring added directly to `cmd/mindspec/validate.go`. |
| B (baseline) | 0/5 | No code produced. Cannot evaluate architecture. |
| C (mindspec) | 5/5 | Created a clean `internal/proof/` package with clear separation: `parse.go` (markdown extraction), `run.go` (command execution + pass/fail logic), `report.go` (text/JSON/evidence output). Each file has a single responsibility. CLI wiring in dedicated `cmd/mindspec/proof.go`. Registered as top-level `proof run` command rather than nested under `validate`. |

### Code Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 3/5 | Functional but has code smells. Custom `splitLines`/`splitByNewline` helpers duplicate `strings.Split`. The `formatProofReport` function uses `[]byte` appends via a closure — unusual pattern for Go. Error handling is adequate. Naming is clear. |
| B (baseline) | 0/5 | No code produced. |
| C (mindspec) | 4/5 | Clean, idiomatic Go throughout. Uses `strings.Builder` for string assembly. Separates exported types (`Proof`, `ProofResult`) cleanly. The `determinePass` function has clear, documented logic. Minor issue: `quotedRe` regex matches across quote types (`'..."` would match), but this is unlikely to matter in practice. |

### Test Quality

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 4/5 | 16 tests covering parsing (valid, invalid, full sections, missing, empty, placeholders, file I/O, nonexistent) and execution (pass, fail, no-proofs error, output capture, timeout, working directory). Includes timeout handling tests — a practical edge case C missed. |
| B (baseline) | 0/5 | No code produced. |
| C (mindspec) | 5/5 | 34 tests across 3 test files, cleanly separated by concern. Parse tests cover basic parsing, empty/missing sections, parenthesized annotations, fenced code block skipping, file I/O. Run tests cover exit code logic, failure inversion, content matching with single/double/multiple quotes, combined checks. Report tests cover text formatting, long command truncation, dry run, JSON validity, evidence file writing, `AllPassed` helper. More comprehensive edge case coverage. |

### Documentation

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 2/5 | Minimal doc comments. No doc-sync (no docs/ to sync with). The implementation summary in the output file is clear but there are no inline docs explaining the pass/fail logic. |
| B (baseline) | 0/5 | No code produced. |
| C (mindspec) | 4/5 | Good doc comments on exported functions and types. Each file has clear package-level purpose. The output summary is thorough, explaining the parsing regex, pass/fail logic rules, and evidence file format. |

### Functional Completeness

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 3/5 | Implements the core requirement (parse proofs, execute, report pass/fail). Has per-command timeout — a useful feature C lacks. Missing: dry-run mode, evidence file persistence, failure inversion for "should fail" proofs, content matching against quoted strings in expected outcomes. Placed as `validate proof` subcommand rather than top-level `proof run`. |
| B (baseline) | 0/5 | No implementation produced. Zero functional completeness. |
| C (mindspec) | 5/5 | Fully implements the prompt requirements and goes beyond: `--dry-run` for proof listing, `--json` for structured output, `--no-capture` to skip evidence writing, evidence file writer with timestamps, failure inversion (`should fail`/`should error`/`fails with`), content matching from quoted strings in expected outcomes, fenced code block handling in parser to avoid false matches. |

### Consistency with Project Conventions

| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | 2/5 | Followed the prompt's hint ("Follow existing patterns in internal/validate/") literally by adding to the validate package. But without docs/, it missed project conventions: the existing CLI structure uses top-level subcommands (`proof`, `validate`, `bead`, `adr`), not deeply nested ones. Also deleted all docs/ as part of neutralization, losing access to convention guidance. |
| B (baseline) | 1/5 | Had access to docs/ but never used them for implementation. The plan description suggests awareness of patterns but no execution. |
| C (mindspec) | 5/5 | Created a new `internal/proof/` package following the exact pattern of `internal/adr/`, `internal/validate/`, etc. Used `findRoot()` helper, cobra command wiring in dedicated file, registered in `root.go`. CLI structure (`proof run <spec-id>`) follows the `adr show`, `bead spec`, `validate spec` patterns. Evidence file path follows the `docs/specs/<id>/proofs/` convention from the spec folder layout. |

### Overall Verdict

Session C (mindspec) produced the best overall result by a significant margin. It created a well-architected, thoroughly tested, feature-complete implementation with 34 tests across 7 files, including advanced features like failure inversion and evidence persistence. Session A (no-docs) delivered a functional but simpler implementation in less time and at lower cost — it works, but lacks the sophistication and project-convention awareness that C demonstrates. Session B (baseline) is the most surprising result: despite having access to the full docs/ directory, it produced zero code, apparently getting stuck in plan mode waiting for human approval that never came in the non-interactive `claude -p` context.

### Key Differentiators

MindSpec provided three clear advantages:

1. **Architectural awareness**: Session C knew to create a separate `internal/proof/` package and register it as a top-level command, matching the project's established patterns. Session A, lacking docs, defaulted to the simplest approach (bolting onto `internal/validate/`).

2. **Feature completeness from spec context**: Session C's failure inversion logic, content matching, and evidence file writing aren't random gold-plating — they address real use cases visible in existing specs' Validation Proofs sections (e.g., "should fail with 'already exists' error"). MindSpec's context gave C awareness of how proofs are actually written in the project.

3. **Execution confidence**: Session C completed the full implementation cycle without hesitation, while Session B (with docs but no MindSpec workflow) got stuck in planning. MindSpec's `--dangerously-skip-permissions` + non-interactive mode + workflow guidance appears to help sessions push through to completion.

### Surprising Findings

1. **Session B produced zero code** — the worst outcome came from the session with docs/ but no MindSpec. Having documentation without workflow tooling may actually be counterproductive in non-interactive mode: the session may have been influenced by planning conventions visible in docs/ but had no `--dangerously-skip-permissions`-aware workflow to proceed past planning.

2. **Session A was the most cost-efficient** — at $1.18 and 3.4 minutes, it delivered a working implementation for roughly half the cost of C. The simplicity of "no context, just code" has real efficiency advantages for straightforward features.

3. **Per-command timeout** — Session A included per-proof timeout handling that Session C did not. This is a practical feature that C's richer context didn't surface as a requirement, suggesting that sometimes freestyle implementation catches pragmatic concerns that spec-driven development may overlook.

4. **Cache behavior** — Session C had the highest cache read tokens (2.87M vs 1.44M for A), indicating that MindSpec's context pack and instruct output contributed significant cached content. The 94.3% cache hit rate suggests efficient reuse of this context across turns.

## Raw Data

Telemetry and output files are in `/tmp/mindspec-bench-020-mindspec-proof/`:
- `session-a.jsonl` — Session A (no-docs) OTEL telemetry
- `session-b.jsonl` — Session B (baseline) OTEL telemetry
- `session-c.jsonl` — Session C (mindspec) OTEL telemetry
- `trace-c.jsonl` — Session C MindSpec trace
- `output-a.txt` — Session A Claude output
- `output-b.txt` — Session B Claude output
- `output-c.txt` — Session C Claude output
