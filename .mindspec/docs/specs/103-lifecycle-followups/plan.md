---
adr_citations:
    - ADR-0035
    - ADR-0030
    - ADR-0023
approved_at: "2026-06-17T07:26:37Z"
approved_by: user
bead_ids:
    - mindspec-5e3o.1
    - mindspec-5e3o.2
    - mindspec-5e3o.3
    - mindspec-5e3o.4
spec_id: 103-lifecycle-followups
status: Approved
version: "1"
---
# Plan: 103-lifecycle-followups

Lifecycle follow-up polish: four small, independent sharp edges deferred from
specs 101 and 095, one per requirement, each removing a silent or hostile
failure mode without touching a public contract or a happy path.

## ADR Fitness

The spec declares three impacted domains — execution, workflow, core — and
every one is covered by an Accepted ADR:

- **ADR-0035 (agent-error-contract)** — Accepted; Domain(s) **workflow,
  execution, core**. Covers all three declared domains by itself. The only new
  error surfaces here (R1's gitutil fast-fail wrapping, R3's not-in-ready-set
  message) stay inside the recovery-line / exit-code agent error contract; no
  new error shape is introduced.
- **ADR-0030 (executor-boundary)** — Accepted; Domain(s) **execution**. The
  Git/process I/O boundary ADR. R1's network ops live in `internal/gitutil`,
  the canonical git-exec edge ADR-0030 governs; the `GIT_TERMINAL_PROMPT=0`
  env hardening is applied at exactly that boundary, threaded through the
  package's existing `execCommand` seam.
- **ADR-0023 (phase derivation from bead statuses)** — Accepted; Domain(s)
  **workflow, git, state**. Enumerates `blocked` children when deriving the
  `plan` phase ("some children closed, some open, some blocked → plan"). R4
  restores the phase cache's child-fetch to the full status breadth that
  derivation contract assumes, so blocked/custom children are not dropped
  before `DerivePhaseFromChildren` sees them.

ADR-coverage check: execution → {0035, 0030}; workflow → {0035, 0023}; core →
{0035}. All three covered → `mindspec validate plan` adr-coverage passes.

## Testing Strategy

All four ACs are CI-verifiable through pre-existing test seams — no live
git / bd / network. Each bead adds RED-first tests that fail against today's
code and pass after the fix, plus leaves the existing success/fallback tests
green:

- **R1** — `execCommand` seam (gitutil). The env-capture assertion is RED
  today (no env set); existing `TestFetchRemote` / `TestDefaultBranch_*`
  success+fallback behavior is unchanged.
- **R2** — `bdSchemaDriftRE.MatchString` table test + the in-repo `fakeBdDir`
  / `t.Setenv("PATH")` / `os.Symlink` hermetic seam.
- **R3** — direct `SelectWorkByName` unit calls (no seam needed).
- **R4** — `SetListJSONForTest` / `listJSONFn` seam (the one `cache_test.go`
  already uses).

Gate per AGENTS.md: `go build ./...` then filtered
`go test -run <Name> -timeout 120s ./internal/...` (NEVER `./internal/harness/...`).

## Decomposition rationale (4 beads, all `depends_on` empty)

The four requirements map 1:1 to four beads that touch four DIFFERENT files in
four DIFFERENT packages with ZERO overlap:

| Bead | Req | File | Package |
|:-----|:----|:-----|:--------|
| 1 (mindspec-o7tp) | R1 | `internal/gitutil/gitops.go` | gitutil (execution) |
| 2 (mindspec-vn4n) | R2 | `internal/doctor/beads.go` | doctor (workflow) |
| 3 (mindspec-y4l9) | R3 | `internal/next/select.go` | next (workflow) |
| 4 (mindspec-7rih) | R4 | `internal/phase/cache.go` | phase (core) |

Heuristic justification:
- **No shared-file merge-order note needed.** Unlike specs 100/101/102, no two
  beads write the same file, so there is no merge-order coupling and every
  `depends_on` is empty — the four can be cycled in any order / in parallel.
- **Right-sized (one concern each).** Each bead is a single localized change
  plus its tests: too small to split further, and combining any two would mix
  unrelated packages/domains in one review.
- **Serial chain length 1.** The dependency graph is four isolated nodes
  (longest path = 1), the cheapest possible structure.
- **Independently testable.** Each bead's AC is pinned by a hermetic test in
  its own package via an existing seam.

### work_chunks depends_on graph

```
mindspec-o7tp (R1)   depends_on: []
mindspec-vn4n (R2)   depends_on: []
mindspec-y4l9 (R3)   depends_on: []
mindspec-7rih (R4)   depends_on: []
```

All four already filed as P3 beads (no creation needed); all roots, no edges.

## Bead 1: mindspec-o7tp — gitutil network ops fast-fail (R1)

**Scope:** `internal/gitutil/gitops.go` — set `GIT_TERMINAL_PROMPT=0` on the
network-touching git exec ops so git fast-fails instead of prompting on stdin;
extend the gitutil test seam to assert the env reaches the child.

**Changed files:** `internal/gitutil/gitops.go`, `internal/gitutil/gitops_test.go`

**Steps**
1. Add a small uniform helper that sets `GIT_TERMINAL_PROMPT=0` on a built
   `*exec.Cmd`, applied to the network-touching ops: `FetchRemote`
   (`git fetch`), `PushBranch` (`git push -u origin`), and
   `DetectDefaultBranch`'s `git remote show` fall-through (and, per the
   resolved Open Question, the cheap `git symbolic-ref` step too — uniform,
   harmless where git never prompts).
   - ⚠️ **IMPL NOTE (panel): the env MUST be APPENDED, not a fresh slice.**
     Use `cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")` — the house
     idiom already at `gitops_test.go:94`. A fresh `[]string{...}` would clobber
     the inherited environment (PATH, HOME, git config discovery) and break git.
2. Apply the helper at each of the three network ops right after the
   `execCommand(...)` cmd is built and before `.CombinedOutput()` /`.Output()`.
3. Leave all non-network ops and the success/fallback control flow untouched:
   git still exits non-zero on a slow/auth-required origin, which the executor
   already treats as the signal to fall back to local base + WARN.

**RED tests**
- **NEW env-capture test** asserting `GIT_TERMINAL_PROMPT=0` is present in the
  child `cmd.Env` for `FetchRemote` / `PushBranch` / `DetectDefaultBranch`'s
  network exec.
  - ⚠️ **TEST NOTE (panel): the current `swapExec` / `capturedCall` seam
    captures only name+args (no env field), and the stub returns the `*exec.Cmd`
    BEFORE production sets `cmd.Env`.** So a TEST-ONLY seam extension is needed:
    either retain the returned `*exec.Cmd` (read `.Env` after the call) or add
    an `env []string` field to `capturedCall` populated from the cmd the seam
    hands back. Assert the captured `.Env` contains `GIT_TERMINAL_PROMPT=0`.
  - This is RED today: no gitutil op sets the env → assertion fails → GREEN
    after step 1.
- **Unchanged (regression guard):** existing `TestFetchRemote` and
  `TestDefaultBranch_*` still pass — success path unchanged, fallback still
  fires only on non-zero exit (covers AC "non-prompting success path
  unchanged").

**Acceptance Criteria**
- [ ] `FetchRemote`, `PushBranch`, and `DetectDefaultBranch`'s `git remote show`
      exec each carry `GIT_TERMINAL_PROMPT=0` in `cmd.Env` (appended, not
      clobbered), asserted via the extended `execCommand` seam.
- [ ] Non-prompting success path is unchanged; fallback still fires only on a
      non-zero exit (`TestFetchRemote` / `TestDefaultBranch_*` stay green).

**Verification**
- [ ] `go build ./...` passes
- [ ] `go test -run TestFetchRemote -timeout 120s ./internal/gitutil/` PASS
- [ ] the new env-capture test PASS; `TestDefaultBranch_*` PASS

**Depends on**
None

## Bead 2: mindspec-vn4n — doctor schema-drift regex + bd-on-PATH dedup test (R2)

**Scope:** `internal/doctor/beads.go` — broaden the bd schema-drift regex to
recognize alternate vendor phrasings (conservatively), and add a hermetic test
for the existing `checkMultipleBdOnPath` symlink/duplicate-PATH dedup.

**Changed files:** `internal/doctor/beads.go`, `internal/doctor/beads_test.go`

**Steps**
1. Broaden `bdSchemaDriftRE` (beads.go:326) to ALSO match the common alternate
   schema-error phrasings, in addition to the existing
   `(column|table) … could not be found`:
   - `no such column` / `no such table` (SQLite)
   - `unknown column` / `unknown table`
   - the MySQL/Dolt `Error 1054` class
2. Stay CONSERVATIVE: match only these distinctive schema signatures — do NOT
   broaden to generic phrasings that an unrelated transient bd error would trip
   (the `TestCheckBdSchemaDrift_UnrelatedFailureSkips` "some unrelated runtime
   failure" string MUST still NOT match → the check still reports OK/skip).
3. Add the symlink/duplicate-PATH dedup test for `checkMultipleBdOnPath`
   (the `filepath.EvalSymlinks` + `seenResolved` + `seenDir` guards already in
   place are currently untested directly).

**RED tests**
- **NEW `bdSchemaDriftRE.MatchString` table test:** representative
  `no such column`, `unknown column`, and `Error 1054` outputs → `true`;
  the existing unrelated-transient string → `false`. RED today (the new
  phrasings don't match the current `could not be found`-only regex).
- **NEW symlink-dedup test** extending the `TestCheckMultipleBdOnPath_*`
  pattern: a SINGLE `bd` reachable via two PATH entries — a real `fakeBdDir`
  plus a second dir that is an `os.Symlink` to it (and/or the same dir
  duplicated on PATH) — via `t.Setenv("PATH", …)` resolves to exactly one bd →
  `OK` ("exactly one `bd` on PATH"), NO false multi-bd Warn. (Maps AC: single
  bd via two PATH entries → OK, no Warn.)

**Acceptance Criteria**
- [ ] `bdSchemaDriftRE.MatchString` returns true for representative
      `no such column`, `unknown column`, and `Error 1054` outputs, AND still
      false for an unrelated transient bd error message.
- [ ] A `checkMultipleBdOnPath` test with a single `bd` reachable via two PATH
      entries (real dir + symlink and/or duplicated dir) reports OK ("exactly
      one `bd` on PATH"), no Warn.

**Verification**
- [ ] `go build ./...` passes
- [ ] `go test -run TestCheckBdSchemaDrift -timeout 120s ./internal/doctor/` PASS
      (incl. the new table test and the unchanged `_UnrelatedFailureSkips`)
- [ ] `go test -run TestCheckMultipleBdOnPath -timeout 120s ./internal/doctor/` PASS

**Depends on**
None

## Bead 3: mindspec-y4l9 — `mindspec next <id>` short-form ID resolution (R3)

**Scope:** `internal/next/select.go` — make `SelectWorkByName` resolve a
positional bead ID in either short (`xxxx`) or full (`mindspec-xxxx`) form to
the same ready bead, without a hardcoded prefix literal and without losing the
spec-101-R1 no-fall-through guarantee.

**Changed files:** `internal/next/select.go`, `internal/next/next_test.go`

**Steps**
1. In `SelectWorkByName` (select.go:36) make the match suffix-aware:
   `item.ID == name || strings.HasSuffix(item.ID, "-"+name)`. This resolves
   both the short form (`xxxx`) and the full prefixed form (`mindspec-xxxx`)
   against whatever issue-prefix the ready set actually carries — no
   hardcoded `"mindspec-"` literal (the prefix is project-derived).
2. Add a CODE COMMENT stating the **single-issue-prefix invariant**: the
   ready set carries exactly one issue-prefix (project-derived, written once
   into `.beads/config.yaml`), so a post-prefix suffix is globally unique
   within the set → no multi-match is possible → the suffix match is safe and
   still resolves to exactly one bead.
3. Preserve the spec-101-R1 guarantee: a name in NEITHER form returns the
   existing clear not-in-ready-set error — NEVER fall through to `items[0]`.

**RED tests** (reuse the existing `TestSelectWorkByName_*` patterns)
- **NEW both-forms test:** with items including `{ID:"mindspec-xxxx"}` that is
  NOT `items[0]`, both `SelectWorkByName(items, "xxxx")` AND
  `SelectWorkByName(items, "mindspec-xxxx")` return the `mindspec-xxxx` item.
  RED today (short form fails the exact `==` match).
- **NEW / extended not-found test:** a name in neither short nor full form
  returns the not-in-ready-set error and does NOT return `items[0]`
  (`TestSelectWorkByName_NamedNotInSet` already covers the no-fall-through
  shape; extend or mirror it for the suffix case).

**Acceptance Criteria**
- [ ] `SelectWorkByName(items, "xxxx")` and `SelectWorkByName(items,
      "mindspec-xxxx")` both return the bead whose `ID == "mindspec-xxxx"`
      (even when it is not `items[0]`).
- [ ] A name in neither short nor full form returns the not-in-ready-set
      error; no fall-through to `items[0]`.

**Verification**
- [ ] `go build ./...` passes
- [ ] `go test -run TestSelectWorkByName -timeout 120s ./internal/next/` PASS

**Depends on**
None

## Bead 4: mindspec-7rih — phase Cache.fetchChildren status breadth (R4)

**Scope:** `internal/phase/cache.go` — widen `fetchChildren` to the full
`bead.AllStatuses` breadth (built-ins + customs) that `advanceState` already
uses, in a single comma-joined `bd list` call, so `blocked`/custom children are
not dropped from the cache before `DerivePhaseFromChildren` sees them.

**Changed files:** `internal/phase/cache.go`, `internal/phase/cache_test.go`

**Steps**
1. Change `fetchChildren` (cache.go:194–195) to query the SAME status breadth
   as `advanceState`/`queryAllChildren` — `bead.AllStatuses(root)` (built-ins
   `open, in_progress, blocked, closed` + every project custom status) — instead
   of the hardcoded `--status=open,in_progress,closed`. `internal/phase` already
   imports `internal/bead` (derive.go, migrate.go).
2. ⚠️ **CRITICAL IMPL NOTE (panel): keep it a SINGLE comma-joined
   `bd list --status=<strings.Join(AllStatuses, ",")>` call — do NOT fan out
   one call per status.** A per-status fan-out (the `queryAllChildren` shape in
   complete.go) would break `TestCache_GetChildren_MemoizesPerEpic`, which
   asserts exactly 1 listJSON call per epic, and the GetChildren single-call
   contract documented on the method. One comma-joined `--status=` arg yields
   the same breadth in one call.
3. **Root threading:** `bead.AllStatuses` needs a `root`, but the current
   `fetchChildren(epicID)` / `GetChildren(epicID)` signatures carry none and
   many call sites pass only epicID. Resolve the status set inside
   `fetchChildren` from the repo root (cwd / `bead.AllStatuses` over the
   active root) WITHOUT changing the public `GetChildren(epicID)` signature,
   so the existing memoize/nil-receiver/call-site tests stay green. (The
   hermetic RED test stubs `listJSONFn`, which short-circuits before any real
   config read, so it is independent of root resolution.)

**RED test** (via `SetListJSONForTest` / `listJSONFn`, the seam `cache_test.go` uses)
- **NEW blocked-child test:** stub `listJSONFn` to return a child with
  `"status":"blocked"`; assert `GetChildren` (→ `fetchChildren`) includes it.
  RED today — `blocked` is omitted from the hardcoded
  `open,in_progress,closed`, so a real bd would drop it (the stub returns it
  regardless, so the meaningful assertion is on the captured `--status` argv).
- **Optionally assert the captured `--status=` argument carries the
  `bead.AllStatuses` breadth** (contains `blocked`) — this is the assertion
  that is genuinely RED against the hardcoded string.
- **Unchanged (regression guard):** `TestCache_GetChildren_MemoizesPerEpic`
  still passes — exactly 1 call per epic (proves the single-call constraint).

**Acceptance Criteria**
- [ ] A cache test stubs `listJSONFn` (via `SetListJSONForTest`) so a `blocked`
      child is in the result and asserts `fetchChildren` / `GetChildren`
      includes it; the captured `--status` arg covers `bead.AllStatuses` breadth.
- [ ] `TestCache_GetChildren_MemoizesPerEpic` stays green — exactly one
      listJSON call per epic (single comma-joined call, no per-status fan-out).

**Verification**
- [ ] `go build ./...` passes
- [ ] `go test -run TestCache_GetChildren -timeout 120s ./internal/phase/` PASS
      (new blocked-child test + unchanged `_MemoizesPerEpic`)

**Depends on**
None

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By (RED test) |
|---------------------|------|-------------|
| R1: `FetchRemote`/`PushBranch`/`DetectDefaultBranch` remote-show run with `GIT_TERMINAL_PROMPT=0` in `cmd.Env`; gitutil unit test via `execCommand` seam asserts it | Bead 1 | NEW env-capture test (seam extended to retain `.Env`) |
| R1: non-prompting success path unchanged (fallback only on non-zero exit) | Bead 1 | existing `TestFetchRemote` / `TestDefaultBranch_*` stay green |
| R2: `bdSchemaDriftRE` true for `no such column` / `unknown column` / `Error 1054`, still false for unrelated transient | Bead 2 | NEW `bdSchemaDriftRE.MatchString` table test + unchanged `_UnrelatedFailureSkips` |
| R2: single `bd` via two PATH entries (real dir + symlink/dup) → OK, no Warn | Bead 2 | NEW symlink-dedup test (`fakeBdDir` + `os.Symlink` + `t.Setenv("PATH")`) |
| R3: `SelectWorkByName(items,"xxxx")` and `(items,"mindspec-xxxx")` both resolve to `mindspec-xxxx`; neither-form name → not-in-ready-set error, no fall-through to `items[0]` | Bead 3 | NEW both-forms test + not-found test (extends `TestSelectWorkByName_*`) |
| R4: cache test stubs `listJSONFn` so a `blocked` child is included; query covers `bead.AllStatuses` breadth | Bead 4 | NEW blocked-child test (+ `--status` argv breadth assertion); unchanged `_MemoizesPerEpic` |
| `go build ./...` passes; the four added/changed tests pass under filtered `go test -run <Name>` | Beads 1–4 | per-bead Verification blocks |
