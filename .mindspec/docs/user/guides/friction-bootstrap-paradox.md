# Friction Reporter — the Bootstrap Paradox (install-failure friction)

> Status: PLACEHOLDER (v1). Examples here are synthetic placeholders only
> (`<path>`, `bead/<id>`, `<fingerprint>`) — this file is git-committed
> OUTSIDE the redaction sink, so never paste a real captured string here.

The owner-local friction self-improvement loop (spec 094, see
[ADR-0038](../../adr/ADR-0038-friction-reporter.md)) captures friction from a
**working** mindspec install: the always-on `PersistentPostRunE` sink records
escape-hatch overrides and `repair phase` invocations to a local journal, which
`mindspec report` consolidates and `mindspec report list` triages.

## The paradox

**Install-failure friction is structurally UNREPORTABLE in-tool.** If the
install fails, mindspec is not present to self-report it — the in-tool sink
never runs. The loop can only ever capture friction from an already-working
installation, so the single highest-friction moment (a broken install) is
exactly the one this loop cannot see.

## The out-of-band home (v1)

Install-failure friction lives OUTSIDE the in-tool loop, in two deferred
out-of-band channels:

1. **Installer-side signal** — an installer-emitted failure signal (the
   installer is the only component present when mindspec itself is not).
2. **A manual GitHub issue** — the human fallback for reporting an install
   that never produced a working binary.

A future enabler may wire the installer path; doing so does not change any
decision in ADR-0038. Until then this is a documented, deliberate boundary —
NOT an oversight.

## See also

- [ADR-0038 §11 — Bootstrap paradox (Req 9)](../../adr/ADR-0038-friction-reporter.md)
- [ADR-0038 §9 — owner-local v1 scope](../../adr/ADR-0038-friction-reporter.md)
