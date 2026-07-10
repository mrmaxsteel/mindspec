---
adr_citations:
    - ADR-0037
    - ADR-0035
    - ADR-0040
    - ADR-0023
spec_id: 114-panel-gate-block-on-request-changes
status: Draft
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/panel/gate.go
        - internal/panel/tally.go
        - internal/panel/panel_decision_test.go
        - internal/panel/votedecision_test.go
        - internal/panel/panel_test.go
        - internal/complete/panel_advisory_test.go
        - internal/complete/panel_gate_e2e_test.go
        - internal/instruct/panelstate_test.go
        - cmd/mindspec/panel_test.go
    - depends_on: [1]
      id: 2
      key_file_paths:
        - internal/panel/panel.go
        - internal/panel/tally.go
        - internal/panel/gate.go
        - internal/complete/panel_advisory.go
        - internal/complete/complete.go
        - internal/complete/panel_advisory_test.go
        - internal/complete/panel_gate_e2e_test.go
        - internal/complete/panel_gate_layout_test.go
        - internal/panel/panel_test.go
        - internal/bead/bdcli.go
        - internal/bead/bdcli_test.go
    - depends_on: [2]
      id: 3
      key_file_paths:
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
---
# Plan: 114-panel-gate-block-on-request-changes

Close the last silent out-vote in the authoritative `mindspec complete` panel
gate: an unresolved REQUEST_CHANGES (or an unrecognized verdict string) now
blocks exactly like a REJECT (R1), and the only per-slot escape is an
explicit, audited, gate-validated **refutation** recorded on `panel.json`
whose `panel_refuted` bead-metadata audit is **durable** — a metadata-write
failure on an applied refutation fails the completion instead of being
swallowed (R2/AC11). All line references below are pinned against HEAD
`fae2bfb6`.

## Decomposition and land order

Three beads, serial chain **Bead 1 → Bead 2 → Bead 3** (longest chain 3,
within the ≤3 bound; `work_chunks` declares the edges):

- **Bead 1 (R1)** — the unresolved-non-APPROVE blocking leg in
  `PanelGateDecision` + the `VoteDecision` lockstep twin, plus the repo-wide
  test-fixture reconciliation (the three spec-named outcome flips and the
  outcome-preserving input updates enumerated below). Big test diff, small
  production diff.
- **Bead 2 (R2)** — the `refutations` schema on `panel.json`, the
  gate-validated per-slot clearing, the applied-refutation plumbing out of
  `PanelGateDecision`/`panelGate`, the **durable** `panel_refuted` audit
  written **BEFORE the bead close** (write-before-close, the airtight form —
  see the durable-audit design below), a **fail-closed `bead.MergeMetadata`**
  so a later transient read-failure can never erase the audit, and
  deduplicated applied-refutation selection. Subtle production ordering; this
  bead edits `internal/bead` (execution domain — a plan-time scope refinement
  the plan-approve panel required; see the domain note below).
- **Bead 3 (docs)** — the ADR-0037 amendment and the /ms-panel-tally
  refutation procedure (both skill copies).

**Why Bead 2 depends on Bead 1 (not parallel):** a refutation clears a block
that only exists once R1 lands — AC2's "refutation unblocks" is meaningless
before AC1's "RC blocks" (the spec's own ordering). **Why Bead 3 depends on
Bead 2:** the ADR §1 amendment names the exact recorded schema and the §7
text names the exact `panel_refuted` audit keys and the
durable-vs-best-effort distinction; the AC6 grep proof should hit names that
exist in code.

**R_scope note (deliberate overlap, justified):** Beads 1 and 2 both touch
`internal/panel/gate.go`, `internal/panel/tally.go`, and `internal/complete`
tests. This overlap is real but SERIAL, not parallel: Bead 2 is implemented
on top of Bead 1's merged state on the spec branch, so there is no
cross-worktree conflict risk. They are kept separate anyway because (a) the
AC5 diff-review proof ("only the named fixtures flipped outcome") is only
tractable against Bead 1's isolated diff, and (b) the durable-audit reorder
in Bead 2 is the subtlest change in the spec and deserves an isolated panel
review. Merging them into one bead was considered and rejected for exactly
those two review-surface reasons; splitting Bead 1 by package was rejected
because the `VoteDecision` lockstep (R1's own falsifier) must land
atomically with the gate leg.

**Impacted-domains refinement (plan-approve panel, G2).** The approved
spec's Impacted Domains scoped `internal/bead` read-only ("no change lands
there"). The plan-approve panel's durable-audit airtightness review found
that `bead.MergeMetadata` (internal/bead/bdcli.go:263-290) must be made
**fail-closed** (item 2 below) to keep the audit un-eraseable, so Bead 2 now
edits `internal/bead` — an **execution-domain** file. This is a genuine
plan-time scope addition: ADR-0037 (cited; Domain(s) workflow, execution)
already covers execution, so no new ADR is needed, but the Bead 2 brief must
declare the execution domain in its OWNERSHIP attribution (the bead touches
both workflow — `internal/panel`, `internal/complete` — and execution —
`internal/bead`) so the complete-time ADR-divergence gate does not
false-block it.

**Reviewer/fixer scratch discipline (inherit into every bead brief and
reviewer prompt):** reviewers and fixers MUST use ABSOLUTE `/tmp` scratch
paths (or `t.TempDir()` inside Go tests) for any file they create, and must
NEVER write relative `.mindspec/` (or any relative repo) paths — the agent
harness resets cwd between bash calls, and a relative write from a reviewer
has previously corrupted SIBLING worktrees, which `mindspec complete` then
auto-committed past review. Reviewer verdict paths must be ABSOLUTE. Verify
the bead worktree is CLEAN (`git status --porcelain` empty) before every
`mindspec complete`.

**Toolchain note:** run the CI-matched `gofmt` (go.mod pins go 1.23.0 —
Go 1.19+ gofmt reformats doc comments). In doc comments and skill prose,
avoid backtick code spans containing shell-escape sequences (single-quote
escapes and the like).

## Bead 1: R1 — unresolved REQUEST_CHANGES / unrecognized verdict blocks the gate (internal/panel + fixture reconciliation)

**Scope**

The decision change lives entirely in `internal/panel` (`gate.go`,
`tally.go`); every other surface inherits it by construction:
`internal/complete`'s authoritative `panelGate`
(`internal/complete/panel_advisory.go:140-215`, invoked at complete.Run step
2.25, `internal/complete/complete.go:337-378`), the read-only
`mindspec panel verify`/`panel tally` verbs (`cmd/mindspec/panel.go:7-10`,
pinned by `TestPanelVerbs_DecisionIsPanelGateDecision`,
`cmd/mindspec/panel_test.go:488-581`), and `mindspec instruct --panel-state`
(`internal/instruct/panelstate.go:121`) all delegate to
`panel.PanelGateDecision`; complete's advisory line renders
`Result.VoteDecision` (`internal/complete/panel_advisory.go:65-75`). Zero
production diff outside `internal/panel` in this bead.

**Steps**

1. **`internal/panel/tally.go` — the single home of unresolved-verdict
   resolution.** Add a method `func (r *Result) UnresolvedVerdicts()
   []Verdict` returning the latest-round verdicts whose canonical `Verdict`
   is neither `VerdictApprove` nor `VerdictReject` (tally.go:18-22) — i.e.
   REQUEST_CHANGES plus anything unrecognized, exactly the "neither" set the
   `Result` doc names (tally.go:107-110). REJECTs are deliberately excluded:
   they are leg (9)'s business (gate.go:243-253) and must never route
   through the (later, Bead-2-refutable) unresolved leg even if leg ordering
   were ever edited. `Verdicts` is already slot-sorted (tally.go:306), so
   the returned slice — and every message built from it — is deterministic.
   The tally counting switch (tally.go:296-301) is untouched: `Approves`
   still counts only canonical APPROVE (load-bearing for AC9 later).
2. **`internal/panel/gate.go` — the new leg (9.5).** In `PanelGateDecision`,
   AFTER leg (9) REJECT/hard_block (gate.go:243-253) and BEFORE the leg (10)
   threshold Allow (gate.go:255-262) — O1's confirmed ordering — insert: if
   `len(f.Res.UnresolvedVerdicts()) > 0`, Block with EXACTLY this message
   shape (the substrings are load-bearing, see step 4):
   `"panel %s round %d: unresolved REQUEST_CHANGES / non-APPROVE verdict(s) from %s — %d/%d APPROVE (threshold is %d/%d); every latest-round verdict must be APPROVE. Run /ms-bead-fix with %s, then re-panel (/ms-panel-run step 0)%s"`,
   interpolating slug, round, the comma-joined unresolved slot ids,
   `f.Res.Approves`, `n`, `p.ApproveThreshold()`, `n`,
   `ConsolidatedName(round)`, and `RawMergeFence(f.BeadID)` as the SUFFIX.
   Message constraints (AC10 + compatibility): (i) names each unresolved
   slot id; (ii) contains NO `refut` substring anywhere (so no
   "refutations", no "panel refute" — and note "unresolved", not
   "unrefuted", precisely for this reason) and never `MINDSPEC_SKIP_PANEL`
   (HC-7); (iii) the raw-merge fence is the message suffix because
   `sanitizeNonBeadDecision` (cmd/mindspec/panel.go) strips
   `RawMergeFence("")` by `strings.HasSuffix` for non-bead panels; (iv) the
   `/ms-bead-fix` + `/ms-panel-run step 0` advice is the OQ4 primary
   recovery (fix-and-re-panel), matching the existing leg-10 phrasing.
   Update the short-circuit-order comment block (gate.go:110-127) to list
   (9.5). Legs (0)-(9) and (10) stay byte-preserved.
3. **`internal/panel/tally.go` — `VoteDecision` lockstep.** In
   `Result.VoteDecision` (tally.go:180-214), between the REJECT/hard_block
   leg (tally.go:206-208) and the threshold legs (tally.go:209-213), add the
   same check returning
   `VoteBlock, "round %d: unresolved non-APPROVE verdict(s) from %s — %d/%d APPROVE, threshold is %d/%d"`.
   Same helper, same ordering — the vote-only twin can never disagree with
   the gate on the vote portion (R1 falsifier).
4. **Message compatibility (this is why AC5's blast radius stays at the
   named set).** The leg-9.5 message deliberately carries a substring
   SUPERSET of the leg-10 Block message ("X/N APPROVE", "threshold is T/N",
   `consolidated-round-N.md`, the fence): every pre-existing sub-threshold
   Block fixture whose panel carries RC/neutral filler now routes through
   leg 9.5 instead of leg 10 but keeps its `mustHave` strings green
   byte-unmodified — pinned sites: `internal/panel/panel_decision_test.go`
   ~196-209 ("sub-threshold ... via neutral": "4/6 APPROVE", "threshold is
   5/6", "consolidated-round-1.md") and ~221-234 ("1/3 APPROVE", "threshold
   is 2/3", mustNotHave "/6"); `internal/panel/votedecision_test.go` ~72-79
   (summary "threshold is 5/6"); `internal/instruct/panelstate_test.go`
   ~113-123 (`below_threshold` wantReason "threshold is 5/6"). **Leg (10)
   stays reachable and is NOT removed** — its Block message still fires,
   with zero refutations, whenever the present verdicts contain NO
   unresolved non-APPROVE yet the genuine-APPROVE count is below the floor:
   the canonical case is the `threshold > 0` guard when
   `ApproveThreshold()` resolves to 0 (a 1-reviewer panel with 1 APPROVE,
   pinned by `TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision`'s
   resolved-threshold-0 sub-case, panel_test.go:260-277 — that fixture has
   NO RC filler, so leg 9.5 does not consume it and it must keep reaching
   leg 10 and Block). The leg keeps its `Approves >= threshold` floor as a
   NECESSARY condition; R1 only adds a second necessary condition (no
   unresolved non-APPROVE) ahead of it.
5. **The AC5 carve-out — FOUR intended outcome flips (R1's whole point).**
   Bead 1's panel gates on exactly this four-fixture set flipping and NO
   other outcome changing. The four: (a)
   `internal/panel/panel_decision_test.go:184-194` "threshold met: 6/6
   present, 5 APPROVE + 1 dissent → Allow" → expect **Block**, mustHave the
   slot id "z" + the fence, mustNotHave any `refut` substring (feeds the
   AC10 predicate); (b) `internal/complete/panel_advisory_test.go:69-81`
   `TestPanelAdvisory_Passing_WouldPass` (2 APPROVE + 1 REQUEST_CHANGES,
   N=3) → expect **"would BLOCK"** (rename to
   `TestPanelAdvisory_Dissent_WouldBlock`; add an all-APPROVE companion
   pinning that "would PASS" still exists); (c)
   `cmd/mindspec/panel_test.go:386-389` "passing (at-threshold) → exit 0"
   (buildResult 5 approve / 6 total = 1 RC filler) → `wantErr: true` with a
   final recovery line; (d) the **VoteDecision lockstep twin** the spec's R1
   falsifier mandates — `internal/panel/votedecision_test.go:64-70`
   "threshold met 5/6 with 6 present → Pass" becomes a 5A+1RC →
   **VoteBlock** row, with a new all-APPROVE 6/6 → VotePass companion row.
   These four are the ONLY expected-outcome changes; every other RC/empty
   filler fixture is an outcome-PRESERVING input update (step 6).
6. **Outcome-PRESERVING input updates (dissent filler → genuine APPROVE;
   these do NOT flip any expected outcome and are named here for the AC5
   diff review).** These pre-existing fixtures used a REQUEST_CHANGES (or
   empty-string) filler verdict incidentally — their SUBJECT is not RC
   tolerance — and must have inputs updated so their pinned outcome
   survives: (a) `internal/panel/panel_test.go:224`
   `TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision` — replace the
   5A+1RC fixture (~230-243, Allow precondition) with 6/6 all-APPROVE (the
   resolved-threshold-0 sub-case at :260-277 has NO RC and is untouched —
   it must keep reaching leg 10; step 3 of Bead 2 also converts its
   `got != want` struct comparison to an Action+Message compare when
   `Decision` gains its slice field); (b) `internal/panel/panel_test.go:304`
   `TestPanel_GateFieldDecisionInert` — same replacement (~316-325 asserts
   Allow); (c) `internal/panel/panel_test.go:393`
   `TestPanel_GateFieldDecisionInertAllEnumValues` — its SHARED `buildFacts`
   closure (panel_test.go:417-428, which appends a `VerdictRequestChanges`
   filler) must feed genuine APPROVEs so the "allow" scenario stays Allow
   for every gate enum value; (d)
   `internal/complete/panel_gate_e2e_test.go:76-84` `approveVerdicts(n)` —
   change from "n-1 APPROVE + 1 REQUEST_CHANGES (the tolerated dissent)" to
   n APPROVEs, updating its doc comment; `fresh_passes` (:200) keeps
   passing and `stale_blocks` (:218) keeps blocking on staleness (leg 6
   precedes 9.5; its "commits landed after review" message is unchanged);
   (e) `internal/instruct/panelstate_test.go:54-79` `beadPanel` —
   synthesize REAL verdict strings (`panel.VerdictApprove` for the approve
   count, `panel.VerdictRequestChanges` for `otherPresent`) instead of
   today's all-empty-string verdicts (empty = unrecognized = blocking under
   R1); the `at_threshold_fresh` row (~91-99) and
   `TestRenderPanelState_Pass` (~267-281) are restructured to at-threshold
   WITHOUT a dissent — recorded `ApproveThresholdExpr: "6"` (the spec-109
   override, internal/panel/panel.go:113-127) with 6/6 APPROVE — preserving
   their at-threshold-boundary (approves == threshold) subject and PASS
   outcome with zero dissent. **Three MORE fixtures the plan-approve panel
   flagged (all four reviewers / G3 — the first "exhaustive" list missed
   them; classified here as outcome-preserving):** (f)
   `internal/panel/panel_decision_test.go:211-219` "expected_reviewers:3 →
   2 APPROVE + 1 RC → Allow, no hardcoded 6" — leg 9.5 would flip it to
   Block, so make it **3/3 all-APPROVE** (drop the RC filler), preserving
   the "no hardcoded 6" subject (its sibling Block row at :221-234 with
   `mustNotHave: ["5/6","/6"]` is genuinely sub-threshold and keeps its
   substring assertions — leg 9.5's superset message still contains "1/3
   APPROVE"/"threshold is 2/3" and no "/6", so it stays green); (g)
   `cmd/mindspec/panel_test.go:557-558` `TestPanelVerbs_DecisionIsPanelGateDecision`'s
   "at-threshold" row (`buildResult(basePanel(), 5, 0, 6, 1, nil)` = 5A+1RC)
   — this is a decision-PARITY row (asserts the CLI adapter equals the live
   `PanelGateDecision`, so it stays green either way), but its NAME now lies;
   update it to a genuine at-threshold (recorded `approve_threshold: "6"`,
   6/6 APPROVE) so "at-threshold" keeps meaning Allow, and keep the sibling
   "sub-threshold" row (:553-554) which stays a genuine Block; (h)
   `internal/instruct/panelstate_test.go:903-905` "at_threshold" row
   (`beadPanel(..., 5, 0, 0, 1)` = 5A+1RC) — with `beadPanel` now
   synthesizing real verdicts (step 6e), this flips PASS→BLOCK, so apply the
   same recorded-`approve_threshold: "6"` + 6/6-APPROVE update, preserving
   its at-threshold PASS subject; its sibling `sub_threshold` row
   (:899-901) stays a genuine Block and keeps its assertions.
7. **New tests (AC1, AC8 block-half, AC10 predicate) + the AC5 diff-review
   proof.** In `internal/panel/panel_decision_test.go`, add table-driven
   `TestPanelGateDecision_UnresolvedRequestChangesBlocks`: (i) 6 expected,
   complete latest-round, fresh SHA, clean tree, 5 APPROVE + 1
   REQUEST_CHANGES → Block, message names the RC slot (AC1); (ii) 5 APPROVE
   + 1 unrecognized verdict (e.g. "MAYBE") → Block (AC8 first half); (iii)
   two RC slots → Block naming both; (iv) the AC10 fixed-string predicate on
   every row: message contains no `refute`, no `refutations`, no
   `panel refute` substring, never `MINDSPEC_SKIP_PANEL`, and carries the
   fence — mirroring `TestPanelGate_BlockNeverNamesSkipVar`
   (`internal/complete/panel_gate_e2e_test.go:290-304`). In
   `internal/complete/panel_gate_e2e_test.go`, add
   `TestPanelGate_RequestChangesBlocksComplete` (AC1 end-to-end, alongside
   the existing `TestPanelGate_*` suite on the real-repo harness,
   `setupPanelGateRepo`): 5A+1RC fresh panel → `complete.Run` returns the
   guard failure, `ex.completeCalled == false` (mutated nothing),
   `guard.HasFinalRecoveryLine` true (ADR-0035, the recovery line appended
   at panel_advisory.go:201-203), and the AC10 predicate re-asserted on the
   full error text. Finally the AC5 proof: run the full four-package suite
   and a `git diff` review asserting that the ONLY tests whose expected
   outcome changed are the step-5 four-flip set, and every step-6 change
   (a-h) preserves its outcome. Record the complete touched-fixture file
   list — the four flips AND all eight outcome-preserving updates (a)-(h),
   spanning panel_decision_test.go, panel_test.go, votedecision_test.go,
   panel_advisory_test.go, panel_gate_e2e_test.go, panelstate_test.go, and
   cmd/mindspec/panel_test.go — in the bead's completion notes for the
   panel.

**Verification**

- [ ] `go test ./internal/panel -run 'PanelGateDecision'` — AC1 decision
  table green, including the 5A+1RC Block, the unrecognized-verdict Block
  (AC8 block-half), and the AC10 no-advertise predicate rows.
- [ ] `go test ./internal/complete -run 'PanelGate'` — AC1 end-to-end Block
  (`TestPanelGate_RequestChangesBlocksComplete`: non-zero, nothing mutated,
  recovery line) plus the whole pre-existing `TestPanelGate_*` suite
  (fail-open, hatches, staleness, HC-7, shared-decision pin) green.
- [ ] `go build ./... && go test ./internal/panel ./internal/complete ./internal/instruct ./cmd/mindspec` — full AC5 sweep green.
- [ ] Diff review (AC5 proof): `git diff` over `*_test.go` confirms outcome
  flips ONLY in the step-5 four-fixture set (panel_decision_test "5 APPROVE
  + 1 dissent", panel_advisory_test `WouldPass`, cmd panel_test
  "at-threshold exit 0", and the votedecision lockstep-twin row); every
  other touched fixture (step-6 (a)-(h), including the three panel-flagged
  additions) preserves its expected outcome.
- [ ] `gofmt -l ./cmd ./internal` prints nothing (CI-matched go 1.23 gofmt).

**Acceptance Criteria**

- 5 APPROVE + 1 unresolved REQUEST_CHANGES (complete, fresh, clean) yields
  Block from `PanelGateDecision`, "would BLOCK" from `VoteDecision`, a
  non-zero `mindspec complete` with nothing mutated, and a Block message
  that names the slot, ends with a recovery line, carries the fence, and
  contains no refutation incantation and no skip variable (AC1, AC10, R1).
- An unrecognized verdict string blocks identically (AC8 block-half).
- Every other gate behavior — skip-env Warn, abandoned Warn, stale-SHA
  Block, incomplete Block, round-mismatch Block, dirty-tree Block,
  missing-ref/transient-gitErr Warns, fail-open — passes its existing tests
  with no semantic modification; only the named fixtures flip outcome (AC5).

**Depends on**
None (first bead).

## Bead 2: R2 — audited refutation escape + DURABLE `panel_refuted` audit (schema, gate validation, write-before-close, fail-closed MergeMetadata)

**Scope**

`internal/panel/panel.go` (schema), `internal/panel/tally.go` +
`internal/panel/gate.go` (gate validation + applied-refutation plumbing),
`internal/complete/panel_advisory.go` + `internal/complete/complete.go` (the
durable audit write, placed BEFORE the bead close, and the dedup), and
`internal/bead/bdcli.go` (making `bead.MergeMetadata` FAIL-CLOSED — the
execution-domain edit the plan-approve panel required; see the domain note in
Decomposition). The write goes through the existing `completeMergeMetadataFn`
seam (`internal/complete/complete.go:42`) → `bead.MergeMetadata`
(`internal/bead/bdcli.go:263-290`, which JSON-marshals the merged map, so a
nested entries array round-trips). No new CLI verb (OQ3: hand-edit
`panel.json`; the gate carries the whole validation burden).

**Steps**

1. **Schema (`internal/panel/panel.go`).** Add a `Refutation` struct with
   exactly the four R2 fields — `Slot string` (json `slot`), `Round int`
   (json `round`), `Reason string` (json `reason`), `Evidence string` (json
   `evidence`) — and `Refutations []Refutation` with
   `json:"refutations,omitempty"` on `Panel` (panel.go:51-92), documented
   parse-lenient EXACTLY like `AbandonReason` (panel.go:59-68): an absent
   array, an empty array, or entries with missing/empty fields are consumer
   concerns, never a parse error — enforcement lives in the gate. Note the
   one asymmetry in the doc comment: a TYPE-mismatched entry (e.g. a string
   `round`) fails `json.Unmarshal` of the whole file → `Registration.Err` →
   leg (2) Block (gate.go:149-153) — the fail-CLOSED direction, which is
   the safe one (the OQ2/R2 requirement is only that lenience never tips
   fail-OPEN). Every pre-existing `panel.json` omits the field →
   byte-identical behavior (the §1 precedent of 109's `approve_threshold`
   and 112's `gate`).
2. **Gate validation, single-homed (`internal/panel/tally.go`).** Extend the
   Bead-1 resolution home: `UnresolvedVerdicts()` now treats a latest-round
   verdict as RESOLVED iff its `Verdict == VerdictRequestChanges` AND some
   `Refutations` entry has `Slot` byte-equal to the verdict's
   filename-derived `Slot` and `Round == r.LatestRound` — and add
   `func (r *Result) AppliedRefutations() []Refutation` returning exactly
   the entries that matched a latest-round REQUEST_CHANGES verdict this way,
   **deterministically deduplicated by (slot, round)** so two `refutations`
   entries naming the same slot+round yield exactly ONE applied record
   (first-wins in the panel's recorded array order, which is stable; the
   returned slice is then slot-sorted — the same determinism the AC12 note
   requires, now extended into the audit path per review item 3). Entries
   matching nothing — stale round, unknown slot,
   REJECT/hard_block/unrecognized target — are NEVER returned. This encodes
   R2(c)/(d) by construction: a
   refutation clears only canonical RC (REJECT is excluded from the
   unresolved set at the source, and leg (9) fires first regardless;
   unrecognized verdicts are not `VerdictRequestChanges` so stay blocking);
   round-binding means a round-N entry never clears a round-N+1 re-RC (R3c,
   AC4b); exact one-slot+round scope (no wildcard, no whole-panel form).
   `Approves` is never touched — a refuted RC is removed from the BLOCKING
   set only, never re-counted (AC9; the tally switch tally.go:296-301 stays
   byte-identical) — so leg (10)'s floor is computed over genuine approvals
   and AC7's sub-threshold-after-refutation Block falls out of the existing
   leg-10 code with zero new logic.
3. **Plumb applied refutations OUT of the decision
   (`internal/panel/gate.go`).** Add `AppliedRefutations []Refutation` to
   `Decision` (gate.go:48-51), populated ONLY on the leg-(10) threshold
   Allow return — the only path where a refutation can have changed the
   outcome (every applied refutation is outcome-relevant by construction:
   without it, its RC would have blocked at 9.5; Warn/short-circuit paths
   never evaluate refutations and return none). `VoteDecision` moves in
   lockstep via the same `UnresolvedVerdicts()`. Mechanical consequence:
   `Decision` gains a slice field and stops being Go-comparable — update
   the ONE struct-equality comparison in the repo,
   `internal/panel/panel_test.go` ~254 (`got != want` in
   `TestPanelGateDecision_ConfigDefaultDoesNotAlterDecision`), to compare
   `Action`+`Message` fields (the AC5-permitted "mechanical fixture
   addition where a struct gains fields"; every other caller already
   compares fields — verified by grep).
4. **`panelGate` returns the applied set
   (`internal/complete/panel_advisory.go:140-215`).** Change the signature
   to `(*panel.Registration, []panel.Refutation, error)`; accumulate
   `d.AppliedRefutations` across evaluated matched panels, **deduplicated by
   (slot, round) across all matched panels** (item 3 — a slot+round cleared
   in two matched panels audits once). The skip-env and config-disabled
   paths (panel_advisory.go:169-179) return nil applied — the gate did not
   evaluate facts or rely on any refutation there. NOTE (review item 7): of
   these two, ONLY the env-skip path has its own audit
   (`panel_gate_skipped`, written by `writePanelAuditMetadata` guarded on
   `panelSkipEnvFn()`, panel_advisory.go:322-330); the config-disabled
   (`enforcement.panel_gate: false`) path writes NO audit at all — do not
   claim it does. The signature change ripples to every `panelGate(...)`
   caller: the one production site complete.Run (complete.go:375) and the
   13 test call sites across `internal/complete/panel_gate_e2e_test.go`
   (:272,:282,:297,:320), `internal/complete/panel_advisory_test.go`
   (:152,:226,:240,:289,:300,:314), and
   `internal/complete/panel_gate_layout_test.go` (:152,:162) — all
   mechanical `reg, err :=` → `reg, _, err :=` updates (AC5-permitted
   struct/return additions; no outcome changes).
5. **The durable audit write — WRITE-BEFORE-CLOSE (`complete.go`, a new
   step at 3.75; carry-forward #1 / review item 1, the airtight form).** Add
   `writePanelRefutedMetadata(beadID string, applied []panel.Refutation)
   error` in panel_advisory.go: builds `panel_refuted: true`,
   `panel_refuted_at: <RFC3339 UTC>`, and
   `panel_refuted_entries: [{slot, round, reason, evidence}, ...]` (keys
   parallel to `panel_abandoned`, panel_advisory.go:332-341) and RETURNS the
   `completeMergeMetadataFn` error — non-swallowing, unlike
   `writePanelAuditMetadata` (panel_advisory.go:304-342) which stays
   best-effort for abandon/skip byte-unchanged. Call it in `Run` at a new
   **step 3.75**: AFTER the last blocking gate (the ADR-divergence decision,
   complete.go:542-544) and BEFORE the step-4 bead close
   (complete.go:546-547), guarded on a non-empty `applied` set (the set
   `panelGate` returned at step 2.25). On error, return a `guard.NewFailure`
   whose body states that the gate passed by relying on recorded
   refutation(s) but the durable `panel_refuted` audit could not be written,
   that nothing was closed or merged, and that the state is recoverable
   (re-run `mindspec complete <bead>` — the panel is still present, so
   `panelGate` re-derives the same applied set and retries the write), with
   recovery line `mindspec complete <bead>`.
   **Why write-BEFORE-CLOSE, not before-merge (the TOCTOU fix — this is the
   load-bearing correction the plan-approve panel required):** the earlier
   write-before-MERGE design wrote `panel_refuted` after the step-4 close.
   If that write failed the bead was ALREADY CLOSED; on retry, if the
   `panel.json` had been removed/lost, `panelGate` FAIL-OPENS (ADR-0037 §6,
   no panel → silent pass) and the already-closed bead merges with NO
   `panel_refuted` — a silent, un-audited, gate-cleared completion, exactly
   what AC11 forbids. The complete.go:695-708 convergence contract
   guarantees merge-RETRY, not audit-obligation preservation, so it does not
   cover this hole. Writing BEFORE the close makes the invariant
   **bead-closed ⟹ audit-was-written-this-run**, hence
   **merged ⟹ closed ⟹ audited**: the bead can never reach the closed (or
   merged) state without the audit already durably on metadata. Placing it
   after the step-3.5 gates (not earlier) still avoids a spurious audit for
   a completion that fails doc-sync/ADR; the only mutation ahead of it is
   the step-2.5 `CommitAll` of the bead's own work, which a normal re-run
   already tolerates idempotently (a clean retry re-measures the same fresh
   tip at step 2.25). The step-5.6 `writePanelAuditMetadata` call
   (complete.go:778-785) is byte-unchanged and does NOT write
   `panel_refuted`. **The durable-vs-best-effort asymmetry is grounded
   SOLELY in AppliedRefutations (review item 5):** a refutation CHANGES the
   gate outcome and therefore produces a non-empty `AppliedRefutations`
   set — abandon and env-skip both Warn→Allow→MERGE too (they are NOT
   "terminal non-merge exits"), but they produce NO `AppliedRefutations`, so
   they carry no un-eraseable audit obligation and stay best-effort.
6. **Fail-closed `bead.MergeMetadata` (`internal/bead/bdcli.go:263-290`;
   carry-forward #1 / review item 2 — closes the post-write erasure hole).**
   Today `MergeMetadata` IGNORES a metadata-READ failure: on a `bd show`
   error it proceeds from an EMPTY `merged` map and replace-writes, ERASING
   every existing key — so a transient read failure during ANY later
   best-effort write (the step-5.5 doc-skew / adr-override / adr-supersede
   writes, or the step-6.5 epic-phase sync, all via the same helper) could
   wipe a just-written `panel_refuted`, yielding a successful completion with
   the audit LOST. FIX: make `MergeMetadata` **fail-closed** — when the
   existing-metadata read (`tracedOutput("show", ...)`) errors OR its JSON
   fails to unmarshal, RETURN that error instead of proceeding from empty (a
   genuinely absent metadata map — a clean read that yields no items — still
   merges from empty, unchanged; only a READ/PARSE FAILURE now aborts). This
   is strictly safer for every caller: the fatal one (our step-3.75
   `panel_refuted` write) surfaces the failure, and the best-effort ones
   warn-and-continue WITHOUT erasing. Blast-radius note: every existing
   `MergeMetadata` caller currently proceeds-from-empty on read error; after
   this change they error on read failure instead — all of them are already
   best-effort (warn) except our new fatal write, so no other behavior
   regresses (a failed read was already a latent corruption, now it is a
   visible no-op). This is the `internal/bead` (execution-domain) edit; the
   Bead 2 brief declares that domain in its OWNERSHIP attribution.
7. **Tests. (A) Refutation semantics (`internal/panel`).** New table-driven
   `TestPanelGateDecision_Refutations` (+ `TestVoteDecision_Refutations`
   lockstep rows and `TestResult_AppliedRefutations` unit rows), all
   synthetic `Result` fixtures (pure decision layer — no real files, so the
   AC12 duplicate-casing rows cannot be flaky on a case-insensitive
   filesystem): (i) **AC2 gate-half**: 5A+1RC plus an entry naming that
   slot at the latest round → Allow, `AppliedRefutations` == that entry;
   (ii) **AC3**: an entry targeting a REJECT slot or hard_block slot → leg
   9 Block unchanged and the applied set empty; refuting slot A while slot
   B holds an RC → Block naming B; (iii) **AC4**: (a) a round-2 all-APPROVE
   re-panel with a round-1 RC on file → Allow with zero refutations (the
   tally already reads only the filename-derived latest round,
   tally.go:273-276 — pinned, zero-ceremony R3b) and (b) an entry with
   round N does not clear the same slot's round-N+1 RC → Block; (iv)
   **AC7**: expected 8 / threshold 7 (default N−1), 6A+2RC both refuted →
   Block via leg (10) with the threshold message ("6/8 APPROVE", "threshold
   is 7/8") and NOT the unresolved-slot message — refutation cannot buy
   past the floor; (v) **AC8 non-refutable-half**: 5A+1 unrecognized
   verdict + an entry naming that slot → still Block; (vi) **AC9**:
   `Result.Approves` is identical before/after the decision and equals the
   genuine-APPROVE count in every refutation row; (vii) **AC12** (the O1
   test-note, recorded): duplicate slot files at the latest round are
   impossible byte-identically (one directory, one filename) but
   near-duplicates differing by case yield DISTINCT filename-derived slots
   ("x" vs "X", tally.go:43-59); a `Dup`-named subtest
   (`TestPanelGateDecision_Refutations/DuplicateSlot...`, so `-run 'Dup'`
   selects it) pins that matching is byte-exact — a refutation naming slot
   "x" clears only slot "x"'s RC, never a REJECT tallied under slot "X"
   (leg 9 still fires), with both "x" and "X" carrying RCs refuting "x"
   leaves the gate blocked naming "X", AND two refutation entries naming the
   SAME slot+round collapse to ONE `AppliedRefutations` record (item-3
   dedup); (viii) the AC10 predicate re-asserted on the Block message of a
   panel that HAS a refutations array (the tempting case): still no `refut`
   substring.
   **(B) Durable audit + write-before-close + fail-closed
   (`internal/complete` + `internal/bead`).** (i) **AC2 audit-half**:
   `TestWritePanelRefutedMetadata` beside
   `TestWritePanelAuditMetadata_Abandoned` (panel_advisory_test.go:111-134):
   captures the merged map — asserts `panel_refuted == true`, an RFC3339
   `panel_refuted_at`, and the applied slot/round/reason/evidence entries —
   and asserts the ERROR RETURN propagates (non-swallowing); plus e2e on the
   `setupPanelGateRepo` harness: a 5A+1RC panel with a matching
   `refutations` entry → `Run` succeeds, `ex.completeCalled == true`, and
   the captured metadata carries the audit. (ii) **AC11 write-fails →
   not-closed → merge never runs**:
   `TestPanelGate_RefutedAuditWriteFailure_FailsCompletion` — stub
   `completeMergeMetadataFn` to fail ONLY on maps containing `panel_refuted`
   (other writes succeed); same fixture → `Run` returns non-zero,
   `ex.completeCalled == false`, AND `closeBeadFn` was NOT called (the bead
   is not closed — the write-before-close guarantee). (iii) **AC11 TOCTOU
   retry (item 1)**: after that failed run, remove the `panel.json`
   (simulating a lost/removed panel) and re-run `Run`; assert the completion
   does NOT merge an un-audited already-closed bead — because the first run
   never closed the bead, the retry is a plain panel-less fail-open of a
   still-`in_progress` bead, and NO `panel_refuted`-relying merge occurs
   (there is no applied refutation to lose). (iv) **Post-write erasure
   survival (item 2)**: a successful refutation completion in which a LATER
   best-effort write (e.g. the step-5.5 doc-skew write, driven via
   `AllowDocSkew`) hits a `MergeMetadata` read failure → assert
   `panel_refuted` SURVIVES on the captured metadata (fail-closed
   `MergeMetadata` no-ops the erasing write instead of replacing from
   empty). (v) **`internal/bead` unit**: `TestMergeMetadata_FailClosedOnReadError`
   in `bdcli_test.go` — inject a `show` read failure and assert
   `MergeMetadata` returns an error and performs NO replace-write (existing
   keys preserved), while a clean empty read still merges. (vi)
   **Already-closed recovery path (item 8)**: qualify — the recovery branch
   (complete.go:547-554) does NOT force the dolt-commit + committed-state
   verify the normal close path (complete.go:555-650) runs, so the
   durability rationale must NOT lean on it. Because step 3.75 writes
   `panel_refuted` BEFORE the close, a recovery re-run (bead already closed,
   panel still present) still re-derives the applied set and re-writes the
   audit at step 3.75 before reaching the recovery branch; a test drives a
   pre-closed bead with a live refutation panel and asserts the audit is
   present after completion. (vii) **Asymmetry control**: an ABANDONED panel
   with the same `panel_refuted`-failing stub still completes successfully
   with only a warning (abandon produces no `AppliedRefutations`, so no
   durable obligation). (viii) **skip-env-with-refutations e2e (item 10)**:
   a `MINDSPEC_SKIP_PANEL` complete over a panel that carries a
   `refutations` array → the gate is skipped, `AppliedRefutations` is empty,
   completion succeeds, and NO `panel_refuted` is written (only
   `panel_gate_skipped`). (ix) An applied-refutations-empty completion
   (plain all-APPROVE pass) writes NO `panel_refuted` key — only genuine
   clears are recorded (the "unused entry never audited" falsifier).

**Verification**

- [ ] `go test ./internal/panel -run 'Refut|Round|Dup|PanelGateDecision'` —
  AC2(gate)/AC3/AC4/AC7/AC8/AC9/AC12 decision rows + the (slot,round) dedup
  row green.
- [ ] `go test ./internal/complete -run 'PanelRefuted|PanelGate'` —
  AC2(audit) write content, AC11 write-failure fence (non-zero, bead NOT
  closed, merge never ran), the TOCTOU retry (panel removed → no un-audited
  merge), the post-write erasure-survival test, the already-closed recovery
  audit test, the skip-env-with-refutations (no `panel_refuted`) row, the
  abandon best-effort asymmetry control, and the whole pre-existing
  `TestPanelGate_*` suite green.
- [ ] `go test ./internal/bead -run 'MergeMetadata'` — the fail-closed
  read-error test green (returns error, no replace-write).
- [ ] `go build ./... && go test ./internal/panel ./internal/complete ./internal/instruct ./cmd/mindspec ./internal/bead` — full sweep green; diff review confirms no fixture outside Bead 1's four-flip set changed outcome (this bead adds tests, the one `Decision` comparability fix, and the mechanical `panelGate` return-arity caller updates).
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- Recording one `refutations` entry for the sole unresolved-RC slot at the
  latest round unblocks `mindspec complete` (all else clean), and success
  leaves a durable `panel_refuted` audit (slot, round, reason, timestamp)
  on bead metadata (AC2).
- A refutation can never clear a REJECT, hard_block, unrecognized verdict,
  another slot, a newer-round re-RC, or any non-vote condition; each RC
  must be refuted individually; a refuted RC never increments `Approves`,
  so a sub-threshold panel still blocks on the floor after every RC is
  refuted; duplicate refutation entries collapse to one audited record
  (AC3, AC4, AC7, AC8, AC9, AC12, item 3).
- A `MergeMetadata` failure on an APPLIED refutation fails the completion
  BEFORE THE BEAD CLOSES — never a silent, un-audited, gate-cleared merge,
  and never an already-closed bead that a panel-removed retry can merge
  un-audited (AC11, TOCTOU item 1) — while abandon/env-skip audits stay
  best-effort (grounded in their empty `AppliedRefutations`, not in any
  "non-merge exit" claim).
- A written `panel_refuted` audit cannot be erased by a later transient
  metadata read failure: `bead.MergeMetadata` is fail-closed (item 2).

**Depends on**
Bead 1 (the unresolved-RC block must exist before an escape from it is
meaningful — AC1 before AC2).

## Bead 3: Docs — ADR-0037 amendment + /ms-panel-tally refutation procedure (both skill copies)

**Scope**

`.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md`,
`plugins/mindspec/skills/ms-panel-tally/SKILL.md`, and its byte-identical
mirror `.claude/skills/ms-panel-tally/SKILL.md` (verified identical at HEAD;
no Go code embeds this skill — `internal/bootstrap/bootstrap.go:341/419`
reference only the name). No production Go diff. The AC10 no-advertise CODE
predicate landed in Beads 1-2 (a Go test inside a docs-only bead would cross
review surfaces for no gain — a deliberate deviation from the suggested
Bead-C contents, recorded here); this bead carries the no-advertise DOC
contract.

**Steps**

1. **ADR-0037 §1 amendment** (dated 2026-07-10, spec 114, appended after the
   spec-112 amendment): `panel.json` gains the optional `refutations` array
   (entries `{slot, round, reason, evidence}`), parse-lenient like
   `abandon_reason` — the same schema-addition precedent §1 records for
   109's `approve_threshold` and 112's `gate`, except this field is NOT
   decision-inert: it is read by exactly one consumer, `internal/panel`'s
   unresolved-verdict resolution.
2. **ADR-0037 §3 amendment** (after the spec-109 amendment): the threshold
   remains a NECESSARY floor on genuine APPROVE count but is no longer
   sufficient — every expected reviewer's latest-round verdict must be
   APPROVE or an explicitly-refuted REQUEST_CHANGES; an unresolved RC (or
   an unrecognized verdict, which is non-refutable) blocks exactly like a
   REJECT; a refuted RC never counts toward the floor, so refutation cannot
   buy past a sub-threshold panel. State the ADR-0040 interaction
   explicitly: a recorded `approve_threshold` below N−1 still sets the
   approve floor but no longer licenses unrefuted dissent — 109's extension
   and 114's rule compose, they do not contradict.
3. **ADR-0037 §7 amendment**: the **audited refutation** joins
   skip/abandonment/config-toggle with the "legitimate precisely because it
   is always audited, never silent" contract. Its `panel_refuted` audit is
   **durable** where the abandon/env-skip audits are best-effort — and the
   amendment must ground that asymmetry CORRECTLY (review item 5): NOT in any
   "terminal non-merge exit" claim (abandonment is a Warn→Allow→MERGE, gate
   leg 3, and env-skip likewise merges — neither is a non-merge exit), but
   solely in the fact that a refutation CHANGES the gate outcome and
   produces an applied-refutation obligation, whereas abandon/skip produce
   none. Concretely: `mindspec complete` writes `panel_refuted`
   **before the bead closes** (so a bead can never close, and thus never
   merge, without the audit already written) and a write failure fails the
   completion pre-close; and `bead.MergeMetadata` is **fail-closed** on a
   metadata read error so no later best-effort write can erase a recorded
   `panel_refuted` (review item 2). Like the skip hatch, the Block message
   never advertises a paste-able refutation incantation (OQ4); the escape is
   documented here and in /ms-panel-tally only, so it stays a deliberate
   looked-up action. State (review item 7) that of the hatches only env-skip
   and abandonment carry a bead-metadata audit (`panel_gate_skipped` /
   `panel_abandoned`); the `enforcement.panel_gate: false` config toggle
   writes no audit. Re-affirm §6 (fail-open without a panel, fail-closed
   with one — unchanged) and §8 (trust boundary unchanged: the record is
   agent-writable by design; an **empty `reason` still clears — a documented
   footgun in parity with empty `abandon_reason`**, not an endorsed use;
   reason + evidence carry the refutation's legitimacy) — the O1 minor,
   recorded.
4. **/ms-panel-tally SKILL.md**: add item 5, "Refutation procedure
   (per-slot, always audited)", directly beside the abandon procedure (item
   4, SKILL.md:84): hand-edit `panel.json`, appending to `refutations` one
   entry naming the filename-derived slot, the latest round, a who/why
   `reason`, and `evidence` for where the disproof lives; the GATE
   validates (it clears only that slot's latest-round REQUEST_CHANGES —
   never a REJECT, hard_block, or unrecognized verdict, never
   staleness/incompleteness/the approve floor; a re-RC at a newer round
   blocks again); completion durably writes the `panel_refuted` audit.
   Carry the abandon-parallel framing: do NOT refute to dodge a finding you
   have not actually disproven — legitimate precisely because always
   audited, never silent; reason + evidence carry its legitimacy (an empty
   reason still clears — footgun, not endorsement). Update the two
   now-stale anti-pattern bullets: "Don't drop a REQUEST_CHANGES because
   'only one reviewer flagged it'" gains "— the gate now BLOCKS on any
   unresolved REQUEST_CHANGES; the only per-slot escape is the audited
   refutation above", and the N−1 bullet notes the threshold is now a
   floor, not a sufficient condition. Copy the updated file byte-identically
   to `.claude/skills/ms-panel-tally/SKILL.md`.
5. **Proof pass**: run the AC6 grep and the mirror diff; run the build and
   the two core test packages once to confirm the docs-only diff is clean.

**Verification**

- [ ] `grep -n 'refut' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md plugins/mindspec/skills/ms-panel-tally/SKILL.md` returns hits in BOTH files (AC6).
- [ ] `diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` reports identical (mirror in sync).
- [ ] Manual review checklist for the bead panel: the amendment text names
  the §1, §3, and §7 changes, re-affirms §6/§8 including the empty-reason
  footgun, and states the durable-vs-best-effort audit distinction in §7.
- [ ] `go build ./... && go test ./internal/panel ./internal/complete` still
  green (docs-only diff; no code drift).

**Acceptance Criteria**

- ADR-0037 carries a dated 2026-07-10 amendment enumerating the §1
  `refutations` schema, the §3 layered-threshold rule (with the explicit
  ADR-0040 composition statement), and the §7 durable, non-advertised
  refutation escape, re-affirming §6/§8 (AC6).
- Both /ms-panel-tally copies document the refutation procedure beside the
  abandon procedure with the reason+evidence-carry-legitimacy /
  empty-reason-footgun framing (AC6, O1 minor).

**Depends on**
Bead 2 (documents the shipped schema, audit keys, and durable semantics so
the AC6 grep hits real names).

## ADR Fitness

- **ADR-0037 (panel gate as enforced contract) — AMENDED; remains the right
  home.** This spec is a §3/§7 semantics change plus a §1 schema addition
  to the exact contract ADR-0037 records; Bead 3 amends it in place
  following its own amendment convention (five prior dated amendments
  already live in the file). Creating a new ADR was considered and
  rejected: the single-home property ("this is the SINGLE home of the
  panel-gate enforced contract", internal/panel/gate.go:10-17) is itself
  part of the contract, and splitting the RC rule from the threshold rule
  it layers on would recreate the two-sources drift the ADR exists to
  prevent. §6 and §8 are deliberately NOT amended, only re-affirmed:
  fail-open without a panel and the anti-footgun (not anti-adversary) trust
  boundary are unchanged — the refutation record is exactly as
  agent-writable as `abandoned`, and no bead adds signing/hashing (the §8
  "do not fix forgeability" fence). The §7 amendment ALSO records the
  write-before-close audit placement and the fail-closed `MergeMetadata`
  (the two durability guarantees the plan-approve panel required).
- **ADR-0035 (agent error contract) — best choice, unchanged.** The new
  leg-9.5 Block routes through the existing `guard.NewFailure` wiring
  (panel_advisory.go:201-203) so the final line is a genuine `recovery:`
  command, and the new step-3.75 pre-close audit-write failure constructs
  its own `guard.NewFailure` with an idempotent-rerun recovery line — both
  conforming to the convention `internal/guard/recovery_convention_test.go`
  enforces. No amendment needed: the contract already covers new failure
  modes by construction.
- **ADR-0040 (orchestration layering ratchet) — best choice, unchanged;
  composition made explicit.** The recorded `approve_threshold` stays the
  sole per-panel floor interpreter (`panel.ApproveThreshold`,
  internal/panel/panel.go:113-127 — this plan adds no second interpreter,
  and leg 10 is byte-preserved); 114 layers a NEW necessary condition on
  top. The Bead-3 §3 text states the interaction (a recorded threshold
  below N−1 still sets the floor but no longer licenses unrefuted dissent)
  so 109 and 114 cannot be read as contradicting — an ADR-0037 amendment
  obligation, not an ADR-0040 change.
- **ADR-0023 (beads as single state authority) — best choice, unchanged;
  now ALSO reached by an execution-domain edit.** The `refutations` array is
  a review artifact, not workflow state (ADR-0037 §8 closing paragraph); the
  gate reads it and writes nothing to it. Lifecycle state stays derived from
  bd: the durable audit lands on bead METADATA via the `bead.MergeMetadata`
  seam, exactly like `panel_abandoned`. Two refinements from the plan-approve
  panel: (1) the audit is written BEFORE the bead CLOSE (not merely before
  the merge) so the state authority never records a closed bead whose
  refutation obligation is unwritten; (2) `bead.MergeMetadata` itself is made
  fail-closed on a read error (the `internal/bead` execution-domain edit) so
  the single state authority's metadata can never be silently corrupted by a
  transient read — a strengthening of ADR-0023's own invariant, fully within
  its spirit. ADR-0037's Domain(s) already include execution, so this edit
  needs no new ADR; it does expand this plan's scope beyond the approved
  spec's "internal/bead read-only" note, recorded in the Decomposition
  domain note.

## Testing Strategy

- **Unit — `internal/panel` (the decision table is the core surface).**
  Extend the pure, fs-free decision fixtures (`panel_decision_test.go`'s
  `result` helper, `votedecision_test.go`'s `makeVerdicts` — upgraded to
  synthesize real verdict strings): Bead 1 adds the
  unresolved-RC/unrecognized rows plus the AC10 no-advertise predicate;
  Bead 2 adds the anti-gaming rows — refuted-RC Allow, REJECT/hard_block
  non-clearable, per-slot scope, round-binding,
  sub-threshold-after-refutation floor Block (AC7), refuted-never-counts
  (AC9), unrecognized-non-refutable (AC8), and the duplicate-slot-casing
  determinism rows (AC12) — built synthetically so case-insensitive
  filesystems cannot flake them. `VoteDecision` lockstep rows mirror every
  gate row that touches the vote portion (the R1 never-disagree falsifier).
  `internal/config` is untouched (N/A — no new keys, per the spec's Out of
  Scope).
- **`internal/complete` — gate wiring + durable audit (write-before-close).**
  Reuse the two existing harnesses: the real-temp-repo `setupPanelGateRepo`
  e2e suite (panel_gate_e2e_test.go:43-70) for AC1's end-to-end Block, AC2's
  passing-by-refutation completion, the AC11 write-fails→not-closed→merge-
  never-runs fence, the TOCTOU retry (panel removed after a failed pre-close
  write → no un-audited merge), the already-closed recovery-path audit test,
  and the skip-env-with-refutations (no `panel_refuted`) row; and the
  seam-stub pattern of `TestWritePanelAuditMetadata_Abandoned`
  (panel_advisory_test.go:111-134, swapping `completeMergeMetadataFn`) for
  the audit-content, the selective `panel_refuted`-only failing stub, the
  post-write erasure-survival test, and the abandon best-effort asymmetry
  control. The not-closed guarantee is asserted via a `closeBeadFn` spy plus
  the harness's `readStubMergeExecutor.completeCalled` flag (neither close
  nor merge ran on the failed write).
- **`internal/bead` — fail-closed `MergeMetadata`.**
  `TestMergeMetadata_FailClosedOnReadError` in `bdcli_test.go` (stubbing the
  `bd show` read via the package's existing test seam) asserts a read/parse
  failure returns an error and performs NO replace-write (existing keys
  preserved), while a clean empty read still merges — the unit floor under
  the complete-side erasure-survival test.
- **`cmd/mindspec` + `internal/instruct` — inheritance surfaces.** No new
  production code; `TestPanelVerbs_DecisionIsPanelGateDecision` and
  `TestSanitizeNonBeadDecision` (equality-pinned against the shared
  decision) plus the panel-state tests keep proving verify/tally/instruct
  can never disagree with the gate. One e2e `panel tally` row (the Bead-1
  carve-out fixture flipped to non-zero + recovery line) pins the CLI exit
  code through the new leg.
- **AC5 regression fence.** Every bead ends with the full
  `go build ./... && go test ./internal/panel ./internal/complete ./internal/instruct ./cmd/mindspec ./internal/bead`
  sweep plus the Bead-1 diff-review proof that outcome flips are confined
  to the four-fixture carve-out set and all other fixture edits are
  outcome-preserving input updates (enumerated exhaustively in Bead 1
  steps 5-6, including the three panel-flagged additions (f)-(h)).
- **Shared test infrastructure (named, reused, never forked):**
  `setupPanelGateRepo` / `readStubMergeExecutor` / `approveVerdicts` /
  `subThresholdVerdicts` (internal/complete/panel_gate_e2e_test.go),
  `writePanel` (internal/complete/panel_advisory_test.go:17-29),
  `result`/`regn` (internal/panel/panel_decision_test.go), `buildResult`
  (cmd/mindspec/panel_test.go:113-146), `beadPanel`/`sixApproves`
  (internal/instruct/panelstate_test.go), and the abandon/skip audit
  fixtures (`TestWritePanelAuditMetadata_*`).

## Provenance

| Spec AC | Bead | Verification step (named, runnable) |
|---|---|---|
| AC1 (out-vote closed) | Bead 1 | `TestPanelGateDecision_UnresolvedRequestChangesBlocks` + `TestPanelGate_RequestChangesBlocksComplete`; `go test ./internal/panel -run 'PanelGateDecision' && go test ./internal/complete -run 'PanelGate'` |
| AC2 (refutation unblocks + audit persists) | Bead 2 | `TestPanelGateDecision_Refutations` (Allow row) + `TestWritePanelRefutedMetadata` + the e2e passing-by-refutation run; `go test ./internal/complete -run 'PanelRefuted\|PanelGate' && go test ./internal/panel -run 'Refut'` |
| AC3 (refutation scope: never REJECT/hard_block; per-slot) | Bead 2 | `TestPanelGateDecision_Refutations` REJECT/hard_block + two-slot rows; `go test ./internal/panel -run 'Refut'` |
| AC4 (round-scoping: zero-ceremony re-panel; no round carry-forward) | Bead 2 | `TestPanelGateDecision_Refutations` round rows; `go test ./internal/panel -run 'Refut\|Round'` |
| AC5 (unchanged paths + four-fixture carve-out) | Bead 1 (established), re-swept in Beads 2-3 | `go build ./... && go test ./internal/panel ./internal/complete ./internal/instruct ./cmd/mindspec ./internal/bead` + the Bead-1 diff-review proof over the step-5 four-flip set and the step-6 (a)-(h) outcome-preserving list (incl. the three panel-flagged additions) |
| AC6 (ADR + skill doc) | Bead 3 | `grep -n 'refut' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md plugins/mindspec/skills/ms-panel-tally/SKILL.md` hits in both + mirror `diff -q` |
| AC7 (sub-threshold after refutation still blocks) | Bead 2 | `TestPanelGateDecision_Refutations` AC7 row (6A+2 refuted RC, threshold 7 → leg-10 Block with the threshold message); `go test ./internal/panel -run 'Refut\|PanelGateDecision'` |
| AC8 (unrecognized verdict blocks and is non-refutable) | Bead 1 (blocks) + Bead 2 (non-refutable) | unrecognized rows in `TestPanelGateDecision_UnresolvedRequestChangesBlocks` and `TestPanelGateDecision_Refutations`; `go test ./internal/panel -run 'Refut\|PanelGateDecision'` |
| AC9 (refuted RC never increments Approves) | Bead 2 | `TestResult_AppliedRefutations` + AC9 assertions in the refutation table; `go test ./internal/panel -run 'Refut'` |
| AC10 (no-advertise Block-message predicate) | Bead 1 (predicate) + Bead 2 (re-asserted with refutations present) + Bead 3 (the doc-side no-advertise contract) | fixed-string predicate rows in both decision tables + `TestPanelGate_RequestChangesBlocksComplete` recovery-line assert; `go test ./internal/panel -run 'PanelGateDecision' && go test ./internal/complete -run 'PanelGate'` |
| AC11 (durable audit: write failure must not silently complete; write-before-close + fail-closed MergeMetadata) | Bead 2 | `TestPanelGate_RefutedAuditWriteFailure_FailsCompletion` (non-zero, bead NOT closed, merge never ran) + the TOCTOU panel-removed retry + the post-write erasure-survival test + `TestMergeMetadata_FailClosedOnReadError` + the abandon best-effort asymmetry control; `go test ./internal/complete -run 'PanelRefuted\|PanelGate' && go test ./internal/bead -run 'MergeMetadata'` |
| AC12 (duplicate slot files at latest round — chosen behavior stated + pinned; dedup extended into the audit path) | Bead 2 (step 2: byte-exact slot matching against the slot's tallied latest-round verdict; a REJECT under a near-duplicate slot is never cleared; same-slot+round entries collapse to one applied record) | `Dup`-named rows in `TestPanelGateDecision_Refutations`; `go test ./internal/panel -run 'Refut\|Dup'` |
