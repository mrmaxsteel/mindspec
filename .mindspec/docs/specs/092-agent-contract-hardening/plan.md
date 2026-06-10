---
adr_citations:
    - id: ADR-0023
    - id: ADR-0025
    - id: ADR-0030
    - id: ADR-0034
approved_at: "2026-06-10T22:53:39Z"
approved_by: user
bead_ids:
    - mindspec-fwo5.1
    - mindspec-fwo5.2
    - mindspec-fwo5.3
    - mindspec-fwo5.4
    - mindspec-fwo5.5
    - mindspec-fwo5.6
    - mindspec-fwo5.7
    - mindspec-fwo5.8
    - mindspec-fwo5.9
spec_id: 092-agent-contract-hardening
status: Approved
version: "1"
---
# Plan: 092-agent-contract-hardening

This plan decomposes spec 092 (Reqs 1-22, HC-1..HC-6) into the nine
beads pinned by the spec's §Proposed bead decomposition. Bead
descriptions cite requirement and AC numbers from the spec rather than
inlining their text (per Req 13 / mindspec-lawq: bead payloads stay
lean; full text lives in `spec.md`).

## ADR Fitness

- **ADR-0023** (beads as single state authority — Accepted, domains:
  workflow, git, state). Doctrinal anchor for Req 1: phase is DERIVED
  from bead statuses; `mindspec_phase` metadata is a cache of that
  derivation, so reconciling a stale cache forward (Bead 3) is state
  repair, not a state change. Sound; adhere. Covers the `workflow`
  impacted domain.
- **ADR-0025** (beads JSONL as build artifact — Accepted, domains:
  workflow, execution, bootstrap). Req 6/7 extend its artifact
  classification from `next.CheckDirtyTree` to `complete` (Bead 5). No
  divergence — the ADR's classification is reused verbatim via a direct
  call (Design Question 2, resolved below). Covers the `execution`
  impacted domain.
- **ADR-0030** (executor boundary — Accepted, domains: execution,
  validation, lifecycle, lint). All new git/process interactions in
  Beads 3-6 (merge abort, branch preservation, artifact follow-up
  commit) stay behind `internal/executor`; `internal/lint`
  boundary test must stay green per HC-2. Sound; adhere.
- **ADR-0034** (ceremony collapse — Accepted, domain: workflow). Req 1
  is a semantic refinement of its Decision 1 (auto-migration currently
  scoped to epics LACKING `mindspec_phase`; Req 1 reconciles keys
  PRESENT but stale). Per the spec this is an **amendment, not a
  supersession** — one paragraph landed unconditionally in Bead 3
  (Req 20), citing ADR-0023 as anchor. No `--supersedes` flow needed.
- **Proposed new ADR — agent-error contract** (Design Question 6):
  Bead 1 documents the Req 12 `recovery:` convention and the HC-4
  exit-code contract as a new short ADR (domains: workflow, execution)
  via `mindspec adr create`, giving the doc-sync gate a stable doc
  target. The plan-approve panel may downgrade this to a
  glossary/core-doc section; Bead 1's steps accommodate either.

No accepted ADR is unfit for this work; no superseding ADR is proposed.

## Design Question Resolutions

Spec §Design Questions, resolved for planning (panel may override at
the plan gate):

1. **Heal breadth (DQ-1)**: forward-only (implement→review/done);
   backward disagreements error with the Req 2 recovery. (Spec draft
   position; encoded in Bead 3.)
2. **Tree-guard home (DQ-2)**: `complete` calls `next.CheckDirtyTree`
   directly (`internal/complete` already imports `internal/next`); no
   `internal/guard` extraction unless Bead 5's implementer finds
   co-location with the Req 12 helper cleaner.
3. **`approve` alias (DQ-3)**: help section + suggestions only; alias
   stays hidden (Bead 8).
4. **Artifact re-dirty (DQ-4)**: follow-up `chore: sync beads artifact`
   commit, not amend (Bead 5).
5. **pi24 boundary (DQ-5)**: yes — Req 14(a)/(b) + Req 18 + guidance
   is this spec's boundary; topology auto-repair stays on mindspec-pi24.
6. **Convention home (DQ-6)**: new short ADR (see ADR Fitness).
7. **`approval_gate_discovery` nondeterminism (DQ-7)**: accept; Req 22's
   recorded baseline run resolves it empirically — if the baseline does
   not fail, Bead 2 redesigns the scenario before any fix bead closes.

## Testing Strategy

- **Unit tests** are the primary gate: every behavior change in Beads
  3-8 lands with unit tests asserting the exact spec AC for that
  requirement (string assertions on rendered output for guidance/
  instruct changes, state assertions for metadata/cwd/merge behavior).
  `go build ./... && go test -short ./...` green on every commit
  (HC-1); `go test ./internal/lint/...` (executor boundary,
  ADR-0030) green on every commit (HC-2).
- **Convention test** (Req 21, Bead 1): walks the Req 12 helper's
  exported guard-failure constructors plus the guard sites this spec
  touches; fails when any failure message lacks a final
  `recovery: <command>` line. This is the forward-enforcement layer —
  it runs under `-short` and gates all later beads.
- **LLM-harness regression scenarios** (Req 16/22, Beads 2 and 9): the
  five field-note failures are pinned as scenarios in
  `internal/harness` (skip under `-short` by design, so HC-1 holds
  while they are red). Bead 2 authors them, records each FAILING
  against a pinned pre-fix baseline commit (Req 22 evidence in bd
  comments), and the dependency graph (Beads 3-8 depend on Bead 2)
  enforces the close-gate. Bead 9 re-runs all five against the fixed
  tree and records green evidence (HC-6 red→green loop).
- **Shared test infrastructure**: existing harness sandbox
  (`internal/harness/sandbox.go`) extended once in Bead 2 with
  `Scenario.StartDir` plumbing; existing bd test seams
  (`phase.SetRunBDForTest` etc.) reused for Bead 3's reconcile tests.
  No new test frameworks.

## Decomposition Notes

Nine beads exceeds the 3-5 heuristic; the count is spec-mandated: the
Req 22 close-gate is ENCODED as dependency edges (every fix bead
depends on Bead 2's baseline evidence), and Beads 3-6 and 8 are
mutually independent on requirements, so the graph is wide, not a
serial chain. Files are NOT fully disjoint: Beads 4, 5, and 6 all
edit `internal/executor/mindspec_executor.go` (Beads 4 and 5 also
share `internal/complete/complete.go`, and Bead 6's guidance-tail
edit at `cmd/mindspec/next.go:271-273` is adjacent to Bead 4's
`:272` rewrite). The edited regions are disjoint, so merge conflicts
are unlikely — but the orchestrator will run Beads 4, 5, and 6
serially anyway to avoid merge friction, since serializing them is
cheap. Bead 7 deliberately serializes after Beads 3-5 because it
edits the same guard error strings last to avoid churn. Dependency
wiring at plan-approve time is best-effort; the orchestrator verifies
all 21 dependency edges post-approve. Expected validator warnings
(bead count, chain depth via Bead 1→3→7→9, parallelism 2/9) are
accepted as the cost of the spec-pinned red→green evidence ordering.

## Bead 1: Recovery-line + worktree-context helpers + convention test

**Scope**
Reqs 12, 21, and the helper half of Req 8. Foundation bead: no
call-site changes. Files: new helper package (e.g. `internal/guard`),
`internal/workspace/workspace.go` (`ContextLine`), new convention
test, new agent-error-contract ADR (DQ-6).

**Steps**
1. Create the Req 12 recovery-line formatter in `internal/guard` (or
   co-located equivalent): guard failures end with a final
   `recovery: <command>` line, one command per line, machine-greppable;
   no emitted command may carry replace/destructive semantics (HC-5).
2. Add `workspace.ContextLine(dir string) string` over
   `workspace.DetectWorktreeContext` producing the Req 8 format:
   `you are in the <main|spec|bead> worktree (<dir>); this check
   evaluated <checkedPath>`.
3. Write the Req 21 convention test: walk everything exported from the
   helper package plus the guard-failure sites this spec touches; fail
   when any produced message lacks a final `recovery:` line.
4. Document the convention + HC-4 exit-code contract as a new short
   ADR (domains: workflow, execution) via `mindspec adr create`
   (DQ-6 position; downgradeable to a core-doc section by the panel).
5. Unit tests for the formatter and `ContextLine` (all three worktree
   kinds).

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/workspace/... ./internal/guard/...` passes
      (adjust path if the panel relocates the helper)
- [ ] The Req 21 convention test exists, runs under `-short`, and
      fails when fed a constructor emitting no `recovery:` line
      (negative fixture in the test itself)
- [ ] `go test ./internal/lint/...` (boundary) passes

**Acceptance Criteria**
- [ ] Spec AC "Convention test (Req 21)": a test walks the guard error
      constructors and fails when any guard failure lacks a final
      `recovery:` line
- [ ] HC-5 groundwork: the helper's output format is
      `recovery: <command>` as a dedicated final line; no
      `bd update --metadata` can be emitted through it (Req 19 ban)

**Depends on**
None

## Bead 2: Harness capability + five scenarios + analyzer verbs + baseline evidence

**Scope**
Reqs 16, 17, 22 (baseline half). Files: `internal/harness/agent.go`
(StartDir plumbing), `internal/harness/scenario.go`
(`AllScenarios()`), five new scenario files,
`internal/harness/analyzer.go` + `analyzer_test.go`
(`lifecycleTurnSet`). Parallel with Bead 1.

**Steps**
1. Add `Scenario.StartDir` (or `RunOpts.Dir`) and plumb through the
   runner and `Agent.Run` (`agent.go:55` hardcodes
   `cmd.Dir = sandbox.Root`; default remains `sandbox.Root`).
2. Author the five scenarios per the Req 16 design constraints:
   `stale_phase_impl_approve` (spec-worktree-only topology),
   `complete_from_doomed_worktree` (StartDir = bead worktree),
   `precommit_reexport_complete` (.gitignore override + absolute
   `bd export -o "$(git rev-parse --show-toplevel)/..."` hook path),
   `wrong_directory_guard_recovery` (pre-seeded main dirt),
   `approval_gate_discovery` (SessionStart markdown channel pin).
3. Register all five in `AllScenarios()` (`scenario.go:28-52`).
4. Extend `lifecycleTurnSet` (`analyzer.go:378-394`) with the
   `spec`+`create` verb pair via `containsAll`; add the regression
   test in `analyzer_test.go` (Req 17).
5. Run each scenario against the pinned pre-fix baseline commit;
   record the failing assertion output and the discriminating
   assertion per scenario as bd evidence on this bead (Req 22); record
   the pinned baseline commit SHA.
6. If any scenario does not fail at baseline (DQ-7 risk), redesign it
   per Req 22 before closing this bead.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes (scenarios skip
      under `-short`; HC-1 holds while they are red)
- [ ] `go test ./internal/harness/... -run TestAnalyzer -short` (or
      the analyzer test's actual name) passes, covering the
      `spec create` turn-set match
- [ ] `grep -n "StartDir" internal/harness/agent.go
      internal/harness/scenario.go` shows the plumbing with
      `sandbox.Root` default
- [ ] bd comment on this bead contains five failing-run excerpts, each
      naming its discriminating assertion and the baseline commit SHA

**Acceptance Criteria**
- [ ] Spec AC "All five scenarios registered in `AllScenarios()`;
      analyzer `lifecycleTurnSet` matches `mindspec spec create` turns"
- [ ] Spec AC "Baseline evidence (Req 22)": each of the five scenarios
      has a recorded pre-fix failing run with the discriminating
      assertion named

**Depends on**
None

## Bead 3: Phase reconcile + repair subcommand + ADR-0034 amendment

**Scope**
Reqs 1, 2, 19, 20. Files: `internal/phase/derive.go`
(`DerivePhaseDetail`, warning recovery line),
`internal/approve/impl.go` (gates-then-heal ordering per Req 1's
placement contract), new `cmd/mindspec/repair.go`,
`.mindspec/docs/adr/ADR-0034-ceremony-collapse.md` (amendment
paragraph). Heal policy: forward-only (DQ-1).

**Steps**
1. Add `phase.DerivePhaseDetail(epicID)` exposing stored vs derived
   phase; add the `mindspec repair phase <spec-id>` recovery command to
   the `derive.go:126-127` consistency warning (Req 2).
2. Rework the `impl.go:121-123` phase gate per Req 1: gate evaluation
   continues on the derived phase read-only; the reconcile write
   (`bead.MergeMetadata` + `event=lifecycle.phase_reconciled` stderr
   line) lands after the LAST pre-terminal gate (`:193-196`) and before
   the epic close (`:198`) — never after the `:206` done write.
3. Implement Req 2's both-phases gate-failure error through the Bead 1
   helper, ending with the exact spec-mandated recovery line; ban raw
   `bd update --metadata` from all output (Req 19/HC-5).
4. Add `cmd/mindspec/repair.go`: `mindspec repair phase <spec-id>`
   re-derives and writes via `bead.MergeMetadata` (merge semantics,
   Req 19).
5. Land the Req 20 ADR-0034 amendment paragraph (cache-vs-truth,
   ADR-0023 anchor, stale-cache forward reconcile as a second
   migration trigger).
6. Unit tests per the 3smk and repair ACs, including the
   later-gate-failure-leaves-metadata-untouched and end-state-`done`
   assertions.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/approve/... ./internal/phase/...` passes,
      including the new reconcile-ordering and gate-failure tests
- [ ] `mindspec repair phase --help` exits 0 and describes the
      re-derive + merge-write behavior
- [ ] `grep -rn "bd update --metadata" cmd/ internal/` shows no
      occurrence in any emitted message string
- [ ] `grep -n "Req 1\|reconcile" .mindspec/docs/adr/ADR-0034-ceremony-collapse.md`
      shows the amendment paragraph citing ADR-0023

**Acceptance Criteria**
- [ ] Spec AC "3smk unit" (success path, deferred write, end-state
      `done`, gate-failure recovery line, no `bd update --metadata`)
- [ ] Spec AC "repair unit (Req 19)" (unrelated metadata keys preserved,
      asserted by before/after diff)
- [ ] Spec AC "Req 20" (ADR-0034 amendment paragraph landed)

**Depends on**
Bead 1, Bead 2

## Bead 4: Terminal-command cwd safety

**Scope**
Reqs 3, 4, 5. Files: `internal/executor/mindspec_executor.go`
(`withWorkingDir` hardening), `cmd/mindspec/impl.go` (invocation-cwd
capture + post-success chdir + NOTE), `internal/complete/complete.go`
(in-`Run` chdir between `:277` and `:339`; `FormatResult`
implement-branch cd hint), `cmd/mindspec/complete.go` (invocation-cwd
capture), `cmd/mindspec/next.go:272` (guidance rewrite).

**Steps**
1. Harden `withWorkingDir` (Req 3a): on deferred-restore failure,
   defensively re-`Chdir` to `dir` and emit
   `event=executor.cwd_restore_failed dir=<dir>`; never return in an
   undefined cwd.
2. `impl approve` (Req 3b): capture invocation cwd before the auto-chdir
   (`impl.go:54-57`); `os.Chdir(root)` immediately after `FinalizeEpic`
   returns, before `emitInstruct`.
3. `complete` (Req 3c): `os.Chdir(root)` INSIDE `complete.Run`, between
   `exec.CompleteBead` (`:277`) and `advanceState` (`:339`), preventing
   the silent `ModeIdle` degradation and skipped `:347-351` phase sync.
4. cd-back NOTE (Req 4): both terminal commands stat the invocation cwd
   after the terminal mutation; when gone, the LAST stdout line is
   `NOTE: your shell's working directory was removed — run: cd <root>`.
   Add the `Run: cd <spec-worktree>` hint to `FormatResult`'s
   implement-mode branch (`complete.go:368-372`).
5. Rewrite the `next.go:272` completion guidance per Req 5
   (root-runnable `mindspec complete`, no cd-then-complete).
6. Unit tests per the four qxsy ACs.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/complete/... ./internal/executor/...
      ./internal/approve/...` passes, including the cwd-after-run,
      NOTE-last-line, phase-integrity, and restore-failure tests
- [ ] String-assertion tests cover the `next.go:272` guidance and the
      `FormatResult` implement-branch hint
- [ ] `go test ./internal/lint/...` (boundary) passes

**Acceptance Criteria**
- [ ] Spec AC "qxsy unit (impl approve)" (cwd at root, NOTE last line,
      exit 0, `executor.cwd_restore_failed` warning path)
- [ ] Spec AC "qxsy unit (complete-side phase integrity, Req 3c)"
- [ ] Spec AC "qxsy unit (Req 5)" (guidance string assertions)
- [ ] Spec AC "qxsy unit (Req 4 FormatResult)"

**Depends on**
Bead 1, Bead 2

## Bead 5: Artifact-aware complete

**Scope**
Reqs 6, 7. Files: `internal/complete/complete.go:181` (replace
`exec.IsTreeClean` with `next.CheckDirtyTree` per DQ-2),
`internal/executor/mindspec_executor.go` (`commitWithExport`-based
follow-up commit). Artifact dirt never blocks; follow-up
`chore: sync beads artifact` commit per DQ-4.

**Steps**
1. Replace the plain clean-tree check at `complete.go:181` with the
   ADR-0025 classification via a direct `next.CheckDirtyTree` call
   (zero new imports; DQ-2).
2. When post-classification dirt is artifact-only, fold it into a
   follow-up `chore: sync beads artifact` commit via the executor's
   `commitWithExport` so the bead→spec merge sees a clean tree (Req 7).
3. Keep the user-dirt blocking path on the existing auto-commit hint,
   now routed through the Bead 1 recovery helper.
4. Unit tests per the i4ad AC: artifact-only dirt (including
   re-introduced after auto-commit) succeeds; user dirt still blocks.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/complete/...` passes, including the
      re-export-after-auto-commit fixture
- [ ] `go test ./internal/lint/...` (boundary) passes — the follow-up
      commit stays behind the executor (ADR-0030)

**Acceptance Criteria**
- [ ] Spec AC "i4ad unit": `complete.Run` succeeds when the only dirt
      is `.beads/issues.jsonl` (incl. post-auto-commit re-export); user
      dirt still blocks with the auto-commit hint; `--no-verify` /
      `core.hooksPath` never necessary

**Depends on**
Bead 1, Bead 2

## Bead 6: Plan-side + merge-conflict hardening

**Scope**
Reqs 13, 14, 15, 18. Files: `internal/approve/plan.go`
(`buildDesignField` by-ID citations, `createImplementationBeads`
mid-batch containment, aggregated validation errors),
`internal/executor/mindspec_executor.go` (bead→spec conflict semantic
abort `:284-294`; direct-merge abort + branch preservation
`:336-344`), implement-phase guidance
(`internal/instruct/templates/implement.md` + the bead-context
completion tail at `cmd/mindspec/next.go:271-273`).
Requirement-independent of Beads 3-5; the orchestrator runs 4/5/6
serially (see Decomposition Notes) because they share
`internal/executor/mindspec_executor.go`.

**Steps**
1. Req 13a: `buildDesignField` cites ADRs by ID + title
   (`see ADR-NNNN — <title>`) instead of inlining Decision snapshots.
2. Req 13b: any mid-batch `bd create` failure aborts with a structured
   error naming the failing bead heading, offending field + byte size
   (when 1105), already-created bead IDs, and a recovery line.
3. Req 14a (semantic change): a bead→spec merge conflict ABORTS
   `FinalizeEpic` — abort the in-progress merge, no worktree removal,
   no direct merge, no branch deletion, non-zero exit; error names
   conflicted files + resolve-in-spec-worktree recovery.
4. Req 14b + 18: on direct spec→main conflict, abort the merge (main
   clean), SKIP `DeleteBranch`, return root-anchored recovery
   referencing no worktree path.
5. Req 14 guidance: add the anti-merge-main warning (do not merge
   `main` into bead branches mid-implementation) to the
   implement-phase guidance in BOTH channels:
   `internal/instruct/templates/implement.md` (the completion section
   around `:93-96`) and the bead-context completion tail emitted at
   `cmd/mindspec/next.go:271-273`.
6. Req 15: plan-approve validation aggregates ALL frontmatter/structure
   issues into one error (one bullet per issue, one recovery line).
7. Unit tests per the lawq, pi24, Req 18, Req 14 guidance, and e6qq
   ACs.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/approve/... ./internal/executor/...` passes,
      including forced bead→spec and spec→main conflict fixtures
      asserting post-abort state (worktree + branches survive, main
      clean, non-zero exit)
- [ ] String-assertion test covers the anti-merge-main guidance
- [ ] `go test ./internal/lint/...` (boundary) passes

**Acceptance Criteria**
- [ ] Spec AC "lawq unit" (by-ID citations, no inlined Decision text;
      mid-batch failure containment with recovery line)
- [ ] Spec AC "pi24 unit (Req 14)" (both conflict sites, post-abort
      assertions, no-worktree-reference property for direct merge)
- [ ] Spec AC "Req 18 unit" (spec branch survives, main clean,
      non-zero exit)
- [ ] Spec AC "Req 14 guidance" (string assertion)
- [ ] Spec AC "e6qq unit (Req 15)" (N violations → one aggregated error
      + recovery line)

**Depends on**
Bead 1, Bead 2

## Bead 7: Guard call-site context lines

**Scope**
Req 8 call sites. Files: `cmd/mindspec/next.go:100-111` (dirty-tree
failure), `internal/complete/complete.go:181-186` (clean-tree failure),
`internal/approve/impl.go:121-123,:143` (phase/bead gates). Appends
`workspace.ContextLine` output (Bead 1 helper) to each guard failure.
Also owns converting `next.go:100-111`'s multi-line "Recovery steps:
1..3" block to the Req 12 `recovery: <command>` final-line format via
the Bead 1 helper — no other bead touches that call site, so the
conversion lands here. Runs after Beads 3-5 because it edits the same
error strings last.

**Steps**
1. Convert `next`'s dirty-tree failure (`next.go:100-111`) from its
   "Recovery steps: 1..3" block to the Req 12 format through the Bead 1
   helper (machine-greppable final `recovery: <command>` lines, one
   command per line), then append the worktree-context line.
2. Append it to `complete`'s clean-tree failure (post-Bead-5 wording).
3. Append it to `impl approve`'s phase and unmerged-bead gate failures
   (post-Bead-3 wording).
4. Confirm every touched message still ends with its `recovery:` line
   (Req 12 ordering: context line precedes the final recovery line).
5. Unit tests per the tjat AC across all three commands.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] Forced-failure tests in `internal/next`/`internal/complete`/
      `internal/approve` assert `you are in the <kind> worktree` with
      the evaluated path
- [ ] `next`'s dirty-tree failure no longer contains a
      "Recovery steps:" block; its message ends with Req 12-format
      `recovery: <command>` lines (string assertion)
- [ ] The Req 21 convention test still passes (context lines did not
      displace final `recovery:` lines)

**Acceptance Criteria**
- [ ] Spec AC "tjat unit": forced guard failures in
      next/complete/impl-approve contain `you are in the <kind>
      worktree` naming the evaluated path

**Depends on**
Bead 2, Bead 3, Bead 4, Bead 5

## Bead 8: Discoverability + instruct + docs

**Scope**
Reqs 9, 10, 11. Files: `cmd/mindspec/root.go` + `cmd/mindspec/approve.go`
(help groups, suggestions; alias stays hidden per DQ-3),
`internal/instruct/templates/review.md:61` +
`internal/instruct/instruct.go:219,222,232` (canonical noun-verb
rewrite), `internal/instruct/instruct_test.go`, README + plugin skill
docs (factual location-claim corrections only). Touches no other fix
bead's files; parallel with Beads 3-7.

**Steps**
1. Req 10a: root help gains an "Approval gates" section listing
   `spec approve`, `plan approve`, `impl approve` with one-line phase
   descriptions.
2. Req 10b: `SuggestionsMinimumDistance` + `SuggestFor`/aliases so
   `mindspec approve ...` and near-miss spellings surface the canonical
   noun-verb form.
3. Req 11a: rewrite both deprecated-form emission channels —
   `templates/review.md:61` (SessionStart markdown channel) and the
   three `gatesForMode` occurrences (JSON `Gates` field).
4. Req 11b: unit tests — review-phase canonical-command assertion plus
   the negative no-deprecated-order assertion against BOTH the markdown
   `Render` output (spec/plan/review) and the `RenderJSON` `Gates`
   field.
5. Req 9: docs audit — one commit touching only docs, correcting stale
   CLI location claims, listing each corrected claim in its message; no
   guidance restructuring (deferred to skills-thin-down).

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `mindspec --help` output contains all three gate commands in an
      Approval-gates section
- [ ] `go test ./internal/instruct/...` passes, including the new
      review-phase and negative deprecated-form assertions
- [ ] A near-miss invocation (e.g. `mindspec aprove impl`) produces the
      canonical `impl approve` suggestion (test or recorded output)
- [ ] `git log --oneline -1 -- README.md` (docs-audit commit) touches
      only docs and lists corrected claims in its message

**Acceptance Criteria**
- [ ] Spec AC "v7ez unit" (help lists gates; markdown instruct canonical
      + no deprecated order; `RenderJSON` `Gates` clean)
- [ ] Spec AC "v7ez unit (Req 10b)" (near-miss suggestion)
- [ ] Spec AC "Docs audit commit landed (Req 9)"

**Depends on**
Bead 1, Bead 2

## Bead 9: Scenario verification (red→green close-out)

**Scope**
Req 22 second half, HC-6. Re-runs all five Bead 2 scenarios against
the fixed tree; records passing evidence alongside Bead 2's failing
baselines; confirms analyzer `skip_next` reports clean. No production
code changes expected; scenario fixes only if a pin is flaky.

**Steps**
1. Run all five scenarios (full LLM runs, not `-short`) against the
   spec branch with Beads 3-8 merged, per the
   `internal/harness/TESTING.md` convention:
   `env -u CLAUDECODE go test ./internal/harness/ -v -count=1 -timeout 30m
   -run 'TestLLM_(StalePhaseImplApprove|CompleteFromDoomedWorktree|PrecommitReexportComplete|WrongDirectoryGuardRecovery|ApprovalGateDiscovery)'`
   (test names follow the harness `TestLLM_<PascalCaseScenario>`
   pattern; adjust to the exact names Bead 2 registered).
2. Record passing evidence (per-scenario assertion output) as bd
   comments alongside Bead 2's failing baselines, closing the HC-6
   red→green loop.
3. Confirm `skip_next` reports are clean for the new scenarios
   (Req 17 precondition).
4. Run the full validation-proof set: `go build ./... && go test
   -short ./...`, `go test ./internal/lint/...`, `mindspec --help`
   gate listing.
5. If any scenario fails, route the defect back to the owning fix bead
   (reopen or follow-up) rather than weakening the assertion.

**Verification**
- [ ] All five Req 16 scenarios PASS against the fixed tree (run log
      attached as bd evidence)
- [ ] `go build ./... && go test -short ./...` and
      `go test ./internal/lint/...` green on the spec branch
- [ ] bd evidence on this bead pairs each scenario's green run with
      Bead 2's red baseline (commit SHAs for both)

**Acceptance Criteria**
- [ ] Spec ACs "Harness — `stale_phase_impl_approve`",
      "`complete_from_doomed_worktree`", "`precommit_reexport_complete`",
      "`wrong_directory_guard_recovery`", "`approval_gate_discovery`"
      all pass post-fix with their spec-mandated assertions
- [ ] Spec AC "`go build ./... && go test -short ./...` green per
      commit; boundary lint green" holds at spec close-out

**Depends on**
Bead 2, Bead 3, Bead 4, Bead 5, Bead 6, Bead 7, Bead 8

## Provenance

Spec acceptance criterion → owning bead + verification:

| Spec AC | Bead | Verified by |
|---|---|---|
| 3smk unit | Bead 3 | reconcile-ordering + gate-failure tests in `internal/approve` (Bead 3 verification) |
| repair unit (Req 19) | Bead 3 | metadata before/after diff test (Bead 3 verification) |
| qxsy unit (impl approve) | Bead 4 | cwd/NOTE/exit-0 + `withWorkingDir` restore-failure tests |
| qxsy unit (complete-side, Req 3c) | Bead 4 | phase-integrity test with cwd inside bead worktree |
| qxsy unit (Req 5) | Bead 4 | `next.go:272` guidance string assertion |
| qxsy unit (Req 4 FormatResult) | Bead 4 | implement-branch cd-hint string assertion |
| i4ad unit | Bead 5 | artifact-only-dirt success + user-dirt block tests |
| tjat unit | Bead 7 | forced guard-failure context-line tests |
| v7ez unit | Bead 8 | help + instruct markdown/JSON assertions |
| v7ez unit (Req 10b) | Bead 8 | near-miss suggestion test |
| lawq unit | Bead 6 | by-ID citation + mid-batch containment tests |
| pi24 unit (Req 14) | Bead 6 | forced-conflict post-abort state tests |
| Req 18 unit | Bead 6 | direct-merge branch-preservation test |
| Req 14 guidance | Bead 6 | anti-merge-main string assertion |
| e6qq unit (Req 15) | Bead 6 | aggregated-error test |
| Harness scenarios ×5 | Beads 2, 9 | authored + red baseline (Bead 2), green close-out (Bead 9) |
| Baseline evidence (Req 22) | Bead 2 | bd evidence with discriminating assertions + pinned SHA |
| AllScenarios + lifecycleTurnSet | Bead 2 | registration + `analyzer_test.go` regression test |
| Convention test (Req 21) | Bead 1 | constructor-walking test, re-checked in Bead 7 |
| Docs audit (Req 9) | Bead 8 | docs-only commit listing corrected claims |
| Req 20 ADR-0034 amendment | Bead 3 | amendment paragraph grep + commit |
| build/test/boundary green | all beads | per-bead `go build`/`go test -short`/lint verification lines |
