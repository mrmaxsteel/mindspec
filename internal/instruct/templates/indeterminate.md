# MindSpec — Cannot Determine Mode

cannot determine mode: {{termsafe .IndeterminateReason}}

This is NOT idle. MindSpec could not derive a lifecycle mode — either a
beads/Dolt query failed, or the result was genuinely ambiguous (e.g.
multiple active specs with no `--spec` target) — and is reporting that
failure honestly instead of guessing a mode it never actually observed.

## What NOT to do

- Do NOT treat this as "no active work" and start something new.
- Do NOT assume `idle.md`'s guidance or Available Actions apply here.

## Next Action

1. Run `mindspec doctor` to diagnose the underlying failure.
2. Once resolved, re-run the command that produced this output (or
   `mindspec instruct`) to get real, derived guidance.

recovery: mindspec doctor
