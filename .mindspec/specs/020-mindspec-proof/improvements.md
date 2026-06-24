# Improvements from Non-MindSpec Sessions

## Summary

Session A (no-docs) produced one concrete technical improvement worth adopting: per-command timeout handling. Session B (baseline) produced no code at all, so offers no implementation improvements — though its failure to execute is itself an insight about workflow design.

## Improvements

### 1. Per-Command Timeout

**Source**: Session A
**What was better**: Session A's `RunProofs` function accepts a `timeout time.Duration` parameter and kills individual proof commands that exceed it. This is practical — a proof like `make test` could hang indefinitely, and without a timeout the proof runner would block forever. Session C has no per-command timeout; it relies on the caller to handle overall timeouts externally.
**Suggestion**: Add an optional `--timeout` flag to `mindspec proof run` that sets a per-proof execution timeout. Default to 60 seconds per proof. The implementation pattern from Session A (using `cmd.Process.Kill()` after a timer fires) is straightforward to adapt.

### 2. Combined Output Helper

**Source**: Session A
**What was better**: Session A's `combineOutput` function merges stdout and stderr into a single output field for display. While Session C captures them separately (which is better for structured JSON output), having a combined view is useful for the text report format where users want to see all output without mentally interleaving two streams.
**Suggestion**: Keep the separate stdout/stderr in `ProofResult` (as Session C does) but add a `CombinedOutput()` method for use in text formatting and evidence files.

### 3. Validate Subcommand Placement

**Source**: Session A
**What was better**: Session A placed proof running under `mindspec validate proof` — which has a conceptual argument in its favor. Proofs *are* a form of validation, and colocating them with `validate spec`, `validate plan`, and `validate docs` creates a unified validation surface. Session C's top-level `proof run` is arguably better for discoverability and aligns with the prompt's "mindspec proof" framing, but the validate placement deserves consideration.
**Suggestion**: No change needed — the top-level `proof` command is the right choice per Spec 013's design. But consider adding `mindspec validate proof` as an alias or at least documenting the relationship in `--help` text.

## Conclusion

The improvements from Session A are incremental rather than architectural. The per-command timeout is the only clear gap — Session C's implementation is more complete and better structured in every other dimension. The most significant finding is negative: Session B's complete failure to produce code suggests that having documentation without workflow tooling may actually impede non-interactive execution, which is worth investigating for the benchmark harness design (perhaps the baseline session needs different prompt framing to avoid getting stuck in plan mode).
