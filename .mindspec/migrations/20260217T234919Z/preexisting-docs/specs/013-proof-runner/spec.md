# Spec 013-proof-runner: Proof Runner (MVP)

## Goal

Automate the "proof-of-done" invariant by parsing Validation Proofs from spec.md, executing the listed commands, and reporting pass/fail with captured evidence — so proof execution is reproducible rather than manual.

## Background

Every MindSpec spec includes a `## Validation Proofs` section listing shell commands and their expected outcomes. Today these are run manually by the agent or developer, with output captured ad-hoc. The "proof-of-done" invariant (ARCHITECTURE.md, MODES.md) requires that beads close only when verification steps pass with evidence, but there's no tooling to enforce this.

CONVENTIONS.md already reserves `docs/specs/<id>/proofs/` for timestamped proof output artifacts. This spec fills the gap between the proof specification (every spec.md has one) and proof automation.

### Validation Proofs format (established convention)

```markdown
## Validation Proofs

- `<command>`: <expected outcome description>
- `<command1> && <command2>`: <expected outcome description>
- `<command> | <command2>`: <expected outcome description>
```

Expected outcomes are free-text descriptions. Common patterns across existing specs:
- **Success expected**: "Should list ...", "Should show ...", "All tests pass"
- **Failure expected**: "Should fail (already exists)", "Should fail with ..."
- **Content expected**: "Should include '...' in output", "Should find the new ..."

## Impacted Domains

- **workflow**: Proof execution is part of the implementation lifecycle; results feed into the completion gate
- **core**: New CLI subcommand (`proof run`) registers under the root command

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): Proof outputs are first-class artifacts alongside specs and context packs
- [ADR-0004](../../adr/ADR-0004.md): Go as implementation language

## Requirements

### `mindspec proof run <spec-id>`

1. **Parse**: reads `docs/specs/<spec-id>/spec.md`, extracts the `## Validation Proofs` section, and parses each bullet into a `(command, expected_outcome)` pair. Commands are the backtick-delimited portion; expected outcomes are the text after the colon.

2. **Execute**: runs each command via `sh -c "<command>"` sequentially, capturing stdout, stderr, exit code, and wall-clock duration per command.

3. **Pass/fail determination**:
   - **Default**: exit code 0 = pass, non-zero = fail
   - **Failure expected**: if the expected outcome contains "should fail", "should error", or "fails with", the assertion is inverted — non-zero exit code = pass, zero = fail
   - **Content matching**: if the expected outcome contains a quoted string (single or double quotes), the runner checks that stdout contains that string. Missing → fail.
   - Both exit code and content checks must pass for a proof to pass.

4. **Report**: after all proofs execute, prints a summary table:
   - One row per proof: index, command (truncated to 60 chars), pass/fail status, duration
   - Final line: "N/M proofs passed" with overall pass/fail
   - Exit code: 0 if all proofs pass, 1 if any fail

5. **Evidence capture**: writes a timestamped artifact to `docs/specs/<spec-id>/proofs/<YYYY-MM-DD>_<HHMM>.txt` containing:
   - Timestamp and spec ID header
   - For each proof: command, expected outcome, actual stdout/stderr, exit code, pass/fail
   - Summary line

6. Creates the `proofs/` directory if it doesn't exist.

7. If the spec has no Validation Proofs section or no proof entries, prints "No validation proofs found in spec <id>" and exits with code 0.

### Flags

8. `--dry-run`: parse and display the proof list without executing. Useful for verifying the parser extracted commands correctly.

9. `--no-capture`: run proofs and report results but skip writing the evidence file.

10. `--json`: emit results as JSON instead of the text table. Each proof entry includes command, expected outcome, stdout, stderr, exit code, pass/fail, and duration.

## Scope

### In Scope

- `cmd/mindspec/proof.go` — cobra command wiring for `proof run`
- `internal/proof/parse.go` — Validation Proofs section extraction and bullet parsing
- `internal/proof/run.go` — command execution, pass/fail logic, timeout handling
- `internal/proof/report.go` — text/JSON formatting and evidence file writing
- Unit tests for parsing, pass/fail logic, and report formatting
- Integration test with a synthetic spec.md containing varied proof patterns

### Out of Scope

- Blocking `mindspec complete` on proof results (future integration — would require 013 to be wired into the completion workflow)
- Smart retries or flaky-test handling
- Parallel proof execution (sequential is simpler and avoids state interference between commands)
- Cross-platform shell support (Unix `sh -c` only for MVP; Windows support is a future enhancement)
- Parsing proofs from plan.md or other documents (spec.md only)

## Non-Goals

- Natural language understanding of expected outcomes beyond the simple patterns (quoted strings, "should fail")
- Sandboxing or resource-limiting proof commands
- Automatic cleanup of side effects from proof commands (e.g., test files created during proofs)

## Acceptance Criteria

- [ ] `mindspec proof run 010-spec-init-cmd` parses and executes the validation proofs from spec 010
- [ ] A proof with exit code 0 and no content assertion is reported as PASS
- [ ] A proof whose expected outcome says "should fail" passes when the command returns non-zero
- [ ] A proof whose expected outcome contains `'test-domain'` fails if stdout doesn't contain "test-domain"
- [ ] `--dry-run` prints the parsed proof list without executing any commands
- [ ] Evidence file is written to `docs/specs/<id>/proofs/<timestamp>.txt` with full output
- [ ] `--json` produces valid JSON with all proof fields
- [ ] `--no-capture` skips evidence file creation
- [ ] Exit code is 0 when all proofs pass, 1 when any fail
- [ ] A spec with no Validation Proofs section exits cleanly with a message
- [ ] All new code has unit tests; `make test` passes
- [ ] `make build` succeeds

## Validation Proofs

- `./bin/mindspec proof run 013-proof-runner --dry-run`: Should list the proofs from this spec without executing them
- `./bin/mindspec proof run 010-spec-init-cmd`: Should execute spec 010's proofs and report results (some may fail if test artifacts don't exist)
- `./bin/mindspec proof run 010-spec-init-cmd --json | jq '.[0].pass'`: Should output a boolean
- `ls docs/specs/010-spec-init-cmd/proofs/`: Should contain a timestamped evidence file after a run
- `make test`: All tests pass

## Open Questions

(none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec