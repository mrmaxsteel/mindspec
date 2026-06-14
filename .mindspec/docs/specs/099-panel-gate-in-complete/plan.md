---
adr_citations:
    - id: ADR-0037
      sections:
        - panel-gate enforced contract — registration §1, round derivation §2, N−1 threshold §3, staleness §4, dirty-tree §5, fail-open/fail-closed asymmetry §6, escape hatches §7, trust boundary §8 (relocated enforcement point; both beads)
    - id: ADR-0035
      sections:
        - agent error contract — recovery line + pre-terminal non-zero exit having mutated nothing (Bead 2 in-binary block via guard.NewFailure with a GENUINE non-empty recovery command, fence in the body BEFORE the recovery line; Bead 1 preserves the hook's existing fence-bearing decision messages, which carry NO recovery line — the hook emits via its own exit-2 path, unchanged)
    - id: ADR-0036
      sections:
        - Zero Framework Cognition — declared structured argument over free-form shell-string heuristic (Bead 2 gates on complete's own parsed bead-id; Bead 1 single-sources the decision so no second matrix copy can drift)
approved_at: "2026-06-14T19:28:26Z"
approved_by: user
bead_ids:
    - mindspec-pqju.1
    - mindspec-pqju.2
spec_id: 099-panel-gate-in-complete
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/panel/gate.go
        - internal/hook/precomplete.go
      title: Extract the shared ADR-0037 decision + injectable fact-gathering into internal/panel; rewire the hook as a backstop
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/complete/complete.go
        - internal/complete/panel_advisory.go
      title: Add the authoritative in-binary panel gate in mindspec complete before CommitAll
---
# Plan: 099-panel-gate-in-complete

> Two beads, serialized 1→2. Bead 1 is a PURE REFACTOR (no behavior change): it relocates the
> ADR-0037 decision matrix (`panelGateDecision`), the `gateFacts` struct, and the decision's
> `Result`/`Action` outcome type FROM `internal/hook` INTO the `internal/panel` leaf package (spec
> Req 3 mechanism (a)), and makes the I/O fact-gathering (`resolvePanelFacts`-equivalent: bead-ref
> rev-parse, porcelain dirty check, ADR-0025 artifact filter, scan-root union) an INJECTABLE seam so
> `internal/panel` STAYS a leaf — the git/config/workspace wiring is injected by each caller, not
> hard-imported by `panel`. The hook (`runPreComplete`) is rewired to call the extracted decision +
> fact-gatherer; its matcher and block behavior are byte-identical, pinned GREEN by the existing
> `internal/hook` decision + run tests (this is also spec R4: the hook is KEPT as a defense-in-depth
> backstop, behaviorally unchanged). Bead 2 is the behavior-changing work (RED-on-revert): it
> promotes the step-2.25 advisory site in `internal/complete` into an AUTHORITATIVE gate that calls
> the SAME extracted decision + fact-gatherer over the DECLARED bead-id, placed BEFORE `exec.CommitAll`
> so the §4 staleness and §5 dirty clauses measure the pre-commit `bead/<id>` tip, and BLOCKS via
> `guard.NewFailure(<relocated decision.Message>, <genuine recovery command>)`. The relocated
> decision.Message already ends with the raw-`git merge` fence (prose, NO `recovery:` line — the hook
> never routed through `internal/guard`, it emitted via its own exit-2 path); so the in-binary caller
> ADDS the ADR-0035 recovery line via the guard arg, leaving the fence in the message BODY BEFORE that
> final `recovery:` line. The HOOK is unchanged — it still emits the same fence-bearing message via its
> own exit-2 path, so Bead 1 stays a pure no-behavior-change refactor and ONLY Bead 2 introduces the
> `guard.NewFailure` wrapping. Bead 2 depends on Bead 1
> (it needs the extracted decision); both touch `internal/panel`, so they are SERIALIZED to avoid a
> shared-file collision. The deferred hook/matcher-retirement follow-up bead (spec R4 / Req 4) is
> FILED at merge — recorded in Bead 1's steps.

## ADR Fitness

- **ADR-0037** (Panel-gate enforced contract; Status: **Accepted**; Domain(s): **workflow,
  execution**) — intersects the impacted **workflow** domain (no `adr-cite-irrelevant`). This spec
  RELOCATES the enforcement point of this ADR (from the shell-parsing PreToolUse hook to inside
  `mindspec complete`) WITHOUT changing the contract. The in-binary gate honors the SAME §§1-8 the
  hook enforces: registration (§1, `panel.json` identity via `panel.ForBead`), round derivation from
  filenames (§2), the N−1 threshold (§3, single home in `panel.ApproveThreshold`), staleness via
  `reviewed_head_sha` vs the live `bead/<id>` ref (§4, BLOCK not warn), the dirty-tree rule (§5,
  user-authored dirt only, ADR-0025 artifact paths filtered), the fail-open-without-a-panel /
  fail-closed-with-one asymmetry (§6), and the escape hatches + trust boundary (§§7-8). Because Req 3
  EXTRACTS the one decision matrix into the leaf package both call sites depend on, the hook and the
  in-binary gate cannot disagree by construction. **No new ADR** — the contract is unchanged; an
  amendment note in `internal/hook/precomplete.go` records that the in-binary path is now
  authoritative and the hook is a backstop.
- **ADR-0035** (Agent error contract — recovery lines; Status: **Accepted**; Domain(s): **workflow,
  execution, core**) — intersects the impacted **workflow** domain (no `adr-cite-irrelevant`). The
  in-binary block (Bead 2) routes through `internal/guard`
  (`guard.NewFailure(<relocated decision.Message>, <genuine recovery command>)`) so it ends with a
  real `recovery:` LAST line and is a PRE-terminal gate failure that exits non-zero having mutated
  NOTHING (HC-4) — placed BEFORE `exec.CommitAll` / `bd close` / merge in `complete.Run`. The
  relocated decision message itself carries the raw-`git merge bead/<id>` fence (block parity, spec
  R5); that fence lands in the message BODY, with the new `recovery:` line appended LAST by the guard
  arg. NOTE: the relocated decision messages do NOT already end with a recovery line — they end with
  the fence prose (the hook emitted them via its own exit-2 path, never through `internal/guard`). So
  Bead 1 moves the fence-bearing messages VERBATIM (no recovery line), the hook keeps emitting them
  unchanged via exit-2, and the in-binary Bead 2 path is the ONLY caller that wraps them with
  `guard.NewFailure` to add the recovery line. **No new ADR** (applies the existing contract).
- **ADR-0036** (Ownership Discovery — Zero Framework Cognition; Status: **Accepted**; Domain(s):
  **workflow, validation, doc-sync, ownership**) — intersects the impacted **workflow** domain (no
  `adr-cite-irrelevant`). This spec is the direct application of ZFC to the gate's INPUT path: Bead 2
  eliminates the heuristic that GUESSES "is this a complete and which bead?" from a free-form shell
  string, replacing it with the bead-id DECLARED on `complete`'s own `cobra` argument — no framework
  cognition, no inferred signal. Bead 1's single-sourcing of the decision is the structural guard
  against a second drifting copy (the ZFC "declare, don't re-derive" stance applied to the matrix
  itself). **No new ADR** (applies the existing ZFC principle).

No new ADR is required by either bead. All three cited ADRs are Accepted, and the sole impacted
domain (**workflow**, which owns `internal/complete/**`, `internal/hook/**`, `internal/panel/**`,
`internal/guard/**` per the spec's Impacted Domains) is covered by all three. Every source path this
spec edits is owned by workflow on current `main`, so no ownership-claim bead is required and
`adr-divergence-unowned` will not hard-block.

## Testing Strategy

Bead 1 is a **NO-BEHAVIOR-CHANGE refactor**, proven by **GREEN behavior parity**: the existing
`internal/hook` tests that exercise the decision and its I/O wiring must pass UNCHANGED after the
relocation + rewire (they move with, or continue to drive, the extracted code). Bead 2 is
behavior-changing and every new gate clause is proven **RED-on-revert** — the new test FAILS if the
in-binary gate is reverted to the prior advisory-only (non-blocking) site:

- **Bead 1 (GREEN parity — the pins that hold the refactor byte-identical):**
  - The pure-decision table test `TestPanelGateDecision` (`internal/hook/precomplete_decision_test.go:59`)
    — every short-circuit row (hatch → no-panel → malformed → abandoned → round-mismatch → missing-ref
    → transient-gitErr → stale → dirty → incomplete → reject → threshold) passes against the relocated
    `panel`-package decision (the test moves to `internal/panel` OR drives it via the rewired hook),
    with the SAME messages.
  - `TestPanelGateDecision_EveryBlockEndsWithFence` (`:263`) — every Block still ends with the
    raw-`git merge` fence (the fence text moves with the decision).
  - The `runPreComplete` wiring tests (`internal/hook/precomplete_run_test.go`):
    `TestRunPreComplete_NonMatch_ZeroCost`, `_BareComplete_Pass`, `_EscapeHatch`, `_ConfigToggle`,
    `_NoPanel_FailOpen`, `_IncompletePanel_Block`, `_WrappedComplete_Block`,
    `_CwdIndependence_CdPrefix`, `_DirtyTree_ArtifactFilter`, `_DirtyTree_WorktreeAbsent_SkipsCheck`,
    `_TransientGitError_DistinctWarn`, `_GenuineMissingRef_MergeLandedWarn`, `_StaleSHA_RealGit` —
    ALL stay green: the hook still matches/strips/extracts identically and now calls the extracted
    decision + fact-gatherer. These pin that the hook block set + fail-open behavior is unchanged.
  - The matcher tests (`internal/hook/precomplete_match_test.go`: `TestMatchMindspecComplete`,
    `TestCompleteBeadID`, `TestCdPrefixPath`) are untouched — the matcher is not relocated.
  - `go build ./...` + `go test ./internal/hook/... ./internal/panel/...` green; golangci-lint clean
    (American spelling; no new gosec). (NOTE: do NOT run `go test ./internal/harness/...` — slow.)

- **Bead 2 (RED-on-revert — the behavior-changing pins):**
  - **Sub-threshold block:** `mindspec complete <id>` for a bead with a registered panel whose tally
    is BELOW threshold (or stale / missing-or-malformed verdicts / REJECT / hard_block) BLOCKS before
    any `bd close` / merge, exits non-zero, mutates nothing, and produces a `guard.NewFailure` error
    whose message PASSES `guard.HasFinalRecoveryLine` (a real `recovery:` LAST line) AND contains the
    raw-`git merge` fence in the body BEFORE that recovery line (R5 hook-parity). RED if the gate
    reverts to advisory-only; this also pins that the block does NOT panic (the literal
    `guard.NewFailure(decision.Message)` with no command would panic per recovery.go:47-48, so the
    test that the err is non-nil-and-well-formed is the guard against the zero-command construction).
  - **Pre-`CommitAll` ordering:** a stale-SHA / dirty-tree block fires against the `bead/<id>` tip as
    it stands BEFORE step-2.5 `exec.CommitAll`, and is NOT spuriously caused (or cleared) by
    `CommitAll` having advanced the tip past `reviewed_head_sha`. RED if the gate is placed after
    `CommitAll`.
  - **Fail-open ACTUALLY COMPLETES (dogfooding safety):** a `complete <id>` for a bead with NO
    `panel.json` referencing it closes + merges + exits 0 — not merely "not blocked" (a no-op that
    exits 0 without merging would falsely pass a weaker assertion).
  - **Different-bead isolation:** a registered SUB-THRESHOLD `panel.json` for bead X does NOT block
    `complete <Y>` (via `panel.ForBead`, which matches only `panel.BeadID == beadID`).
  - **Freshness pass:** a bead whose panel meets N−1 AND is fresh (`reviewed_head_sha` == live
    `bead/<id>` ref) AND has a clean tree proceeds to close+merge. The test asserts EXPLICITLY that
    `reviewed_head_sha == rev-parse(beadHead)` (the in-binary HEAD source resolved at complete.go:238)
    so a future `beadHead`-resolution change that diverges from the hook's `bead/<id>` source is
    caught RED-on-divergence — proving the in-binary HEAD source is the same `bead/<id>` ref the hook
    rev-parses.
  - **Escape-hatch parity:** `MINDSPEC_SKIP_PANEL=1`, audited abandonment, and
    `enforcement.panel_gate: false` all behave identically to the hook from inside `complete`; the
    block message never names the skip variable (HC-7).
  - **Shared-decision pin:** the hook and the in-binary call site resolve to the IDENTICAL extracted
    `panelGateDecision` over the IDENTICAL `gateFacts` produced by the IDENTICAL fact-gatherer (one
    source) — a test that the two call sites reach the same decision so they cannot diverge.
  - `go build ./...` + `go test ./internal/complete/... ./internal/panel/... ./internal/hook/...`
    green; golangci-lint clean.

## Bead 1: Extract the ADR-0037 decision + injectable fact-gathering into `internal/panel`; rewire the hook as a backstop (R3 + R4)

PURE REFACTOR, no behavior change. Relocate the ADR-0037 DECISION MATRIX
(`panelGateDecision`), the `gateFacts` struct, the decision's outcome enum/struct, and the
fence/skip-hint helpers FROM `internal/hook/precomplete.go` (`:33-241`) INTO the `internal/panel`
LEAF package (spec Req 3 mechanism (a), a new `internal/panel/gate.go`). Relocate the I/O
fact-gathering (`resolvePanelFacts` `:761-820`: bead-ref rev-parse, porcelain dirty check, ADR-0025
artifact filter, scan-root union helpers) as an **injectable seam**: `internal/panel` declares the
fact-gatherer signature with the git/status/ref-not-found I/O passed in as function-typed parameters
(e.g. `revParse func(root, ref string) (string, error)`, `status func(wt string) (string, error)`,
and a `refNotFound func(error) bool` predicate), so `internal/panel` STAYS a leaf — it imports NO
internal package (the spec's leaf invariant). The git wiring (`gitutil.RevParseRef`/`gitutil.Status`/
`gitutil.ErrRefNotFound`, `workspace.BeadBranch`) stays in the CALLERS (`internal/hook` today,
`internal/complete` in Bead 2), injected at the call boundary; tests inject fakes and never shell out
(the spec's residual-risk note). The decision's `hook.Result`/`hook.Action` coupling is resolved by
moving a `panel.Decision`/`panel.GateAction` outcome type into `panel` and having the hook map it back
to `hook.Result` at the wiring layer (the hook's public dispatch contract is unchanged). Then REWIRE
`runPreComplete` (`:683-756`) to call `panel.PanelGateDecision` over facts produced by the extracted
fact-gatherer (the hook supplies the git closures); the matcher
(`matchMindspecComplete`/`strippedCompleteFields`/`segmentInvokesComplete`/`completeBeadID`/
`cdPrefixPath`) is LEFT IN PLACE and behaviorally UNCHANGED (spec R4 — the hook remains a
defense-in-depth backstop). Add the amendment comment recording that the in-binary gate (Bead 2) is
authoritative and the hook is the backstop. (M.)

**Steps**
1. Create `internal/panel/gate.go`: move `gateFacts` (`precomplete.go:51-86`), `panelGateDecision`
   (`:111-241`), the outcome type (a new `panel.Decision{Action GateAction; Message string}` with a
   `GateAction` enum `Allow/Block/Warn` mirroring `hook.Pass/Block/Warn`), and the message helpers
   `rawMergeFence` (`:33`), `skipHumanHint` (`:44`), `presentVerdictFiles` (`:247`), `shaEqual`
   (`:268`), `short` (`:283`), and the `SkipPanelEnv` const (`:26`) into `internal/panel`, exporting
   the ones both callers need (`PanelGateDecision`, `GateFacts`, `Decision`, `GateAction`,
   `SkipPanelEnv`, `RawMergeFence`). Keep messages byte-identical (the `/ms-panel-tally` references,
   the fence, the threshold/stale/incomplete wording all move verbatim). **LEAF-INVARIANT CAVEAT:**
   `panelGateDecision` today calls `workspace.BeadBranch(f.beadID)` INSIDE the missing-ref Warn
   (precomplete.go:167) and the transient-gitErr Warn (:181) message text. Moving that verbatim into
   `internal/panel` would add an `internal/workspace` edge and BREAK the leaf. Inline the literal
   `"bead/"+beadID` (== `workspace.BeadBranchPrefix`) inside the relocated decision (OR add a
   pre-resolved `branchRef` field to `GateFacts` that the caller populates) — byte-identity is pinned
   by the existing decision-table 'missing ref' row asserting `bead/mindspec-bd01`. `internal/panel`
   must still import NO internal package — verify with `go list -deps`.
2. In `internal/panel/gate.go` add an INJECTABLE fact-gatherer: relocate `resolvePanelFacts`'s logic
   (`precomplete.go:761-820`) as `panel.ResolveGateFacts(reg Registration, beadID string, deps
   GateIO)` where `GateIO` carries the injected closures — `RevParse(root, ref) (string, error)`,
   `Status(wt) (string, error)`, `IsRefNotFound(error) bool`, and the worktree/scan-root resolver
   (or pass the already-resolved `scanRoot`/`worktreePath` so `panel` does no path math that needs
   `workspace`). Relocate `userDirtPaths` (`:889`) + `isArtifactPath`/`artifactPaths` (`:912-923`)
   into `panel` (pure string filtering — no imports). The two-subprocess budget (rev-parse +
   porcelain) is preserved.
3. Rewire `internal/hook/precomplete.go`: delete the moved definitions; have `runPreComplete`
   (`:683`) and `resolvePanelFacts`-call-site (`:740`) call `panel.ResolveGateFacts` (supplying
   `gitutil.RevParseRef`/`gitutil.Status`/`gitutil.ErrRefNotFound` via the `GateIO` closures and the
   existing `resolveScanRoots`/`resolveBeadWorktree` for paths) and `panel.PanelGateDecision`, then
   map the returned `panel.Decision` back to `hook.Result` (Allow→Pass). Keep the env read
   (`os.Getenv(panel.SkipPanelEnv)`), the config toggle, and the per-registration loop
   (`:738-754`) identical. Update `internal/complete/panel_advisory.go`'s `hook.SkipPanelEnv`
   reference to `panel.SkipPanelEnv` (it already imports `panel`; this drops nothing).
4. Add/keep the amendment comment in `precomplete.go` (and a short note in `gate.go`) recording that
   `mindspec complete`'s in-binary gate is now the AUTHORITATIVE enforcement point and the hook is a
   defense-in-depth backstop (spec ADR-0037 amendment note + R4).
5. Move the decision + run tests that pin the refactor: relocate `TestPanelGateDecision` +
   `TestPanelGateDecision_EveryBlockEndsWithFence` to `internal/panel` (driving
   `panel.PanelGateDecision`), and ADD a transient-gitErr (5b) row to the relocated
   `TestPanelGateDecision` table so the leaf package self-tests the gitErr branch directly (today the
   pure decision-table has NO 5b row — only the wiring test
   `TestRunPreComplete_TransientGitError_DistinctWarn` pins it). Confirm the `runPreComplete` tests in
   `internal/hook/precomplete_run_test.go` still pass against the rewired hook (they assert the SAME
   blocks/warns/passes — the GREEN-parity pins); **preserve the unexported seam var names** the run
   tests reach into (`preCompleteRevParseFn`/`preCompleteStatusFn`/`worktreeListFn` or equivalents) as
   the `GateIO` closures the rewired hook injects, so `stubScanRoots` and the GREEN-parity suite
   compile UNCHANGED — or update those stubs in this same bead. The matcher tests stay put, untouched.
6. (Handoff) Note that the deferred hook + heuristic-matcher RETIREMENT follow-up bead (spec R4 /
   Req 4) must be FILED at merge — the orchestrator files it, or it is recorded here so it is not
   dropped. This spec removes NOTHING from the matcher.

**Verification**
- [ ] `go build ./... && go test ./internal/hook/... ./internal/panel/...` green (do NOT run
      `./internal/harness/...`)
- [ ] `go list -deps ./internal/panel` shows NO `internal/{hook,complete,gitutil,config,workspace,
      bead,phase}` edge — `internal/panel` is still a leaf (git I/O is injected, not imported)
- [ ] `panelGateDecision` + `gateFacts` + the fence/skip/threshold messages now live in
      `internal/panel`; `internal/hook` and (Bead 2) `internal/complete` both import the ONE decision
- [ ] The relocated `TestPanelGateDecision` table (every short-circuit row) and the
      every-block-ends-with-fence test pass against `panel.PanelGateDecision`
- [ ] ALL `internal/hook/precomplete_run_test.go` wiring tests pass UNCHANGED in intent (same
      blocks/warns/passes) — the hook is behaviorally byte-identical (GREEN parity)
- [ ] The matcher (`strippedCompleteFields`/`segmentInvokesComplete`/`completeBeadID`/`cdPrefixPath`)
      and its tests are untouched; golangci-lint clean (American spelling; no new gosec)

**Acceptance Criteria**
- [ ] The ADR-0037 `panelGateDecision` + the `gateFacts` struct + the fact-gathering
      (`resolvePanelFacts`-equivalent) are RELOCATED from `internal/hook` into the `internal/panel`
      leaf package (Req 3 mechanism (a)); `internal/panel` imports no internal package (git I/O is an
      injected seam so it stays a leaf and tests don't shell out).
- [ ] `internal/hook`'s `runPreComplete` is rewired to invoke the EXTRACTED `panel.PanelGateDecision`
      over facts from the EXTRACTED fact-gatherer; the hook's matcher + block behavior are
      BEHAVIORALLY UNCHANGED (GREEN-parity, pinned by the existing decision + `runPreComplete` tests),
      keeping the hook as a defense-in-depth backstop (spec R4).
- [ ] An amendment note records that the in-binary gate is now authoritative and the hook is a
      backstop; the deferred hook/matcher-retirement follow-up bead is recorded to be FILED at merge.

**Depends on**
None

## Bead 2: Add the authoritative in-binary panel gate in `mindspec complete` before `CommitAll` (R1 + R2 + R5)

Behavior-changing (RED-on-revert). Promote the step-2.25 advisory site in
`internal/complete/complete.go` (`:254-264`, where `panelAdvisory` runs) into an AUTHORITATIVE gate
that calls the SAME extracted `panel.PanelGateDecision` over `panel.GateFacts` produced by the SAME
extracted `panel.ResolveGateFacts` (Bead 1), over the DECLARED `beadID` argument that `complete.Run`
already holds (`:187`). CRITICAL ordering: the gate runs at the existing step-2.25 site, which is
BEFORE step-2.5 `exec.CommitAll` (`:271-275`) and before `bd close` (`:418`) / the bead→spec merge
(`:351` onward) — because `CommitAll` advances the `bead/<id>` tip PAST `reviewed_head_sha` (false-
firing §4 staleness) and clears the user dirt (false-clearing §5). The staleness rev-parse target is
the `bead/<id>` ref (`beadHead`, already resolved at `:238-251`), evaluated BEFORE `CommitAll` —
identical to the hook's `gateFacts.headSHA` source. On an unmet gate, BLOCK via
`guard.NewFailure(<relocated decision.Message>, <genuine recovery command>)`: the relocated message
already carries the raw-`git merge` fence in its body but NO `recovery:` line (the hook emitted it via
its own exit-2 path, never through `internal/guard`; a zero-command `guard.NewFailure` would PANIC),
so the caller passes a genuine non-empty recovery command (re-panel + re-complete) and `FormatFailure`
appends the `recovery:` line LAST — fence in the body, recovery line last — exiting non-zero having
mutated nothing. Honor the §6 fail-open-without-a-panel rule (no `panel.json` → pass
silently → bead ACTUALLY completes) and the §7 hatches: `MINDSPEC_SKIP_PANEL` (via `os.Getenv`, never
named in a block — HC-7), audited abandonment, and `enforcement.panel_gate: false` (via the already-
loaded `config`). Wire the git closures (`gitutil.RevParseRef`/`gitutil.Status`/`ErrRefNotFound`) and
the worktree/scan-root paths (`wtPath`/`root` already in scope) into `panel.ResolveGateFacts`. The
existing `panelAdvisory` call is either replaced by, or kept-then-superseded-by, the authoritative
gate (the advisory's `*panel.Registration` return is still needed for the post-completion audit
writes at `writePanelAuditMetadata`, so reuse the same scan). (L — new gate wiring + Block plumbing +
fail-open + three hatches + the full new-test matrix; do NOT split — the ordering + fail-open + parity
ACs are one atomic enforcement change.) (S.)

**Steps**
1. In `internal/complete/complete.go` at the step-2.25 site (`:254-264`), after `panelReg` is
   resolved but BEFORE `exec.CommitAll` (`:271`), add the authoritative gate: resolve the config
   toggle and the `MINDSPEC_SKIP_PANEL` env. **CONFIG AVAILABILITY:** `cfg` is currently loaded at
   complete.go:528, which is AFTER this step-2.25 gate site — it is NOT in scope here. Add an EARLIER
   `config.Load(root)` at the gate site (or thread `cfg` down to it) to read `enforcement.panel_gate`;
   do NOT try to reuse the `:528` load. Then, if
   neither short-circuits, scan + `panel.ForBead(... beadID)` (reuse the advisory's scan), and for
   each matched registration call `panel.ResolveGateFacts(reg, beadID, GateIO{RevParse:
   gitutil.RevParseRef, Status: gitutil.Status, IsRefNotFound: func(e){return errors.Is(e,
   gitutil.ErrRefNotFound)}, ...})` with the rev-parse target `beadHead` (`:238`) and the bead
   worktree path (`wtPath`), then `panel.PanelGateDecision(facts)`. A Block from any matched panel
   wins; a Warn surfaces (abandoned/missing-ref note); else proceed. No `panel.json` → no
   registration → pass (fail-open §6).
2. On a Block decision, return `nil, guard.NewFailure(decision.Message, <recovery command>)`. The
   relocated `decision.Message` already ends with the raw-`git merge` fence prose but carries NO
   `recovery:` line (the hook emitted it via its own exit-2 path, never through `internal/guard`); a
   bare `guard.NewFailure(decision.Message)` with ZERO commands would PANIC (recovery.go:47-48,
   "requires at least one recovery command"). So the in-binary caller MUST pass a GENUINE, non-empty
   recovery command — model it on how the 096/098 close-verify soft-blocks construct their
   `guard.NewFailure` recovery line (e.g. complete.go:499/506 pass `fmt.Sprintf("mindspec complete
   %s", beadID)`). For the panel gate the recovery command is the action an agent runs to SATISFY the
   gate, e.g. re-panel then re-complete (`/ms-panel-run step 0` for the bead, then
   `mindspec complete <id>`). Net: `FormatFailure` puts the fence-bearing body FIRST and appends the
   `recovery:` line LAST, so the message passes `guard.HasFinalRecoveryLine` (ADR-0035) AND still
   carries the fence in the body (R5 hook-parity). It is a PRE-`CommitAll`/PRE-`bd close`/PRE-merge
   gate failure that exits non-zero having mutated nothing (ADR-0035 HC-4). On a Warn, print the note
   to `advisoryOut` and proceed. Keep the audit-write path (`writePanelAuditMetadata`) on the matched
   `panelReg`. (Division of labor: Bead 1 leaves the fence-bearing message recovery-line-FREE and the
   hook unchanged; only THIS path adds the `guard.NewFailure(+recovery)` wrapping — so Bead 1 stays
   GREEN-parity.)
3. Honor the hatches with parity to the hook: `MINDSPEC_SKIP_PANEL=1` → skip the gate (audited via
   the existing `writePanelAuditMetadata` skip path), NEVER naming the variable in any block;
   `enforcement.panel_gate: false` → skip; audited abandonment → the decision's Warn path passes.
   Single-source the env name on `panel.SkipPanelEnv` (Bead 1).
4. Reconcile with the existing advisory: the vote-only `panelAdvisory` (`panel_advisory.go:45`) is
   now SUPERSEDED by the authoritative decision for the enforcement signal; keep enough of it (or its
   scan) to still return the `*panel.Registration` for the post-completion audit writes, and drop the
   now-misleading "would PASS/BLOCK (advisory)" wording or replace it with the authoritative result.
5. Add the Bead-2 test matrix in `internal/complete` (precedent: the existing complete tests +
   `panelScanFn`/`panelTallyFn` seams, plus a `RevParse`/`Status` injection for `panel.GateFacts`):
   sub-threshold block (exits non-zero, no `bd close`/merge, error message PASSES
   `guard.HasFinalRecoveryLine` AND contains the fence — the block does not panic and is R5-parity);
   pre-`CommitAll` ordering (stale/dirty block fires on the pre-commit `beadHead` tip, not
   caused/cleared by `CommitAll`); fail-open panel-less ACTUALLY COMPLETES (closed + merged + exit 0);
   different-bead isolation (sub-threshold panel for X does not block Y); freshness pass — the test
   asserts `reviewed_head_sha == rev-parse(beadHead)` (the in-binary HEAD source at complete.go:238)
   so a future beadHead-resolution divergence from the hook source is caught RED-on-divergence, then
   the fresh+clean+threshold-met bead proceeds; the three hatches; and the shared-decision pin (hook +
   complete reach the same `panel.PanelGateDecision` over the same `panel.GateFacts`).

**Verification**
- [ ] `go build ./... && go test ./internal/complete/... ./internal/panel/... ./internal/hook/...`
      green (do NOT run `./internal/harness/...`)
- [ ] The gate runs at step-2.25 BEFORE `exec.CommitAll` (`:271`), `bd close` (`:418`), and the merge
      — a stale/dirty block fires on the pre-`CommitAll` `beadHead` tip (RED if moved after CommitAll)
- [ ] A sub-threshold / stale / incomplete / REJECT panel BLOCKS via
      `guard.NewFailure(decision.Message, <recovery command>)` (does NOT panic — a zero-command call
      would), exits non-zero, mutates nothing, and the error PASSES `guard.HasFinalRecoveryLine` AND
      contains the raw-`git merge` fence in the body before the recovery line (RED on revert to
      advisory-only)
- [ ] A panel-less `complete <id>` ACTUALLY COMPLETES (closed + merged + exit 0) — fail-open §6
      preserved (not a silent no-op)
- [ ] Different-bead isolation, freshness pass, and the three hatches (`MINDSPEC_SKIP_PANEL`,
      `enforcement.panel_gate: false`, audited abandonment) all behave identically to the hook; the
      block never names the skip variable (HC-7)
- [ ] The shared-decision pin proves the hook and in-binary call sites reach the SAME
      `panel.PanelGateDecision` over the SAME `panel.GateFacts`; golangci-lint clean

**Acceptance Criteria**
- [ ] `mindspec complete <id>` enforces the full ADR-0037 contract over the DECLARED bead-id at the
      step-2.25 site BEFORE `exec.CommitAll` / `bd close` / merge: a sub-threshold (or stale, missing/
      malformed-verdict, REJECT/hard_block) registered panel BLOCKS via
      `guard.NewFailure(decision.Message, <genuine recovery command>)`, exits non-zero, mutates
      nothing, and the error PASSES `guard.HasFinalRecoveryLine` AND carries the raw-`git merge` fence
      in the body before that recovery line — for EVERY invocation form, with no shell parsing. The stale/dirty block measures the pre-`CommitAll`
      `bead/<id>` tip (RED-on-revert if placed after `CommitAll`).
- [ ] A `complete <id>` for a bead with NO `panel.json` referencing it ACTUALLY COMPLETES — closed +
      merged + exit 0 (fail-open §6, dogfooding safety). A sub-threshold panel for bead X does NOT
      block `complete <Y>` (different-bead isolation via `panel.ForBead`). A fresh, threshold-met,
      clean-tree bead proceeds to close+merge (freshness pass proving the in-binary HEAD source is the
      same `bead/<id>` ref the hook uses).
- [ ] The `MINDSPEC_SKIP_PANEL=1` skip (never named in a block, HC-7), audited abandonment, and
      `enforcement.panel_gate: false` behave identically to the hook; the in-binary gate and the hook
      invoke the IDENTICAL `panel.PanelGateDecision` over the IDENTICAL `panel.GateFacts` from the
      IDENTICAL fact-gatherer (one source, no second matrix copy) — pinned by a shared-decision test.

**Depends on**
Bead 1 (needs the EXTRACTED `panel.PanelGateDecision` + `panel.ResolveGateFacts` from `internal/panel`;
both beads also touch `internal/panel`, so they are SERIALIZED 1→2 to avoid a shared-file collision —
Bead 1 lands the decision in `internal/panel`, Bead 2 only consumes it from `internal/complete`)

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By |
|-----------------------------|------|-------------|
| R3: relocate `panelGateDecision` + `gateFacts` + fact-gathering into `internal/panel` leaf (mechanism (a)); both call sites invoke the IDENTICAL decision over IDENTICAL facts (one source) — leaf preserved via injected git I/O seam | Bead 1 (relocate) + Bead 2 (shared-decision pin) | Bead 1 Steps 1–3, 5 + Bead 2 Step 5 |
| R4: hook + heuristic matcher LEFT IN PLACE, behaviorally unchanged (hook now calls extracted decision); retirement follow-up bead recorded | Bead 1 | Steps 3, 4, 6 + verification (GREEN-parity run tests) |
| R1: in-binary authoritative gate over the declared bead-id at step-2.25 BEFORE `CommitAll`/`bd close`/merge; full §§2-6 matrix + §7 hatches; BLOCK via `guard.NewFailure` exiting non-zero, mutating nothing | Bead 2 | Steps 1–3 + verification |
| R1 ordering: stale/dirty block measures the pre-`CommitAll` `bead/<id>` tip (not false-fired/cleared by CommitAll) | Bead 2 | Step 5 (pre-`CommitAll` ordering test) |
| R2: panel-less `complete` ACTUALLY COMPLETES (closed + merged + exit 0), fail-open §6 preserved | Bead 2 | Step 5 (fail-open test) |
| R5: block parity with the hook (name panel/round, stale SHA pair / present-missing verdicts, raw-`git merge` fence in the body); the in-binary block ADDS the ADR-0035 `recovery:` line via `guard.NewFailure`'s command arg (the hook emits the same fence-bearing message via exit-2, no recovery line) | Bead 1 (fence-bearing messages move verbatim, recovery-line-free) + Bead 2 (wraps with `guard.NewFailure(+recovery)`) | Bead 1 Step 1 + Bead 2 Steps 1–2 |
| Different-bead isolation (`panel.ForBead`); freshness pass (HEAD source == hook's `bead/<id>` ref) | Bead 2 | Step 5 |
| `MINDSPEC_SKIP_PANEL` / audited abandonment / `enforcement.panel_gate` parity; skip never named (HC-7) | Bead 2 | Step 3 + verification |
| `go build` + `go test ./internal/{complete,hook,panel}/...` + golangci-lint green | Both beads | Each bead's verification |
| `mindspec validate spec 099-panel-gate-in-complete` passes (workflow domain owned; ADR-0037/0035/0036 Accepted + intersecting) | Both beads | ADR Fitness + frontmatter `adr_citations` |
