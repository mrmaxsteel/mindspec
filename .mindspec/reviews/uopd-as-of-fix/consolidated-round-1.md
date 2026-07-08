# uopd-as-of-fix — round 1 consolidated outcome

**Decision: PASS (5 APPROVE / 1 REQUEST_CHANGES / 0 REJECT; threshold 5/6; no hard_block; reviewed_head_sha d7f5c67a fresh).** Merge proceeds; the following convergent non-blocking asks are deferred to a follow-up bead (not merge blockers — all paths verified fail-safe by R3/R4/R5 independently).

## Deferred asks (follow-up bead)

1. **(R5, medium) Test coverage for the `(empty stdout, nil error)` --as-of failure shape.** bd 1.1.0's `bd show <id> --as-of <ref>` exits 0 with empty stdout + stderr-only diagnostic on genuine read failures (missing id, bad ref) — confirmed independently by R4 and R5. Today this hard-fails via `parseBeadShowJSON`'s empty-output check (correct, never proceeds), but the shape has zero direct test coverage.
2. **(R5) Preserve/surface bd's stderr diagnostic in the exit-0-failure path.** The real bd error text is discarded; operators debugging a failed close-verify see only "empty output". Capture stderr (RunBD uses cmd.Output(), which populates stderr only on ExitError) or re-run diagnostics.
3. **(R5) Caveat `TestDefaultVerifyCommitted_AsOfHardReadFailureNeverFallsBack`'s premise** — its synthesized `*exec.ExitError` "Dolt lock" scenario may not match how bd 1.1.0's --as-of failures actually manifest (exit 0).
4. **(R3+R4 convergent, low) Direct unit test for `bead.IsUnsupportedFlagError`** in its owning package (currently only indirect coverage via internal/complete).
5. **(R6, low) Direct unit test for `next.FetchBeadAsOf`** at the internal/next layer (currently seam-faked in internal/complete + a pointer-equality pin).

## Non-blocking notes recorded, no action

- R4: HONESTY-CLAUSE phrasing on auto-commit is "slightly imprecise but not incorrect in effect" (forced `bd dolt commit` always precedes the verify).
- R1: committed-vs-session distinctness rests on bd's documented semantics; 2u0u divergence not reproducible on demand.
