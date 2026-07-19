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

1. **Preflight** — resolve every immutable gate fact available before any
   commit is made (lineage and `--spec` hint agreement, epic/phase
   membership, branch existence/ancestry and reconcile evidence,
   orphan-sibling state, the panel decision, plan-content facts,
   pre-existing durable obligations) and evaluate every refusal *derivable
   from those facts* — all before the first mutation. A preflight refusal
   leaves the repository, the tracker, and the plan artifact
   **byte-identical** to their pre-call state (verified by the
   byte-identical refusal tests each verb's own bead pinned: `complete`'s
   AC-2 `--spec`-mismatch refusal, `plan approve`'s misaligned-work-chunks
   and absent-epic refusals, `impl approve`'s zero-mutation-on-refusal
   call-order test). The byte-identical claim is made **only** for these
   enumerated preflight refusals — see the artifact-materialization
   exception below.
2. **Commit** — the mutation sequence proper, in two parts:
   - **Artifact materialization** (`complete` only): the optional user
     `CommitAll` (`--commit-msg`) and the pathspec-scoped beads-artifact
     sync commit. These are *local, bead-branch-only, never-`main`*
     commits that materialize the very tip the two content gates measure —
     a property that is ENFORCED, not assumed: BOTH legs refuse (a
     `guard.NewFailure` with the worktree-recreating `mindspec next`
     recovery) when no bead worktree resolves, and a worktree-enumeration
     failure propagates as a preflight error; neither leg ever falls back
     to committing on the root/`main` checkout.
     The **doc-sync** and **ADR-divergence** gates deliberately validate
     the resulting committed bead tip *after* this subphase — their
     `base..beadHead` range must include the just-committed user work —
     so a doc/ADR refusal MAY land after those local commits. This is a
     documented, forward-reconcilable exception to the byte-identical
     preflight claim (like the migration exemption below): no tracker
     close, bead→spec merge, branch/worktree deletion, epic close, or
     `main` mutation has occurred; the worktree and its bead-branch
     commits are retained; and re-running after repair converges. The
     ordering is load-bearing in both directions: the panel gate must run
     *before* `CommitAll` (committing advances the bead tip past
     `reviewed_head_sha` and clears the dirt the panel decision measures),
     and doc-sync/ADR-divergence must run *after* it (or they would
     vacuously miss the work being completed).
   - **Lifecycle-affecting mutation**: the ordered set of tracker/git/Dolt
     operations the verb exists to perform (`bd close`, a bead→spec merge,
     branch/worktree deletion, an epic close, `FinalizeEpic`'s multi-leg
     git chain). Every mutation in this part is preceded by every refusal
     above — preflight refusals AND the two post-materialization content
     gates — none can fire mid-sequence and find work already undone,
     because there is no "undo": recovery is always forward (§3).
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
- **`complete`'s commits never target protected `main`.** The artifact-
  sync leg is pathspec-scoped and refuses rather than commit onto a main
  checkout (Spec 119 R3/AC-3/AC-4), and the user `--commit-msg` `CommitAll`
  leg refuses identically when no bead worktree matches (it targets the
  matched bead worktree ONLY — no root fallback) — a forward-reconcile
  contract that can commit onto the wrong branch is not actually safe to
  retry blindly.
- **A failure that cannot forward-reconcile is a refusal, not a silent
  partial state.** Every mutation whose failure cannot be absorbed by a
  bounded re-invocation (a durable-obligation marker write, a post-close
  Dolt durability check, a pending-obligation settlement write) returns a
  `guard.NewFailure` naming the exact re-run command — it never lets the
  command exit 0 having only partially mutated state.

#### §2(i)–(iii): Convergence-completeness (Spec 121 amendment)

Spec 119 stated the forward-reconcile contract above in general terms —
"re-invocation is always safe," "state re-derivation, never state
assumption." Spec 121 found two concrete ways that promise could still fail
to hold in practice, and pins the completing doctrine here as three named
clauses of this section (§2), because each is a refinement of the same
forward-reconcile contract rather than a new one:

- **§2(i) — Deadlock-free recovery graph, genuine forward exits.** Every
  refusal's named recovery MUST be a step that can actually change the fact
  being refused on. A refusal whose only named recovery is bare
  re-invocation, when re-invocation alone cannot change that fact, is a
  deadlock, not a convergence path — the CONVERGENCE promise of §2 is
  false for that state until a genuine forward exit is named. Two
  instances this spec closes: **the `mindspec-tpjn` all-orphans
  sequence** (`complete`'s step-1.6 preflight previously refused naming
  only the first orphaned sibling, so two closed-but-unmerged siblings each
  refusing on the other had no non-manual exit; the fix demotes to a WARN
  when the invoked bead is itself orphaned-closed, and otherwise names
  EVERY orphaned sibling with the full recovery sequence, so a finite
  chain of `mindspec complete` invocations converges); and **the
  `mindspec-q9ea` attested-restore exit** (the landed-merge identity
  predicate's no-durable-datum state now refuses with a NAMED, explicitly
  non-mechanical recovery — restoring the bead branch ref at the candidate
  merge's second-parent SHA, carrying its own human-verification marker —
  rather than a refusal with no forward path at all).
- **§2(ii) — Durably corroborated, revert/reapply-aware landed evidence.**
  Landed-work evidence a forward-reconcile decision relies on to CLOSE a
  bead MUST be corroborated by a durable identity datum the lifecycle
  itself recorded — a registered panel's `reviewed_head_sha`, a surviving
  bead-branch ref, or the merge-time landed-binding this amendment
  introduces — NEVER subject text alone, and never a content heuristic
  over what the second parent's commits contain (that a merge carries
  non-empty work is not evidence it carries THIS bead's work). The
  corroborating datum MUST additionally be revert/reapply-aware BY NET
  EFFECT: the first-parent chain SINCE the candidate merge, not merely the
  merge's historical presence, so a subsequently-reverted merge is not
  misidentified as landed while a revert-then-reapplied one still is (a
  permanent "ever-reverted ⇒ reject" rule would itself violate §2(i)'s
  deadlock-free rule by manufacturing a refusal an honest landed state can
  never clear). The merge-time binding itself MUST be recorded FAIL-CLOSED
  BEFORE any branch/worktree cleanup for that bead: a failed write
  suppresses cleanup and refuses recoverably rather than warning and
  continuing, so the branch survives as the corroborating datum and
  re-invocation converges. This closes `mindspec-q9ea`'s subject-only
  acceptance gap.
- **§2(iii) — Content-aware already-merged re-derivation.** Where the
  hosting workflow can discard a branch's SHAs entirely (a squash merge),
  "already landed on the target ref" MUST be re-derived from CURRENT-state
  net-effect content equivalence, not from SHA ancestry alone — ancestry
  remains a valid SUFFICIENT condition where it holds, but its ABSENCE must
  not be read as "not landed" when a squash (or an equivalent SHA-discarding
  merge) is a normal part of the workflow, and ancestry HOLDING must not be
  read as "landed forever" where the target's content can itself move
  backward (a true-merge-then-revert). This closes the `mindspec-3xqm`
  item-1 squash blind spot at both of its consumers (the protected-main
  already-merged probe and the doctor merged-carrier suppression), which
  route through one shared exported predicate so neither consumer can drift
  into its own reimplementation.

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

### §4: The machine-owned finalize carrier (Spec 121 amendment)

Spec 121 closes the one terminal step the forward-reconcile contract above
still left an operator to perform by hand: on a protected `main`, the
`chore/finalize-<specID>` carrier `FinalizeEpic` pushes (§2 already governs
its content — the regenerated tracker export, nothing else) still needed a
human to open, and then merge, its PR (`mindspec-uxl4`). §4 names the
machine's authority over that carrier explicitly, as a COMPLETION of §2's
forward-reconcile rather than a new grant:

- **The carrier is tracker-only.** `chore/finalize-<specID>` never carries
  reviewed code — the spec's implementation merges via the panel-gated
  impl PR, exactly as before this amendment. Because the carrier's entire
  content is a machine-regenerated `.beads/issues.jsonl` export, opening
  its PR is **always safe**: the automation MAY auto-open (and
  idempotently adopt an already-open) PR for it with no config gate.
- **Auto-merging it is opt-in, never default.** Merging that PR into
  `main` is admissible only behind an explicit config key
  (`auto_merge_finalize_pr`, default **false**) — merging to `main`
  without a human stays a deliberate operator policy, never a framework
  default — **and** affirmative green checks (an absent/unreported checks
  result is NOT green) **and** the head/base adoption pin (only a PR
  whose head is exactly the machine-owned carrier and whose base is
  `main` is ever adopted or merged; a same-head PR targeting a different
  base is left alone, never auto-merged) **and** a TRUE MERGE COMMIT,
  never squash or rebase, so the ancestry/net-effect consumers §2(iii)
  and the doctor merged-carrier suppression already rely on observe the
  carrier as landed.
- **Every failure degrades, never fails the verb.** Every leg of this
  automation (`pr create`, the existing-PR lookup, `pr checks`,
  `pr merge`, the reconcile-by-query) is classified
  DOCUMENTED-FORWARD-SAFE under §3 above: its failure is absorbed by a
  warning naming the leg, the shipped NOTE plus the doctor
  finalize-orphan surfacing, and the process exits 0 — the finalize
  itself already succeeded durably before this automation ever runs, so
  this automation can neither fail `impl approve` nor un-finalize it. A
  leg failure after the server-side mutation may have already landed
  (GitHub creating or merging while the client-side call itself errors)
  is reconciled by re-querying the exact head→base PR state through the
  same seam, rather than assumed unmerged.
- **Amend, not a new ADR.** §4 grants NO new code-review or merge
  AUTHORITY — the carrier holds no reviewed code, the merge is opt-in
  (default off), checks-gated, and pinned to the machine-owned head and
  `main` base, so it opens no new governance domain. It COMPLETES §2's
  forward-reconcile of the merged-unclosed-on-protected-`main` state this
  ADR already governs; ADR-0037's panel gate provably does not reach a
  panel-less, tracker-only carrier, so no panel-doctrine question is
  reopened either.

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
