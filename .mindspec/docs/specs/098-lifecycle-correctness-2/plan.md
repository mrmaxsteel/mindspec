---
adr_citations:
    - id: ADR-0035
      sections:
        - recovery-line agent error contract (Bead 2 unverified-close recoverable error keeps worktree / BeadClosed unset; covers workflow + execution)
    - id: ADR-0037
      sections:
        - pre-complete gate semantics — catch unquoted wrapper prefixes; quoted sh -c / eval residual documented (Bead 4)
    - id: ADR-0036
      sections:
        - Ownership Discovery — the ownership + Accepted-ADR + plan-citation coverage triple the impl-approve CheckADRDivergence gate enforces (Bead 1)
    - id: ADR-0012
      sections:
        - Compose with external CLIs — the bd dolt commit durability step and committed-state re-read go through internal/bead wrappers, not ad-hoc shelling (Bead 2, workflow domain)
approved_at: "2026-06-13T22:27:50Z"
approved_by: user
bead_ids:
    - mindspec-dn4h.1
    - mindspec-dn4h.2
    - mindspec-dn4h.3
    - mindspec-dn4h.4
    - mindspec-dn4h.5
spec_id: 098-lifecycle-correctness-2
status: Approved
version: "1"
---
# Plan: 098-lifecycle-correctness-2

> Five independent, file-disjoint beads closing the five genuinely-open lifecycle-correctness
> defects from the spec-091/096 re-audit. Bead 1 (R1/myn3) wires the ownership + Accepted-ADR +
> plan-citation coverage triple into the `ScenarioImplApprove` fixture so a clean impl-approve stops
> exiting 1 on a spurious `adr-divergence-unowned`. Bead 2 (R2/9n2h — the load-bearing one) hardens
> `complete` close-verify with a forced `bd dolt commit` + committed-state re-read behind new
> injectable seams, and is gated on a VERIFY-FIRST empirical probe of whether `bd dolt commit` +
> `bd show` actually crosses the session→committed boundary (honesty-clause fallback if not). Bead 3
> (R3/e6qq) auto-fills the plan `version` to `"1"` when absent. Bead 4 (R4/7eup) extends the
> pre-complete matcher to catch the unquoted `env`/`timeout`/`xargs`/`command` wrappers around
> `mindspec complete`. Bead 5 (R5/pi24) makes the harness merge-to-main assertion direction-aware.
> NONE of the five depends on another (the requirements are file-disjoint per the spec's
> decomposition); Beads 1 and 5 both live under `internal/harness` but touch different files
> (`scenario_spec_lifecycle.go` vs `asserts.go`), so whichever lands SECOND re-runs
> `go test ./internal/harness/...` (the standard re-run-on-merge guardrail — no dependency edge).
> Impacted domains workflow + execution both map to a cited Accepted ADR; NO new ADR is required.

## ADR Fitness

- **ADR-0035** (Agent Error Contract — Recovery Lines and Exit Codes; Status: **Accepted**;
  Domain(s): workflow, execution, core): governs Bead 2's recoverable-error contract — a close
  whose durability cannot be confirmed (commit failure OR a committed-state mismatch) must surface a
  clear, re-runnable `guard.NewFailure` recovery line, KEEP the worktree, leave `BeadClosed` unset,
  and NEVER print `closed` + exit 0 — mirroring the existing case-(b)/(c) PRE-MERGE returns in
  `complete.go`. Domain(s) include **workflow + execution** (both impacted) → no `adr-cite-irrelevant`.
  **No new ADR** (applies the existing error contract).
- **ADR-0037** (Panel Gate as Enforced Contract; Status: **Accepted**; Domain(s): workflow,
  execution): governs Bead 4's pre-complete gate semantics — the matcher must catch the unquoted
  wrapper prefixes (`env`/`timeout`/`xargs`/`command`) so `mindspec complete` is still detected,
  while the QUOTED `sh -c '…'` / `eval '…'` forms remain an explicitly-documented accepted residual
  (a non-executing tokenizer structurally cannot reach them). Domain(s) include **workflow +
  execution** (both impacted) → no `adr-cite-irrelevant`. **No new ADR**.
- **ADR-0036** (Ownership Discovery — Zero Framework Cognition; Status: **Accepted**; Domain(s):
  workflow, validation, doc-sync, ownership): governs the ownership + Accepted-ADR-citation coverage
  triple that the impl-approve `CheckADRDivergence` gate enforces. Bead 1 SATISFIES that gate inside
  the harness sandbox (commits OWNERSHIP.yaml + Accepted ADR-0001 + plan `adr_citations` at the
  sandbox ROOT before the worktree fork) rather than bypassing it with `--override-adr`. Domain(s)
  include **workflow** (an impacted domain) → no `adr-cite-irrelevant`. **No new ADR**.
- **ADR-0012** (Compose with External CLIs, Don't Wrap Them; Status: **Accepted**; Domain(s): bead,
  workflow, core): governs Bead 2's `bd`-composition mitigation — the forced Dolt durability step
  (`bd dolt commit`) and the committed-state re-read go through the `internal/bead` CLI wrappers
  (`bdcli.go`) per this ADR rather than ad-hoc shelling. Cited for the **workflow** domain only
  (ADR-0012's Domains are `bead, workflow, core` — it does NOT list execution; the execution
  impacted domain is covered by ADR-0035 + ADR-0037 above, both Accepted, both listing execution).
  Domain(s) include **workflow** (an impacted domain) → no `adr-cite-irrelevant`. **No new ADR**.

Every impacted domain is covered by at least one cited Accepted ADR: **workflow** →
ADR-0035/0037/0036/0012; **execution** → ADR-0035/0037. All four cited ADRs are Accepted and each
intersects an impacted domain, so no `adr-divergence`/`adr-cite-irrelevant` fires and no widening of
any ADR's Domains is required.

## Testing Strategy

Every behavior-affecting change is proven **RED-on-revert** — the test FAILS if the fix is reverted
to the cited pre-fix code:

- Bead 1 (R1): the RED-on-revert **pin is a HERMETIC, CI-runnable Go test** that proves the coverage
  triple flips the `CheckADRDivergence` gate, **independent of the haiku agent** (the live-LLM
  `ScenarioImplApprove` run is skip-gated behind agent login — `skipUnlessClaudeCode`,
  `internal/harness/scenario_test.go:34` — and does NOT execute in CI, so it CANNOT carry the pin; it
  re-creates the exact "Not logged in" CI gap that hit 097). Preferred pin: an `internal/approve`-level
  test modeled on `TestApproveImpl_WholeBranchOwnershipFromRef`
  (`internal/approve/impl_test.go:976`) that builds a tmp sandbox matching the `ScenarioImplApprove`
  fixture shape and calls `ApproveImpl(...)` directly — WITHOUT the OWNERSHIP+ADR-0001+`adr_citations`
  triple it asserts the `adr-divergence-unowned` block (the RED state, mirroring `impl_test.go:1045`);
  WITH the triple it asserts the gate passes (FinalizeEpic runs). RED if the triple is removed (the
  gate re-fires). The login-gated LLM `ScenarioImplApprove` run stays a bonus/manual end-to-end check,
  NOT the RED pin.
- Bead 2 (R2): the make-or-break RED test drives the NEW seams — `closeBeadFn` returns nil, the
  normal `fetchBeadByIDFn` returns `"closed"`, but `verifyCommittedFn` reports not-closed/error (or,
  under the honesty-clause fallback, `doltCommitFn` returns an error) → asserts a NON-ZERO error,
  the worktree is RETAINED, and `BeadClosed == false`. This is the exact case today's seams cannot
  simulate. RED if the durability/verify step is reverted.
- Bead 3 (R3): a unit test on a plan missing ONLY `version` (all other required frontmatter present)
  asserts it no longer hard-errors on `frontmatter-version` AND the defaulted value is exactly the
  string `"1"`. RED if reverted to the hard `r.AddError("frontmatter-version", …)`.
- Bead 4 (R4): a table-driven test asserts `segmentInvokesComplete` returns TRUE for
  `env VAR=x mindspec complete`, `timeout 30 mindspec complete`, `xargs … mindspec complete`, and
  `command mindspec complete`, and FALSE for the negatives (`timeout 30 go test`,
  `env FOO=bar mindspec next`). RED if reverted to the env/cd-only stripper.
- Bead 5 (R5): a test covering BOTH directions asserts the assertion no longer flags
  `git merge main` (safe — into a feature/spec branch) but STILL flags the genuinely-bad merge
  landing onto `main`. RED if reverted to the direction-blind `containsAll(args,"merge") &&
  containsAll(args,"main")` check.

Each bead gates on `go build ./...` + the relevant `go test ./internal/<pkg>/...` green and
**golangci-lint locally** (CI Lint-job parity — American spelling only, e.g. `behavior` not
`behaviour`; no new gosec findings).

## Bead 1: Wire the ADR/ownership coverage triple into `ScenarioImplApprove` (R1 / myn3)

Apply the proven `writeSandboxDomainCoverage` recipe (`internal/harness/scenario_contract_hardening.go:61`,
already wired into 6 other scenarios) to the legacy `ScenarioImplApprove` builder
(`internal/harness/scenario_spec_lifecycle.go`, ~lines 360-444). Model the wiring on
`ScenarioStalePhaseImplApprove` (`scenario_contract_hardening.go:139`) — the closest impl-approve-shaped
sibling, which calls `writeSandboxDomainCoverage(sandbox, "stale.go")` + `sandbox.Commit(...)` pre-fork
exactly as Bead 1 must. Today the scenario commits a source
`done.go` at the sandbox root (landing as `done.go` on `main` after merge) but ships NO
`.mindspec/docs/domains/*/OWNERSHIP.yaml` coverage and leaves the fixture plan's `adr_citations`
empty, so `CheckADRDivergence` (unconditional in impl-approve, `internal/approve/impl.go`) returns a
hard `adr-divergence-unowned` and a perfectly clean approve exits 1. The fix commits the minimal
ownership + Accepted-ADR-0001 + plan-`adr_citations` triple at the sandbox ROOT BEFORE
`setupWorktrees` forks (so the coverage is on the base ref the divergence gate diffs against), with
the path claim (`done.go`) matching where the impl file lands on `main`. Helper REUSE only — no edit
to `scenario_contract_hardening.go`. Independent of all other beads (`ScenarioSpecToIdle` is
explicitly OUT of scope per spec sub-req 1a). (M.)

**Steps**
1. In `ScenarioImplApprove`'s `Setup` (`internal/harness/scenario_spec_lifecycle.go` ~370-415), BEFORE
   the `wt := setupWorktrees(sandbox, specID, "", "plan")` call forks the worktree, call
   `writeSandboxDomainCoverage(sandbox, "done.go")` and then `sandbox.Commit(...)` so the
   `.mindspec/docs/domains/sandbox/OWNERSHIP.yaml` (claiming `done.go`) and the Accepted
   `.mindspec/docs/adr/ADR-0001-sandbox-domain.md` are present on the base `main` ref the
   impl-approve divergence gate diffs the merged branch against. The `done.go` path claim matches
   where the impl file lands on `main` after the spec-branch merge.
2. Add `adr_citations:\n- ADR-0001` to the inline fixture plan's frontmatter (the
   `sandbox.WriteFile(...plan.md...)` block ~386-395, which currently has only
   `status`/`spec_id`/`bead_ids`), so `IsDomainCovered` sees the covering ADR as plan-CITED —
   without the citation the gate flips from `adr-divergence-unowned` to `adr-divergence-uncovered`,
   not pass.
3. Keep the existing `wt.SpecWtDir+"/done.go"` impl write + git add/commit, the
   `sandbox.Commit("setup: review mode")`, and all preconditions intact; the only behavior change is
   the pre-fork coverage commit + the plan `adr_citations` line.
4. **Add the HERMETIC, CI-runnable RED pin (the AC's RED-on-revert proof, independent of the haiku
   agent):** add an `internal/approve`-level Go test modeled on
   `TestApproveImpl_WholeBranchOwnershipFromRef` (`internal/approve/impl_test.go:976`). Build a tmp
   sandbox matching the `ScenarioImplApprove` fixture shape (a committed `done.go` + an inline plan)
   and call `ApproveImpl(...)` directly through the `MockExecutor`/`implRunBDFn` seams the existing
   test already uses (`FileAtRefOrAbsentFn`/`TreeDirsAtRefFn` supply the ref-read of the
   OWNERSHIP claim; the fixture plan supplies `adr_citations`). Assert BOTH arms:
   - WITHOUT the OWNERSHIP+ADR-0001+`adr_citations` coverage triple → `ApproveImpl` returns an error
     whose message `strings.Contains(... , "adr-divergence-unowned")` and `FinalizeEpic` does NOT run
     (the RED state, mirroring `impl_test.go:1045`/:1048).
   - WITH the triple committed at the diffed ref → `ApproveImpl` returns nil and `FinalizeEpic` runs
     exactly once.
   This is the CI-runnable pin that proves the divergence-gate fix hermetically. The login-gated LLM
   `ScenarioImplApprove` run (`TestLLM_ImplApprove`, `scenario_test.go:185`, skipped without agent
   login) stays a BONUS/manual end-to-end check confirming the wired fixture also exits 0 with no
   `adr-divergence-unowned` block — it is explicitly NOT the RED pin.
   Alternatively (if the `internal/approve` arm is impractical), add a Setup-only `internal/harness`
   Go test that builds the sandbox and invokes impl-approve via `Sandbox.Run` (`sandbox.go:128`)
   asserting exit 0 + no `adr-divergence-unowned` on stderr WITH the triple, independent of the agent.
5. Confirm `go test ./internal/harness/...` and `go test ./internal/approve/...` are green (the
   hermetic pin runs in CI; the LLM scenario is skipped there but, run under agent login, also shows a
   clean impl-approve with no `adr-divergence-unowned`).
6. FILE the deferred follow-up bead for `ScenarioSpecToIdle` per spec sub-req 1a: the
   agent-authored `ScenarioSpecToIdle` cannot be fixed by a fixture-only triple (the agent writes
   its own plan and is not prompted to cite the covering ADR, so a Setup-committed wildcard claim
   only flips `unowned`→`uncovered`). The impl subagent FILES `mindspec-098-spectoidle-coverage` at
   impl / session-close — it solves the distinct agent-prompting problem and is NOT a child of this
   epic. (Handoff note only — this bead does NOT touch `ScenarioSpecToIdle`.)

**Verification**
- [ ] The coverage triple (`sandbox` OWNERSHIP.yaml claiming `done.go` + Accepted ADR-0001 +
      plan `adr_citations: [ADR-0001]`) is committed at the sandbox ROOT BEFORE `setupWorktrees` forks
- [ ] **HERMETIC CI PIN:** `go test ./internal/approve/...` — the new
      `TestApproveImpl_WholeBranchOwnershipFromRef`-modeled test PASSES in CI (no agent login): WITHOUT
      the triple → `adr-divergence-unowned` block + no FinalizeEpic (RED state); WITH the triple →
      `ApproveImpl` returns nil + FinalizeEpic runs once. This is the RED-on-revert pin.
- [ ] `go test ./internal/harness/...` is green (the LLM `ScenarioImplApprove` / `TestLLM_ImplApprove`
      is skip-gated without agent login and is the BONUS check — run under login it exits 0, spec
      branch merged + deleted, worktree removed, `done.go` on main, NO `adr-divergence-unowned` block —
      but it is NOT the CI RED pin)
- [ ] `writeSandboxDomainCoverage` / `scenario_contract_hardening.go` is REUSED, not edited
- [ ] The deferred follow-up bead `mindspec-098-spectoidle-coverage` is filed (handoff); golangci-lint clean

**Acceptance Criteria**
- [ ] A HERMETIC, CI-runnable Go test (the `internal/approve`-level test modeled on
      `TestApproveImpl_WholeBranchOwnershipFromRef`, `internal/approve/impl_test.go:976`) proves the
      ownership + Accepted-ADR-0001 + plan-`adr_citations` coverage triple flips `CheckADRDivergence`
      from an `adr-divergence-unowned` block (RED state) to PASS — independent of the haiku agent and
      the `skipUnlessClaudeCode` gate. RED if the triple is removed (the gate re-fires). This test
      carries the RED-on-revert pin; the login-gated LLM `ScenarioImplApprove` run does NOT.
- [ ] The `ScenarioImplApprove` fixture carries the same coverage triple committed at the sandbox ROOT
      before the worktree fork (path claim `done.go` matching the merged-to-main location); run under
      agent login a clean impl-approve in `ScenarioImplApprove` exits 0 with NO `adr-divergence-unowned`
      block (spec branch merged + deleted, worktree removed, impl content on main) as a bonus end-to-end
      confirmation (NOT the CI RED pin).
- [ ] `ScenarioSpecToIdle` is NOT touched here; the follow-up bead `mindspec-098-spectoidle-coverage`
      is FILED at impl / session-close (not a child of this epic).

**Depends on**
None

## Bead 2: Harden `complete` close-verify with forced `bd dolt commit` + committed-state re-read (R2 / 9n2h)

The load-bearing bead. After `closeBeadFn` returns nil, `complete` re-reads status via
`fetchBeadByIDFn` → `next.FetchBeadByID` → `bd show --json` — the SAME bd/Dolt session path the close
wrote through, with NO forced commit and NO committed-state read, so an in-session-only close reads
back `"closed"` (case (a) at `internal/complete/complete.go:432`) and `complete` proceeds to merge +
worktree removal on an UNVERIFIED close. The fix forces durability with `bd dolt commit` (a new
`internal/bead` wrapper) and performs a committed-state verification re-read; if EITHER the commit
fails OR the committed read shows not-`closed`/errors, it emits a recoverable ADR-0035 error that
KEEPS the worktree, does NOT set `BeadClosed`, and is safe to re-run (both the close and
`bd dolt commit` are idempotent) — mirroring the existing case-(b)/(c) PRE-MERGE returns at
`complete.go:435-444`. New injectable seams (`doltCommitFn`, `verifyCommittedFn`) alongside
`closeBeadFn`/`fetchBeadByIDFn` (`complete.go:28-34`) make the make-or-break case testable. (M.)

**Steps**
1. **VERIFY-FIRST (s1 — gating, do BEFORE committing to the mechanism):** in a SCRATCH beads repo,
   empirically verify whether `bd dolt commit` + a fresh `bd show` actually crosses the
   session→committed boundary — i.e. does the sequence DETECT an in-session-only close that does NOT
   persist to committed Dolt state? Concretely: produce (or simulate) a close that lands in-session
   but not committed, run `bd dolt commit`, then `bd show --json`, and observe whether the committed
   read can distinguish committed-closed from session-only-closed. If `bd show` reads the SAME
   session state even after `bd dolt commit` (no committed-vs-session discriminator exists), DO NOT
   invent a `verifyCommittedFn` that cannot be grounded — proceed to the honesty-clause fallback in
   step 6. Record the probe result in the bead before writing the mechanism.
   NOTE: an early plan-panel probe observed that `bd` may run in an EMBEDDED mode where each write
   AUTO-COMMITS (so a close is already durable and `bd dolt commit` is a clean-working-set no-op). The
   impl's verify-first Step 1 must EMPIRICALLY confirm whether `bd dolt commit` + a committed-state
   read actually DISCRIMINATES an in-session-only close (the primary mechanism — full committed
   verify) or whether, because writes auto-commit and no session-vs-committed gap is observable, the
   honesty-clause fallback (step 6) applies instead. Do not assume; ground the choice in the probe.
2. Add a `DoltCommit()` wrapper to `internal/bead/bdcli.go` running `bd dolt commit` (model it on the
   existing `Close`/`Export` wrappers — `tracedCombined`/`execCommand`, trace event, wrap non-zero
   exit as a Go error per ADR-0012). It is the strongest available bd durability primitive
   ("Create a Dolt commit from any uncommitted changes in the working set …"). `DoltCommit()` MUST be
   idempotent: it must NOT return an error when there are ZERO uncommitted changes (a re-run after a
   successful commit, or a close that auto-committed, is a no-op — `bd dolt commit` on a clean working
   set is treated as success, NOT a failure to surface).
3. Add the new package-level seams alongside `closeBeadFn`/`fetchBeadByIDFn`
   (`internal/complete/complete.go:28-34`): `doltCommitFn = bead.DoltCommit` and `verifyCommittedFn`
   (the committed-state verifier — its concrete read is settled by step 1's probe). Add BOTH seams to
   the `saveAndRestore` save+restore list (`complete_test.go:81-109`) — capture `origDoltCommit`/
   `origVerifyCommitted` and restore them in the LIFO `t.Cleanup` block alongside the existing
   `origClose`/`origFetchBeadByID` seams — so tests reset them the same way.
4. In the post-close path, force `doltCommitFn()` and then run `verifyCommittedFn(beadID)` INSIDE the
   `closeBeadFn`-returned-nil **else-branch** (`complete.go` ~:380-446, the `switch` over the re-read
   result) — NOT the already-closed idempotent branch (~:372-379, where `closeBeadFn` returned an
   error and the bead was found already-closed). The forced commit + committed-verify gates the
   case-(a) "re-read AFFIRMS closed → proceed" arm (`complete.go:432`): even when the session re-read
   says `closed`, force `doltCommitFn()` + `verifyCommittedFn(beadID)` before proceeding to merge. On
   `doltCommitFn` error OR a committed-state read that shows not-`closed`/errors, return
   `guard.NewFailure(msg, "mindspec complete <id>")` — KEEPING the worktree, NOT setting
   `BeadClosed`, with a recoverable ADR-0035 recovery line — mirroring the existing case-(b) return.
   `complete` must NEVER print `closed` + exit 0 on an unverified close.
5. Add the **RED-on-revert** regression test in `internal/complete/`: set `closeBeadFn` → nil, the
   normal `fetchBeadByIDFn` → `"closed"` (the case-(a) session-affirm path), but `verifyCommittedFn` →
   not-closed/error → assert a NON-ZERO error, the worktree is RETAINED, and `BeadClosed == false`
   (using the `saveAndRestore`-managed `doltCommitFn`/`verifyCommittedFn` seams from step 3). Add a
   `internal/bead` test exercising the new `DoltCommit` wrapper (via the `execCommand` seam
   precedent), including the idempotent zero-changes no-op asserted in step 2.
6. **HONESTY-CLAUSE FALLBACK (s6 — taken iff step 1's probe shows no committed-vs-session
   discriminator):** scope `verifyCommittedFn` to commit-FAILURE detection only: (a) force
   `doltCommitFn()` after every close, (b) DETECT + error on `doltCommitFn` non-zero / wrapper error
   (never silently proceed on an unverified close), (c) emit the same recoverable ADR-0035 error
   keeping the worktree / `BeadClosed=false`, and (d) FILE upstream-bd bead
   `mindspec-098-bd-committed-read` for the residual durability-read gap. Under this fallback the RED
   test asserts the `doltCommitFn`-returns-error path instead (→ recoverable error, retained
   worktree, `BeadClosed == false`). Either path is implementable, testable RED-on-revert, and
   shippable. State explicitly in the bead which path step 1 selected.

**Verification**
- [ ] Step 1's empirical probe result is recorded; the mechanism (full committed-verify vs
      honesty-clause fallback) is chosen from that probe, not assumed
- [ ] `go test ./internal/bead/...` exercises the new `bd dolt commit` (`DoltCommit`) wrapper
- [ ] `go test ./internal/complete/...` — the RED test (close→nil, fetch→`"closed"`,
      verifier→not-closed/error OR `doltCommitFn`→error) yields a NON-ZERO error, retained worktree,
      `BeadClosed == false`
- [ ] `complete` never prints `closed` + exit 0 on an unverified close; the recovery line is
      ADR-0035-shaped and re-runnable; golangci-lint clean
- [ ] (fallback only) upstream-bd bead `mindspec-098-bd-committed-read` is filed

**Acceptance Criteria**
- [ ] After `bd close`, `complete` forces `bd dolt commit` (via the new `internal/bead` `DoltCommit`
      wrapper) and performs a committed-state verification; if the commit FAILS or the committed-state
      read shows not-`closed`/errors, it emits a clear recoverable ADR-0035 error that KEEPS the
      worktree and does NOT set `BeadClosed`, and never prints `closed` + exit 0 on an unverified close.
- [ ] RED-on-revert: a regression test using the new `doltCommitFn`/`verifyCommittedFn` seams
      simulates the make-or-break case (`closeBeadFn`→nil, `fetchBeadByIDFn`→`"closed"`,
      committed-verifier→not-closed/error; honesty-clause fallback: `doltCommitFn`→error) and asserts
      a non-zero error, retained worktree, and `BeadClosed == false`.

**Depends on**
None

## Bead 3: Auto-fill plan `version` to `"1"` when absent (R3 / e6qq)

`internal/validate/plan.go:304` (`checkFrontmatterFields`) raises a HARD error
`frontmatter-version: missing required field: version` when `fm.Version == ""`, even though sibling
checks were already relaxed for ZFC alignment (`StepsCount < 3` demoted to a warning; empty
`adr_citations` demoted when `## ADR Fitness` is present). Nothing downstream reads
`PlanFrontmatter.Version` (the only consumers, `internal/approve/plan.go` and
`internal/validate/divergence.go`, read `ADRCitations`/`WorkChunks` only), so any well-defined
default is safe. This requirement LOOSENS the gate: when `fm.Version == ""`, AUTO-FILL it to the
canonical default value `"1"` (the string `"1"`) instead of erroring. `status`/`spec_id` stay
hard-required. Independent of all other beads (single-file). (S.)

**Steps**
1. In `checkFrontmatterFields` (`internal/validate/plan.go` ~301-307), replace the
   `if fm.Version == "" { r.AddError("frontmatter-version", …) }` branch with an auto-fill:
   `if fm.Version == "" { fm.Version = "1" }` (mutating the parsed `*PlanFrontmatter` so the
   defaulted value is exactly the string `"1"`). Leave the `Status`/`SpecID` hard errors UNCHANGED.
2. Confirm the auto-fill propagates to any consumer that reads `fm.Version` AFTER validation (the
   pointer mutation makes the defaulted value visible); since no current consumer reads `Version`,
   the only observable effect is that the plan no longer hard-errors on `frontmatter-version`.
3. Add a unit test in `internal/validate/`: a plan missing ONLY `version` (all other required
   frontmatter present) (a) no longer produces a `frontmatter-version` error and (b) has its
   `version` defaulted to exactly `"1"`; assert a plan missing `status` or `spec_id` STILL hard-errors.
4. UPDATE the existing `TestValidatePlan_MissingRequiredFields` (`internal/validate/plan_test.go:71-88`),
   which currently asserts `frontmatter-version` FIRES on missing version (`foundVersion` /
   `"expected frontmatter-version error"`). After auto-fill that assertion INVERTS: a missing version no
   longer errors (it defaults to `"1"`). Drop/flip the `foundVersion` expectation so it no longer
   requires the `frontmatter-version` error (keep the `frontmatter-spec-id` expectation — `spec_id`
   stays hard-required), leaving no stale failing assertion. Within-bead test edit (single file).

**Verification**
- [ ] `go test ./internal/validate/...` — a plan missing only `version` validates clean; the
      defaulted value is exactly the string `"1"`
- [ ] `status` and `spec_id` remain hard-required (their `AddError` branches unchanged)
- [ ] RED-on-revert: restoring `r.AddError("frontmatter-version", …)` breaks the missing-version test;
      golangci-lint clean

**Acceptance Criteria**
- [ ] A plan missing only `version` (all other required frontmatter present) no longer hard-errors on
      `frontmatter-version` AND the defaulted/written `version` value is exactly the string `"1"`
      (tested). `status`/`spec_id` remain hard-required. RED-on-revert.

**Depends on**
None

## Bead 4: Catch unquoted wrapper-prefixed `mindspec complete` in the pre-complete matcher (R4 / 7eup)

`segmentInvokesComplete` (`internal/hook/precomplete.go:366-394`) strips only leading env-assignments
and `cd`/`pushd` prefixes before checking `isMindspecBinary(fields[0]) && fields[1] == "complete"`, so
unquoted wrapper commands fail OPEN: `env VAR=x mindspec complete`, `timeout 30 mindspec complete`,
`xargs … mindspec complete`, and `command mindspec complete` are not detected. This requirement
TIGHTENS the gate: extend the prefix-stripping loop to skip the four catchable unquoted wrappers
(`env`, `timeout`, `xargs`, `command`) with correct per-wrapper arg/flag skipping, while PRESERVING
the terminal `isMindspecBinary + fields[1]=='complete'` check unchanged so the tightening fires ONLY
when `complete` is genuinely the wrapped command (no false positives). The QUOTED `sh -c '…'` /
`eval '…'` forms stay a documented accepted ADR-0037 residual (in-code comment). Single-file,
independent of all other beads. (S.)

**Steps**
1. In the prefix-stripping `for` loop in `segmentInvokesComplete` (`precomplete.go:368-385`), add
   cases for the four unquoted wrappers AFTER the existing `isEnvAssignment` / `cd`/`pushd` cases,
   each consuming exactly its wrapper token plus its own args so the loop continues at the wrapped
   command:
   - `env`: skip the bare `env` keyword, then skip any leading `VAR=val` assignments (reuse
     `isEnvAssignment`) — `env FOO=bar mindspec complete` → `mindspec complete`. NOTE: the existing
     stripper ALREADY consumes the `VAR=val` form (via `isEnvAssignment`); the genuinely-NEW token
     this case adds is the BARE `env` keyword (the `env` literal itself, not an assignment).
   - `timeout`: skip `timeout`, skip its optional flags (`-s SIG`/`--signal=…`, `-k DURATION`/
     `--kill-after=…`, `--preserve-status`, `--foreground`), then skip EXACTLY ONE non-flag operand —
     the mandatory DURATION — before the loop returns to the terminal check —
     `timeout 30 mindspec complete` → `mindspec complete`; `timeout -s KILL 30 …` handled.
   - `command`: skip `command`, skip its `-p`/`-v`/`-V` flags — `command mindspec complete` →
     `mindspec complete`.
   - `xargs`: skip `xargs` and its flags (e.g. `-n`/`-I`/`-0`/`-P` with their operands) up to the
     command token, consuming EXACTLY ONE non-flag operand (the command being run) before the
     terminal check — `xargs … mindspec complete` → `mindspec complete`.
   PIN the rule that the `timeout`/`xargs` skip consumes EXACTLY ONE non-flag operand (the
   duration/command) before returning to the terminal `isMindspecBinary(fields[0])` check — NOT a
   greedy run of operands — so the matcher neither UNDER-skips (misses the wrapped complete) NOR
   OVER-skips past the wrapped command onto a wrong `fields[0]` (e.g. consuming `mindspec` itself and
   landing on `complete`).
2. PRESERVE the terminal check unchanged: after stripping, still require
   `len(fields) >= 2 && isMindspecBinary(fields[0]) && fields[1] == "complete"`. This keeps the
   negatives FALSE: `timeout 30 go test` → `go test` → not mindspec → false;
   `env FOO=bar mindspec next` → `fields[1] != "complete"` → false.
3. Add an in-code comment documenting that the QUOTED `sh -c '…'` / `eval '…'` wrapper forms are an
   explicitly-accepted ADR-0037 residual (a non-executing tokenizer structurally cannot reach them)
   and are deliberately NOT matched.
4. EXTEND the existing table test `TestMatchMindspecComplete`
   (`internal/hook/precomplete_match_test.go`, the table at ~line 20 already exercising
   `matchMindspecComplete` → `segmentInvokesComplete` with env-prefix/cd-prefix rows) — do NOT create
   a new `precomplete_test.go` file. Add rows: each unquoted wrapper form around `mindspec complete`
   (`env VAR=x …`, bare `env mindspec complete`, `timeout 30 …`, `timeout -s KILL 30 …`, `xargs … …`,
   `command …`) returns TRUE; the negatives (`timeout 30 go test`, `env FOO=bar mindspec next`,
   bare `go test`) return FALSE. Include a row pinning that the `timeout`/`xargs` single-operand skip
   does NOT over-skip (e.g. `timeout 30 mindspec complete X` matches — the matcher lands on
   `mindspec`/`complete`, not on `complete`/`X`).

**Verification**
- [ ] `go test ./internal/hook/...` — `segmentInvokesComplete` returns TRUE for each unquoted wrapper
      form around `mindspec complete` and FALSE for `timeout 30 go test` / `env FOO=bar mindspec next`
- [ ] The terminal `isMindspecBinary + fields[1]=='complete'` check is unchanged (no false positives)
- [ ] The quoted `sh -c`/`eval` residual is documented in an in-code comment (ADR-0037)
- [ ] RED-on-revert: reverting the wrapper cases makes the wrapper-true table rows fail; golangci-lint clean

**Acceptance Criteria**
- [ ] `segmentInvokesComplete` returns true for `env VAR=x mindspec complete`,
      `timeout 30 mindspec complete`, `xargs … mindspec complete`, and `command mindspec complete`;
      `timeout 30 go test` and `env FOO=bar mindspec next` are NOT caught; the quoted `sh -c`/`eval`
      residual is documented. Table-driven tests cover the wrapper prefixes. RED-on-revert.

**Depends on**
None

## Bead 5: Make the harness merge-to-main assertion direction-aware (R5 / pi24)

`preApproveImplMainMergeOrPRViolation` (`internal/harness/asserts.go` ~199-207) flags ANY `git merge`
whose args contain both `merge` and `main` as a pre-approve "merge-to-main occurred" violation,
regardless of DIRECTION — so the legitimate `git merge main` (merging main INTO a feature/spec
branch) matches the same as the genuinely-bad `git merge <spec-branch>` while on `main`, and the
message is misleading for the safe direction. The agent-facing root cause was already fixed in
`d5d8ee54`/`implement.md:98`; this is the residual test-layer defect. Make the assertion
direction-aware — flag ONLY a merge that LANDS content onto `main` (run while the checkout is on
`main` / `git merge <spec-branch>` into main), allow `git merge main` into another branch — and
correct the violation message. Single-file (`asserts.go`); independent of all other beads (Bead 1
also lives under `internal/harness` but in `scenario_spec_lifecycle.go` — different file, no
serialization; whichever lands SECOND re-runs `go test ./internal/harness/...`). (S.)

**Steps**
1. In `preApproveImplMainMergeOrPRViolation` (`internal/harness/asserts.go` ~199-207), replace the
   direction-blind `containsAll(args,"merge") && containsAll(args,"main")` condition with a
   DIRECTION-aware one that flags only a merge LANDING ONTO `main`. The bad direction is identified
   by the merge OPERAND being a spec/feature branch (e.g. `spec/` / a non-`main` ref) — i.e.
   distinguish `git merge main` (operand == `main`, the SAFE update INTO a branch) from
   `git merge spec/<id>` (operand is the spec branch, landing onto main). Keep the existing
   `isCanonicalInternal` allow-list (the approve-impl `--no-ff spec/<id> … into main` pattern)
   untouched.
2. Correct the violation message to describe the ACTUAL bad direction (a merge landing content onto
   `main` before approve-impl), not the misleading "merge-to-main occurred" wording that also fired
   on the safe `git merge main`.
3. UPDATE the message-substring assertion in the existing `TestAssertNoPreApproveImplMainMergeOrPR`
   (`internal/harness/scenario_test.go:596`), which asserts the violation error
   `strings.Contains(... , "merge-to-main occurred before approve impl")`. Since step 2 corrects the
   violation message to describe the actual bad direction, this substring assertion changes — point it
   at the corrected message text so the existing "rejects merge spec into main before approve impl"
   sub-test still passes (its bad-direction event stream — `git merge --no-ff spec/001-test main` —
   must remain flagged). Within-bead test edit.
4. Add a test in `internal/harness/` covering BOTH directions: a `git merge main` (into a
   feature/spec branch) is NOT flagged; a genuinely-bad merge landing onto `main` (the
   non-canonical, non-`main`-operand direction) before approve-impl IS still flagged with the
   corrected message. (Re-run `go test ./internal/harness/...` whole-package since Bead 1 shares the
   package — whichever lands second re-runs it.)

**Verification**
- [ ] `go test ./internal/harness/...` — the assertion no longer flags `git merge main` (safe
      direction into a branch); still flags the bad direction (a merge landing onto `main`)
- [ ] The `isCanonicalInternal` approve-impl allow-list is preserved; the violation message describes
      the actual bad direction
- [ ] RED-on-revert: restoring the direction-blind check makes the `git merge main` (safe) test row
      fail (false positive); golangci-lint clean

**Acceptance Criteria**
- [ ] `preApproveImplMainMergeOrPRViolation` flags only merges landing onto `main` before
      approve-impl (with an accurate message) and no longer flags a legitimate `git merge main` into
      a feature/spec branch. A test covers both directions. RED-on-revert.

**Depends on**
None

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By |
|-----------------------------|------|-------------|
| R1 (myn3): `ScenarioImplApprove` carries the ownership + Accepted-ADR-0001 + plan-`adr_citations` triple committed at the sandbox ROOT before the fork; a HERMETIC CI-runnable `internal/approve`-level test (modeled on `TestApproveImpl_WholeBranchOwnershipFromRef`) is the RED pin (triple → `adr-divergence-unowned` block flips to pass), the login-gated LLM `ScenarioImplApprove` is a bonus check; `ScenarioSpecToIdle` deferred to `mindspec-098-spectoidle-coverage` | Bead 1 | Steps 1–6 + verification |
| R2 (9n2h): after `bd close`, `complete` forces `bd dolt commit` (new `internal/bead` wrapper) + committed-state verify; commit-failure / committed-mismatch → recoverable ADR-0035 error, worktree kept, `BeadClosed` unset, never `closed`+exit 0; RED test via `doltCommitFn`/`verifyCommittedFn` seams (verify-first probe selects mechanism vs honesty-clause fallback) | Bead 2 | Steps 1–6 + verification |
| R3 (e6qq): a plan missing only `version` validates with `version` auto-filled to exactly `"1"`; `status`/`spec_id` remain hard-required | Bead 3 | Steps 1–3 + verification |
| R4 (7eup): `segmentInvokesComplete` catches `env`/`timeout`/`xargs`/`command`-wrapped `mindspec complete`; negatives not caught; quoted `sh -c`/`eval` residual documented (ADR-0037) | Bead 4 | Steps 1–4 + verification |
| R5 (pi24): `preApproveImplMainMergeOrPRViolation` direction-aware — flags only merges landing onto `main`, allows `git merge main` into a branch, corrected message | Bead 5 | Steps 1–3 + verification |
| `go build` + `go test ./internal/{harness,complete,bead,validate,hook}/...` + golangci-lint green | All beads | Each bead's verification |
| `mindspec validate plan 098-lifecycle-correctness-2` passes (adr-coverage: workflow + execution each mapped to a cited Accepted ADR) | All beads | ADR Fitness + frontmatter `adr_citations` |
