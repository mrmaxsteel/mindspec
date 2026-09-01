# Bead 1 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.1
- Branch: bead/mindspec-ta30.1
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.1
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R3 ("No
  self-worsening hints; `repair phase` backward-move guard"), the AC-5 text
  in §Acceptance Criteria, and the settled-facts block (esp. settled fact 3:
  run harness-adjacent tests with `-short` only).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §"Plan-level
  choices the spec delegates, resolved" (the R3.3 lock home + sanction
  boundary + AC-5 leg mapping) and §Bead 1 in full. The bead-1 steps are
  quoted verbatim below; the plan is the authority if any paraphrase here
  drifts.
- `internal/journal/lock.go:55-61` — the `withReportsLock` read-modify-write
  SHAPE precedent (see Shared helpers below for what it is and is not).

## Files in scope (from plan §Bead 1, quoted verbatim)

The plan's five step texts govern; work ONLY on these surfaces:

1. "`internal/phase/order.go` (new): the lifecycle order comparator
   (`spec < plan < implement < review < done`; unknown values compare
   incomparable → treated as backward for guard purposes, stated in the doc
   comment) + `internal/phase/order_test.go`. Beside it, the **sanctioned
   backward API** (round-1 G4.2 pin): exported
   `phase.SanctionedDerivedSync(root, epicID) error` — re-derives the
   child-derived phase INSIDE the lock's critical section and writes it,
   backward permitted, doc-commented as the ONLY sanctioned backward write
   surface; its exported-ness is fenced by a call-site enumeration test (the
   `impl adopt` house pattern) whose bead-1 form asserts ZERO production
   callers outside the allowlist `{cmd/mindspec/release.go}` — satisfiable at
   bead 1's own gate, before release is wired (round-2 G3.1); bead 2 TIGHTENS
   the same test to exactly-one when it wires the caller."
2. "`internal/phase/phaselock.go` (new): kernel-held file lock — `flock(2)`
   on unix, `LockFileEx` on Windows (round-4 G2: the journal package's
   O_EXCL-lockfile idiom is NOT carried to this lock; only its
   `withReportsLock` read-modify-write shape is) — exposing
   `WithPhaseWriteLock(root, epicID, fn)` — reads stored phase inside the
   critical section, passes it to `fn`, performs the conditional
   `MergeMetadata` write `fn` returns, releases." Operational design, pinned
   by the plan: "lock path = `.mindspec/phase.lock`; the managed
   `.gitignore` block gains that entry (beside the existing
   `.mindspec/session.json` line) and a test asserts lock acquisition leaves
   zero blockable git dirt; acquisition uses a BOUNDED wait (poll-with-
   deadline, default 10s, constant pinned in the file) — on timeout, an
   ADR-0035 refusal naming the lock path and a re-run recovery line (never a
   delete-the-lock instruction); unix crashed holders self-release (flock is
   kernel-held); the Windows path uses a mechanically atomic primitive:
   `LockFileEx` (exclusive byte-range lock) on a lock file opened with a
   RETAINED handle WITHOUT `FILE_SHARE_DELETE`, held for the whole critical
   section — atomicity and ownership are the OS's, release is owner-only
   (`UnlockFileEx` + `CloseHandle`), and a crashed holder's lock releases
   with its process, mirroring flock semantics. … No PID/timestamp content,
   no takeover, no deletion step."
3. "Wrap every existing `mindspec_phase` writer in the lock (values
   unchanged in this bead): `plan.go:455`, `spec.go:76`, `complete.go:1366`,
   `release.go:152`, `impl.go:577-579`, `impl.go:611-613`,
   `adopt.go:424/:437`, `migrate.go:59-61` (its absent-check moves inside
   the critical section), `repair.go:189-191`. The `spec.go:84` creation
   seed stays excluded (spec's stated rationale)." (Paths: internal/approve/
   plan.go, internal/approve/spec.go, internal/complete/complete.go,
   cmd/mindspec/release.go, internal/approve/impl.go,
   internal/approve/adopt.go, internal/phase/migrate.go,
   cmd/mindspec/repair.go. Line numbers were pinned at the spec base —
   re-locate by content if drifted; the WRITER SET is the contract.)
4. "`cmd/mindspec/repair.go`: the backward-move guard — refuse a write that
   moves stored backward unless `--force-backward "<reason>"` (flag name
   pinned here) is passed; refusal carries an ADR-0035 recovery line
   (inspection action); forced success output names the recorded reason;
   the guard compares against the stored value read under the lock."
5. "`internal/phase/writer_inventory_test.go` (new): the AC-5(vi) anti-drift
   test — every `MergeMetadata` call site writing `mindspec_phase` executes
   inside the lock's critical section; a new unlocked writer is red."
6. "Legacy-migration contention test (AC-5(vii)) via
   `SetMergeMetadataForTest` (`migrate.go:21`) + a contending locked
   writer."
7. `.gitignore` — the `.mindspec/phase.lock` entry (step 2's operational
   design; also add it to the setup-managed gitignore content if a builder
   manages one — check at impl time, note the finding in Deviations if none
   exists).

## Pinned design decisions — implement as written, do NOT relitigate
- Lock path `.mindspec/phase.lock`; gitignored; no-blockable-git-dirt test.
- Bounded poll-with-deadline wait, default 10s, constant pinned in
  phaselock.go; timeout = ADR-0035 refusal naming the lock path with a
  re-run recovery line that NEVER advises deleting the lock file.
- Windows = `LockFileEx` exclusive byte-range lock on a RETAINED handle
  opened WITHOUT `FILE_SHARE_DELETE`; owner-only release
  (`UnlockFileEx`+`CloseHandle`); death-releasing; the stale-file class is
  moot (the FILE is not the lock) — a pre-existing empty/partial lock file
  must not block (acquisition simply succeeds). No PID/timestamp content,
  no takeover, no deletion step.
- Backward comparator over the total order `spec < plan < implement <
  review < done`; unknown values treated as backward (guard-conservative).
- `phase.SanctionedDerivedSync(root, epicID)` exported NOW, fenced at THIS
  bead to ZERO production callers outside the allowlist
  `{cmd/mindspec/release.go}` (bead 2 wires release and tightens the same
  test to exactly-one — do not wire release yourself).
- `repair phase --force-backward "<reason>"`: reason required and recorded;
  the SUCCESS OUTPUT LINE itself names the reason (not only metadata).
- Writer wrap changes NO written values in this bead — bead 2 owns value
  semantics. `spec.go:84`'s epic-creation seed stays outside the lock.
- Fixture-state note (plan round-3 G2, forward-compatibility): construct the
  guard fixtures for AC-5(ii)/(iii)/(v) with the epic metadata
  `mindspec_phase_semantics: "v2"` PRESENT. The key becomes meaningful in
  bead 2 (its reconciliation would otherwise consume the stored-implement/
  all-open condition at repair's entry); setting it now is harmless and
  keeps these fixtures valid unmodified when bead 2 lands.

## Spec ACs this bead claims (quoted verbatim from spec.md AC-5; this bead
owns legs (ii),(iii),(v),(vi),(vii) — legs (i),(iv),(viii) are bead 2's)
- "(ii) invoking the repair-phase code path directly on the fixture refuses
  the backward write, leaves `mindspec_phase=implement`, and exits non-zero
  with a recovery line;"
- "(iii) with the force flag + reason, the write proceeds, the reason is
  recorded, AND the success output line itself names the recorded reason
  (round-2 G9 — matching R2.1/R3.3), not merely the stored metadata;"
- "(v) concurrency leg (round-2 G6): with the R3.3 lock in place, a
  concurrent writer advancing stored phase to `review` while repair is
  inside its critical section (driven via the `repairMergeMetadataFn` seam
  plus a contending goroutine on the real lock in a fixture repo) never
  ends with stored regressed to `implement` — repair either refuses on the
  fresh under-lock read or its stale write provably never lands; the final
  stored value is `review`;"
- "(vi) writer-inventory anti-drift (round-3 G1): the grep/seam-anchored
  assertion that every `MergeMetadata` call site writing `mindspec_phase`
  executes inside the R3.3 lock's critical section — introducing a new
  unlocked writer is red;"
- "(vii) legacy-migration contention leg (round-3 G1): `EnsureMigrated`'s
  backfill (driven via `SetMergeMetadataForTest`, `migrate.go:21`)
  contending with a locked writer never lands its derived value over the
  fresher locked write — under the lock its absent-check re-runs and
  no-ops; the final stored value is the locked writer's;"
- Also binding, from AC-5's closing sentence: "No test executes an emitted
  string."

## Shared helpers to REUSE (do not reimplement)
- None — bead 1 is the root of the dependency graph (plan: "Depends on:
  None"). Nothing from this spec's other beads exists yet.
- `internal/journal/lock.go:55-61` (`withReportsLock` + `acquireFileLock`)
  is a SHAPE precedent to imitate — the read-modify-write-under-file-lock
  structure — NOT a helper to import: the phase lock is kernel-held on both
  platforms (flock / LockFileEx) while journal's Windows path is the
  O_EXCL-lockfile idiom the plan explicitly does NOT carry over.
- Existing seams you WILL use in tests (do not duplicate):
  `repairMergeMetadataFn` (cmd/mindspec/repair.go:28),
  `SetMergeMetadataForTest` (internal/phase/migrate.go:21), and the
  per-writer `*MergeMetadataFn` seam vars in their packages.

## Tests to add
- `internal/phase/order_test.go` — comparator total-order + unknown-value
  conservatism (supports AC-5(ii)).
- `internal/phase/phaselock_test.go` — real-goroutine contention under
  `-race`; timeout error propagation (ADR-0035 refusal, no
  delete-the-lock text); crashed-holder recovery (kill the flock holder;
  next acquire succeeds — Windows leg seam-driven on non-Windows CI);
  barrier-controlled two-contender exclusivity (exactly one inside, loser
  blocked until release); leftover-file leg (empty/partial pre-existing
  lock file: acquisition succeeds); no-blockable-git-dirt assertion.
  Covers AC-5(v)'s lock substrate.
- `internal/phase/phaselock_test.go` (or repair_test) — the AC-5(v)
  concurrent-writer leg exactly as quoted (contending goroutine advances
  stored to `review` mid-critical-section; final stored value `review`).
- `internal/phase/writer_inventory_test.go` — AC-5(vi) as quoted.
- `internal/phase/migrate` contention test — AC-5(vii) as quoted.
- `cmd/mindspec/repair_test.go` — AC-5(ii) backward refusal (stored
  unchanged, non-zero, recovery line) and AC-5(iii) forced write (reason
  recorded AND printed in the success line); fixtures marker-PRESENT per
  the pinned note.
- Call-site enumeration fence test for `SanctionedDerivedSync` — zero
  production callers outside `{cmd/mindspec/release.go}` (the `impl adopt`
  house pattern; see cmd/mindspec/impl_adopt_test.go for the shape).

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above,
no reimplementing existing helpers, exactly ONE commit whose message ends
with `Deviations: <list or "none">`, and tests must PASS —
`go build ./... && go test ./... -short` green in the bead worktree
(`-short` is mandatory: without it an LLM-gated harness test shells out to
a real claude CLI and hangs — spec settled fact 3). Use absolute paths in
every command; scratch files ONLY under
/private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.1): phase write lock, backward guard, repair hardening

- internal/phase: order comparator + SanctionedDerivedSync (zero-caller fence)
- internal/phase/phaselock.go: flock/LockFileEx lock, bounded wait, tests
- wrap all nine mindspec_phase writers (values unchanged)
- cmd/mindspec/repair.go: --force-backward guard + reason in output
- writer-inventory + contention + migration-contention tests

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.1`.
- Test counts: passed / failed / skipped for the bead's test scope
  (`internal/phase`, `cmd/mindspec` repair tests, plus the full
  `go test ./... -short` result line).
- Flagged deviations from this prompt (one line each + reason).
