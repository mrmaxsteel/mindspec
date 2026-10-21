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
   - **Artifact materialization** (`complete` only) — **REVERSED by spec
     128 Bead 6 (OQ-1(a)): gates now run BEFORE any complete-authored
     commit, via a prospective-snapshot protocol.** The paragraph this
     replaces described the ORIGINAL design — the optional user
     `CommitAll` and the pathspec-scoped beads-artifact sync commit,
     committed FIRST, with the doc-sync/ADR-divergence gates validating
     the resulting committed tip AFTER. That ordering had a real cost: a
     gate refusal landed AFTER those local commits, so a retry that fixed
     the gate's complaint could still find the bead branch's tip already
     advanced past `reviewed_head_sha` (a false panel-staleness block on
     the very next run) or its dirt already cleared (masking what the
     gate needed to see). Spec 128 Bead 6 closes this by construction
     instead of documenting it as an accepted exception:

     The **prospective-snapshot protocol**: after the panel gate, the
     bead worktree's PROSPECTIVE content — tracked, untracked, and
     deleted, exactly what `git add -A` would stage — is captured as an
     immutable candidate tree `T` via a TEMPORARY index
     (`gitutil.SnapshotWorktreeTree`): the temp index is seeded from the
     worktree's own HEAD tip FIRST, then `add -A` stages over that seed
     with the named-artifact set (today `.beads/issues.jsonl`) excluded.
     Seeding is load-bearing: an unseeded temp index plus the exclusion
     would leave the excluded, currently-tracked artifact ABSENT from
     `T`, and a commit built from `T` would silently DELETE it from the
     branch; seeded, the exclusion means "unchanged at tip" — the
     artifact's tip blob survives in `T` untouched. The real index, refs,
     and worktree are never touched by the snapshot — only unreferenced
     loose objects are written. The doc-sync and ADR-divergence gates
     then measure `base..T` — the SAME candidate tree, via a tree-capable
     executor leg (`ChangedFilesTree`) and every OWNERSHIP/content read
     anchored at `T` too (a bare tree OID is a valid ref for `git show`/
     `git ls-tree`) — so attribution and diffed content come from the
     identical candidate any eventual commit will contain. "Committed
     tree ≡ measured tree" is thereby mechanical, not asserted.

     Only on a FULL gate pass does materialization run, building the real
     commit(s) FROM `T` itself via `git commit-tree` (plumbing — no
     hooks): (M) the user's own work materializes as commit `C`
     (`-p <tip>`), (R) the real index is refreshed to `C`
     (`reset --quiet --mixed`, index-only — the worktree is untouched),
     (S) the tracker artifact's refreshed blob materializes as commit `D`
     (`-p C`, or directly on the tip if `C` elides). Empty stages elide
     (a `T` equal to the tip's tree builds no `C`; an unchanged artifact
     builds no `D`) rather than stamping a contentless commit onto every
     ordinary completion. Because no complete-authored commit exists
     before the gates run, a gate refusal now ALWAYS leaves the bead
     branch tip at `reviewed_head_sha`, byte-identical to the pre-call
     state for this subphase too — the retry meets the identical gate,
     never a false panel-staleness block and never masked dirt.

     **S's own interval-edit classification (round-1 panel fix, spec
     128 Bead 6)**: `D`'s content is a SECOND, SEPARATE snapshot of the
     worktree taken after `R`'s index refresh — a real window exists
     between that snapshot and any earlier read of worktree state, into
     which an external writer (an editor flush, a concurrent agent) can
     land a GENUINE non-artifact edit. The delivered classification
     closes this at the exact tree about to be committed, not at an
     earlier, separate read: after computing `D`'s own delta from the
     current tip (`ChangedFilesTree(currentTip, T')`, where `T'` is the
     S-stage's own snapshot), the run REFUSES — named paths, before
     `MaterializeStageCommit`/the marker publication for `D` — if that
     delta contains ANY path outside the named-artifact set. Because the
     check inspects the SAME data the commit is built from rather than a
     prior read, the classification-to-snapshot race is closed by
     construction, not narrowed: there is no window left in which a
     swept edit could reach `D` undetected. The pre-existing post-`D`
     S-exit dirt re-classification (§3(i)) still runs, but is now
     DEFENSIVE only — a belt-and-suspenders re-confirmation that nothing
     changed between the delta check and the merge, not the primary
     enforcement point.
     Reconcile mode (the landed-merge path, `beadHead` anchored at the
     already-landed merge commit) is UNCHANGED — it keeps its own
     `M^1..M` measurement over the committed range; no commits are
     authored on that path at all (reconcile never runs `CommitPaths` or
     any other commit-producing helper — its gates only ever READ an
     already-landed range), so there is nothing for this protocol to
     reorder there. Both content gates keep the artifact
     (`.beads/issues.jsonl`) unmeasured as either source or documentation
     in EVERY mode — reconcile's committed-range diff and this
     protocol's prospective-snapshot diff alike — because it matches
     neither classifier's path prefixes; only the sync stage's own
     commit ever touches it.

     **Honest scope**: this closes the GATE-REFUSAL class — no
     complete-authored commit can precede a fallible content-gate
     refusal. A POST-gate failure (a bead→spec merge conflict, a
     `bd close` failure) still occurs after the materialization commits,
     exactly as at HEAD; that residual is spec 127's preserved-merge and
     forward-reconcile machinery (§3's staged-boundary classification,
     below), out of THIS reorder's scope and stated, not papered over.
     The panel-gate-before-materialization ordering is UNCHANGED and
     remains load-bearing in the same direction as before: the panel gate
     must still run before any complete-authored commit, or committing
     would advance the bead tip past `reviewed_head_sha` (false-firing
     staleness) and clear the dirt the panel decision measures
     (false-clearing).
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

**Fourth-verb clause (spec 124):** `mindspec next` is a preflight-leg-only addition.
`mindspec next` — which claims the selected bead in bd
(`bd update --claim`), records session/recording state, and creates the
`bead/<id>` branch and its worktree — adopts the **preflight leg** of this
contract for bead readiness: the mechanical readiness floor (the four
`mindspec bead ready-check` signals MF-1..MF-4) is evaluated after bead
selection and **before any mutation** — no claim, no branch, no worktree —
and a NOT-READY refusal exits non-zero leaving bd state, branches, and
worktrees **byte-identical** to their pre-call state (exit non-zero implies
nothing changed). The distinct `--allow-not-ready` override proceeds past a
failing floor, emits a warning naming every failing signal, and records a
durable override marker on the bead; it is orthogonal to the pre-existing
`--force` (session-freshness only), which gains no readiness authority.

This clause is a **preflight-leg-only addition** — a scope-deferral, not a
governance certification. It brings ONLY the readiness preflight under this
contract and makes **no claim** about `next`'s existing success-path
claim/branch/worktree mutation chain, which retains whatever
commit/forward-reconcile contract it already carries (spec 124 neither
grants nor removes one). In particular, this clause is NOT a certification
that `next`'s success path needs no commit/reconcile phase — that path DOES
mutate. Where this section says "all three verbs", read `next` as a fourth
member **for the preflight leg only**. Alternatives §1 below anticipated
exactly this shape: a fourth verb must not silently re-introduce the
gate-after-mutate defect; this clause records `next`'s adoption instead of
leaving the enumerated scope false.

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
- **`complete`'s commits never target protected `main`.** Spec 128 Bead 6
  (OQ-1(a)) replaced the pathspec-scoped artifact-sync commit and the
  user `--commit-msg` `CommitAll` with the prospective-snapshot
  protocol's commit-tree-built M/S-stage commits (§1, above); the
  never-main property survives that reorder as an ENFORCED refusal, not
  an assumption: `runMaterialization` refuses (rather than falling back
  to the root/main checkout) whenever no bead worktree matches — it
  targets the matched bead worktree ONLY (Spec 119 R3/AC-3/AC-4) — a
  forward-reconcile contract that can commit onto the wrong branch is
  not actually safe to retry blindly.
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

#### §3(i): The M→R→S→G→T staged-boundary classification (Spec 128 Bead 6)

The prospective-snapshot protocol's own materialization sequence (§1's
"Artifact materialization" bullet, above) is itself a mutation CHAIN with
internal kill points, classified here as a single named staged boundary
rather than five independent, ad-hoc mutation points:

- **M** — build commit `C` (from `T`, `-p <tip>`).
- **R** — refresh the real index to `C` (`reset --quiet --mixed`,
  index-only; the worktree is untouched).
- **S** — classify the exact delta about to be committed (any path
  outside the named-artifact set REFUSES here, before `D` is ever
  built — round-1 panel fix, above), then build commit `D` (`-p C`, or
  `-p <tip>` when `C` elides), then the S-exit dirt contract as a
  defensive re-check.
- **G** — the bead→spec merge (unchanged — the pre-existing
  preserved-merge/`--resolve-merge` machinery this protocol hands off
  to, never reimplemented here).
- **T** — terminal clear of the write-ahead marker (below), on both the
  ordinary and the landed-merge-reconcile completion paths.

**The write-ahead marker invariant**: before EITHER of `C`/`D`'s branch
advances, the intended tip is durably published as a marker ref
(`refs/mindspec/materialized/<bead-id>`) via per-ref compare-and-swap —
`update-ref` CREATE semantics for the first publication of a run
(absent→`C` or absent→`D` when `C` elides), the prior OID as the
expected-old operand for `C`→`D` — deliberately NOT the phase-metadata
lock spec 128 Bead 1 introduces elsewhere in `complete`: the two
subsystems stay uncoupled, so no lock-ordering edge exists between them.
The marker's single OID encodes BOTH the intended tip (its own value) and
the expected predecessor (its own first parent, since every intended tip
is built with `-p <predecessor>`), and the commit it names carries a
pinned `Mindspec-Stage` trailer identifying which of `C`/`D` it is — read
from that encoding, NEVER inferred from a tree diff (which is ambiguous
when a stage's own content happens to be a no-op). A failed marker
publication (an unexpected existing value: a concurrent invocation, or a
leftover) REFUSES naming the marker — one invocation can never overwrite
another's recovery evidence; a failed bead-ref advance leaves the
already-published marker in place as write-ahead evidence for the next
run's re-entry discriminator.

**Marker provenance (spec 128 final-review amendment)**: the ref and the
trailer are both authorable with ordinary repo git access — a
hand-authored trailer-bearing commit plus `git update-ref` reproduces
every structural property above, and the final review's adversary slot
executed exactly that forgery to resume-skip the panel and content gates.
Publication therefore pairs the marker with a binary-written bd-metadata
**attestation** (`mindspec_stage_marker_oid`, recorded through the bead
metadata seam immediately BEFORE each ref publish — mirroring the
spec-127 landed-merge evidence pattern), and the discriminator honors a
resume ONLY when the attestation names the observed marker OID exactly. A
trailer-bearing marker WITHOUT the matching attestation is classified
FOREIGN: cleared, then a normal fully-gated run. The attestation persists
after completion as an audit record; each publication overwrites it. An
attestation-read failure PROPAGATES (never mapped to "unattested", which
would destroy genuine recovery evidence on a bd blip; never to
"attested").

**Honest trust boundary (final-review round 2, R12)**: the attestation
NARROWS the forgery, it does not close it. bd metadata is written with
ordinary `bd` CLI access — which every implementer agent in this
project's own workflow holds by design — so an actor who forges the ref
and trailer can also write the pairing attestation with one additional
ordinary bd call. What the attestation actually buys: it defeats
git-only forgery (the exact round-1 executed repro), it forces the
forger to leave a durable, attributable write in the tracker's audit
trail rather than only in reflog-able git refs, and it converts the
premise from "a marker-shaped ref proves gates passed" to "the local
operator environment's bd store corroborates the publication". The
resume-skip-the-gates optimization therefore explicitly TRUSTS the local
operator environment; it is not an adversarial-integrity control against
a bd-capable actor. The escalation that would close it — when commit
signing is enabled, require the marker commit's signature to verify
against an operator-trusted key before honoring any resume (attested AND
signature-verified) — is tracked as follow-up bead `mindspec-an8q`.

**The re-entry discriminator**, evaluated before the panel gate on every
non-reconcile run: absent marker → normal run (re-snapshot, re-gate,
rebuild); tip == marker AND the attestation names the marker OID → the
published stage COMPLETED (its kind read from the trailer) — resume at
R+S (`C`) or the sole S-exit-then-merge route (`D`), skipping the
re-snapshot, the content gates, AND the panel gate entirely, because the
ATTESTED publication proves those already passed in the invocation that
published it (never the marker's bare existence — see the provenance
amendment above); tip == marker's own first parent AND the attestation
matches → the write-ahead advance is UNFINISHED — finish it (expected-old
= tip), then resume as above (the provenance check runs BEFORE the
advance, so a forged marker can never pull the branch ref forward); an
attestation mismatch on either leg → FOREIGN — clear the marker and
proceed as a normal run; tip ∉ {marker, marker's parent} → FOREIGN — a
genuinely different commit landed on the branch after materialization —
delete the marker and proceed as a normal run, the stale-SHA panel gate
evaluating exactly as it would with no marker at all (the cannot-launder
boundary is retained). The execution contract this discriminator makes true is
honestly **AT-LEAST-ONCE with per-operation idempotent replay** — NOT
exactly-once, which is unprovable after a kill immediately following a
no-op stage: R's index refresh is a pure state-function of (tip,
worktree); S's dirt classification is a pure read plus refusal; the
merge stage (G) carries the pre-existing preserved-merge/ancestry
convergence machinery unmodified. "Not skipped" is guaranteed by
routing — every path to the merge passes through R and the S-exit
contract; "not looped" is bounded by convergence — each replay is a
state-function of current, re-read state, never of the interrupted run's
remembered intent.

**The refusal guarantee, stated precisely**: a refused content gate
leaves the bead ref/tip, tracker state, real index, and worktree
unchanged, while UNREACHABLE LOOSE OBJECTS from the temp-index snapshot
may exist (gc-collectable — the same accepted class as the pre-existing
non-mutating merge preview's "unreferenced loose objects only"
residual). No repository-byte-identical claim is made beyond that —
the equality this protocol guarantees is the commit-tree construction
itself ("committed tree ≡ measured tree" is mechanical, not asserted),
not the total absence of any object ever written to the store.

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
  idempotently adopt an already-open) PR for it, gated by the
  `auto_open_finalize_pr` config key — **default true** (on by default;
  an operator may disable it, which also renders `auto_merge_finalize_pr`
  inert).
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

## Amendment (Spec 125): Re-attested landed-bindings under §2(ii)

*(Finalized by the spec-125 R4 bead that lands the first citing code —
the explicit `mindspec reattest` surface,
`internal/lifecycle.ReattestLandedMerge` — per the 122/123
amendment-lifecycle precedent.)*

§2(ii) above admits "the merge-time landed-binding" as a corroborating
datum. This amendment records that the binding's ADMISSIBILITY derives
from its corroborated-identity discipline, not from WHEN it is written:
a landed-binding written AFTER merge time by an explicit,
operator-invoked re-attestation is admissible iff it is DERIVED from an
independent git EXACT-second-parent match (exact match required;
fail-closed on every ambiguity — no exact match, ancestor-only
candidates, or competing candidates naming different beads) — never
from an operator-asserted SHA pair corroborating itself, never from
subject text alone, and never from an agent-writable binding
uncorroborated to a real exact merge. Ownership and landed-ness remain
two DISTINCT facts from two DIFFERENT sources, and the amendment is
careful not to conflate them: the merge subject's bead-branch name
NOMINATES which bead a merge belongs to (OWNERSHIP), and git topology
(a two-parent first-parent merge whose second parent EQUALS the bead's
landed tip) proves it LANDED (LANDED-NESS); the binding is a
git-corroborated cache of that joint fact, never an authority. Each
datum below corroborates LANDED-NESS by independent topology; NONE of
them is claimed to prove ownership independently of the subject. For a
bare bead (no surviving branch, no binding, no panel-sha), OWNERSHIP is
SUBJECT-NOMINATED — the same trust in the subject-to-name mapping that
spec 121 already relied on, and which sits squarely inside the
documented threat boundary (a party who can forge spec-branch history
is already out of the defended model; §2(ii) is not a git-only
anti-forgery mechanism). Re-attesting a bare bead is therefore the
EXPLICIT, operator-vouched, AUDITED recovery that trusts the subject
for ownership and topology for landed-ness — it is NOT claimed to be
independent of the subject.

The admissible LANDED-NESS corroborating data for a re-attestation,
each STANDALONE as topology evidence (ownership per the paragraph
above):

- **(a)** the git-derived exact-second-parent merge from the identity
  scan itself — the branch-deleted happy path; no surviving tip is
  required. This datum is STANDALONE LANDED-NESS (a real exact
  second-parent merge exists); it is NOT standalone OWNERSHIP proof —
  ownership of the scanned merge is subject-nominated per the paragraph
  above, so datum (a) is not circular: topology, not the subject,
  supplies the independent corroboration, and the subject only names
  which bead;
- **(b)** a surviving bead-branch tip that the exact match EQUALS;
- **(c)** a registered panel's `reviewed_head_sha` equal to an exact
  match's second parent (set semantics and non-corroborating values are
  governed by Amendment (Spec 129), part (ii));
- **(d)** an existing binding git-corroborated to an exact merge;
- **(e)** the ADR-0035 `mindspec-q9ea` human attested-restore marker —
  admitted ONLY as an explicitly-blessed, audited exit for the
  genuinely-no-mechanical-corroboration case, and NEVER the sole
  corroboration where a scan could instead derive the identity.

Every re-attest write carries an INSPECTABLE audit record held in bd
metadata via the existing helpers — the acting identity/authority, the
before/after binding values, the timestamp, the invoking operation, and
the corroborating git datum — so a wrong backfill is detectable and
reconstructable BY INSPECTION of the recorded before/after values. The
record lives in the same mutable metadata map: it is
detectable-by-inspection, NOT cryptographically tamper-proof, and this
amendment claims exactly that and no more.

The recovery is honestly scoped: re-attestation recovers exactly the
git-corroborable (exact-second-parent) subset of merged beads and
refuses the rest to the audited exit rather than guessing; it does not
claim fleet-wide recovery of the merged-bead history.

## Amendment (Spec 129)

### (i) Reachable fallback and audited read-back

`lifecycle.FindLandedMergeReachable` (the MF-3 reader) and
`lifecycle.ReattestLandedMerge` use the first-parent scan as primary.
Only zero owned two-parent candidates permits the reachable fallback:
`git log --merges --topo-order <spec-branch>`. The fallback admits only
source-named `Merge bead/<id>`, `Merge branch 'bead/<id>' into <spec-branch>`
and `Merge bead/<id> into <spec-branch>`. Explicit non-spec targets,
refreshes into a bead branch, and unrecognized subjects are excluded.
The undirected-subject target gap remains: `Merge bead/<id>` names no
target, so a merge into a sibling bead can be nominated.

The fallback is finder-only: no automatic acceptance of a fallback
candidate from a panel, surviving tip or binding alone. It returns
`LandedMergeOffFirstParent`, naming every candidate and
`mindspec reattest <id>`. Only the matching audited read-back below can
identify it. `MergedUnclosed`, `complete`'s reconcile, `stale_open`,
`orphan_hints`, plan approval and lattice adoption stay first-parent only;
the executor's merge-time binding and merge-resumption scans do too.

F1, F3, F4 and F5 are all git-derived safety checks:

- **F1:** exactly one admitted candidate; multiple candidates refuse as
  `multiple-fallback-candidates`, even with the same second parent. There
  is no grouping or tie-break that can let an empty sibling mask a revert.
- **F3:** a surviving bead tip must equal the candidate's second parent.
- **F4:** the candidate's net effect is checked at the CURRENT tip, with
  the Spec-125 revert/clean-full-removal discrimination anchored on that
  single merge. An accepted-then-reverted landing fails again.
- **F5:** any git or metadata read error is a plain error, never an
  absence result or a positive identification; a refusal writes nothing.

F2 is retired. Registrations are advisory output only, scanned in the
invoking checkout and its spec directory. Outside-history and locally
indeterminable SHAs name their registration and round; absent objects say
`(not determinable locally)`. Malformed revision expressions never reach
git. Empty and placeholder SHAs are skipped. Advisories change neither
exit status nor the binding and audit tuple; their count and scanned
roots are recorded as provenance. They are not read by automatic read-back.

Acceptance writes the binding and audited tuple in ONE `MergeMetadata`
call: `mindspec_landed_reattest_merge_sha`,
`mindspec_landed_reattest_second_parent`, and
`mindspec_landed_reattest_scanned_branch`, alongside the existing audit
keys. The fallback-provenance corroboration string is:

```
off-first-parent fallback; single-candidate, branch-tip, not-reverted and read checks passed; advisory registrations outside tip: <n>; panel roots scanned: <roots>
```

The string and the reattest refusal name the checks, not their labels,
because a consumer cannot look up F1–F5: single-candidate is F1,
branch-tip is F3, not-reverted is F4, and read is F5.

The validity contract requires `op == "mindspec reattest"`, RFC-3339 `at`,
non-empty `actor`, `scanned_branch` equal to the scanned spec branch, and
both SHA keys well-formed git object IDs. Absent or MALFORMED audits grant
nothing. A validity-qualified no-op requires a VALID audit whose tuple
equals both the binding and the single scan-derived candidate. Otherwise
explicit reattest rechecks safety and writes a valid record even when the
binding values are unchanged. Automatic read-back requires that same
three-way tuple equality AND F1, F3, F4 and F5 at read time. A later
executor binding rewrite leaves a stale audit that confirms nothing.

Read-back returns a plain error for F5, `LandedMergeOffFirstParent` for
multiple candidates or an invalid/mismatched audit,
`ErrLandedMergeContradiction` for a surviving-tip contradiction, and
`ErrLandedMergeReverted` for a revert. Both sentinels wrap
`ErrLandedMergeNotFound`; the off-first-parent error never wraps
`ErrLandedMergeNoCandidate`.

q9ea attested-restore is excluded from fallback exits: restoring the tip
can satisfy only F3 and cannot change the other refusal facts. The audited
forward exit for the dependent is
`mindspec next <dependent> --allow-not-ready=MF-3 --evidence <path-to-justification>`.

**Metadata-forge residual and bounded new reach.** The binding and audit
are agent-writable metadata, exported to tracked JSONL; one metadata
update can forge them together. F1, F3 and F4 are re-derived from git on
every read, so the forge gains only a candidate that reattest itself
would accept: a real, reachable, source-named, single, non-reverted merge
with no contradicting surviving tip. This includes an undirected
off-first-parent merge into a sibling bead with a later unlanded review
round. Spec 125 refused that topology: this is NEW REACH, bounded as
stated, and the forge remains visible in JSONL history.

### (ii) Non-corroborating panel datum and binding veto

Datum (c) is the SET of all well-formed `reviewed_head_sha` values across
registrations. See [Amendment (Spec 130)](#amendment-spec-130) for the
R3.5 ledger-matched `rebinds[].from_sha` extension. An exact second-parent match confirms; a non-matching set
is non-corroborating, never a contradiction or a veto. Empty, placeholder
and malformed values are non-corroborating and never git operands.
Reattest records a non-matching set as non-corroborating and can proceed
on datum (a). The set alone never confirms an ancestor or a fallback.

These semantics reach the CLOSE paths through `FindLandedMerge`:
`MergedUnclosed`, `complete`'s reconcile and `stale_open`, as well as
plan approval, adoption and orphan hints. They introduce no CLOSE
evidence Spec 125 did not admit: an exact tip, binding or panel match
confirms; a non-matching set alone yields `LandedMergeNoEvidence`, which
`MergedUnclosed` propagates as an error and never a close. On the normal
completion path, complete lands a commit no panel reviewed (OQ-3), so
the panel leg is normally non-corroborating; binding review to landing
belongs to the panel substrate.

**Binding veto versus audited repair:** a contradictory surviving tip or
landed binding still vetoes automatic identification regardless of panel
matches. Only explicit reattest may replace a contradictory binding with
the independently scan-derived exact identity, after ownership,
ambiguity, surviving-tip and revert checks (and F1/F3/F4/F5 on fallback).
The prior pair is recorded in the audit; a refusal writes nothing.

### (iii) Scoped, fingerprint-bound fourth-verb override

`mindspec next` checks plan-bead form, lineage/scope agreement, and the
existence of the lineage spec branch before mutation. The lineage spec
is the dispatch spec, including an unscoped claim with no resolved target.

`--allow-not-ready=<ids> --evidence <path>` names only MF-1 through MF-4.
Every currently failing signal must be named; indeterminate read failures
cannot be overridden. Bare, space-separated and `all` forms refuse.
Named-but-passing signals are not recorded. An entirely passing floor
needs no evidence and writes no marker.

Each claim that records failures requires fresh readable regular file
evidence of 1 byte to 64 KiB. Before ClaimBead, a schema-v2 marker REPLACES
the old marker with the failing named signals, complete sorted
failure-record digests, RFC-3339 timestamp, and evidence path/sha256/bytes,
never evidence text. Marker-write failure refuses; claim failure rolls
the marker back. Existing markers never authorize a claim, even after release.

`mindspec bead ready-check` honors only VALID markers whose recorded
digest sets cover every current failure record. Growth uncovers;
shrinkage stays covered. LEGACY and MALFORMED markers grant nothing.
Re-recording a claimed bead requires release and a fresh scoped re-claim
with all currently failing signals and evidence; release is offered only
when a claim exists. An indeterminate read instead requires fixing the
read first, never an unusable re-claim hint. Marker shape and
self-consistency provide no authentication of agent-writable metadata.


## Amendment (Spec 130)

- **Datum (c) — Rebind evidence (R3.5, R13.3).** The reviewed-SHA set also
  includes each R5.3-matched `rebinds[].from_sha`, including entries matched
  to the first round's carried baseline. Unbacked or hand-changed entries
  supply no evidence. Corroboration uses exact equality with the merge's
  second parent; no prefix matching or ancestor tolerance is permitted.
- **Pre-close audit (R4.5).** `panel_rebound_entries` copies the bead panel's
  rebind entries through the same fail-closed pre-close metadata stage as
  `panel_refuted`. A failed audit write refuses completion before close.
