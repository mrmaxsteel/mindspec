---
adr_citations:
    - ADR-0037
    - ADR-0036
approved_at: "2026-06-16T23:46:23Z"
approved_by: user
bead_ids:
    - mindspec-ettz.1
    - mindspec-ettz.2
spec_id: 102-zfc-cleanup
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
    - depends_on: []
      id: 2
---
# Plan: 102-zfc-cleanup

## ADR Fitness

Two Accepted ADRs are cited. Both intersect this spec's Impacted Domains
(`workflow` + `execution`), so `checkADRCoverage` finds every impacted domain
covered and `checkADRCitations` finds no irrelevant citation.

- **ADR-0037 (Panel-Gate Enforced Contract, Accepted; Domain(s): workflow,
  execution)** — the governing decision for the panel-gate enforcement surface.
  It covers BOTH impacted domains by itself: `workflow` (the hook + complete
  gate) and `execution` (the harness). Its 2026-06-14 (spec 099) amendment
  recorded the PreToolUse heuristic command-string matcher as "retained but
  non-authoritative, its retirement deferred to a follow-up bead." **This plan
  AMENDS ADR-0037**: Bead 1 (`mindspec-lfi3`) adds a dated 2026-06-17 amendment
  note recording that the retirement has landed — the PreToolUse pre-complete
  hook gate and its heuristic matcher are removed; the in-binary `mindspec
  complete` gate over the shared `internal/panel` decision remains the single
  authoritative enforcement point. Because ADR-0037 already declares both
  `workflow` and `execution`, it is BOTH the amended ADR AND a valid frontmatter
  citation (unlike the 100/099 pattern where the amended ADR did not yet
  intersect the impacted domain). Contract semantics (thresholds §3, staleness
  §4, §6 fail-open/closed, §7 hatches) are unchanged.
- **ADR-0036 (Ownership Discovery / Zero-Framework-Cognition anchor, Accepted;
  Domain(s): workflow, validation, doc-sync, ownership)** — the canonical ZFC
  stance: heuristic classification is forbidden; semantic decisions must come
  from declared/structured data, not from string-scanning free-form prose. This
  is the governing decision for BOTH retirements. R1 removes the hook matcher
  that parses a free-form shell command to guess a bead ID (the exact ADR-0036
  anti-pattern); R2 removes the prose path-extractor that scrapes file paths out
  of plan text by hard-coded extension. ADR-0036 declares `workflow` (one of the
  two impacted domains), so it is a relevant, non-irrelevant citation.

Coverage check: `workflow` is covered by both ADR-0037 and ADR-0036; `execution`
is covered by ADR-0037. No impacted domain is left uncovered, and neither
citation is irrelevant (each declares ≥1 impacted domain). ADR-0036 is the ZFC
anchor that justifies the removals; ADR-0037 is the contract ADR amended by R1.

## Testing Strategy

Test-first where there is behavior to pin (R2); removal-verification where the
change is a deletion (R1). Every gate is hermetic and CI-runnable. **⚠️ NEVER run
`go test ./internal/harness/...`** — it executes real LLM scenarios (minutes
each, gets KILLED). The harness is touched by BOTH beads, so the ONLY permitted
harness gates are `go build ./...`, `go vet ./internal/harness/`, and FILTERED
`go test -run <Name> -timeout 120s ./internal/harness/`.

- **R1 is a removal bead.** Its "RED→GREEN" is inverted: the RED state is that
  the removed-symbol greps are NON-empty and the broken scenario file exists
  BEFORE the change; the GREEN state is post-removal grep-clean + `go build`
  green + `go vet ./internal/harness/` clean + the preserved hermetic coverage
  still passing. The authoritative in-binary gate's coverage that the deleted LLM
  scenario stood in for is pinned HERMETICALLY by the spec-099 tests in
  `internal/complete/panel_gate_e2e_test.go` (the 8 `TestPanelGate_*` tests:
  `FailOpen_ActuallyCompletes`, `SubThreshold_Blocks`, `DifferentBead_DoesNotBlock`,
  `Freshness_PassAndStaleBlock`, `Hatch_SkipEnv`, `Hatch_ConfigToggle`,
  `BlockNeverNamesSkipVar` [HC-7], `SharedDecisionPin`). `internal/complete` does
  NOT import `internal/hook`, so removing the hook matcher cannot affect in-binary
  block parity — these tests prove it.
- **R2 is a behavior-reshaping bead.** `PlanFidelity` loses its path term and
  becomes commands-only. The genuine RED→GREEN pin is a NEW **exact-score witness
  test**, because the existing `_HighScore`/`_LowScore` threshold assertions
  (`score<0.5` / `score>0.5`) do NOT flip when the path term is removed — they pass
  IDENTICALLY old and new (High: 2/2=1.0 old → 1/1=1.0 new; Low: 0/3=0.0 old →
  0/1=0.0 new), so they cannot serve as the RED pin. The RED witness instead asserts
  the EXACT score on a plan+events shape constructed so path-aware and commands-only
  scoring DISAGREE: a plan that mentions BOTH a file path AND a known command, events
  that TOUCH the mentioned path but run a DIFFERENT (non-mentioned) command. Under
  OLD path-aware scoring this scores `0.5` (1 path matched of 2 = path+command total);
  under NEW commands-only scoring it scores `0.0` (0 of 1 command matched, path term
  gone). Asserting the exact `0.0` FAILS on the pre-change function (returns `0.5`)
  and PASSES after the path-term removal = genuine RED→GREEN. The reworked High/Low
  tests are kept as commands-only regression LOCKS (not the RED pin). All run FILTERED
  (`go test -run TestAnalyzer_PlanFidelity -timeout 120s ./internal/harness/`).

Gate commands per bead (NEVER the whole harness package):
`go build ./...` + `go vet ./internal/{hook,setup,harness}/` (R1) and filtered
`go test -run <Name> -timeout 120s ./internal/...` (both beads).

## Decomposition (heuristic justification)

Two beads, one per requirement, fully independent. Justification against the
mindspec plan heuristics:

- **Independence (the dominant signal).** R1 (retire the PreToolUse hook) and R2
  (retire the harness `extractMentionedPaths`) are two unrelated ZFC retirements.
  They share NO line, NO function, and NO runtime data dependency: R1 lives in
  `internal/hook/**` + `internal/setup/**` + `cmd/mindspec/setup.go` + the
  ADR doc + (in the harness) `scenario_panel_gate*.go` + `scenario.go`; R2 lives
  in `internal/harness/analyzer.go` + `analyzer_test.go`. Neither bead reads the
  other's output. Per spec 097 R3, only a genuine output-consumption relationship
  earns a `depends_on` edge, so BOTH `depends_on` arrays are empty.
- **Target band (3–5).** 2 beads is below the 3–5 target band, which is
  acceptable here: the spec is two truly-independent retirements with no
  cross-cutting refactor to split out, and fusing them would create a single bead
  touching two domains (`workflow` + `execution`) and two unrelated subsystems
  for no merge benefit. Splitting either retirement further would produce
  sub-beads with no independent verifiable AC. 2 cleanly-separated beads is the
  right granularity.
- **Serial-chain ≤3.** The dependency graph is fully parallel: longest serial
  chain = 1, well under the ≤3 limit. Both beads are immediately ready.

**work_chunks depends_on graph** (structured frontmatter — the wired form; `id`
is the 1-based positional index of the `## Bead N` section per the spec 097 R3
schema, `depends_on` is a `[]int`):

```
id 1 = ## Bead 1 (mindspec-lfi3, R1)  depends_on: []
id 2 = ## Bead 2 (mindspec-qcjf, R2)  depends_on: []
```

Both are independent / parallelizable.

**File-overlap merge-ORDER note (advisory, NOT a `depends_on` edge — like spec
100/101).** BOTH beads touch the `internal/harness/**` package, but DIFFERENT
files with NO line overlap: B1 touches `scenario_panel_gate.go` +
`scenario_panel_gate_test.go` (both DELETED) + `scenario.go` (one registration
line removed ~L64); B2 touches `analyzer.go` + `analyzer_test.go`. There is no
hunk-level collision. To keep two parallel agents from racing in the harness
package, record a **preferred merge order: B1 (lfi3) first, then B2 (qcjf)**. B1
is the larger, structural change (it DELETES files and removes a registration
from `scenario.go`); B2 then rebases its `analyzer.go`/`analyzer_test.go` edits
on top. This is a sequencing PREFERENCE only — it does not earn a `depends_on`
edge because neither bead consumes the other's runtime output (spec 097 R3).

---

## Bead 1 (mindspec-lfi3) — R1: retire the PreToolUse heuristic complete-matcher

**Satisfies spec ACs:** all R1 ACs (spec L251–258) and the spec-level build/no-new-heuristic
ACs (L261–262). Authors the ADR-0037 amendment note.

**Steps**

1. Capture the RED baseline (removal bead): confirm the removed-symbol greps are
   NON-empty and `internal/harness/scenario_panel_gate*.go` exist BEFORE the
   change (see **Verification** below).
2. DELETE `internal/hook/precomplete.go` entirely + its two test files
   `precomplete_match_test.go` / `precomplete_run_test.go`. Every symbol in
   `precomplete.go` (`matchMindspecComplete`, `commandSegments`,
   `segmentInvokesComplete`, `strippedCompleteFields`, `completeBeadID` /
   `segmentCompleteBeadID`, the wrapper-stripping helpers `isEnvAssignment` /
   `isMindspecBinary` / `isValueFlag` / `cdPrefixPath` / `leadingCdPath`, the gate
   body `runPreComplete` + its `hookGateIO` / `toResult` / `resolvePanelFacts` /
   `resolveScanRoots` / `resolveBeadWorktree` wiring, and the `SkipPanelEnv`
   re-export) is part of the retired matcher/gate and has NO live consumer outside
   those deleted tests. (Verified: no `hook.SkipPanelEnv` reference exists outside
   `precomplete.go`'s own comment; the canonical home is `panel.SkipPanelEnv`,
   which `internal/complete` reads directly.)
3. `internal/hook/dispatch.go` — remove the `case "pre-complete":` arm (and its
   `runPreComplete(inp)` call) from `Run` (the `default: Pass` stays);
   `internal/hook/hook.go` — remove `"pre-complete"` from the `Names` slice
   (leaving `"pre-commit"`, `"session-start"`, still sorted); `hook_test.go` —
   update `TestNames_Sorted` for the 2-element slice (confirm no other test
   asserts `pre-complete` membership).
4. `internal/setup/claude.go` — drop the PreToolUse `mindspec hook pre-complete`
   entry from `wantedHooks()` (~L316–330) so it is removed-by-omission from
   existing installs via `removeStaleMindspecEntries` (the spec-072 retired-hook
   precedent already present in `claude.go`).
   **`internal/setup/claude_test.go` — rework the pre-complete assertions
   (REQUIRED in the same bead, or the suite inverts to RED).** Once `wantedHooks()`
   no longer ships the entry, every test that hard-asserts the entry is PRESENT
   flips. There are **two distinct classes** here — fix both:
   - **(a) `wantedHooks()`-shipped assertions (DELETE / INVERT — the entry is gone
     from the wanted set):**
     - `TestRunClaude_FreshSetup` L45–49 — `hooksContainCommand(hooks, "PreToolUse",
       "mindspec hook pre-complete")` must now be ASSERTED ABSENT (invert: expect
       `false` / no PreToolUse pre-complete entry after a fresh setup), and update
       the L22–25 created-item count if dropping the entry changes it (it does not —
       the count tracks files, not hook entries; confirm at edit time).
     - `TestWantedHooks_HasPreCompletePreToolUse` L223–246 — DELETE this test
       wholesale (it pins that `wantedHooks()` ships the PreToolUse entry, which is
       exactly what R1 removes). Optionally replace it with a
       `TestWantedHooks_NoPreCompletePreToolUse` that asserts `wantedHooks()` has NO
       `PreToolUse` key (the removed-by-omission witness — mirrors the spec-072
       retired-hook precedent: a retired hook is absent from the wanted set and so
       stripped from existing installs).
     - `TestRunClaude_CleansStalePreToolUse` L755–770 — the tail asserts the
       "legitimate pre-complete gate entry should be present after cleanup"
       (L768–769). INVERT: the pre-complete entry is now STALE/retired, so after the
       second run it must be ABSENT (assert `hooksContainCommand(... "mindspec hook
       pre-complete")` is `false`), exactly like the worktree-file / instruct legacy
       entries already asserted gone just above it.
   - **(b) synthetic-fixture assertions (KEEP — they pin the merge machinery against
     a SYNTHETIC entry, not `wantedHooks()`):** the `preCompleteWantedEntry()` /
     `wantedWithPreComplete()` helpers (L264–287) and every test that drives the
     merge/strip machinery through them — `TestMergeWantedEntry_AppendsAlongsideUserEntry`
     (L348), `TestRemoveStaleMindspecEntries_WantedEntrySurvives` (L397),
     `TestEnsureSettings_MergePathInstallsPreCompleteEntry` (L550),
     `TestEnsureSettings_UserPreToolUseEntrySurvives` (L583),
     `TestEnsureSettings_RerunIdempotent` (L641) — pass `wantedWithPreComplete()`
     (a SYNTHETIC wanted set that *injects* a PreToolUse entry) and assert the
     merge/append/self-strip behavior against THAT injected shape. They are
     **machinery tests independent of whether `wantedHooks()` itself ships the
     entry**, so they remain valid and should be KEPT AS-IS. (At edit time, confirm
     none of them call the real `wantedHooks()` for the PreToolUse arm — they use the
     synthetic helper; the only `wantedHooks()`-keyed pre-complete assertions are the
     three class-(a) tests above.)
   READ `claude_test.go` and re-grep `pre-complete` + `PreToolUse` to confirm the
   class-(a)/class-(b) split before editing; the precedent to mirror for the
   inverted assertions is the spec-072 retired-hook handling already in this file
   (the `worktree-file` / `mindspec instruct` legacy entries asserted REMOVED in
   `TestRunClaude_CleansStalePreToolUse` and `TestEnsureSettings_RerunIdempotent`).
   `cmd/mindspec/setup.go` — the L30 doc line `.claude/settings.json with
   SessionStart and PreToolUse hooks` does NOT mention `pre-complete` (it is a
   generic enumeration). SessionStart stays, so only the bare `PreToolUse` mention
   goes stale. This is a **light generic doc-string touch, NOT a `pre-complete`
   removal**: either drop ` and PreToolUse` so the line reads `.claude/settings.json
   with SessionStart hooks`, or leave it — it is non-blocking and earns no
   verification AC. (Decision: trim it to `SessionStart hooks` so the enumeration
   stops advertising a hook event mindspec no longer installs.)
5. `internal/harness/scenario_panel_gate.go` + `scenario_panel_gate_test.go` —
   DELETE both: the `ScenarioPanelGateBlocksPrematureComplete` scenario and its
   `assertPanelGateBlocksIncomplete` probe shell out to the retired `mindspec hook
   pre-complete` subprocess (hard-asserting exit 2 + the incomplete-panel block
   text + the `git merge bead/` fence), assertions that go FALSE once the hook is
   gone; `scenario_panel_gate_test.go` carries the
   `TestLLM_PanelGateBlocksPrematureComplete` wrapper AND a
   `TestAllScenariosRegistersPanelGate` registration check — both are removed with
   the whole-file delete (the registration check would otherwise go stale once
   `scenario.go` drops the registration in step 6).
6. `internal/harness/scenario.go` — remove the
   `ScenarioPanelGateBlocksPrematureComplete(),` registration entry (~L64) so the
   harness no longer references the deleted scenario.
7. `.mindspec/docs/adr/ADR-0037-panel-gate-enforced-contract.md` — add a dated
   **2026-06-17 (spec 102)** amendment note after the existing 2026-06-14
   amendment block (~L83): record that the retirement deferred by the 099
   amendment has landed — the PreToolUse pre-complete hook gate and its heuristic
   command-string matcher are REMOVED; the in-binary `mindspec complete` gate over
   the shared `internal/panel` decision (`PanelGateDecision` / `ResolveGateFacts`
   / `gate.go`) is now the SINGLE authoritative enforcement point; contract
   semantics (thresholds §3, staleness §4, §6 fail-open/closed, §7 hatches)
   unchanged. ALSO touch the now-stale scenario enumeration (~L270) that names
   `panel_gate_blocks_premature_complete` as an LLM-harness scenario, since that
   scenario is deleted. Then run the **Verification** gates until clean/GREEN.

**Changed files**

- `internal/hook/precomplete.go` — DELETED (whole file is the retired matcher +
  gate body + helpers + the `SkipPanelEnv` re-export; nothing else lives there).
- `internal/hook/dispatch.go` — remove the `case "pre-complete":` arm from `Run`.
- `internal/hook/hook.go` — remove `"pre-complete"` from `Names`.
- `internal/hook/precomplete_match_test.go` — DELETED.
- `internal/hook/precomplete_run_test.go` — DELETED.
- `internal/hook/hook_test.go` — update `TestNames_Sorted` (and any other test
  asserting on `Names`) for the removed entry.
- `internal/setup/claude.go` — drop the PreToolUse `mindspec hook pre-complete`
  entry from `wantedHooks()` (removed-by-omission via `removeStaleMindspecEntries`).
- `internal/setup/claude_test.go` — rework the three `wantedHooks()`-keyed
  pre-complete assertions (class-(a) above) so they expect the entry ABSENT after
  the migration: `TestRunClaude_FreshSetup` (L45–49 invert),
  `TestWantedHooks_HasPreCompletePreToolUse` (L223–246 delete, optionally replace
  with a `_NoPreComplete…` witness), `TestRunClaude_CleansStalePreToolUse`
  (L755–770 invert the post-cleanup "present" assertion to "absent"). KEEP the
  synthetic-fixture / merge-machinery tests (class-(b): `preCompleteWantedEntry`,
  `wantedWithPreComplete`, and the five merge/strip tests that drive them) UNCHANGED.
- `cmd/mindspec/setup.go` — light generic doc-string touch ONLY: trim the stale
  ` and PreToolUse` from the L30 `SessionStart and PreToolUse hooks` enumeration
  (it never said `pre-complete`); SessionStart stays. Non-blocking, no AC.
- `internal/harness/scenario_panel_gate.go` — DELETED.
- `internal/harness/scenario_panel_gate_test.go` — DELETED.
- `internal/harness/scenario.go` — remove the
  `ScenarioPanelGateBlocksPrematureComplete()` registration (~L64).
- `.mindspec/docs/adr/ADR-0037-panel-gate-enforced-contract.md` — dated 2026-06-17
  amendment note + stale scenario-name enumeration touch (~L270).

**Verification** (removal bead: RED baseline → GREEN; hermetic — NEVER the slow harness scenarios)

RED baseline (BEFORE removal, expected non-empty / present):
- `grep -rn "matchMindspecComplete\|segmentInvokesComplete\|strippedCompleteFields\|completeBeadID" internal/hook/` → non-empty.
- `grep -n "pre-complete" internal/hook/hook.go` → matches.
- `internal/harness/scenario_panel_gate.go` exists; `grep -rn "ScenarioPanelGateBlocksPrematureComplete" internal/harness/` → matches.

GREEN (AFTER removal):
- [ ] `grep -rn "matchMindspecComplete\|segmentInvokesComplete\|strippedCompleteFields\|completeBeadID" internal/hook/` → no production-code matches (spec L251).
- [ ] `internal/hook/dispatch.go` has no `pre-complete` case; `go build ./...` succeeds (spec L252).
- [ ] `grep -n "pre-complete" internal/hook/hook.go` → no match, AND `go test -run TestNames_Sorted -timeout 120s ./internal/hook/` PASS (spec L253).
- [ ] `internal/harness/scenario_panel_gate.go` + `_test.go` deleted;
  `grep -rn "ScenarioPanelGateBlocksPrematureComplete\|TestLLM_PanelGateBlocksPrematureComplete" internal/harness/` → no match; `go build ./... && go vet ./internal/harness/` clean (spec L254). **⚠️ build/vet ONLY — NEVER `go test ./internal/harness/...`.**
- [ ] Whole-tree grep for the removed symbols
  (`segmentInvokesComplete\|strippedCompleteFields\|completeBeadID\|runPreComplete\|ScenarioPanelGateBlocksPrematureComplete\|scenario_panel_gate`)
  → no LIVE refs remain.
- [ ] `go build ./... && go vet ./internal/hook/ ./internal/setup/ ./internal/harness/` clean (BUILD/VET only).
- [ ] In-binary gate coverage preserved: `go test -run 'TestPanelGate' -timeout 120s ./internal/complete/` → all 8 `TestPanelGate_*` PASS (the deleted LLM scenario's intent is pinned hermetically here; proves retiring the hook didn't touch the authoritative gate) (spec L255).
- [ ] Block parity preserved: `go test -run TestPanelGateDecision -timeout 120s ./internal/panel/` and `go test -run 'TestPanelGate|TestComplete' -timeout 120s ./internal/complete/` PASS unchanged (`internal/complete` never imported `internal/hook`) (spec L256).
- [ ] Setup no longer installs the hook + idempotently removes a pre-existing one: with the class-(a) assertions in `claude_test.go` REWORKED to expect the pre-complete entry ABSENT after setup (fresh-setup absent, `wantedHooks()` has no PreToolUse key, post-cleanup absent), `go test -run 'TestSetup|TestClaude' -timeout 120s ./internal/setup/` PASS (spec L257). The reworked assertions are the GREEN witness that the migration removes the entry — without this rework the suite would invert to RED, since the unmodified tests hard-assert the entry is PRESENT (L47–48, L240, L768–769).
- [ ] ADR-0037 carries the dated retirement amendment: `grep -n "retire" .mindspec/docs/adr/ADR-0037-panel-gate-enforced-contract.md` shows the new note dated 2026-06 (spec L258).

**Acceptance Criteria**

- [ ] R1 (L251): heuristic matcher symbols gone from `internal/hook/` (grep clean).
- [ ] R1 (L252): no `pre-complete` case in `dispatch.go`; `go build ./...` succeeds.
- [ ] R1 (L253): `pre-complete` gone from `hook.go` `Names`; `TestNames_Sorted` PASS.
- [ ] R1 (L254): broken LLM scenario files deleted, no refs remain, build+vet clean.
- [ ] R1 (L255): in-binary block coverage still pinned — `TestPanelGate` (8) PASS in `internal/complete/`.
- [ ] R1 (L256): in-binary block parity — `TestPanelGateDecision` + `TestPanelGate|TestComplete` PASS unchanged.
- [ ] R1 (L257): setup no longer installs the hook + removes a pre-existing one — `claude_test.go` class-(a) assertions reworked to expect the pre-complete entry ABSENT after setup; `TestSetup|TestClaude` PASS (the rework is what keeps this GREEN — the migration's removal-by-omission is asserted, not the old PRESENT expectation).
- [ ] R1 (L258): ADR-0037 carries a dated 2026-06 retirement amendment.
- [ ] Spec-level (L261): `go build ./...` green with R1 applied.
- [ ] Spec-level (L262): no new function in `internal/hook/**` infers a bead ID by string-scanning prose.

**Depends on**

None.

---

## Bead 2 (mindspec-qcjf) — R2: retire the harness `extractMentionedPaths`

**Satisfies spec ACs:** both R2 ACs (spec L259–260) and the spec-level build/no-new-heuristic
ACs (L261–262).

**Steps**

1. Author the RED pin FIRST — a NEW exact-score witness test
   `TestAnalyzer_PlanFidelity_PathTermRetired` (internal/harness/analyzer_test.go,
   add near the existing `_HighScore`/`_LowScore` ~L524/L563) that distinguishes
   path-aware from commands-only scoring (see **RED tests first** below for the
   exact plan+events shape and the computed old=`0.5` / new=`0.0` scores). Confirm
   it FAILS against the pre-change path-aware `PlanFidelity` (returns `0.5`).
   Then rework `TestAnalyzer_PlanFidelity_HighScore` / `_LowScore` (~L524 / ~L563)
   into commands-only regression LOCKS (their `score<0.5`/`score>0.5` thresholds
   pass both old and new — they are guards, NOT the RED pin).
2. In `internal/harness/analyzer.go` `PlanFidelity` (~L537), remove the path term
   AS A UNIT:
   - delete the call site `expectedPaths := extractMentionedPaths(content)` (~L544);
   - delete the `touchedPaths` Write/Edit/Read basename-match loop + the
     `for _, p := range expectedPaths { ... }` matcher (~L554–569);
   - drop the `expectedPaths` references in the empty-expectations short-circuit
     (`if len(expectedPaths)+len(expectedCommands) == 0`, ~L547 → becomes
     `if len(expectedCommands) == 0`) and in the `total` denominator
     (`total := len(expectedPaths) + len(expectedCommands)`, ~L551 → becomes
     `total := len(expectedCommands)`);
   - delete the now-orphaned `extractMentionedPaths` function (~L694–716).
   Result: `PlanFidelity` is commands-only, internally consistent (no dangling
   `expectedPaths`, no orphaned path-match block), still compiles, still returns a
   defined score. `extractMentionedCommands` and the command-match loop are
   UNCHANGED. (If `filepath` becomes unused after removing the basename loop,
   drop its import to keep `go vet`/build clean — confirm at edit time.)
3. Run the filtered gate until GREEN.

**Changed files**

- `internal/harness/analyzer.go` — remove `extractMentionedPaths` + its call site
  (~L544) + the path-match block (~L554–569) + the `expectedPaths` refs in the
  empty-expectations short-circuit (~L547) and the `total` denominator (~L551);
  `PlanFidelity` becomes commands-only.
- `internal/harness/analyzer_test.go` — ADD the RED-pin
  `TestAnalyzer_PlanFidelity_PathTermRetired` (exact-score witness: old `0.5` →
  new `0.0`) and rework `TestAnalyzer_PlanFidelity_HighScore` / `_LowScore` into
  commands-only regression LOCKS.

**RED tests first (then GREEN), filtered — NEVER the slow harness scenarios**

**The RED PIN — `TestAnalyzer_PlanFidelity_PathTermRetired` (NEW, exact-score
witness).** This is the only test that genuinely flips old→new. Construct it so
path-aware and commands-only scoring DISAGREE:
- **Plan:** mentions BOTH a file path AND a known command — e.g. a steps body with
  `1. Edit internal/foo/bar.go` (a path: contains `/` + `.go`) and `2. Run go test
  ./...` (a known command: `extractMentionedCommands` matches the substring `go
  test`). So `extractMentionedPaths(content) = [internal/foo/bar.go]` and
  `extractMentionedCommands(content) = [go test]`. (Keep the rest of the plan body
  free of `mindspec`/`git`/`make`/`go test`/extra path-like tokens so the two
  expectation lists are exactly one element each — verify at edit time.)
- **Events:** TOUCH the mentioned path but run a DIFFERENT, non-mentioned command:
  `{ToolName: "Write", Args: {"file_path": "internal/foo/bar.go"}}` +
  `{Command: "ls"}`. The Write matches the path (`touchedPaths` includes
  `internal/foo/bar.go`); `ls` does NOT match `go test`.
- **Exact scores (computed against the current `PlanFidelity` body, ~L537):**
  - OLD path-aware: `total = len(expectedPaths)+len(expectedCommands) = 1+1 = 2`;
    matched = 1 (path `internal/foo/bar.go` touched) + 0 (no command matched) = 1 →
    **`1/2 = 0.5`**.
  - NEW commands-only: `total = len(expectedCommands) = 1`; matched = 0 (no command
    matched; path term removed) → **`0/1 = 0.0`**.
- **Assertion:** assert the EXACT new score `0.0` (e.g. `if score != 0.0 { t.Errorf(...) }`).
  This FAILS on the pre-change function (which returns `0.5`) — the RED — and PASSES
  after the path-term removal — the GREEN. Pin the empty-expectations guard
  separately is unnecessary here: `expectedCommands = [go test]` is non-empty, so
  the commands-only short-circuit (`if len(expectedCommands) == 0 { return 1.0 }`)
  is NOT taken — the witness exercises the real numerator/denominator.

**Regression LOCKS (kept, NOT the RED pin).** Rework `TestAnalyzer_PlanFidelity_HighScore`
/ `_LowScore` to read as commands-only guards. Their `score<0.5` / `score>0.5`
thresholds pass IDENTICALLY old and new, so they DO NOT pin the change — they only
lock the commands-only contract going forward:
- **High case:** plan mentions `go test` + events run it (`Command: "go"`, arg
  `test`, matching plan's `go test`) → 1/1 = 1.0 ≥ 0.5. (The current High events
  already yield this once paths drop.)
- **Low case:** plan mentions `go test` + events run `ls` and touch only unrelated
  files → 0/1 = 0.0 < 0.5. (The current Low events already yield this once paths
  drop.) Adjust plan text/assertions so neither case depends on the path-match
  term; they remain commands-only regression LOCKS.

**Verification** (GREEN — filtered, NEVER the slow harness scenarios)
- [ ] `grep -rn "extractMentionedPaths\|expectedPaths\|touchedPaths" internal/harness/analyzer.go` → no match (path term gone AS A UNIT; `PlanFidelity` commands-only, internally consistent) (spec L259).
- [ ] RED→GREEN witness: `TestAnalyzer_PlanFidelity_PathTermRetired` asserts exact `0.0` — FAILS pre-change (path-aware returns `0.5`), PASSES post-change (commands-only returns `0.0`). `go test -run TestAnalyzer_PlanFidelity -timeout 120s ./internal/harness/` PASS (filtered — runs the witness + the High/Low LOCKS; `PlanFidelity` compiles + returns a defined score; **NEVER** the slow `TestLLM_*` scenarios) (spec L260).
- [ ] `go build ./...` green (spec L261).
- [ ] `go vet ./internal/harness/` clean.

**Acceptance Criteria**

- [ ] R2 (L259): `extractMentionedPaths` + its call site + the path-match block all removed (grep clean for `extractMentionedPaths\|expectedPaths\|touchedPaths` in analyzer.go); `PlanFidelity` commands-only + internally consistent.
- [ ] R2 (L260): `PlanFidelity` still compiles + returns a defined score, and the path-term removal is pinned by the exact-score RED witness `TestAnalyzer_PlanFidelity_PathTermRetired` (old `0.5` → new `0.0`) — `TestAnalyzer_PlanFidelity` PASS (filtered).
- [ ] Spec-level (L261): `go build ./...` green with R2 applied.
- [ ] Spec-level (L262): no new function in `internal/harness/**` infers a file path by string-scanning prose.

**Depends on**

None. (Advisory: merge AFTER Bead 1 to avoid racing in `internal/harness/**` —
disjoint files, no hunk overlap; see the merge-ORDER note above.)

---

## Provenance

| Acceptance Criterion (spec line) | Verified By |
|---|---|
| R1: heuristic matcher symbols gone from `internal/hook/` (L251) | Bead 1 — grep clean |
| R1: no `pre-complete` case in `dispatch.go`; build succeeds (L252) | Bead 1 — grep + `go build ./...` |
| R1: `pre-complete` gone from `Names`; `TestNames_Sorted` PASS (L253) | Bead 1 — grep + `go test -run TestNames_Sorted ./internal/hook/` |
| R1: broken LLM scenario files deleted, no refs, build+vet clean (L254) | Bead 1 — grep + `go build ./... && go vet ./internal/harness/` |
| R1: in-binary block coverage still pinned — 8 `TestPanelGate_*` (L255) | Bead 1 — `go test -run TestPanelGate ./internal/complete/` |
| R1: in-binary block parity preserved (L256) | Bead 1 — `TestPanelGateDecision` (panel) + `TestPanelGate\|TestComplete` (complete) |
| R1: setup no longer installs hook + removes pre-existing (L257) | Bead 1 — `claude_test.go` class-(a) assertions reworked to expect the entry ABSENT, `go test -run 'TestSetup\|TestClaude' ./internal/setup/` |
| R1: ADR-0037 dated 2026-06 retirement amendment (L258) | Bead 1 — `grep -n "retire" ADR-0037…md` |
| R2: path term gone AS A UNIT (L259) | Bead 2 — grep `extractMentionedPaths\|expectedPaths\|touchedPaths` analyzer.go |
| R2: `PlanFidelity` compiles + returns a defined score (L260) | Bead 2 — exact-score RED witness `TestAnalyzer_PlanFidelity_PathTermRetired` (old `0.5`→new `0.0`) + High/Low LOCKS, `go test -run TestAnalyzer_PlanFidelity ./internal/harness/` (filtered) |
| Spec-level: `go build ./...` green with both changes (L261) | Beads 1 + 2 — `go build ./...` |
| Spec-level: no new heuristic string-scan (L262) | Beads 1 + 2 — removals introduce no new parser |
