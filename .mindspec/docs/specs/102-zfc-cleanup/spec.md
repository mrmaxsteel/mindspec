---
approved_at: "2026-06-16T23:28:59Z"
approved_by: user
status: Approved
---
# Spec 102-zfc-cleanup: ZFC cleanup: retire the PreToolUse heuristic complete-matcher + structured path-extraction in the harness analyzer

## Goal

Close two residual ADR-0036 Zero-Framework-Cognition (ZFC) violations where code
still parses free-form strings to guess semantic facts, by either consuming
declared/structured data instead or retiring the heuristic outright:

- **R1** retires the PreToolUse hook's heuristic command-STRING complete-matcher
  in `internal/hook/precomplete.go`. The authoritative panel-gate enforcement is
  the in-binary gate inside `mindspec complete` (spec 099), which reads the
  DECLARED bead-ID argument and covers every invocation form ZFC-cleanly. The
  hook's matcher must parse a free-form shell command to guess a bead ID — the
  exact ADR-0036 anti-pattern — and is now redundant, so it is removed.

- **R2** removes the harness analyzer's hard-coded extension heuristic
  (`extractMentionedPaths` in `internal/harness/analyzer.go`) that scrapes file
  paths out of plan prose. This is the secondary ZFC site that spec 097 Bead 5
  explicitly deferred (097 retired the prose extractor only for contextpack's
  `## Key File Paths`).

Target outcome: no remaining production code path in `internal/hook/**` or
`internal/harness/**` infers a semantic identifier (bead ID, expected file path)
by string-scanning free-form prose; the authoritative behaviors (the in-binary
complete gate, the harness PlanFidelity score) are preserved or removed cleanly.

## Background

ADR-0036 establishes the ZFC stance: heuristic classification is forbidden —
semantic decisions must come from declared/structured data, not from guessing
over free-form strings. Spec 099 acted on this for the panel gate: it relocated
the AUTHORITATIVE enforcement from the PreToolUse hook into `mindspec complete`'s
in-binary gate, which reads the bead ID as a DECLARED command argument and so
covers every invocation form (Claude-Code-routed, codex, raw-shell) regardless of
how completion was spawned. The shared decision logic moved to the
`internal/panel` leaf (`PanelGateDecision` / `ResolveGateFacts` / `gate.go`), so
the in-binary gate and the hook call one decision and cannot drift.

099 R4 deliberately LEFT the PreToolUse hook in place as a non-authoritative
defense-in-depth backstop and DEFERRED retirement of its heuristic
command-string matcher to a follow-up bead. ADR-0037 records exactly this: its
2026-06-14 amendment says the matcher "is retained but non-authoritative, its
retirement deferred to a follow-up bead." **R1 (bead mindspec-lfi3) is that
retirement.**

Separately, spec 097 retired a prose path-extractor for contextpack's
`## Key File Paths` in favor of the structured per-WorkChunk `key_file_paths`
plan frontmatter — but scoped that change to `internal/contextpack` +
`internal/approve`, explicitly DEFERRING the analogous heuristic in the harness
analyzer (097 Bead 5). **R2 (bead mindspec-qcjf) is that deferred cleanup.**

Both requirements are pure ADR-0036 hygiene; neither changes the authoritative
behavior of the complete gate (099), and neither touches the 2u0u / uopd
close-verify dolt-session work.

## Impacted Domains

- workflow: owns `internal/hook/**` and the ADR docs (`.mindspec/docs/adr/**`).
  R1 edits `internal/hook/precomplete.go` (removing the heuristic matcher + the
  dead `runPreComplete` gate body / `pre-complete` dispatch arm), removes the
  retired `"pre-complete"` entry from `internal/hook/hook.go`'s `Names` slice,
  and AMENDS `ADR-0037-panel-gate-enforced-contract.md`. Per OWNERSHIP.yaml,
  workflow owns `internal/hook/**`, `internal/adr/**`, `internal/complete/**`,
  `internal/panel/**`, and `cmd/**`.
- execution: owns `internal/harness/**`. R1 ALSO removes the now-broken
  `ScenarioPanelGateBlocksPrematureComplete` LLM scenario in
  `internal/harness/scenario_panel_gate.go` (its deterministic probe shells out
  to the retired `mindspec hook pre-complete` subprocess — see R1). R2 edits
  `internal/harness/analyzer.go` (`extractMentionedPaths`, its call site + the
  path-match block, and its consumer `PlanFidelity`). Per OWNERSHIP.yaml,
  execution owns `internal/harness/**`.

(Every source file changed by this spec — `internal/hook/precomplete.go`,
`internal/hook/hook.go`, optionally `internal/hook/dispatch.go`,
`internal/harness/scenario_panel_gate.go`, and `internal/harness/analyzer.go` —
is owned by exactly one declared domain above. R1 touches BOTH workflow and
execution domains; it is not workflow-only.)

## ADR Touchpoints

- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md): the ZFC anchor for both
  requirements. Its "Zero Framework Cognition" stance — heuristic classification
  forbidden; semantic decisions come from declared/structured data, not from
  parsing free-form strings — is precisely what the heuristic command-string
  matcher (R1) and the prose path-extractor (R2) violate. Status: Accepted.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): AMENDED by R1.
  Its 2026-06-14 (spec 099) amendment records the heuristic command-string
  matcher as "retained but non-authoritative, its retirement deferred to a
  follow-up bead." This spec adds a dated amendment note (mirroring the
  099/100/101 amendment pattern) recording that the retirement has landed: the
  PreToolUse pre-complete hook gate and its heuristic matcher are removed; the
  in-binary `mindspec complete` gate over the shared `internal/panel` decision
  remains the single authoritative enforcement point. Contract semantics
  (thresholds, staleness, fail-open/closed, hatches) are unchanged. Status:
  Accepted (amended).

ADR coverage of the changed source: ADR-0037 covers the workflow-owned
`internal/hook/**` + complete-gate surface (R1). The execution-owned
`internal/harness/**` analyzer (R2) is anchored by ADR-0036 (its ZFC stance is
the governing decision for retiring the heuristic). No Accepted ADR specifically
governs the harness analyzer's PlanFidelity scoring beyond ADR-0036 — see Open
Questions.

## Requirements

1. **R1 (bead mindspec-lfi3, P2) — Retire the PreToolUse heuristic
   complete-matcher.** Remove the heuristic command-string matching from
   `internal/hook/precomplete.go` — including `matchMindspecComplete`,
   `commandSegments`, `segmentInvokesComplete`, `strippedCompleteFields`,
   `completeBeadID` / `segmentCompleteBeadID`, the wrapper-stripping helpers
   (`isEnvAssignment`, `isMindspecBinary`, `isValueFlag`, `cdPrefixPath`,
   `leadingCdPath`), and the now-unreachable `runPreComplete` gate body plus its
   scan-root / fact-resolution wiring — and remove the `pre-complete` arm from
   the hook dispatcher (`internal/hook/dispatch.go`) and its installation in
   `internal/setup/claude.go` / `cmd/mindspec/setup.go`. Removing the dispatch
   arm leaves `"pre-complete"` a DANGLING registered hook name, so ALSO remove
   `"pre-complete"` from `internal/hook/hook.go`'s `Names` slice (the
   valid-hook gate `cmd/mindspec/hook.go` reads against `hook.Names`) and update
   `TestNames_Sorted` (and any other test asserting on `Names`) accordingly.
   FINALLY, retiring the live `mindspec hook pre-complete` subprocess BREAKS the
   harness LLM scenario `ScenarioPanelGateBlocksPrematureComplete`
   (`internal/harness/scenario_panel_gate.go`, run by
   `TestLLM_PanelGateBlocksPrematureComplete`): its deterministic probe
   `assertPanelGateBlocksIncomplete` shells out to `mindspec hook pre-complete
   --format=claude` and hard-asserts exit 2 + the incomplete-panel block text +
   the `git merge bead/` fence — assertions that go FALSE once the hook is gone.
   This break is invisible to this spec's Validation Proofs (which never run
   `go test ./internal/harness/...`). **DESIGN DECISION — REMOVE the scenario:**
   delete `internal/harness/scenario_panel_gate.go` and its
   `TestLLM_PanelGateBlocksPrematureComplete` registration. The scenario's
   INTENT ("an incomplete registered panel blocks `mindspec complete`") is the
   AUTHORITATIVE in-binary gate's behavior, which is already pinned HERMETICALLY
   and deterministically by the spec-099 tests in
   `internal/complete/panel_gate_e2e_test.go` (8 `TestPanelGate_*` tests —
   incl. `TestPanelGate_SubThreshold_Blocks`, the `git merge bead/` fence, the
   `guard.HasFinalRecoveryLine` recovery line, `TestPanelGate_BlockNeverNamesSkipVar`
   for HC-7, and `TestPanelGate_SharedDecisionPin` for the shared-decision
   parity). The REPOINT alternative (re-point the probe at `mindspec complete
   <bead>` and assert it blocks pre-merge) was REJECTED as disproportionate: it
   would require fabricating a full completable-bead repo state (claimed bead,
   approved spec+plan, bead worktree, merge plumbing) inside the LLM sandbox to
   reach the in-binary gate — heavy machinery duplicating coverage the 099
   hermetic tests already provide, for a slow LLM scenario. NOTED LOSS: the
   LLM-level e2e assertion that an agent told to complete a premature bead is
   actually stopped by the gate (the behavioral-envelope half: no successful
   `mindspec complete`, no agent-set `MINDSPEC_SKIP_PANEL` hatch) is retired with
   the scenario; the in-binary gate's enforcement remains fully covered
   hermetically, only the LLM-driven behavioral-envelope check is lost. **DESIGN
   DECISION —
   option (a): RETIRE the precomplete hook gate ENTIRELY.** Identifying a
   `mindspec complete` invocation at the PreToolUse boundary REQUIRES heuristic
   string parsing (there is no declared bead ID at the hook boundary — only the
   agent-writable command string), so any "thin non-heuristic backstop" is
   impossible without re-introducing the ZFC violation. The authoritative
   in-binary gate (099) already covers every invocation form by reading the
   declared argument, so the hook adds no coverage it cannot already provide
   ZFC-cleanly. The shared `internal/panel` leaf decision (`PanelGateDecision` /
   `ResolveGateFacts` / `gate.go`, exercised by the in-binary path) is the single
   source of truth and is RETAINED untouched. The in-binary gate's block parity
   MUST remain intact: `internal/complete` does not import `internal/hook`, so
   removing the hook matcher cannot affect it.

2. **R2 (bead mindspec-qcjf, P3) — Structured-or-retire path-extraction in the
   harness analyzer.** Address `extractMentionedPaths` in
   `internal/harness/analyzer.go` (the hard-coded Go/TS/JS/PY/YAML/MD/JSON
   extension heuristic that scrapes file paths from plan prose), whose only
   consumer is `PlanFidelity`. **DESIGN DECISION — option (b): RETIRE the harness
   heuristic.** The structured per-WorkChunk `key_file_paths` plan frontmatter
   that 097 introduced lives in `internal/validate` + `internal/approve` and is
   NOT reachable from the harness analyzer (no reference to `key_file_paths` /
   `KeyFilePaths` exists anywhere under `internal/harness/**`); the analyzer
   receives only a raw plan file path. `PlanFidelity` is non-gating context
   enrichment whose score is surfaced only in the harness report
   (`report.go`: `PlanFidelityScore`), never used to pass/fail a scenario, and
   the bead itself classifies it as "non-gating context enrichment, low
   priority." Plumbing the structured frontmatter into the harness for a
   non-gating score is disproportionate. Therefore: remove the PATH TERM from
   `PlanFidelity` AS A UNIT — `extractMentionedPaths` AND its call site
   (`expectedPaths := extractMentionedPaths(content)`, analyzer.go ~L544) AND the
   path-match block that consumes it (the `touchedPaths` Write/Edit/Read
   basename-match loop + the `for _, p := range expectedPaths` matcher,
   ~L554-569) AND the `expectedPaths` references in the empty-expectations
   short-circuit (~L547) and the `total` denominator (~L551) — so `PlanFidelity`
   becomes commands-only and stays INTERNALLY CONSISTENT (no dangling
   `expectedPaths` variable, no orphaned path-match block). The function still
   compiles, still returns a defined score, and no longer parses free-form prose.
   The exact reshaping is a planning detail; the AC is that the path term
   (`extractMentionedPaths` + its call site + the path-match block) is gone AS A
   UNIT and `PlanFidelity` still returns a sensible score for its existing tests.

## Scope

### In Scope
- `internal/hook/precomplete.go` — remove the heuristic matcher + the dead
  pre-complete gate body and wiring (R1).
- `internal/hook/dispatch.go` — remove the `pre-complete` dispatch arm (R1).
- `internal/hook/hook.go` — remove the `"pre-complete"` entry from the `Names`
  slice so the retired hook is no longer a dangling registered name (R1).
- `internal/setup/claude.go` + `cmd/mindspec/setup.go` — remove the PreToolUse
  pre-complete hook installation; ensure setup idempotently REMOVES a previously
  installed `mindspec hook pre-complete` entry (mirroring the spec-072 retired-
  hook precedent already present in `claude.go`) (R1).
- `internal/hook/precomplete_match_test.go` + `internal/hook/precomplete_run_test.go`
  — delete or reduce to whatever remains after the gate is retired (R1).
- `internal/hook/hook_test.go` — update `TestNames_Sorted` (and any other test
  asserting on `Names`) for the removed `"pre-complete"` entry (R1).
- `internal/harness/scenario_panel_gate.go` — DELETE the file: the
  `ScenarioPanelGateBlocksPrematureComplete` scenario and its probe
  `assertPanelGateBlocksIncomplete` shell out to the retired `mindspec hook
  pre-complete` subprocess, so its assertions break; its intent is covered
  hermetically by `internal/complete/panel_gate_e2e_test.go` (R1).
- `internal/harness/scenario_panel_gate_test.go` — DELETE: it holds
  `TestLLM_PanelGateBlocksPrematureComplete`, which only wraps the deleted
  scenario (R1).
- `internal/harness/scenario.go` — remove the
  `ScenarioPanelGateBlocksPrematureComplete()` entry (~L64) from the registered
  scenario list so the harness no longer references the deleted scenario (R1).
- `internal/harness/analyzer.go` — remove the path term as a unit
  (`extractMentionedPaths` + its call site + the path-match block), rework
  `PlanFidelity` to be commands-only (R2).
- `internal/harness/analyzer_test.go` — update `TestAnalyzer_PlanFidelity_*` to
  match the reworked behavior (R2).
- `.mindspec/docs/adr/ADR-0037-panel-gate-enforced-contract.md` — dated
  amendment recording the retirement (R1).

### Out of Scope
- The AUTHORITATIVE in-binary `mindspec complete` panel gate (spec 099) and the
  shared `internal/panel` leaf decision — unchanged.
- `internal/contextpack/**` + `internal/approve/**` prose extractor (already
  handled by spec 097).
- The `key_file_paths` structured frontmatter producers/consumers in
  `internal/validate` / `internal/approve`.

## Non-Goals

- This spec does NOT change the authoritative in-binary gate's enforcement,
  thresholds, staleness, fail-open/closed semantics, or escape hatches (099 /
  ADR-0037 §3-§7).
- This spec does NOT plumb the structured `key_file_paths` frontmatter into the
  harness analyzer (R2 chooses retirement, not the structured-consume option).
- This spec does NOT touch bead uopd / the 2u0u close-verify dolt-session issue.
- No new ZFC-style heuristic string parsing is introduced anywhere.

## Acceptance Criteria

- [ ] R1: `grep -rn "matchMindspecComplete\|segmentInvokesComplete\|strippedCompleteFields\|completeBeadID" internal/hook/` returns no production-code matches (the heuristic matcher is gone).
- [ ] R1: `internal/hook/dispatch.go` has no `pre-complete` case (the gate is fully retired) and `go build ./...` succeeds.
- [ ] R1: `"pre-complete"` is gone from `internal/hook/hook.go`'s `Names` slice — `grep -n "pre-complete" internal/hook/hook.go` returns no match — and `go test -run TestNames_Sorted -timeout 120s ./internal/hook/` PASS (no dangling registered hook name; the `cmd/mindspec/hook.go` valid-hook gate no longer admits `pre-complete`).
- [ ] R1: the broken LLM scenario is removed — `internal/harness/scenario_panel_gate.go` and `internal/harness/scenario_panel_gate_test.go` are deleted, `grep -rn "ScenarioPanelGateBlocksPrematureComplete\|TestLLM_PanelGateBlocksPrematureComplete" internal/harness/` returns no match, and `go build ./... && go vet ./internal/harness/` succeeds (no reference to the removed scenario remains in `scenario.go`). ⚠️ verify by build/vet only — NEVER `go test ./internal/harness/...`. The in-binary gate's coverage is preserved by `internal/complete/panel_gate_e2e_test.go` (next AC).
- [ ] R1: the in-binary gate's incomplete/sub-threshold block coverage that the deleted LLM scenario stood in for is still pinned hermetically — `go test -run 'TestPanelGate' -timeout 120s ./internal/complete/` PASS (all 8 `TestPanelGate_*` tests, incl. `SubThreshold_Blocks`, the `git merge bead/` fence, the recovery line, `BlockNeverNamesSkipVar`, `SharedDecisionPin`).
- [ ] R1: In-binary block parity preserved — `go test -run TestPanelGateDecision -timeout 120s ./internal/panel/` and `go test -run 'TestPanelGate|TestComplete' -timeout 120s ./internal/complete/` PASS unchanged (the authoritative gate still blocks correctly, proving it never depended on the removed hook code).
- [ ] R1: `mindspec setup` (or its unit test in `internal/setup`) no longer installs a `mindspec hook pre-complete` PreToolUse entry, and idempotently removes a pre-existing one: `go test -run 'TestSetup|TestClaude' -timeout 120s ./internal/setup/` PASS.
- [ ] R1: ADR-0037 carries a dated amendment note recording the retirement; `grep -n "retire" .mindspec/docs/adr/ADR-0037-panel-gate-enforced-contract.md` shows the new note dated 2026-06.
- [ ] R2: the path term is gone AS A UNIT — `grep -rn "extractMentionedPaths\|expectedPaths\|touchedPaths" internal/harness/analyzer.go` returns no match (the extractor, its call site, AND the path-match block all removed; `PlanFidelity` is commands-only and internally consistent with no dangling `expectedPaths`).
- [ ] R2: `PlanFidelity` still compiles and returns a defined score — `go test -run TestAnalyzer_PlanFidelity -timeout 120s ./internal/harness/` PASS (filtered; NEVER the slow `TestLLM_*` scenarios).
- [ ] Build green: `go build ./...` succeeds with both changes applied.
- [ ] No new heuristic: no new function in `internal/hook/**` or `internal/harness/**` parses a free-form command/prose string to infer a bead ID or file path.

## Validation Proofs

- `mindspec validate spec 102-zfc-cleanup`: 0 errors (lifecycle-binding WARN is expected/OK pre-approval).
- `go build ./...`: compiles clean after both R1 and R2.
- `go test -run TestPanelGateDecision -timeout 120s ./internal/panel/`: PASS — authoritative decision intact (R1 parity).
- `go test -run 'TestPanelGate|TestComplete' -timeout 120s ./internal/complete/`: PASS — in-binary gate still blocks (R1 parity).
- `go test -run 'TestSetup|TestClaude' -timeout 120s ./internal/setup/`: PASS — hook no longer installed (R1).
- `go test -run TestNames_Sorted -timeout 120s ./internal/hook/`: PASS — `Names` no longer carries `pre-complete` (R1).
- `go test -run 'TestPanelGate' -timeout 120s ./internal/complete/`: PASS — the in-binary gate's block coverage (the deleted LLM scenario's intent) stays pinned hermetically (R1).
- `go build ./... && go vet ./internal/harness/`: clean — the deleted `ScenarioPanelGateBlocksPrematureComplete` is no longer referenced (R1). ⚠️ build/vet only — NEVER `go test ./internal/harness/...`.
- `go test -run TestAnalyzer_PlanFidelity -timeout 120s ./internal/harness/`: PASS — fidelity score sane without the heuristic (R2). ⚠️ filtered only — NEVER `go test ./internal/harness/...`.

## Open Questions

- [x] **ADR coverage of the harness analyzer (R2).** No Accepted ADR governs the
  `PlanFidelity` scoring model beyond ADR-0036's ZFC stance. RESOLVED: ADR-0036
  is the governing anchor; R2 retires the prose-scraped path-expectation term.
  If a plan-gate reviewer judges the path-fidelity signal worth preserving, the
  alternative is plumbing the structured `key_file_paths` frontmatter into the
  harness — to be raised at plan review, not a blocker for this spec.
- [x] **External `PlanFidelityScore` consumers (R2).** RESOLVED: the harness
  report still emits the `PlanFidelityScore` field; only its value changes when
  the path term is dropped. Planning must confirm no CI dashboard/script depends
  on the exact value, but the field's presence/shape is unchanged, so this is a
  plan-time check, not a spec blocker.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-16
- **Notes**: Approved via mindspec approve spec