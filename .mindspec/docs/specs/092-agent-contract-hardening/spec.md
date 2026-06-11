---
approved_at: "2026-06-10T20:59:57Z"
approved_by: user
drafted_at: "2026-06-10"
drafted_by: spec-drafting research agent
revised_at: "2026-06-10"
revised_by: panel revision subagent (Rounds 1-3, CONFIRM_READY)
roadmap_step: mindspec-jkhd.2
status: Approved
---
# Spec 092-agent-contract-hardening: Field-note fixes + LLM-harness regression scenarios

## Goal

Harden the agent-facing CLI contract around three invariants:

1. **Every guard failure in the commands this spec touches emits its
   recovery command** — a copy-pastable command on a dedicated final
   line, so an LLM agent can recover without reading source or docs.
   Scope: the five field-note paths plus every guard routed through the
   Req 12 helper. The contract is forward-enforcing: new guards MUST
   route through the helper, and a convention test (Req 21) fails when a
   guard error constructor produces a failure without a `recovery:` line.
2. **Every terminal command leaves the caller in a valid state** — the
   mindspec process never exits standing in (or silently pointing the
   caller's shell at) a deleted directory, and never leaves partial
   mutations without naming the cleanup command.
3. **Exit codes never lie in the commands this spec touches** — a
   command with a terminal mutation exits 0 if and only if that mutation
   succeeded; post-terminal cleanup failures are warnings that carry
   their own recovery commands. Scope mirrors HC-4: this invariant binds
   the writes this spec adds or modifies; the two pre-existing
   `impl approve` exceptions are acknowledged in HC-4's footnote and
   explicitly out of scope.

Each of the five 2026-06-10 field-note failures (mindspec-3smk, -qxsy,
-i4ad, -tjat, -v7ez) is pinned by an LLM-harness regression scenario in
`internal/harness` that replays the original failure — and each scenario
is demonstrated to FAIL against the pre-fix baseline before its fix bead
closes (Req 22). Agent UX becomes a tested product surface.

## Background

A 2026-06-10 Claude Code session (field notes recorded in beads
mindspec-3smk..v7ez) surfaced five contract violations:

- **3smk (P1)** — `impl approve` deadlocks when the epic's stored
  `mindspec_phase` metadata is stale (e.g. the last bead was closed via
  `bd close` or a crashed `complete`). `internal/approve/impl.go:121-123`
  fails with `expected review mode, got "implement"`;
  `DerivePhaseWithStatusWithCache` (`internal/phase/derive.go:120-130`)
  trusts the stored phase and only warns on stderr. The only escape is
  undocumented metadata surgery.
- **qxsy (P1)** — `impl approve` auto-chdirs INTO the spec worktree
  (`cmd/mindspec/impl.go:54-57`), then `FinalizeEpic` removes that
  worktree. The mindspec process itself survives — FinalizeEpic's
  cleanup runs under `withWorkingDir(g.Root, ...)`
  (`internal/executor/mindspec_executor.go:317`), and a failed deferred
  cwd restore leaves the process at the still-valid `g.Root` (chdir is
  atomic). The actual victims are: (a) the **invoking shell**, whose cwd
  mindspec can never change and which is left standing in a deleted
  directory — agent harnesses see `getcwd` errors and exit 1 AFTER the
  command fully succeeded, then retry/repair a success; and (b) the
  **`complete` path**, which has no auto-chdir at all
  (`cmd/mindspec/complete.go:61-73` chdirs only when `--spec` is set):
  run from inside the bead worktree, `exec.CompleteBead`
  (`internal/complete/complete.go:277`) deletes the process's own cwd,
  after which the bd calls in `advanceState` (`complete.go:402-430`)
  silently swallow errors, degrade the result to `ModeIdle`, and skip
  the `mindspec_phase` sync at `complete.go:347-351` — recreating the
  exact stale-phase state of 3smk. Compounding: `mindspec next`'s
  completion guidance (`cmd/mindspec/next.go:272`) tells agents to cd
  into the very directory `complete` will delete.
- **i4ad (P2)** — `complete`'s step-3 clean-tree check
  (`internal/complete/complete.go:181` → `exec.IsTreeClean`,
  `internal/executor/mindspec_executor.go:382-391`) is a plain git-status
  check, unlike `mindspec next`'s artifact-aware `next.CheckDirtyTree`
  (`internal/next/guard.go:106-127`, ADR-0025). A pre-commit hook that
  re-exports `.beads/issues.jsonl` re-dirties the tree immediately after
  the auto-commit and blocks completion; field workaround was
  `git commit --no-verify`.
- **tjat (P2)** — remaining guard failures don't name location. Commands
  are largely location-agnostic (Spec 079), but e.g. `next`'s dirty-tree
  guard run from main blocks on main's dirt with no hint that the agent's
  work lives in a worktree, and stale docs still claim location
  requirements the code no longer has.
  `workspace.DetectWorktreeContext` (`internal/workspace/workspace.go:250-275`)
  already classifies main/spec/bead contexts and has informational call
  sites (`cmd/mindspec/next.go:68`, `internal/phase/derive.go:308`,
  `internal/resolve/target.go:100`) — but no guard FAILURE message uses
  it.
- **v7ez (P3)** — the canonical gate syntax is noun-verb
  (`mindspec impl approve`, `cmd/mindspec/impl.go:20-29`); the verb-noun
  form exists only as a `Hidden:` deprecated alias
  (`cmd/mindspec/approve.go:7-12,30-36`). Agents guessing `approve impl`
  see nothing in `--help` and conclude the command doesn't exist. Worse,
  mindspec's own `instruct` output still TEACHES the deprecated
  verb-noun form on TWO channels: (i)
  `internal/instruct/templates/review.md:61` ("do NOT run `mindspec
  approve impl` until the human explicitly approves") reaches the
  markdown render that the SessionStart hook emits
  (`cmd/mindspec/hook.go:127-133` calls `instruct.Run` with format `""`
  = markdown) — this is what a review-mode agent actually sees at
  session start; and (ii) `internal/instruct/instruct.go:219,222,232`
  emit `run mindspec approve spec/plan/impl <id>` in `gatesForMode`,
  which feeds ONLY `RenderJSON`'s `Gates` field (`instruct.go:199`,
  reached via `mindspec instruct --format json`).

The same session class produced backlog beads triaged in §Triage below.

## Impacted Domains

- workflow
- execution
- core

## Affected packages (per domain)

- **`internal/phase/derive.go`** (workflow) — new
  `DerivePhaseDetail(epicID) (stored, derived string, err error)` (or
  equivalent) exposing both the stored and child-derived phase so callers
  can reconcile; the stderr warning at `derive.go:126-127` gains the
  `mindspec repair phase <spec-id>` recovery command (Req 19).
- **`internal/approve/impl.go`** (workflow) — phase gate at lines 121-123
  becomes reconcile-aware, with the heal write deferred until all
  pre-terminal gates pass (Req 1, 2); merge-conflict messaging (Req 14).
- **`cmd/mindspec/impl.go`** (execution) — invocation-cwd capture before
  the auto-chdir at lines 54-57; post-success `os.Chdir(root)` + cd-back
  NOTE line (Req 3, 4).
- **`cmd/mindspec/complete.go`**, **`internal/complete/complete.go`**
  (workflow/execution) — artifact-aware clean check replacing
  `exec.IsTreeClean` at `complete.go:181` (Req 6, 7); `os.Chdir(root)`
  INSIDE `complete.Run` between worktree removal (`complete.go:277`) and
  `advanceState` (`complete.go:339`) (Req 3c); cd-back NOTE (Req 4);
  `FormatResult` implement-mode branch (`complete.go:368-372`) gains the
  cd hint the plan/review branches already have.
- **`internal/executor/mindspec_executor.go`** (execution) —
  `withWorkingDir` (lines 513-528) restore-failure hardening (Req 3a);
  `FinalizeEpic` direct-merge conflict handling: abort + preserve spec
  branch + root-anchored recovery (Req 14, 18).
- **`cmd/mindspec/repair.go`** (execution, NEW) —
  `mindspec repair phase <spec-id>` subcommand re-deriving and writing
  the epic phase via `bead.MergeMetadata` (Req 19).
- **`internal/next/guard.go`** (workflow) — `next.CheckDirtyTree` called
  from `complete` (internal/complete already imports internal/next,
  `complete.go:13`); optional extraction per Design Question 2.
- **`cmd/mindspec/next.go`** (execution) — completion guidance at line
  272 (Req 5); dirty-tree guard error at lines 100-111 gains worktree
  context (Req 8).
- **`internal/workspace/workspace.go`** (workflow) — small
  `ContextLine(dir)` helper over `DetectWorktreeContext` (Req 8).
- **`cmd/mindspec/root.go`**, **`cmd/mindspec/approve.go`** (execution) —
  approval-gate discoverability (Req 10).
- **`internal/instruct/`** (workflow) — canonical gate command surfaced
  per phase; `templates/review.md:61` (the markdown/SessionStart
  channel) and the `gatesForMode` call sites `instruct.go:219,222,232`
  (JSON `Gates` field only) rewritten to the canonical noun-verb form
  (Req 11).
- **`internal/approve/plan.go`** (workflow) — `buildDesignField`
  (lines 450-483) cites ADRs by ID instead of inlining Decision-section
  snapshots; mid-batch `bd create` failure handling in
  `createImplementationBeads` (lines 266-345; Req 13); aggregated
  single-pass plan validation errors (Req 15).
- **`internal/harness/`** (workflow) — five new regression scenarios +
  `AllScenarios()` registration (`internal/harness/scenario.go:28-52`);
  `Scenario.StartDir` capability plumbed through the runner and
  `Agent.Run` (`internal/harness/agent.go:24-28,55`); `lifecycleTurnSet`
  verb extension (`internal/harness/analyzer.go:378-394`; Req 16, 17);
  baseline-failure evidence per scenario (Req 22).
- **`internal/phase/`**, **`internal/workspace/`** (core) — both
  packages are claimed by the core domain's OWNERSHIP manifest, so the
  `DerivePhaseDetail` split and the `workspace.ContextLine` helper make
  core an impacted domain alongside their workflow/execution callers.
- **ADR docs** — one-paragraph ADR-0034 amendment (Req 20).

## ADR Touchpoints

- **ADR-0023** (beads as single state authority) — doctrinal anchor for
  the Req 1 self-heal: phase is DERIVED from bead statuses (§3); bead
  statuses are the single source of truth (§5); `mindspec_phase` metadata
  is a cache of that derivation. Reconciling a stale cache forward is
  state repair, not a state change.
- **ADR-0025** (beads artifact handling) — Req 6/7 extend its
  classification to `complete`. No divergence; cited for coverage.
- **ADR-0030** (executor boundary) — all new git/process interactions stay
  behind the executor; boundary lint must stay green.
- **ADR-0034** (ceremony collapse) — Req 1 is a semantic refinement of
  Decision 1 by ADR-0034's own text (it scopes auto-migration to epics
  that LACK the `mindspec_phase` key; Req 1 reconciles keys that are
  PRESENT but stale). A one-paragraph amendment is therefore an
  **unconditional in-scope deliverable** (Req 20), not panel-contingent.
- **NEW ADR (proposed): agent-error contract** — records the
  recovery-line convention (Req 12) and the exit-code contract (HC-4).
  Domain(s): workflow, execution. Panel to confirm whether this warrants
  an ADR or a section in an existing doc (Design Question 6).

## Requirements

### Hard Constraints

- **HC-1** Existing test suite preserved; `go build ./... && go test
  -short ./...` green on every commit. No test skipped relative to main.
  (LLM-harness scenarios skip under `-short` by design,
  `internal/harness/scenario_test.go`; registering pre-fix-failing
  scenarios in Bead 2 does not violate this constraint.)
- **HC-2** Executor boundary (ADR-0030) respected;
  `internal/lint/boundary_test.go` stays green.
- **HC-3** Solo-developer UX preserved: self-heals are silent-on-success
  (one structured stderr line following the existing
  `event=<ns>.<name> key=value` convention, see
  `internal/phase/migrate.go:54`), no new interactive prompts, no new
  required flags.
- **HC-4** Exit-code contract: every lifecycle command WITH a terminal
  mutation exits 0 iff that mutation succeeded; commands that mutate
  nothing exit 0 iff they completed their read/guard evaluation.
  Post-terminal cleanup failures warn (with recovery commands) but do
  not change the exit code; pre-terminal gate failures exit non-zero
  having mutated nothing. HC-4 binds the writes this spec ADDS or
  MODIFIES (Req 1's reconcile write is sequenced after the last
  pre-terminal gate precisely so it complies — see Req 1). Two
  PRE-EXISTING `impl approve` behaviors already violate this contract
  as written; they are acknowledged here and explicitly OUT OF SCOPE:
  (i) `phase.EnsureMigrated` writes `mindspec_phase` +
  `mindspec_migrated_at` BEFORE any gate when the key is absent
  (`internal/approve/impl.go:101`, `migrate.go:48-51`), so a later gate
  failure exits non-zero having mutated metadata; (ii) the
  Spec-086-pinned `CommitCount` preflight (`impl.go:218-223`) can fail
  non-zero AFTER the epic close (`:200`) and phase=done write (`:206`).
  Neither is touched by any requirement, and the Req 21 convention/
  review lens must not flag them as regressions of this spec.
- **HC-5** Every guard failure message touched by this spec ends with a
  recovery line containing a copy-pastable command (format per Req 12).
  Recovery commands must be SAFE to paste: no emitted command may have
  replace/destructive semantics over state the agent did not name
  (see Req 19 — raw `bd update --metadata` is banned from output).
- **HC-6** Each field-note fix lands with its harness regression scenario
  in the same spec, and each scenario is demonstrated to fail against
  the pre-fix baseline (Req 22) — the spec is not implementable-complete
  without all five pins demonstrated red, then green.

### Numbered requirements

1. **Phase-mismatch self-heal at `impl approve` (mindspec-3smk).**
   At `internal/approve/impl.go:121-123`: when the stored phase fails the
   review/done gate, re-derive the phase from children
   (`deriveFromChildrenOrStatusWithCache`, `internal/phase/derive.go:167`).
   If the child-derived phase satisfies the gate, CONTINUE gate
   evaluation using the derived phase **without writing anything yet**.
   The reconcile write — `mindspec_phase=<derived>` via
   `bead.MergeMetadata` (`internal/bead/bdcli.go:220-249`), plus the
   stderr line `event=lifecycle.phase_reconciled spec=<id> epic=<id>
   stored=<s> derived=<d>` — happens after the LAST pre-terminal gate
   passes and BEFORE MUTATION (1/3). Precise placement against the
   Spec-086 ordering contract (`impl.go:80-92`): the pre-terminal gates
   are the phase gate (`:121-123`), the plan-bead gate (`:143`), the
   doc-sync gate (`:158`), and the ADR-divergence gate (`:193-196`) —
   the LAST of them; the reconcile write lands after `:193-196` passes
   and before the epic close at `:198`. It must NEVER land after the
   `mindspec_phase=done` write at `:206`: that would clobber `done`
   with the derived `review`, leaving every successfully approved spec
   permanently stale — the fix would recreate the bug. The
   `CommitCount` preflight (`:218-223`) is NOT a pre-terminal gate for
   this purpose: its placement after MUTATIONs 1-2 is pinned by Spec
   086 (panel CONSENSUS revision 9), and the reconcile write does not
   wait for it. After a fully successful `ApproveImpl` the stored phase
   is therefore `done` — the `:206` write runs after (and supersedes)
   the reconcile. This ordering keeps HC-4 intact for the write this
   spec introduces: if a later gate fails, the command exits non-zero
   with the reconcile write not yet performed (see HC-4's footnote for
   the two pre-existing exceptions). (The ordering works because the
   derivation is deterministic and read-only; no gate downstream of the
   phase check consumes the stored metadata value mid-run. If the
   terminal mutation subsequently fails, the already-written value
   equals the child-derived truth — an idempotent repair, not a partial
   work mutation.) A new `phase.DerivePhaseDetail` (or `phase.Reconcile`)
   helper exposes stored-vs-derived so the caller does not duplicate
   cache plumbing. Note: this does not fight `phase.EnsureMigrated`
   (`migrate.go:37-57`), which only writes when the key is ABSENT.
2. **Actionable phase-gate failure (mindspec-3smk fallback).** When
   neither stored nor derived phase satisfies the gate, the error names
   both phases and ends with the recovery line:
   `recovery: close remaining beads with 'mindspec complete <bead-id>', or if bead states are already correct run: mindspec repair phase <spec-id>`.
   The consistency warning at `internal/phase/derive.go:126-127` likewise
   gains the `mindspec repair phase <spec-id>` recovery command. Raw
   `bd update --metadata` commands are never emitted (see Req 19).
3. **Process never ends in a deleted directory (mindspec-qxsy).**
   (a) `withWorkingDir` (`internal/executor/mindspec_executor.go:513-528`
   — a package-level `func(dir string, fn func() error)`; it has no
   access to executor state): when the deferred restore `os.Chdir(wd)`
   fails because `wd` was removed, the process remains at `dir` (chdir
   is atomic and `dir` was just valid); make this explicit — re-`Chdir`
   to `dir` defensively and emit one structured stderr warning
   (`event=executor.cwd_restore_failed dir=<dir>`), never returning with
   the process in an undefined cwd.
   (b) `impl approve` (cmd layer): immediately after `FinalizeEpic`
   returns, `os.Chdir(root)` before `emitInstruct`, so all tail output
   runs from a valid cwd and the process exits 0.
   (c) `complete`: the `os.Chdir(root)` happens INSIDE `complete.Run`,
   immediately after `exec.CompleteBead` returns — between
   `internal/complete/complete.go:277` and `advanceState` at `:339` —
   NOT at the cmd layer. The failure being prevented: `advanceState`
   (`complete.go:402-430`) swallows all bd errors and silently degrades
   to `ModeIdle`, so bd subprocesses spawned from a deleted cwd produce
   a false `Mode: idle` AND skip the `mindspec_phase` sync at
   `complete.go:347-351`, recreating the exact stale-phase condition
   Req 1 exists to heal.
4. **cd-back NOTE on removed invocation directory (mindspec-qxsy).**
   Both terminal commands capture the invocation cwd at entry (before any
   auto-chdir, `cmd/mindspec/impl.go:54-57`,
   `cmd/mindspec/complete.go:61-73`). After the terminal mutation, if
   `os.Stat(invocationCwd)` fails, the LAST line of stdout is:
   `NOTE: your shell's working directory was removed — run: cd <root>`.
   (The mindspec process can never change the invoking shell's cwd; the
   NOTE is the only available channel.) `complete.FormatResult`
   (`internal/complete/complete.go:368-372`) emits the existing
   `Run: cd <spec-worktree>` hint in the implement-mode branch too
   (today only plan/review branches have it).
5. **Location-agnostic completion guidance (mindspec-qxsy /
   mindspec-tjat).** `cmd/mindspec/next.go:272` no longer instructs
   "`cd` into the worktree, then run `mindspec complete`". New wording:
   do the work in the worktree; `mindspec complete <id> "..."` may be run
   from the repo root (it resolves the bead worktree itself,
   `internal/complete/complete.go:150-162`) and will remove the bead
   worktree when it succeeds.
6. **Artifact-aware clean-tree check in `complete` (mindspec-i4ad).**
   `internal/complete/complete.go:181` replaces the plain
   `exec.IsTreeClean(checkPath)` with the ADR-0025 classification used by
   `next.CheckDirtyTree` (`internal/next/guard.go:106-127`): artifact dirt
   (`.beads/issues.jsonl`) is normalized via `bd export` and never blocks;
   only user-authored dirt blocks, with the existing auto-commit hint.
7. **Artifact dirt is committed, not ignored, before merge
   (mindspec-i4ad).** When the only remaining dirt after classification is
   artifact dirt (e.g. a pre-commit hook re-exported the JSONL during the
   auto-commit), `complete` folds it into a follow-up commit
   (`chore: sync beads artifact` via the executor's `commitWithExport`)
   so the subsequent bead→spec merge (`complete.go:277`) operates on a
   genuinely clean tree. Field workarounds (`--no-verify`,
   `core.hooksPath`) must never be necessary.
8. **Worktree-context lines on guard failures (mindspec-tjat).** New
   helper (e.g. `workspace.ContextLine(dir string) string`) built on
   `DetectWorktreeContext` (`internal/workspace/workspace.go:250-275`)
   producing: `you are in the <main|spec|bead> worktree (<dir>); this
   check evaluated <checkedPath>`. Appended to: `next`'s dirty-tree
   failure (`cmd/mindspec/next.go:100-111`), `complete`'s clean-tree
   failure (`complete.go:181-186`), and `impl approve`'s phase/bead gates
   (`impl.go:121-123` and `:143`).
9. **Docs/location audit (mindspec-tjat).** README and plugin skill docs
   are audited for stale location-requirement CLAIMS (e.g. "next must run
   from the spec worktree") and aligned with actual Spec-079
   location-agnostic behavior. Scope is strictly factual corrections of
   CLI location claims — NO guidance restructuring; any skill-doc
   restructuring belongs to skills-thin-down (mindspec-jkhd.3), and this
   requirement must not pre-empt it. Deliverable: one commit touching
   only docs, listing each corrected claim in its message.
10. **Approval gates discoverable from `--help` (mindspec-v7ez).**
    (a) Root help gains an "Approval gates" section (cobra Groups or help
    template) listing `spec approve`, `plan approve`, `impl approve` with
    one-line phase descriptions. (b) `rootCmd.SuggestionsMinimumDistance`
    plus `SuggestFor`/aliases so `mindspec approve ...` and near-miss
    spellings surface the canonical noun-verb form. The hidden deprecated
    `approve` command (`cmd/mindspec/approve.go:7-12`) keeps working; the
    panel decides whether to unhide it with a `Deprecated:` marker
    (Design Question 3).
11. **`instruct` teaches ONLY the canonical gate command per phase
    (mindspec-v7ez).** Two halves:
    (a) Rewrite BOTH emission channels of the deprecated verb-noun
    form to the canonical noun-verb form:
    (i) `internal/instruct/templates/review.md:61` ("do NOT run
    `mindspec approve impl` until the human explicitly approves") —
    this is the occurrence a review-mode agent actually SEES: the
    SessionStart hook runs `mindspec hook session-start`, which calls
    `instruct.Run(ctx, root, "", ...)` — format `""` = the MARKDOWN
    render (`cmd/mindspec/hook.go:127-133`) — and the markdown
    templates never include `gatesForMode`.
    (ii) The `gatesForMode` listing
    (`internal/instruct/instruct.go:219,222,232`), which feeds ONLY
    `RenderJSON`'s `Gates` field (`instruct.go:199`) and reaches agents
    via `mindspec instruct --format json`. Rewrite all four
    occurrences.
    (b) The instruct output for spec/plan/review phases always contains
    the exact canonical command (`mindspec spec approve <id>` /
    `mindspec plan approve <id>` / `mindspec impl approve <id>`) —
    asserted by unit test on the rendered output, not by eyeball.
    Canonical-command tests already exist for spec/plan
    (`instruct_test.go:83,107`); the genuinely new assertions are the
    review-phase canonical test and a negative assertion that no
    rendered output contains the deprecated `approve <noun>` order. The
    negative assertion runs against the MARKDOWN `Render` output for
    spec/plan/review (the SessionStart channel — this is what catches
    `review.md:61`) and additionally against the `RenderJSON` `Gates`
    field (catches `instruct.go:219,222,232`).
12. **Recovery-line convention (cross-cutting).** A single helper (new
    or in `internal/guard`) formats guard failures so the final line is
    `recovery: <command>` (machine-greppable, one command per line). All
    guards touched by Reqs 1-8 route through it; NEW guards added after
    this spec MUST route through it (enforced by Req 21). The convention
    is documented (ADR or doc per Design Question 6).
13. **`bead create-from-plan` survives large plans (mindspec-lawq,
    fold-in).** The oversized payload is the `--design` field:
    `buildDesignField` (`internal/approve/plan.go:450-483`) inlines the
    Decision-section snapshot of EVERY cited ADR into every bead's design
    field (`--description` carries the plan workChunk,
    `plan.go:327-337`). Fix, two layers:
    (a) **Structural**: `buildDesignField` stops inlining Decision
    snapshots and cites by ID + title line (`see ADR-0029 — <title>`,
    full text at `.mindspec/docs/adr/`), bounding the payload by
    construction. No size-limit constant is invented — none exists in the
    codebase, and the ~65,535-byte ceiling is a Dolt server behavior
    (`Error 1105`), not a mindspec contract.
    (b) **Batch failure containment**: ANY mid-batch `bd create` failure
    in `createImplementationBeads` (`plan.go:266-345`) — Error 1105,
    daemon crash, lock contention — aborts with a structured error that
    names the failing bead heading, the offending field and its byte
    size (when the cause is 1105), LISTS the bead IDs already created,
    and ends with a recovery line (how to delete the partial set before
    re-running plan approve). Never a raw `Error 1105` with a silent
    partial bead set — exit codes never lie, and partial mutations always
    name their cleanup (Goal invariant 2).
14. **Merge-conflict failures at `impl approve` emit recovery
    (mindspec-pi24, partial fold-in).** Split by site — the two merges
    fail in different worlds:
    (a) **bead→spec auto-merge**
    (`internal/executor/mindspec_executor.go:284-294`): the spec worktree
    still exists. SEMANTIC CHANGE, not just messaging: today a conflict
    here only prints a stderr warning (`:288-292`) and `FinalizeEpic`
    CONTINUES — removing the spec worktree (`:329-334`), direct-merging
    spec→main WITHOUT the conflicted bead's commits (`:336-341`),
    deleting the spec branch (`:344`), and exiting 0. New behavior: a
    bead→spec merge conflict ABORTS `FinalizeEpic` — abort the
    in-progress merge in the spec worktree, perform NO worktree
    removal, NO direct merge to main, NO branch deletion, and return
    non-zero (the bead→spec merge is part of the terminal mutation, so
    HC-4 requires the non-zero exit). The error names the conflicted
    files and ends with recovery commands: resolve in the spec worktree
    (which still exists, because the abort preserved it), commit,
    re-run `mindspec impl approve <id>` — the recovery text matches the
    post-abort reality instead of naming a directory the same function
    deletes moments later.
    (b) **direct spec→main merge** (`:336-341`): this runs at `g.Root`
    on main, AFTER the spec worktree was removed at `:329-334` — recovery
    must NOT reference the spec worktree (it is gone). On conflict the
    error names the conflicted files and emits root-anchored recovery:
    `cd <root>`, `git merge <spec-branch>`, resolve, commit. Branch
    survival on this path is Req 18.
    Additionally, the implement-phase guidance (template + bead context
    tail) warns against merging `main` into bead branches
    mid-implementation. Automatic conflict resolution / rebase topology
    repair is explicitly deferred (see Triage).
15. **Single-pass plan validation errors (mindspec-e6qq, partial
    fold-in).** Plan-approve validation collects ALL frontmatter/structure
    issues and reports them in one aggregated error (one bullet per issue,
    one recovery line), instead of failing on the first issue and forcing
    one-discovery-per-attempt loops. Auto-fixing/relaxing individual
    fields is deferred.
16. **Five harness regression scenarios (HC-6).** New scenarios in
    `internal/harness` (style of `scenario_bead_lifecycle.go`), registered
    in `AllScenarios()` (`internal/harness/scenario.go:28-52`). Includes
    one harness capability extension: a `Scenario.StartDir` (or
    `RunOpts.Dir`) field plumbed through the runner and `Agent.Run` —
    today `cmd.Dir = sandbox.Root` is hardcoded
    (`internal/harness/agent.go:55`) and neither `Scenario`
    (`scenario.go:16-25`) nor `RunOpts` (`agent.go:24-28`) carries a
    working directory; default remains `sandbox.Root`. Scenario design
    constraints (each must fail pre-fix, per Req 22):
    - **`stale_phase_impl_approve`** (3smk): setup creates
      spec-worktree-only topology (`setupWorktrees` with empty beadID) or
      merges/deletes bead branches before the session, so the stale
      `mindspec_phase` is the ONLY blocking condition (otherwise the
      unmerged-bead gate at `impl.go:143` masks the pin — cf.
      `ScenarioUnmergedBeadGuard`).
    - **`complete_from_doomed_worktree`** (qxsy): uses `StartDir` to
      start the agent inside the bead worktree.
    - **`precommit_reexport_complete`** (i4ad): must defeat two sandbox
      properties that would otherwise make it pass today: (i) the default
      sandbox `.gitignore` ignores `.beads/` entirely
      (`internal/harness/sandbox.go:87`) — override it to track
      `.beads/issues.jsonl` (pattern exists in
      `ScenarioBeadsArtifactPassthrough`,
      `scenario_bead_lifecycle.go:570`); (ii) the pinned bd shim cd's to
      sandbox root before exec (`recorder.go:78-79`; the only remaining
      tracking is the code comment at
      `scenario_bead_lifecycle.go:703-706`, which cites mindspec-4u93 —
      a bead CLOSED as obsolete on 2026-06-10 for an unrelated reason)
      — the hook must resolve an absolute output path in
      its OWN worktree, e.g.
      `bd export -o "$(git rev-parse --show-toplevel)/.beads/issues.jsonl"`,
      so the re-export actually dirties the tree `complete` checks.
    - **`wrong_directory_guard_recovery`** (tjat): pre-seeds user dirt on
      main; assertions per AC forbid `git stash` ANYWHERE and verify
      main's dirt survives untouched.
    - **`approval_gate_discovery`** (v7ez): pins the deprecated-form
      teaching on the channel the agent actually reads. The sandbox
      installs the SessionStart hook (`sandbox.go:74-77`), which emits
      the MARKDOWN instruct render (`cmd/mindspec/hook.go:127-133`); in
      review mode that rendered text includes `templates/review.md:61`
      — "do NOT run `mindspec approve impl` until the human explicitly
      approves" — so once the prompt conveys human approval, this reads
      as the command to run, teaching the deprecated hidden form.
      (`instruct.go:232` is the JSON-only sibling: it feeds
      `RenderJSON`'s `Gates` field and never reaches the SessionStart
      output; Req 11 fixes both.) The scenario's assertions are
      anchored on the rendered SessionStart text plus the event stream:
      the gate succeeded via the CANONICAL form and NO event used the
      deprecated `approve impl` order; pre-fix, agents copy the
      deprecated form `review.md:61` feeds them, so the scenario fails;
      post-Req-11 it passes.
    Per-scenario assertions in Acceptance Criteria. Either-order command
    assertions use `assertCommandRanEither` (`asserts.go:57`); note
    `containsAll` is exact-token match (`analyzer.go:623`), not substring.
17. **`lifecycleTurnSet` verb coverage (fold-in from mindspec-u3jg
    item 3; originally noted on closed mindspec-vgp8).** The
    `lifecycleTurnSet` function (`internal/harness/analyzer.go:378-394`;
    line 332 is merely its call site) matches only
    `lifecycleVerbs = {approve, spec-init, complete}` — `mindspec spec
    create` matches none. The acute false positives were already fixed by
    the early bail-out + lastApproveIdx exemption (2ae8e09); this is
    defense-in-depth so spec-phase worktree commits do not rely solely on
    surrounding exemptions: extend the matcher with the verb pair
    `spec`+`create` (`containsAll(args, "spec", "create")`). A
    precondition for the new scenarios' `skip_next` reports being
    trustworthy.
18. **Direct-merge conflict never deletes the spec branch
    (mindspec-qxsy/pi24 adjacency; new).** Today `FinalizeEpic`'s direct
    spec→main merge conflict only WARNS
    (`internal/executor/mindspec_executor.go:339`) and flow proceeds to
    `gitutil.DeleteBranch(specBranch)` at `:344` — destroying the only
    recovery source moments after the merge failed. New behavior: on
    direct-merge failure, abort the in-progress merge (`git merge
    --abort` semantics via the executor, leaving main clean), SKIP branch
    deletion, and return an error carrying the Req 14(b) root-anchored
    recovery. Per HC-4: for no-remote workflows the direct merge is part
    of the terminal mutation, so the command exits non-zero with state
    preserved (spec branch intact, main clean).
19. **`mindspec repair phase <spec-id>` subcommand; raw metadata
    commands banned from output (new).** A small subcommand that
    re-derives the epic phase from children and writes it via
    `bead.MergeMetadata` (`internal/bead/bdcli.go:220-249`) — merge
    semantics. Rationale: raw `bd update <id> --metadata '{...}'`
    REPLACES the entire metadata map (`bdcli.go:246`), silently wiping
    `mindspec_migrated_at`, doc-skew audit keys, and ADR-override keys
    when an agent pastes it. Every recovery line that needs a phase
    metadata fix (Req 2's gate failure, the `derive.go:126-127` warning,
    any future caller) emits `mindspec repair phase <spec-id>` instead;
    NO emitted message anywhere in this spec may contain a raw
    `bd update --metadata` command (HC-5). The healed write path is
    identical to Req 1's (`bead.MergeMetadata`).
20. **ADR-0034 amendment (unconditional deliverable; new).** One
    paragraph amending ADR-0034 Decision 1: child bead statuses remain
    ground truth per ADR-0023 §3/§5; `mindspec_phase` is a trusted CACHE
    of the derivation, and lifecycle gates mechanically reconcile it
    forward when stale (Req 1) — a second migration trigger beyond the
    key-absent case. ADR-0023 is cited as the doctrinal anchor. The
    genuinely open policy half stays in Design Question 1 (forward-only
    vs any-disagreement heal).
21. **Recovery-line convention test (forward enforcement; new).** A
    convention test that walks the guard-failure error constructors
    (everything exported from the Req 12 helper's package, plus the
    guard-failure sites in the commands this spec touches) and fails when
    any produces a message without a final `recovery: <command>` line.
    This makes Goal invariant 1 enforceable going forward instead of
    aspirational: a new guard that bypasses the helper breaks the build's
    test gate, not just a convention.
22. **Baseline-failure demonstration (new; closes the HC-6 loop).** Each
    of the five scenarios is RUN against the pre-fix baseline (the code
    as of Bead 2, before any fix bead lands) and its failure recorded —
    failing assertion output captured in the authoring bead's evidence
    (bd comment/notes) — before the corresponding fix bead may close.
    The spec identifies the discriminating assertion per scenario; in
    particular, for `complete_from_doomed_worktree` the `ExitCode 0`
    check may ALREADY hold pre-fix (the recording shim captures
    mindspec's own exit code, `recorder.go:23-24`; the field exit-1 came
    from the invoking shell's getcwd), so the discriminators are the
    no-retry/no-repair assertions. A scenario that cannot be made to
    fail pre-fix is not a regression pin and must be redesigned or
    replaced before its fix bead closes (HC-6). This close-gate is
    ENCODED in the bead dependency graph — every fix bead (Beads 3-8)
    declares Bead 2 as a dependency — so autopilot, which orders work
    from bd dependencies, cannot merge a fix before the baseline
    evidence exists; baseline runs execute against a pinned pre-fix
    commit recorded in Bead 2's evidence.

## Scope

### In Scope
- Requirements 1-22 above; Hard Constraints HC-1..HC-6.
- Unit tests for every behavior change; harness scenarios per Req 16;
  baseline-failure evidence per Req 22.

### Out of Scope
- Automatic merge-conflict resolution or topology repair in
  `impl approve` (pi24 remainder — deferred follow-up).
- Plan-validation auto-fix/defaults (e6qq remainder).
- Skill-guidance restructuring (belongs to skills-thin-down,
  mindspec-jkhd.3; Req 9 is factual claim corrections only).
- Dolt schema change (MEDIUMTEXT) for bd descriptions (lawq option 3 —
  upstream bd concern; this spec fixes the mindspec-side payload).
- Removing the deprecated `approve` alias.
- Fixing the pinned bd shim's cwd redirect generally
  (`recorder.go:78-79`). Formerly cited as mindspec-4u93, but that bead
  was CLOSED as obsolete on 2026-06-10 for an unrelated reason (bd
  1.0.4 embedded mode spawns no sidecar Dolt server); the cwd-redirect
  mechanism it described persists and is exactly what the i4ad
  scenario's absolute hook path defeats. Remaining tracking is the
  code comment at `scenario_bead_lifecycle.go:703-706` (or a fresh
  bead, at the implementer's discretion).
- Any change under `viz/`, `agentmind/`, `bench/`.

## Non-Goals

- This spec does not attempt automatic merge-conflict resolution,
  rebase topology repair, plan-validation auto-fixing, upstream bd/Dolt
  schema changes, removal of the deprecated `approve` alias, or a
  general fix of the pinned bd shim's cwd redirect — see ### Out of
  Scope for the authoritative list with rationale and tracking.
- It does not restructure skill guidance (deferred to skills-thin-down,
  mindspec-jkhd.3) and does not retrofit the HC-4 exit-code contract
  onto the two pre-existing `impl approve` violations footnoted in HC-4.

## Acceptance Criteria

- [ ] **3smk unit**: with stored `mindspec_phase=implement` and all
  children closed, `approve.ApproveImpl` succeeds, writes
  `mindspec_phase` forward via `bead.MergeMetadata` ONLY after all
  pre-terminal gates pass (a later-gate failure leaves metadata
  untouched — explicit test), and logs `lifecycle.phase_reconciled`;
  AFTER the fully successful run, the stored `mindspec_phase` is
  exactly `done` (explicit end-state assertion — the reconcile write
  precedes, and is superseded by, the `impl.go:206` done write; a
  placement that clobbers `done` with `review` fails this test).
  With children NOT all closed, the error contains both phases and the
  exact `mindspec repair phase <spec-id>` recovery command; no emitted
  message contains `bd update --metadata`.
- [ ] **repair unit (Req 19)**: `mindspec repair phase <spec-id>`
  re-derives and writes the phase while PRESERVING unrelated metadata
  keys (e.g. `mindspec_migrated_at`) — asserted by diffing metadata
  before/after.
- [ ] **qxsy unit (impl approve)**: a test simulating invocation from
  inside the spec worktree verifies: process cwd after `ApproveImpl` is
  `root`; stdout's last line is the `NOTE: ... run: cd <root>` recovery;
  exit status 0. `withWorkingDir` restore-failure path leaves the
  process at `dir` and emits the `executor.cwd_restore_failed` warning.
- [ ] **qxsy unit (complete-side phase integrity, Req 3c)**: running
  `complete.Run` with the process cwd inside the bead worktree leaves
  the epic's `mindspec_phase` metadata equal to the child-derived phase
  and the returned mode NOT falsely `idle` — i.e. the chdir between
  `complete.go:277` and `:339` prevents the silent `ModeIdle`
  degradation and the skipped `:347-351` phase sync.
- [ ] **qxsy unit (Req 5)**: the rendered `mindspec next` completion
  guidance (string assertion on the `next.go:272` output, same style as
  the Req 11 instruct test) does NOT instruct cd-into-worktree-then-
  complete and DOES state `mindspec complete` may run from the repo
  root.
- [ ] **qxsy unit (Req 4 FormatResult)**: `complete.FormatResult`'s
  implement-mode branch (`internal/complete/complete.go:368-372`)
  emits the `Run: cd <spec-worktree>` hint — string assertion on the
  rendered output, same style as the plan/review branches.
- [ ] **i4ad unit**: `complete.Run` succeeds when the only dirt is
  `.beads/issues.jsonl` (including dirt re-introduced after the
  auto-commit, simulating a pre-commit re-export); user dirt still blocks
  with the auto-commit hint plus worktree-context line.
- [ ] **tjat unit**: forced guard failures in next/complete/impl-approve
  contain `you are in the <kind> worktree` naming the evaluated path.
- [ ] **v7ez unit**: `mindspec --help` output contains all three gate
  commands; the MARKDOWN-rendered instruct output (the SessionStart
  channel) for spec/plan/review phases contains the canonical
  noun-verb commands AND contains no deprecated `approve <noun>` order
  (pins `templates/review.md:61`); the `RenderJSON` `Gates` field
  likewise contains no deprecated order (pins
  `instruct.go:219,222,232`).
- [ ] **v7ez unit (Req 10b)**: a near-miss invocation (e.g.
  `mindspec aprove impl` or an unknown `mindspec approve` subcommand
  path) produces output containing the canonical `impl approve`
  suggestion.
- [ ] **lawq unit**: a plan citing 8 large ADRs produces bead design
  fields containing by-ID citations (`see ADR-NNNN — <title>`) and no
  inlined Decision text; a forced mid-batch `bd create` failure aborts
  with the failing bead heading, the already-created bead IDs, and a
  recovery line — never a raw `Error 1105` with a silent partial set.
- [ ] **pi24 unit (Req 14)**: a forced bead→spec merge conflict yields
  an error naming the conflicted file(s) and ending with the
  resolve-in-spec-worktree + re-run recovery lines, AND aborts
  `FinalizeEpic`: afterwards the spec worktree still exists, both the
  bead and spec branches still exist, main is untouched (no direct
  merge happened, no in-progress merge state), and the command exited
  non-zero. A forced direct spec→main conflict yields an error whose
  recovery operates at `<root>` and references no worktree path.
- [ ] **Req 18 unit**: after a forced direct-merge conflict, the spec
  branch still exists, main has no in-progress merge state, and the
  command exited non-zero.
- [ ] **Req 14 guidance**: the implement-phase guidance (template +
  bead context tail) contains the anti-merge-main warning (string
  assertion on the rendered output — do not merge `main` into bead
  branches mid-implementation).
- [ ] **e6qq unit (Req 15)**: a plan with N distinct
  frontmatter/structure violations produces ONE error listing all N
  (one bullet each) plus a recovery line, in a single plan-approve
  invocation.
- [ ] **Harness — `stale_phase_impl_approve`** (3smk): Setup builds an
  epic with `mindspec_phase=implement` metadata and all child beads
  closed (last one via `bd close`, replaying the field note), with
  spec-worktree-only topology so no unmerged-bead gate masks the pin.
  Prompt: take the spec to idle. Assert success via
  `assertCommandRanEither(t, events, "mindspec", ["impl","approve"],
  ["approve","impl"])` with no `bd update --metadata` surgery event and
  no `mindspec repair` needed.
- [ ] **Harness — `complete_from_doomed_worktree`** (qxsy): agent starts
  with cwd inside the bead worktree via `Scenario.StartDir`; completes
  the bead. Assert the `mindspec complete` event has ExitCode 0, the
  bead is closed, and — the discriminating assertions — the agent did
  NOT re-run `complete`/repair after success (no second complete event
  for the same bead, no `git worktree` repair commands).
- [ ] **Harness — `precommit_reexport_complete`** (i4ad): sandbox
  `.gitignore` overridden to track `.beads/issues.jsonl`; pre-commit
  hook runs
  `bd export -o "$(git rev-parse --show-toplevel)/.beads/issues.jsonl"`
  (the absolute path defeats the pinned-shim cwd redirect,
  `recorder.go:78-79`). Assert: `mindspec complete` succeeded; no event
  used `--no-verify` or `core.hooksPath`; AND no agent-issued
  `git commit` (or `git add` + commit pair) touching
  `.beads/issues.jsonl` appears after the first failed `complete`
  event — i.e. the successful `mindspec complete` is not preceded by a
  manual artifact-commit recovery, which would otherwise let the
  pre-fix baseline pass (the second hook re-export of unchanged DB
  state produces no new dirt).
- [ ] **Harness — `wrong_directory_guard_recovery`** (tjat): agent starts
  in main with user dirt pre-seeded on main while work is ready in a spec
  worktree; prompt says to claim and implement. Assert: a later
  `mindspec next` succeeded; NO `git stash` event anywhere in the event
  stream; and post-session, main's pre-seeded dirt file is still present
  with unmodified content (deterministic check via sandbox file read /
  `git status` at root) — so the only passing path is the fixed guard
  steering the agent to the worktree.
- [ ] **Harness — `approval_gate_discovery`** (v7ez): review-mode spec;
  prompt says "approve the implementation so the project returns to
  idle" WITHOUT naming the command. Assert an `impl approve` succeeded
  in the CANONICAL noun-verb order within the turn budget AND no event
  used the deprecated `approve impl` order (pre-fix, the SessionStart
  markdown render teaches the deprecated form via
  `templates/review.md:61` — `instruct.go:232` feeds only the JSON
  `Gates` field and never reaches SessionStart output — so this fails
  today; assertions anchor on the rendered SessionStart text).
- [ ] **Baseline evidence (Req 22)**: each of the five scenarios has a
  recorded pre-fix failing run (failing assertion output in the
  authoring bead's evidence), with the discriminating assertion named,
  before its corresponding fix bead closes.
- [ ] All five scenarios registered in `AllScenarios()`; analyzer
  `lifecycleTurnSet` matches `mindspec spec create` turns (regression
  test in `analyzer_test.go` against `analyzer.go:378-394`).
- [ ] **Convention test (Req 21)**: a test walks the guard error
  constructors and fails when any guard failure lacks a final
  `recovery:` line; every error message added/changed by this spec ends
  with a `recovery:` line.
- [ ] Docs audit commit landed (Req 9).
- [ ] **Req 20**: the ADR-0034 amendment paragraph landed, citing
  ADR-0023 as the doctrinal anchor and naming the stale-cache forward
  reconcile (Req 1) as a second migration trigger beyond the
  key-absent case.
- [ ] `go build ./... && go test -short ./...` green per commit; boundary
  lint green.

## Validation Proofs

- `go build ./... && go test -short ./...`: green on every commit (HC-1;
  harness scenarios skip under `-short` by design).
- `go test ./internal/lint/...`: executor-boundary lint
  (`boundary_test.go`) green (HC-2, ADR-0030).
- `mindspec --help`: output lists `spec approve`, `plan approve`, and
  `impl approve` in an Approval-gates section (Req 10).
- Full LLM-harness runs of the five Req 16 scenarios: each recorded
  FAILING against the pinned pre-fix baseline (Bead 2 evidence, Req 22),
  then PASSING against the fixed tree (Bead 9 close-out, HC-6).

## Open Questions

None blocking approval — the seven panel-facing design questions (each
with a recorded draft position) are tracked in §Design Questions below
for resolution during planning.

## Design Questions (for the panel)

1. **Heal breadth for the 3smk phase mismatch** — the ADR-0034 amendment
   itself is now unconditional (Req 20); the open question is policy
   breadth only: heal strictly forward (implement→review/done, the draft
   position) or on ANY stored/derived disagreement? Draft position:
   forward-only; disagreements that would move the phase backward error
   with the Req 2 recovery instead.
2. **Where does the shared artifact-aware tree guard live?**
   `internal/complete` ALREADY imports `internal/next`
   (`complete.go:13`), so calling `next.CheckDirtyTree` from `complete`
   requires zero new imports. Draft position (revised): call
   `next.CheckDirtyTree` directly; extracting the classification into
   `internal/guard` is OPTIONAL and justified only by co-location with
   the Req 12 recovery helper if the implementer finds it cleaner —
   not required by this spec.
3. **Unhide `approve` with `Deprecated:` vs help-section only?** Unhiding
   doubles every gate's help surface; help-section keeps one canonical
   form. Draft position: help section + suggestions; keep alias hidden.
4. **Artifact re-dirty handling: amend vs follow-up commit?** Amending the
   auto-commit rewrites a commit a hook already saw; a follow-up
   `chore: sync beads artifact` commit is noisier but honest. Draft
   position: follow-up commit.
5. **pi24 boundary** — is conflict-recovery hardening (the Req 14(a)
   semantic abort + Req 14(b) root-anchored recovery + Req 18 branch
   preservation) + guidance enough for this spec, with topology
   auto-repair as a follow-up bead outside the spec? (Draft says yes;
   keep pi24 OPEN, partially addressed.)
6. **Recovery-line convention home** — new short ADR ("agent error
   contract") vs a section in `.mindspec/docs/glossary.md`/core docs? The
   doc-sync gate will want a doc target either way.
7. **`approval_gate_discovery` residual nondeterminism** — the redesigned
   scenario pins the review.md:61 deprecated-form defect (the
   SessionStart markdown channel), but a
   pre-fix agent could in principle guess the canonical form unprompted
   (false baseline pass) — Req 22's recorded baseline run resolves this
   empirically. If the baseline run does NOT fail, the scenario must be
   redesigned per Req 22. Acceptable, or should it also pin a larger
   model / higher MaxTurns?

## Proposed bead decomposition (dependency order)

1. **Bead 1 — Recovery-line + worktree-context helpers + convention
   test** (Req 12, 21, Req 8 helper half): `recovery:` formatter in
   `internal/guard`, `workspace.ContextLine`, convention test walking the
   constructors; unit tests. No call-site changes yet. (Foundation:
   blocks 3-8.)
2. **Bead 2 — Harness capability + five scenarios + analyzer verbs +
   baseline evidence** (Req 16, 17, 22): `Scenario.StartDir` plumbing
   (`agent.go:55`), five scenarios authored per the Req 16 design
   constraints, `AllScenarios()` registration, `lifecycleTurnSet`
   spec+create verbs + analyzer test, and the recorded PRE-FIX failing
   run per scenario (Req 22 evidence). Scenarios will fail full LLM runs
   until Beads 3-8 land — that is the point; `go test -short` is
   unaffected (HC-1). Parallel with Bead 1. The baseline evidence
   records the pinned pre-fix commit each run executed against; Beads
   3-8 all depend on this bead so the Req 22 close-gate is enforced by
   the dependency graph, not by convention.
3. **Bead 3 — Phase reconcile + repair subcommand + ADR-0034 amendment**
   (Req 1, 2, 19, 20): `phase.DerivePhaseDetail`, gates-then-heal
   ordering in `approve/impl.go`, `mindspec repair phase`, recovery
   command in derive.go warning, ADR-0034 amendment paragraph. Depends
   on Beads 1 and 2 (Bead 2 per the Req 22 close-gate).
4. **Bead 4 — Terminal-command cwd safety** (Req 3, 4, 5):
   `withWorkingDir` hardening, cmd-layer chdir+NOTE in `impl approve`,
   in-`Run` chdir between `complete.go:277` and `:339`, `next.go:272`
   guidance rewrite, `FormatResult` implement-branch cd hint. Depends on
   Beads 1 and 2.
5. **Bead 5 — Artifact-aware complete** (Req 6, 7): `complete.go:181`
   rewire onto `next.CheckDirtyTree` (per DQ-2), artifact follow-up
   commit. Depends on Beads 1 and 2.
6. **Bead 6 — Plan-side + merge-conflict hardening** (Req 13, 15, 14,
   18): `buildDesignField` by-ID citations, mid-batch failure
   containment, aggregated plan validation, the Req 14(a) bead→spec
   conflict semantic abort (replacing today's warn-and-continue) plus
   Req 14(b) root-anchored direct-merge recovery, direct-merge branch
   preservation, implement guidance. Depends on Beads 1 and 2; parallel
   with 3-5.
7. **Bead 7 — Guard call-site context lines** (Req 8 call sites):
   context lines on the next/complete/impl-approve guards. Depends on
   Beads 2-5 (Bead 2 per the Req 22 close-gate; 3-5 because it touches
   the same error strings last to avoid churn).
8. **Bead 8 — Discoverability + instruct + docs** (Req 9, 10, 11):
   root-help gates section + suggestions, `review.md:61` +
   `gatesForMode` canonical rewrite + instruct tests, docs audit
   commit. Depends on Beads 1 and 2 (Bead 2 per the Req 22 close-gate)
   — touches no other fix bead's files; autopilot may parallelize with
   3-7 (split from old Bead 5 because Reqs 9-11 touch none of the
   guard error strings).
9. **Bead 9 — Scenario verification (red→green close-out)** (Req 22
   second half, HC-6): re-run all five scenarios against the fixed tree,
   record passing evidence alongside Bead 2's failing baselines, confirm
   `skip_next` reports clean. Depends on Beads 2-8.

## Triage verdicts (candidate beads)

Post-cross-check status (2026-06-10): four candidates were verified
already-fixed during triage and CLOSED with evidence in their bd close
reasons; three remain folded in, plus one nit inherited via mindspec-u3jg.

| Bead | Verdict | Justification |
|---|---|---|
| mindspec-pi24 | **fold-in (partial)** | Bead→spec conflict semantic abort (Req 14(a), replacing today's warn-and-continue) + root-anchored direct spec→main recovery (Req 14(b)) + direct-merge branch preservation (Req 18) + anti-merge-main guidance folded in; topology auto-repair deferred — keep bead open for the remainder. |
| mindspec-e6qq | **fold-in (partial)** | Single-pass aggregated validation errors folded in as Req 15 (pure "guard failure emits everything needed to recover"); auto-fix defaults deferred. |
| mindspec-lawq | **fold-in** | Design-field ADR inlining overflow + mid-batch partial bead sets — squarely "exit codes never lie"; Req 13 (FINDINGS item 5 pulled upstream per jkhd.2). |
| mindspec-u3jg (item 3) | **fold-in** | `lifecycleVerbs` lacks `spec`+`create` — defense-in-depth analyzer fix (Req 17) that the new scenarios depend on for honest `skip_next` reporting. Items 1-2 (dead-code deletion) stay on u3jg. |
| mindspec-2c80 | **CLOSED (already fixed)** | Fixed by d959d6a (2026-03-06): `hook/dispatch.go:69-85` blocks protected-branch commits in any mode incl. idle; idle.md instructs branch-first; TestLLM_BugfixBranch passes in both 2026-03-06 Opus runs. Evidence in bd close reason. |
| mindspec-1y47 | **CLOSED (already fixed)** | Fixed by 2d088e3 (Spec 060, PR #36): assertFocusMode/assertFocusFields removed; `WriteFocus` is a documented no-op (`sandbox.go:341-347`, ADR-0023); no test reads `.mindspec/focus`. Evidence in bd close reason. |
| mindspec-ku9d | **CLOSED (already fixed)** | skip_next analyzer fixes (c8b10a6, 2ae8e09) + spec-init test redesign; SpecInit and ApproveSpecFromWorktree pass in both 2026-03-06 Opus full-suite runs. Residual discoverability concern is covered by v7ez (Reqs 10, 11) here. Evidence in bd close reason. |
| mindspec-vgp8 | **CLOSED (already fixed)** | Fixed by 2ae8e09 early bail-out (`analyzer.go:308-327`) + lastApproveIdx exemption (`:334-358`); scenarios pass since 2026-03-06. Cosmetic residue (lifecycleVerbs) recorded on mindspec-u3jg and folded in as Req 17. Evidence in bd close reason. |

## Appendix: Panel revision logs

### Panel revision log (Round 1)

| Finding | Severity | Resolution in this revision |
|---|---|---|
| R1-1 | major | Req 14 split by merge site: (a) bead→spec (worktree exists, resolve-in-worktree recovery) vs (b) direct spec→main (runs at root AFTER worktree removal at executor:329-334; root-anchored recovery, no worktree references). Warn-then-DeleteBranch got its own Req 18: conflict aborts the merge, skips branch deletion, exits non-zero. New Req 18 unit AC. |
| R1-2 | major | New Req 19: `mindspec repair phase <spec-id>` subcommand writing via `bead.MergeMetadata`; raw `bd update --metadata` (replace semantics, bdcli.go:246) banned from ALL emitted output (HC-5 amended). Req 2, the derive.go:126-127 warning, and the 3smk AC now emit the repair command; new repair unit AC asserts unrelated metadata keys survive. |
| R1-3 | major | Req 3 gained explicit sub-site (c): chdir-to-root INSIDE `complete.Run` between `complete.go:277` and advanceState at `:339`, naming the silent ModeIdle degradation (`:402-430`) and skipped `:347-351` phase sync as the prevented failure. New AC asserts phase metadata correct after a complete run from inside the bead worktree. Cmd-layer placement retained for impl approve only. |
| R1-4 | major | Req 13 re-anchored to the real payload: `buildDesignField` (plan.go:450-483) inlining ADR Decision snapshots into `--design` (description carries the workChunk). Invented size constant dropped: fix is structural (by-ID citations) + bd-error containment (any mid-batch failure incl. 1105 reports field byte size, created bead IDs, recovery line). lawq AC rewritten accordingly. |
| R1-5 | minor | Background qxsy rewritten: withWorkingDir restore failure leaves the process at the valid `dir` (g.Root for FinalizeEpic); real victims are the invoking shell and the no-auto-chdir complete path. Req 3(a) restated as remain-at-`dir` + structured warning (helper has no executor state). |
| R1-6 | minor | Resolved with R3-1: Req 16 explicitly scopes `Scenario.StartDir`/`RunOpts.Dir` plumbed through Agent.Run (agent.go:55 hardcode), default sandbox.Root; Bead 2 deliverable. |
| R1-7 | minor | HC-3 now cites the existing `event=<ns>.<name> key=value` stderr convention (migrate.go:54) instead of the nonexistent internal/log. |
| R1-8 | minor | Req 17 re-anchored to the `lifecycleTurnSet` definition at analyzer.go:378-394 (line 332 noted as call site); fix stated as the spec+create verb pair via containsAll. |
| R1-9 | minor | Req 11 split into (a) rewriting the deprecated verb-noun forms at instruct.go:219,222,232 and (b) canonical-command template assertions, noting spec/plan tests exist (instruct_test.go:83,107) and the review-phase + negative deprecated-form assertions are the new ones. |
| R1-10 | minor | Background tjat reworded to "no guard FAILURE message uses it", listing the existing informational call sites. |
| R2-1 | major | Ordering fix adopted (orchestrator-preferred): Req 1 defers the reconcile write until ALL pre-terminal gates pass; gates evaluate the derived phase read-only. HC-4 holds verbatim, no carve-out needed; rationale (deterministic read-only derivation, no downstream consumer of the stored value mid-run) documented in Req 1, with an explicit AC that a later-gate failure leaves metadata untouched. |
| R2-2 | major | ADR-0034 amendment is now unconditional Req 20 (cache-vs-truth paragraph, ADR-0023 anchor); ADR-0023 added to ADR Touchpoints; DQ-1 narrowed to the genuinely open forward-only-vs-any-disagreement policy question. |
| R2-3 | major | Goal invariant 1 reworded scope-honestly (touched commands + helper-routed guards, forward-enforced); new Req 21 convention test walking guard error constructors; "where practical" removed from the AC. |
| R2-4 | minor | Req 13(b) extended beyond the size case: ANY mid-batch bd create failure lists already-created bead IDs and ends with a recovery line. |
| R2-5 | minor | Req 9 constrained to factual CLI location-claim corrections with an explicit no-restructuring coordination note deferring skill-doc restructuring to jkhd.3; Out of Scope updated. |
| R2-6 | minor | HC-4 reworded: commands WITH a terminal mutation exit 0 iff it succeeded; non-mutating commands exit 0 iff their read/guard evaluation completed. |
| R2-7 | minor | DQ-2 updated: internal/complete already imports internal/next (complete.go:13); draft position flipped to direct `next.CheckDirtyTree` call, internal/guard extraction optional. Affected-packages entry aligned. |
| R3-1 | major | Req 16 / Bead 2 explicitly scope the harness StartDir extension (Scenario.StartDir or RunOpts.Dir, runner + Agent.Run plumbing, sandbox.Root default). |
| R3-2 | major | `precommit_reexport_complete` redesigned to actually fail pre-fix: sandbox .gitignore overridden to track .beads/issues.jsonl (ScenarioBeadsArtifactPassthrough pattern, scenario_bead_lifecycle.go:570) and the hook uses an absolute `$(git rev-parse --show-toplevel)` output path to defeat the pinned-shim cwd redirect (recorder.go:78-79; mindspec-4u93 cited and added to Out of Scope). |
| R3-3 | major | `approval_gate_discovery` redesigned to pin the real defect: SessionStart instruct (sandbox.go:80-84) teaches the deprecated `approve impl` order from instruct.go:232; scenario asserts canonical-order success AND no deprecated-order event — fails pre-fix, passes post-Req-11. Req 11 unit test extended with the negative deprecated-form assertion. DQ-7 updated; residual false-baseline-pass risk handled by Req 22's recorded baseline run. (Historical log entry; line anchors as recorded at the time — superseded by NEW-3 and the round-3 notes: current anchors are `sandbox.go:74-77` and `templates/review.md:61`.) |
| R3-4 | major | `wrong_directory_guard_recovery` AC tightened: `git stash` forbidden anywhere in the event stream, plus a deterministic post-session check that main's pre-seeded dirt file survives with unmodified content. |
| R3-5 | major | Added pi24 unit AC (conflicted files + per-site recovery lines, incl. the no-worktree-reference property for the direct merge) and e6qq unit AC (N violations → one error listing all N + recovery line). |
| R3-6 | major | Added Req 5 unit AC: string assertion on the rendered next.go:272 completion guidance (no cd-then-complete; states root-runnable). |
| R3-7 | major | New Req 22: every scenario demonstrated failing at the pre-fix baseline with evidence recorded before its fix bead closes; discriminating assertion named per scenario (complete_from_doomed_worktree's discriminators are the no-retry/no-repair assertions since the shim records mindspec's own exit code, recorder.go:23-24). Beads reordered: scenarios are authored + baseline-failed in Bead 2 (before fixes), verified green in Bead 9. |
| R3-8 | minor | Req 16 stale_phase scenario constraint: spec-worktree-only topology (setupWorktrees with empty beadID) or pre-merged/deleted bead branches, so the stale phase is the only blocking condition. |
| R3-9 | minor | New Req 10b AC: near-miss invocation surfaces the canonical `impl approve` suggestion. |
| R3-10 | minor | Old Bead 5 split: guard call-site context (new Bead 7, depends on 3-5) vs discoverability/instruct/docs (new Bead 8, depends only on Bead 1, explicitly parallelizable). |
| R3-11 | minor | Req 17 line anchor fixed (analyzer.go:378-394); 3smk harness AC names `assertCommandRanEither` (asserts.go:57) for either-order matching and notes containsAll is exact-token (analyzer.go:623). |
| — (orchestrator) | — | Triage table updated to post-cross-check reality: 2c80/1y47/ku9d/vgp8 CLOSED as already-fixed with evidence in bd close reasons; fold-ins remain pi24, e6qq, lawq; the vgp8 lifecycleVerbs nit is folded in via mindspec-u3jg item 3 (Req 17, reframed as defense-in-depth since 2ae8e09 fixed the acute false positives). |

### Panel revision log (Round 2)

Confirmation pass on the Round-1 revision: 26 of 27 Round-1 findings
verified resolved against code (R3-3 partially — superseded by NEW-3);
8 new findings (3 major, 5 minor), all applied below. The Round-3
targeted confirmation pass verified all 8 RESOLVED with verdict
CONFIRM_READY (no new findings; its three editorial notes are applied
in this transcription: Req 16 line anchors `sandbox.go:87` and
`sandbox.go:74-77`, Bead 6 + Triage pi24 wording aligned with the
Req 14(a) semantic abort, Goal invariant 3 scoped symmetrically with
HC-4's footnote).

| Finding | Severity | Resolution in this revision |
|---|---|---|
| NEW-1 | major | Req 1's reconcile-write placement made precise against the Spec-086 ordering contract: after the LAST pre-terminal gate (ADR-divergence, `impl.go:193-196`) and BEFORE MUTATION (1/3) epic close at `:198` — never after the `mindspec_phase=done` write at `:206`, which would clobber `done` with `review` and recreate the stale-phase bug. Gate enumeration now includes doc-sync (`:158`) and ADR-divergence; the `CommitCount` preflight (`:218-223`) explicitly classified as NOT a pre-terminal gate (Spec-086-pinned post-mutation placement). 3smk AC extended: after a fully successful ApproveImpl the stored phase is exactly `done`. |
| NEW-2 | major | Req 14(a) is now a SEMANTIC change, not messaging: a bead→spec merge conflict ABORTS FinalizeEpic (abort the in-progress merge in the spec worktree; no worktree removal, no direct merge to main, no branch deletion; non-zero exit per HC-4) — replacing today's warn-and-continue (`mindspec_executor.go:288-292`) that merges to main minus the conflicted bead's work and deletes the recovery target. pi24 AC extended with the post-abort state assertions (spec worktree + both branches survive, main untouched, exit non-zero). |
| NEW-3 | major | Req 11(a) re-anchored to BOTH emission channels: `templates/review.md:61` (the markdown render the SessionStart hook actually emits, `hook.go:127-133`) and `instruct.go:219,222,232` (JSON-only `Gates` field via `RenderJSON`, `instruct.go:199`). Background v7ez, Affected packages, the `approval_gate_discovery` scenario rationale, the v7ez unit AC, the harness AC, and DQ-7 all re-anchored: the SessionStart channel teaches the deprecated form via review.md:61; assertions run against the rendered markdown (plus RenderJSON for the Gates field). |
| NEW-4 | minor | Three missing ACs added: Req 20 ADR-0034 amendment paragraph (citing ADR-0023); Req 4 `FormatResult` implement-branch cd hint (string assertion); Req 14 implement-phase anti-merge-main guidance (string assertion). |
| NEW-5 | minor | Req 22's close-gate encoded in the dependency graph: Beads 3-8 now explicitly depend on Bead 2 (baseline evidence) in the decomposition; Bead 2 records the pinned pre-fix commit its baseline runs executed against; Req 22 states the gate is enforced by bd dependencies, not convention. |
| NEW-6 | minor | `precommit_reexport_complete` AC gains the discriminating assertion: no agent-issued git commit touching `.beads/issues.jsonl` after the first failed complete — closing the manual-artifact-commit recovery loophole that could make the pre-fix baseline pass. |
| NEW-7 | minor | Stale mindspec-4u93 citations replaced in Req 16(ii) and Out of Scope: the bead was CLOSED obsolete 2026-06-10 for an unrelated sidecar-Dolt reason; the pinned-shim cwd mechanism it described persists at `recorder.go:78-79` (tracked by the comment at `scenario_bead_lifecycle.go:703-706`) and is what the absolute hook path defeats. The i4ad harness AC cites the shim code directly. |
| NEW-8 | minor | HC-4 footnoted honestly: it binds writes this spec adds/modifies; two pre-existing violations acknowledged as out of scope — `phase.EnsureMigrated`'s pre-gate write (`impl.go:101`) and the post-mutation `CommitCount` preflight (`impl.go:218-223`). Req 1's "HC-4 intact verbatim" claim softened to "intact for the write this spec introduces". |

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-10
- **Notes**: Approved via mindspec approve spec