---
approved_at: "2026-06-17T07:15:26Z"
approved_by: user
status: Approved
---
# Spec 103-lifecycle-followups: Lifecycle follow-up polish: gitutil fetch hardening, doctor regex, next short-IDs, phase cache status breadth

## Goal

Close four small, independent sharp edges deferred from specs 101 and 095, each tracked by an already-filed P3 bead. None changes a public contract or a happy path; each removes a failure mode that is currently silent or hostile:

- **R1** — a slow / auth-prompting origin can HANG (or prompt on stdin) during `mindspec spec create` instead of fast-failing into the documented local-HEAD + WARN fallback.
- **R2** — `mindspec doctor`'s bd schema-drift probe only recognizes one vendor's phrasing, so real drift in other phrasings goes undetected; and the multi-`bd`-on-PATH symlink-dedup logic is untested.
- **R3** — `mindspec next <id>` rejects a short-form bead ID (`xxxx`) even though the user clearly named a specific bead.
- **R4** — the phase `Cache.fetchChildren` queries a NARROWER status set than `advanceState` considers, silently dropping `blocked` and custom-status children from the cache, which can skew the derived phase / child counts.

## Background

These items were filed as P3 follow-ups and deferred so the parent specs could ship:

- **R1 / R3** came out of the GH #146 panel on spec 101 (the `spec create` remote-base + `next` claim-by-name work): `internal/gitutil.FetchRemote` was added in 101 R4 without the `GIT_TERMINAL_PROMPT=0` / timeout hardening the pre-existing `PushBranch` pattern hints at, and `next.SelectWorkByName` (101 R1) does an exact `item.ID == name` match.
- **R2** is a 101 follow-up on `internal/doctor`'s bd-health checks: `checkBdSchemaDrift`'s regex was anchored only on the Dolt `(column|table) … could not be found` phrasing, and `checkMultipleBdOnPath`'s symlink/duplicate-PATH dedup shipped without a direct test.
- **R4** descends from spec 095 / the phase-cache work (ADR-0023's phase-derivation, refined into the `phase.Cache` memoizer): the `advanceState` path was already widened to read `bead.AllStatuses(root)` (open, in_progress, blocked, closed + customs), but the sibling `Cache.fetchChildren` was left at the legacy hardcoded `open,in_progress,closed`.

All four are CI-verifiable through existing test seams (`execCommand`, `bdSchemaDriftRE`, `SelectWorkByName`, `SetListJSONForTest`) — no live git/bd/network required.

## Impacted Domains

- execution: `internal/gitutil` (R1) — the Git/process I/O boundary; FetchRemote and the other network-touching ops (push / remote show / DetectDefaultBranch) gain `GIT_TERMINAL_PROMPT=0` so git fails fast instead of prompting/hanging on stdin.
- workflow: `internal/doctor` (R2) and `internal/next` (R3) — doctor's bd schema-drift / multiple-bd-on-PATH health checks, and the `mindspec next <id>` claim-by-name path.
- core: `internal/phase` (R4) — the phase `Cache.fetchChildren` child-fetch that feeds `DerivePhaseFromChildren`.

## ADR Touchpoints

- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): Accepted; Domain(s) **workflow, execution, core** — covers all three declared domains by itself. Any new error surface (R1 fast-fail wrapping, R3 not-in-ready-set message) stays in the recovery-line / exit-code agent error contract.
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md): Accepted; Domain(s) execution — the Git/process I/O boundary ADR. R1's network ops live in `internal/gitutil`, the canonical git-exec edge ADR-0030 governs; the `GIT_TERMINAL_PROMPT=0` env hardening is applied at that boundary.
- [ADR-0023](../../adr/ADR-0023.md): Accepted; Domain(s) workflow, git, state — the "Phase derivation from bead statuses" contract, which explicitly enumerates `blocked` children (e.g. "some children closed, some open, some blocked → plan"). R4 restores the cache's child-fetch to the full status breadth that derivation contract assumes, so blocked/custom children are not dropped before `DerivePhaseFromChildren` sees them.

Every declared domain is covered by an Accepted ADR: execution (0035, 0030), workflow (0035, 0023), core (0035).

## Requirements

1. **R1 — gitutil network ops fast-fail instead of hanging (bead mindspec-o7tp).**
   `internal/gitutil.FetchRemote` and the other network-touching exec ops — `PushBranch`, `DetectDefaultBranch`'s `git remote show` fall-through — MUST set `GIT_TERMINAL_PROMPT=0` in the child process environment so git never blocks on a stdin credential/host prompt; on a slow or auth-required origin git then exits non-zero quickly, which the executor already treats as the signal to fall back to a local base + WARN. The env MUST be threaded through the `execCommand` seam so it is assertable in a unit test. **Design decision (env vs timeout):** `GIT_TERMINAL_PROMPT=0` is the minimal high-value fix and resolves the actual reported failure mode (an auth/host prompt blocking on stdin during `spec create`); it is chosen as the required behavior. A hard wall-clock `exec.CommandContext` timeout is the fuller fix but adds a tunable knob and a new failure surface (legitimately slow large fetches aborting) for a less-common cause; it is OUT OF SCOPE for this spec and may be filed as a follow-up if a non-prompting hang is observed in practice. Justification: every observed hang in the #146 report was a stdin prompt, which `GIT_TERMINAL_PROMPT=0` eliminates deterministically.

2. **R2 — broaden the doctor schema-drift regex and test the bd-on-PATH dedup (bead mindspec-vn4n).**
   `internal/doctor.bdSchemaDriftRE` MUST additionally match the common alternate schema-error phrasings so real drift in those phrasings is surfaced as a Warn instead of soft-skipping to OK: at minimum `no such column`/`no such table` (SQLite), `unknown column`/`unknown table` and the MySQL/Dolt `Error 1054` class, in addition to the existing `(column|table) … could not be found`. The broadening MUST stay conservative — an unrelated transient bd failure with none of these signatures still skips (OK), never false-warns. SEPARATELY, `checkMultipleBdOnPath`'s symlink/duplicate-PATH dedup (the `filepath.EvalSymlinks` + `seenResolved` + `seenDir` guards) gains a direct hermetic test: a single `bd` reachable via two PATH entries (a real dir plus a symlink to it, and/or a duplicated PATH dir) resolves to exactly-one-bd → OK with no false multi-bd Warn.

3. **R3 — `mindspec next <id>` resolves short-form bead IDs (bead mindspec-y4l9).**
   `internal/next.SelectWorkByName` MUST resolve a positional bead ID supplied in EITHER the short form (`xxxx`) or the full prefixed form (`mindspec-xxxx`) to the same ready bead, instead of requiring an exact `item.ID == name` match. **Design decision (normalization):** mindspec has no standalone ID-normalization helper, and the issue prefix is project-derived (`filepath.Base(root)`, written once into `.beads/config.yaml` `issue-prefix`), so a hardcoded `"mindspec-"` literal is wrong. The chosen normalization is suffix-aware matching inside `SelectWorkByName`: a name matches an item when `item.ID == name` OR `item.ID` ends with `"-" + name` (the short form is the suffix after the prefix's `-`). This resolves both forms against whatever prefix the ready set actually carries, invents no new helper, and keeps the "names a SPECIFIC bead → resolve exactly or fail, never fall through to items[0]" guarantee (spec 101 R1). When the normalized name is genuinely not in the ready set, the existing clear not-in-ready-set error (ADR-0035 shaped) is unchanged.

4. **R4 — phase Cache.fetchChildren uses the same status breadth as advanceState (bead mindspec-7rih).**
   `internal/phase.fetchChildren` MUST query children across the SAME status breadth that `advanceState`/`queryAllChildren` use — `bead.AllStatuses(root)` = built-ins (`open`, `in_progress`, `blocked`, `closed`) plus every project custom status — rather than the hardcoded `--status=open,in_progress,closed`. **Root cause confirmed:** `fetchChildren` (cache.go:194-195) issues `bd list --parent <id> --status=open,in_progress,closed -n 0`, which OMITS `blocked` and all custom statuses; the parallel `advanceState` path (complete.go:856-875, `queryAllChildren`) iterates `bead.AllStatuses(root)` and so DOES include `blocked` + customs. Because `Cache.GetChildren`→`fetchChildren` feeds `DerivePhaseFromChildren` at every phase-resolution call site (derive.go), a child in `blocked` (or a custom status) is silently dropped from the cache, so the derived phase / child count can disagree with the `advanceState` view (ADR-0023 explicitly counts blocked children when deriving `plan`). FIX makes `fetchChildren` status-breadth-equivalent to `queryAllChildren`. A hermetic test (via `SetListJSONForTest`) MUST pin that a `blocked` child is included in the returned set.

## Scope

### In Scope
- `internal/gitutil/gitops.go` — R1: set `GIT_TERMINAL_PROMPT=0` on `FetchRemote`, `PushBranch`, and `DetectDefaultBranch`'s network exec(s), threaded through `execCommand`; gitutil test asserting the env reaches the child.
- `internal/doctor/beads.go` — R2: broaden `bdSchemaDriftRE`; doctor test(s) for the new phrasings AND for `checkMultipleBdOnPath` symlink/duplicate-PATH dedup.
- `internal/next/select.go` — R3: short/full-form resolution in `SelectWorkByName`; `internal/next` test for both forms (and the still-not-found error).
- `internal/phase/cache.go` — R4: `fetchChildren` status breadth = `bead.AllStatuses(root)`; `internal/phase` cache test pinning a blocked child is included.

### Out of Scope
- `internal/complete/complete.go` `queryAllChildren` / `advanceState` (already correct; the reference implementation R4 converges toward).
- Any change to `DerivePhaseFromChildren`'s phase rules, the executor's local-HEAD + WARN fallback wording, or the `mindspec next` cobra wiring beyond `SelectWorkByName`.

## Non-Goals

- A wall-clock `exec.CommandContext` timeout for gitutil network ops (R1 design decision: deferred; `GIT_TERMINAL_PROMPT=0` is the required fix).
- The broader `adwu` "golangci-lint in the gates" process change (separate spec).
- Any new public API, ID-normalization helper package, or change to bd's own ID/prefix scheme.
- Cross-repo / live-network / live-bd integration testing — all ACs are hermetic via existing seams.

## Acceptance Criteria

- [ ] R1: `FetchRemote`, `PushBranch`, and `DetectDefaultBranch`'s `git remote show` exec each run with `GIT_TERMINAL_PROMPT=0` in `cmd.Env`; a `internal/gitutil` unit test using the `execCommand` seam asserts the env entry is present.
- [ ] R1: behavior on a non-prompting success path is unchanged (the fallback still fires only on non-zero exit).
- [ ] R2: `bdSchemaDriftRE.MatchString` returns true for representative `no such column`, `unknown column`, and `Error 1054` outputs, AND still false for an unrelated transient bd error message; covered by `internal/doctor` table tests.
- [ ] R2: a `internal/doctor` test for `checkMultipleBdOnPath` constructs a single `bd` reachable via two PATH entries (real dir + symlink, and/or duplicated dir) and asserts the check reports OK ("exactly one `bd` on PATH"), no Warn.
- [ ] R3: `SelectWorkByName(items, "xxxx")` and `SelectWorkByName(items, "mindspec-xxxx")` both return the bead whose `ID == "mindspec-xxxx"`; a name in neither form still returns the not-in-ready-set error; no fall-through to `items[0]`.
- [ ] R4: a `internal/phase` cache test stubs `listJSONFn` (via `SetListJSONForTest`) so a `blocked` child is in the result and asserts `fetchChildren` / `GetChildren` includes it; the query covers `bead.AllStatuses` breadth.
- [ ] `go build ./...` passes; the four added/changed tests pass under filtered `go test -run <Name> -timeout 120s ./internal/...`.

## Validation Proofs

- `mindspec validate spec 103-lifecycle-followups`: 0 errors (a lifecycle-binding WARN is acceptable pre-approval); ParseSpec extracts Impacted Domains {execution, workflow, core}.
- `go build ./...`: compiles clean.
- `go test -run TestFetchRemote -timeout 120s ./internal/gitutil/` (and the R2/R3/R4 named tests, each filtered): PASS.

## Open Questions

- [x] R1: should `DetectDefaultBranch`'s cheap cached `git symbolic-ref` step (offline, no network) also carry `GIT_TERMINAL_PROMPT=0`? RESOLVED: set it uniformly across every gitutil network-touching op (fetch / push / remote show / symbolic-ref). `symbolic-ref` never prompts, so the env is harmless there, and a single uniform helper keeps the seam consistent and avoids a per-op judgment call.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-17
- **Notes**: Approved via mindspec approve spec