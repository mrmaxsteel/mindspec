# AgentMind Phase 6 soak gate — deferral record

Spec 083 Bead 6 step 5 converts the spec-mentioned "7-day soak before
v1.0.0 pin" into a verifiable, date-stamped CI check. This document
records the current state of that gate and the mechanism by which it
becomes armed.

## What the gate does

A nightly CI job runs the Phase 3 integration test
(`scripts/test-e-continuity.sh` cross-repo grep + `make test-live-capture`
end-to-end against the published agentmind binary) and appends a line to
the artifact `agentmind-phase3-soak-history.txt` of the form:

```
<ISO-8601 date>  <result: pass|fail>  <agentmind-binary-sha>
```

Before `make pin-agentmind-release` (or `scripts/pin-agentmind-release.sh`)
is run against `v1.0.0`, mindspec maintainers assert that the artifact
contains **seven consecutive `pass` entries** ending at a date no earlier
than `today - 1 day`. If that streak is intact, the soak gate is
**armed** and Phase 6 may proceed.

## Current state (today): NOT YET ARMED

The soak gate prerequisites are unmet on every axis:

1. **Upstream binary does not yet exist.** `github.com/mrmaxsteel/agentmind`
   has not published a release; there is no binary to run the nightly
   integration test against.
2. **Nightly CI is not yet wired.** The `live-capture` workflow job in
   `.github/workflows/ci.yml` already exists, but it currently uses
   `cmd/agentmind-fake` as a stand-in. Once the upstream release exists,
   nightly CI will switch to downloading the released binary (via the
   manual-install URL pattern documented in `agentmind.md`) and the
   soak-history artifact will start accumulating entries.
3. **`pin-agentmind-release.sh v1.0.0` exits 2 today.** The script's
   upstream-tag gate guarantees that nobody can accidentally drop the
   `replace` directive against a non-existent upstream.

The soak gate is therefore **"not yet armed"**: the deferral mechanism
is the same one the spec uses for the Test G / Test A / Test B
preconditions — a single command flips the state once upstream is
ready.

## Arming the gate (when upstream ships)

1. AgentMind side publishes `v1.0.0` with the release artifact layout
   documented in `agentmind.md`.
2. Mindspec side updates `.github/workflows/ci.yml` `live-capture` job
   to download the released binary instead of building
   `cmd/agentmind-fake`, and adds a nightly cron trigger that appends
   to `agentmind-phase3-soak-history.txt`.
3. Once seven consecutive nightly `pass` entries accumulate
   (≈ 1 week), the soak gate is armed.
4. A maintainer runs `make pin-agentmind-release` (or
   `scripts/pin-agentmind-release.sh v1.0.0`). The script's
   verification step re-runs `go build` and `go test -short`, and on
   green commits the go.mod edit.

## Deferral path (alternative to nightly soak)

The plan (Bead 6 step 5) explicitly allows a documented alternative if
nightly CI is not wired in time:

> if the nightly soak artifact is not yet wired (depends on
> agentmind-side CI), record the deferral explicitly in the commit
> message and gate merging on a documented alternative (e.g. one-shot
> manual integration test re-run on mindspec's PR branch before merge).

Today the alternative is a one-shot manual integration test on the PR
that performs the pin. That PR's CI run must produce a green
`live-capture` and `test-e-continuity` job against the real published
binary. The PR description must link to the run.

## Cross-reference

- `agentmind.md` — manual-install URL pattern + checksum verification
  procedure that the nightly download step uses.
- `scripts/pin-agentmind-release.sh` — the script the soak gate gates.
- `scripts/test-e-continuity.sh` — the cross-repo Test E gate exercised
  by the nightly soak run.
- `.github/workflows/ci.yml` — `live-capture` and `test-e-continuity`
  jobs; the nightly cron trigger will be added when the soak gate is
  armed.
- Spec 083 spec.md line 397 ("7 days of nightly runs") — original
  spec language; this document is the implementation.
- Plan 083 Bead 6 step 5 — verification mechanism.
