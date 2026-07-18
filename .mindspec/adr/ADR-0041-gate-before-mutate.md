# ADR-0041: Gate-Before-Mutate — Preflight, Commit, Forward-Reconcile

- **Date**: 2026-07-18
- **Status**: Accepted
- **Domain(s)**: workflow, execution, core
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (the panel gate this contract's ordering rule keeps load-bearing — it must evaluate before the first mutation, not after), [ADR-0034](ADR-0034-ceremony-collapse.md) (the idempotent migration this contract explicitly exempts), [ADR-0023](ADR-0023.md) (beads/Dolt as the single state authority the forward-reconcile protocol re-derives from, never a git rollback), [ADR-0035](ADR-0035-agent-error-contract.md) (the recovery-line convention every refusal and every forward-reconcile failure carries), [ADR-0030](ADR-0030-executor-boundary.md) (the executor boundary the mutating legs this contract orders are dispatched through)

---

## Context

Spec 119 found the same defect shape three times across the three lifecycle
verbs `mindspec complete`, `mindspec plan approve`, and `mindspec impl
approve`: a refusal that is **derivable from facts already available before
any mutation** was instead discovered mid-sequence, after one or more
mutations had already landed. Concretely, before spec 119:

- `complete` resolved its owning spec from **cwd**, not from the bead's
  lineage — a `--spec` mismatch, or running from the wrong worktree, was
  discovered only after the tracker auto-commit and the artifact-sync commit
  had already mutated a checkout (R1/R2, AC-1/AC-2/AC-3).
- `plan approve`'s epic-resolution failure (`phase.FindEpicBySpecID`)
  degraded silently, so a plan could be marked `Approved` in its frontmatter
  with **zero** implementation beads created — the mutation (the frontmatter
  write) landed before the fact (a resolvable epic) was even checked (R1).
- `impl approve`'s orphan/obligation gate (spec 115) and the ADR-divergence
  backstop's supersede-ADR placeholder pre-create both ran interleaved with
  mutations rather than strictly before them, so a refusal on one leg could
  follow a placeholder file already written to disk by another (R1/P1).

Each of these is the same class of bug: **the gate-evaluation order was not
declared as a contract**, so it drifted independently in each verb as
features accreted. A refusal that lands after a mutation is not merely
cosmetic — it means "exit non-zero" no longer implies "nothing changed",
breaking the invariant every recovery message in this codebase (ADR-0035)
depends on: that a refusal is safe to retry blindly.

Spec 119 also found the flip side of the same problem: mutations that DO
need to happen in sequence (the tracker commit, `bd close`, the bead→spec
merge, the epic close, `FinalizeEpic`'s multi-stage git chain) had never had
their own failure-recovery contract made explicit. Three review rounds on
this spec's own fault-injection bead (Bead 6) found that "just retry" is not
automatically true for every mutation point — it has to be verified true,
point by point, and the points where it ISN'T (a swallowed error, a
best-effort write) have to be named as such rather than silently assumed
kill-testable.

## Decision

### 1. The three-phase contract: preflight → commit → reconcile

Every mutating lifecycle verb (`mindspec complete`, `mindspec plan approve`,
`mindspec impl approve`) follows the same three-phase shape:

1. **Preflight** — resolve every immutable gate fact (lineage, epic
   resolution, child-set state, plan-content facts, doc-sync/ADR-divergence
   diffs) and evaluate every refusal *derivable from those facts* — all
   before the first mutation. A preflight refusal leaves the repository,
   the tracker, and the plan artifact **byte-identical** to their pre-call
   state (verified by the byte-identical refusal tests each verb's own bead
   pinned: `complete`'s AC-2 `--spec`-mismatch refusal, `plan approve`'s
   misaligned-work-chunks and absent-epic refusals, `impl approve`'s
   zero-mutation-on-refusal call-order test).
2. **Commit** — the mutation sequence proper: the ordered set of
   tracker/git/Dolt operations the verb exists to perform (a tracker
   auto-commit, `bd close`, a bead→spec merge, an epic close, `FinalizeEpic`'s
   multi-leg git chain). Every mutation in this phase is preceded by every
   preflight refusal — none can fire mid-sequence and find work already
   undone, because there is no "undo": recovery is always forward (§3).
3. **Reconcile** — the recovery contract for an interruption anywhere in
   phase 2 (§3): a bounded, idempotent forward path back to either
   completion or a clean, named refusal. Never a rollback.

**The idempotent ADR-0034 migration is EXEMPT** from the preflight-before-
mutation ordering: `phase.EnsureMigrated`/`phase.EnsureMigratedWithCache`
runs ahead of every preflight in all three verbs. It is exempt on its own
terms, not a hole in this contract — it is read-only-or-idempotent by
construction (a no-op on an already-migrated epic or a nonexistent one), so
running it before preflight can never produce a mutation a subsequent
preflight refusal would need to have prevented. Treating it as part of
"phase 1" would be equally correct; it is called out explicitly here so a
future reviewer does not mistake it for an ordering violation.

### 2. Forward-reconcile, never rollback

When phase 2 is interrupted — a process kill, an infrastructure failure, an
operator's Ctrl-C — between two of its mutations, the recovery contract is:

- **Re-invocation is always safe.** The same command, run again, either
  converges to the same terminal state (completion) or refuses with a
  clean, named, ADR-0035-shaped recovery line. It never requires manual git
  surgery, a rollback of a landed commit, or "just delete the branch and
  start over."
- **State re-derivation, never state assumption.** Every reconcile step
  re-reads the CURRENT tracker/git state and decides from there — it does
  not assume the interrupted run's in-memory state. This is what makes
  Spec 119 R4's merged-unclosed / branch-less reconcile possible: a bead
  whose worktree and branch are already gone (because a prior `complete`
  invocation's merge-and-cleanup mutation landed but the invocation died
  before recording durable evidence) is recognized from the landed merge
  commit itself (`lifecycle.MergedUnclosed`/`FindLandedMerge`), never from
  an assumption that the branch must still exist.
- **Idempotent by construction, not by accident.** `bd close` tolerates an
  already-closed bead; `bd dolt commit` is a no-op on a clean working set;
  a re-attempted git merge of an already-merged branch is an ancestor
  no-op; `isAlreadyRemovedErr`/`isAlreadyClosedErr` absorb re-removals and
  re-closes. Each of these is a DELIBERATE idempotency property of the
  underlying operation, not a hope.
- **Tracker-only commits never target protected `main`.** The tracker
  auto-commit / artifact-sync-commit legs of `complete` are pathspec-scoped
  and refuse rather than commit onto a main checkout (Spec 119 R3/AC-3/AC-4)
  — a forward-reconcile contract that can commit onto the wrong branch is
  not actually safe to retry blindly.
- **A failure that cannot forward-reconcile is a refusal, not a silent
  partial state.** Every mutation whose failure cannot be absorbed by a
  bounded re-invocation (a durable-obligation marker write, a post-close
  Dolt durability check, a pending-obligation settlement write) returns a
  `guard.NewFailure` naming the exact re-run command — it never lets the
  command exit 0 having only partially mutated state.

### 3. The kill/forward-safe classification (AC-26's fault-injection matrix)

Not every mutation point needs — or can honestly receive — an individual
kill test. Spec 119 Bead 6's fault-injection suite (`internal/complete`,
`internal/approve`, `internal/executor`) classifies every significant
post-preflight mutation point in all three verbs into exactly one of two
buckets:

- **KILL-TESTED** — the point's failure genuinely TERMINATES the run (a
  `return err`/`guard.NewFailure`, not a swallowed warning), and a real
  mechanism can enact that termination while the mutation itself still
  lands for real: a real-git decorator executor (the actual git mutation
  happens, then the decorator forces a terminal error), a terminating
  tracker seam (a package-var wrapper that mutates an in-memory tracker
  fake then fails), or a stage-labeled executor hook (for a mutation chain,
  like `FinalizeEpic`'s, with no existing seam separating its internal
  steps). Each kill test re-invokes the same verb and asserts convergence
  to completion or a clean, named, recoverable refusal.
- **DOCUMENTED-FORWARD-SAFE** — the point's error is swallowed by design
  (a `result.Warnings` append, a `_ =` discard, a warn-print) and the run
  CONTINUES regardless — no mechanism can enact a "kill" there because
  there is no termination to enact. These points are named in the test
  file with the exact code cite proving the swallow, rather than
  fictitiously "kill-tested" through a seam that cannot actually terminate
  anything. An interruption at one of these points leaves an idempotent,
  re-runnable state by the SAME construction that makes the error safe to
  swallow in the first place (the audit-trail record is the metadata's
  absence, or a value a subsequent run simply rewrites).

Fabricating a kill test against a swallowed-error point (or asserting a
"kill" that never actually terminates the run) is a worse failure mode than
not testing that point at all — it launders an untested claim into a
green checkmark. The classification above is the standing rule for every
future mutation point this contract's phase 2 grows: classify it as one of
these two, honestly, before writing its test.

## Consequences

### Positive

- "Exit non-zero" now provably means "nothing changed, or the change already
  landed durably" for every preflight refusal in all three lifecycle verbs —
  the invariant every ADR-0035 recovery line depends on.
- The forward-reconcile contract gives every future mutation-chain
  interruption (a crash, an operator Ctrl-C, an infra blip) a **named**
  recovery path instead of an implicit "hope re-running works" — and the
  fault-injection suite proves it for the mutation points that exist today.
- The kill/forward-safe classification stops a future contributor from
  either (a) skipping a mutation point's testing entirely because "it's
  hard to kill-test" or (b) faking a kill test that doesn't actually
  terminate anything — both failure modes this spec's own three review
  rounds caught in earlier drafts of Bead 6.

### Negative / Tradeoffs

- The contract adds real ordering discipline cost: a future feature that
  wants to add a new mutation to `complete`/`plan approve`/`impl approve`
  must place its refusal-derivable checks in preflight, not interleaved
  with mutations — a constraint, not a suggestion.
- The kill/forward-safe classification is a discipline enforced by review
  and by this ADR's text, not by a mechanized checker — nothing in the
  binary currently detects a newly-added mutation point that was never
  classified. This mirrors ADR-0040 §2's own "the ratchet is a discipline,
  not a mechanism" tradeoff.

## Alternatives Considered

### 1. Leave gate-ordering as an unwritten per-verb convention

Rejected: this is exactly the status quo that produced the R1/R2/R4
defects this spec fixes — each verb's ordering drifted independently
because no shared contract named the invariant. Writing it down once, and
citing it from all three verbs' preflight code, is what stops a fourth
verb (or a future edit to one of these three) from silently re-introducing
the same defect shape.

### 2. Rollback (git revert / branch reset) instead of forward-reconcile

Rejected: a rollback needs to know exactly what to undo, which is precisely
the information an interrupted process has lost. Forward-reconcile instead
re-derives the CURRENT state from the tracker/git ground truth and decides
from there — it needs no memory of what the killed process was doing, only
what it left behind. This is the same principle ADR-0023 already commits
to for tracker state (Dolt as the single source of truth, never a git-side
shadow); this ADR extends it to the mutation-recovery contract.

### 3. Kill-test every mutation point uniformly, including swallowed-error ones

Rejected: a mutation point whose error is swallowed by design has no seam
that can make it terminate the run — forcing one open would require either
changing production behavior (turning a deliberate best-effort write into a
hard failure, a regression) or writing a test that doesn't actually test
what it claims to (asserting "the run stopped" when nothing made it stop).
The classification in §3 names this distinction explicitly instead of
letting it stay implicit and get faked under time pressure.
