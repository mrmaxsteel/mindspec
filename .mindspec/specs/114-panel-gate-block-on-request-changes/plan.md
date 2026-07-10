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
  `PanelGateDecision`/`panelGate`, and the full **durable-obligation
  protocol**: a fail-closed `refutation_pending` marker on bead metadata whose
  persistence is what makes a refutation "applied" at all (UNIONED into any
  existing unsatisfied pending, never a bare replace), and a pre-close
  **every-path reconciliation** (panel-present, no-panel, AND hatch) that
  satisfies (`panel_refuted`), verified-discharges (`refutation_discharged`,
  only on affirmative tally evidence the RC resolved), or refuses EVERY entry
  in the unioned pending set, plus fail-closed `bead.GetMetadata`/`MergeMetadata`
  so no read-failure fails open or erases an audit, and deduplicated
  applied-refutation selection. This bead edits `internal/bead` (execution
  domain — see the amendment note below).
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

**Durable-obligation stays IN Bead 2 (not a new Bead 4) — decomposition
decision (round-3 G2).** The durable-obligation protocol (the applied≡persisted
`refutation_pending` marker, the every-path satisfy/discharge/refuse
reconciliation, the fail-closed `GetMetadata`/`MergeMetadata`) is folded into
Bead 2 rather than split into a Bead 4. The
protocol is inseparable from the `panel_refuted` audit it enforces: a Bead 2
that shipped `panel_refuted` WITHOUT the obligation would itself carry the
exact G2 hole round-2 rejected (a panel-removed retry merging un-audited), so
that intermediate merged state would not be individually panel-passable — and
every bead must pass its own gate. The three pieces touch the SAME files as
the rest of Bead 2 (`complete.go`, `panel_advisory.go`, `internal/bead`) and
cannot be tested without Bead 2's applied-refutation plumbing, so a Bead 4
would be a pure R_scope-overlap dependency with no independence. The chain
stays A→B→C (length 3). Bead 2 grows but the mechanism lands atomically.

**Impacted-domains amendment (plan-approve rounds 2-3, operator-authorized).**
Bead 2 edits `internal/bead/bdcli.go` (making `bead.MergeMetadata`
fail-closed — item 2 below), an **execution-domain** file. The round-2 plan
proposed to have the Bead 2 brief "declare the execution domain in its
OWNERSHIP attribution" — round-3 finding O3 proved that mechanically WRONG:
the complete-time ADR-divergence gate derives its CANDIDATE domains from the
**spec's `## Impacted Domains`** (`internal/validate/divergence.go:104/155/162`,
`attributeDomainCached` in `ownership.go:373`), which listed workflow only and
said `internal/bead` had "no change"; a file attributed to `execution` (not in
the candidate set) hard-errors at `divergence.go:221` BEFORE the ADR-coverage
check — a brief-side OWNERSHIP note cannot fix that. **Fix (operator-authorized
amendment, applied):** the spec's `## Impacted Domains` now ADDS **execution /
`internal/bead/**`** (owned per `.mindspec/domains/execution/OWNERSHIP.yaml:3`)
with a good-reason note, and strikes the "no change lands there" line. ADR-0037
(Domain(s): workflow, execution) and ADR-0035 (workflow, execution, core), both
cited, cover execution — so ADR-coverage still passes and no new ADR is added;
`mindspec validate spec` passes with the amendment (verified). The
`--override-adr "<reason>"` fallback (precedented, spec 110) is therefore NOT
needed and is not used.

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

## Bead 2: R2 — audited refutation escape + DURABLE-obligation protocol (schema, gate validation, applied≡persisted marker, every-path satisfy/discharge/refuse reconciliation, fail-closed GetMetadata/MergeMetadata)

**Scope**

`internal/panel/panel.go` (schema), `internal/panel/tally.go` +
`internal/panel/gate.go` (gate validation + applied-refutation plumbing),
`internal/complete/panel_advisory.go` + `internal/complete/complete.go` (the
durable-obligation protocol — the pre-close `refutation_pending` marker, the
pre-close `panel_refuted` audit, the fail-open-path obligation check, and the
dedup), and `internal/bead/bdcli.go` (making `bead.MergeMetadata` FAIL-CLOSED
and adding a `GetMetadata` read helper — the execution-domain edit; see the
amendment note in Decomposition). The writes go through the existing
`completeMergeMetadataFn` seam (`internal/complete/complete.go:42`) →
`bead.MergeMetadata` (`internal/bead/bdcli.go:263-290`, which JSON-marshals the
merged map, so a nested entries array round-trips). No new CLI verb (OQ3:
hand-edit `panel.json`; the gate carries the whole validation burden).

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
   in two matched panels audits once). **Signature scope (F2 resolution):**
   panelGate returns the applied-refutation set (for the pending-marker write
   in (a)) and the matched `*panel.Registration` (whose `Dir` reconciliation
   re-tallies for discharge evidence) — it does NOT need to thread a `*Result`
   through its return; `reconcilePendingRefutations` gets the vote-state by its
   own `panel.Tally(panelReg.Dir)` (c), which works uniformly on the
   panel-present, no-panel, and hatch paths without depending on whether
   panelGate tallied. So the 3-tuple `(*Registration, []Refutation, error)`
   stands; no 4th return value is added. The skip-env and config-disabled paths
   (panel_advisory.go:169-179) return nil applied — the gate did not evaluate
   facts or rely on any refutation there. NOTE (review item 7): of
   these two, ONLY the env-skip path has its own audit
   (`panel_gate_skipped`, written by `writePanelAuditMetadata` guarded on
   `panelSkipEnvFn()`, panel_advisory.go:322-330); the config-disabled
   (`enforcement.panel_gate: false`) path writes NO audit at all — do not
   claim it does. The signature change ripples to every `panelGate(...)`
   caller: the one production site complete.Run (complete.go:375) and the
   12 test call sites across `internal/complete/panel_gate_e2e_test.go`
   (:272,:282,:297,:320), `internal/complete/panel_advisory_test.go`
   (:152,:226,:240,:289,:300,:314), and
   `internal/complete/panel_gate_layout_test.go` (:152,:162) — all
   mechanical `reg, err :=` → `reg, _, err :=` updates (AC5-permitted
   struct/return additions; no outcome changes).
5. **The DURABLE-OBLIGATION protocol (`complete.go` + `panel_advisory.go`;
   round-4 "applied" ≡ durably-persisted + round-5 TOTAL reconciliation — the
   full unioned pending SET, verified-vote-state discharge, every path incl.
   hatches).** A refutation is "applied" (clears its RC) ONLY once its
   obligation is durably on bead metadata; and the FULL set of recorded
   obligations is reconciled — satisfied, verified-discharged, or refused —
   before EVERY close (panel-present, no-panel, AND hatch). All metadata writes
   go through the (now fail-closed, step 6) `completeMergeMetadataFn` seam; the
   read through `completeGetMetadataFn`; all writes are fail-closed; all happen
   BEFORE the step-4 close.
   - **(a) "Applied" ≡ durably-persisted, UNION not replace (O1#2/G2 + round-5
     item 1).** A refutation is "applied" ONLY once its `refutation_pending`
     entry is durably persisted. Fold the marker write INTO `panelGate`
     (`panel_advisory.go`, at the step-2.25 decision, caller
     complete.go:375-378): when `PanelGateDecision` returns an Allow carrying a
     non-empty `AppliedRefutations`, `panelGate` **reads the existing
     `refutation_pending_entries` via `completeGetMetadataFn` (fail-closed),
     UNIONS the current run's applied (slot, round) entries into that set
     (dedup by slot+round; never a bare replace), and writes the merged array**
     — because `bead.MergeMetadata` replaces at top-level key granularity, a
     naive write of only the current entries would OVERWRITE an older
     unsatisfied pending (the loss path: run-1 refutes X@1, marker (X,1), its
     `panel_refuted` fails, bead stays `in_progress`; a round-2 re-panel's
     marker write of (B,2) would clobber (X,1), which then never gets audited).
     Reading-then-unioning-then-writing preserves every still-unsatisfied
     obligation. If ANY part (the read OR the write) FAILS, the refutation is
     NOT applied → the RC is NOT cleared → `panelGate` returns a **Block** (a
     `guard.NewFailure`: "the refutation could not be durably recorded, so the
     REQUEST_CHANGES from <slot> remains unresolved — retry, or resolve the
     finding"; bead stays `in_progress`, no CommitAll/close/merge) — a genuine
     Block, NOT an abort-with-applied. So applied-in-a-run ⟺ the entry is in the
     durable unioned set. (This marker-failure Block message is an operational
     write-failure, distinct from the leg-9.5 unresolved-RC gate Block that
     AC10's no-advertise predicate governs — it is deliberately OUTSIDE AC10's
     scope: AC10 keeps the gate from advertising the refutation ESCAPE at an
     unresolved RC; this message reports that an already-attempted refutation
     could not be recorded, and naming "refutation" there is correct, not an
     advertised escape.)
   - **(b) `panel_refuted` satisfying audit (pre-close).** Add
     `writePanelRefutedMetadata(beadID, entries) error` in panel_advisory.go
     building `panel_refuted: true`, `panel_refuted_at: <RFC3339 UTC>`,
     `panel_refuted_entries: [{slot, round, reason, evidence}, ...]` (keys
     parallel to `panel_abandoned`, panel_advisory.go:332-341); like the marker,
     it UNIONS into any existing `panel_refuted_entries` (read-then-union,
     fail-closed) so a later satisfy never clobbers an earlier one. It RETURNS
     the merge error (non-swallowing, unlike `writePanelAuditMetadata`,
     panel_advisory.go:304-342, best-effort for abandon/skip). Runs inside the
     reconciliation step (c), before close; a failure fails completion
     pre-close.
   - **(c) RECONCILE the ENTIRE unioned pending SET on EVERY path, before close
     (G3 + round-5 items 1 & 3).** A `reconcilePendingRefutations` step in `Run`
     — reached on the panel-present, the no-panel, AND the hatch paths, AFTER
     the last blocking gate (ADR-divergence, complete.go:542-544) and BEFORE
     the step-4 close — reads bead metadata via `completeGetMetadataFn` (step 6,
     fail-closed) and, for EVERY recorded `refutation_pending` entry NOT already
     covered by a durable `panel_refuted`/`refutation_discharged` entry (the
     full unioned set, not just this run's), applies exactly one of. **How
     reconciliation obtains the discharge vote-state (F2 precision fix — the
     mechanism, made explicit):** `reconcilePendingRefutations` **re-tallies the
     matched panel itself from `panelReg.Dir` via `panel.Tally` (fs-only, no
     git)** to get each slot's latest-round verdict + `LatestRound`; it does NOT
     rely on `panelGate` having tallied (`panelGate` does not tally on the hatch
     paths — see (e) — and its 3-tuple return threads no `*Result`, so a direct
     re-tally is the uniform, path-independent source; `panelReg` is the matched
     registration `panelGate` already returns). If there is **no panel dir**
     (no-panel path) OR the **re-tally errors / yields no readable verdicts**,
     the discharge vote-state is UNAVAILABLE → discharge cannot fire → the entry
     Refuses (fail-closed, over-conservative: never a lost obligation, never a
     false discharge). The three cases:
     - **Satisfy** — the current run's `AppliedRefutations` (the set `panelGate`
       returns) covers the pending (slot, round): write `panel_refuted` for it
       (b), reason/evidence from the current panel.json refutations entry.
     - **Discharge — ONLY on VERIFIED re-tallied vote-state, never a bare "gate
       passed" (round-5 item 2 / G1+G3).** `PanelGateDecision` ALSO passes via
       **Warn** — abandoned (gate.go:160-169), missing-ref (180-189),
       transient-gitErr (192-203) — WITHOUT the RC resolving, so "gate passed ⟹
       moot" would FALSELY discharge a still-live RC. Discharge a pending (slot,
       round) ONLY on affirmative evidence from the **re-tally of `panelReg.Dir`**
       that the slot is no longer a latest-round `REQUEST_CHANGES` at/after its
       recorded round — precisely the two-disjunct test: **(i)** the re-tallied
       `LatestRound` > the pending round (a later round exists, superseding it),
       OR **(ii)** at `LatestRound == pending round`, the slot's latest verdict
       is present and is not `REQUEST_CHANGES` (the reviewer flipped). Then write
       a `refutation_discharged` audit (`refutation_discharged: true` + `_at` +
       unioned `_entries: [{slot, round, reason: <synthetic: "RC resolved
       naturally — the slot's latest-round verdict is no longer REQUEST_CHANGES
       at/after round N">}]`; the synthetic reason is intentional — a discharge
       is a system-verified fact, not an operator-authored refutation, so it
       needs no `evidence` field). A Warn panel (abandoned / missing-ref /
       transient-gitErr) whose re-tally still shows the slot as a latest-round
       RC at the pending round does NOT meet this test → Refuse.
     - **Refuse** — the resolution cannot be affirmatively verified: no panel dir
       / an unreadable or erroring re-tally; a re-tally that still shows the RC at
       the pending round; or a `completeGetMetadataFn` read/parse error (d). A
       recoverable `guard.NewFailure` ("this bead carries an unsatisfied
       refutation obligation for <slot@round> that cannot be verified as
       satisfied or resolved — restore the panel so it can be satisfied or
       discharged, or restore the audit", recovery `mindspec complete <bead>`). A
       bead with NO recorded pending reconciles to a no-op and completes exactly
       as today (§6 fail-open preserved for genuinely pristine beads).
     Every pending entry must reach Satisfy or Discharge for the completion to
     proceed; if ANY entry Refuses, the whole completion refuses pre-close. All
     satisfy/discharge writes are fail-closed (a write failure fails completion
     pre-close). Because these audit reads/writes read-then-union (a/b), a
     same-run satisfy that immediately follows this run's own marker write sees
     the just-written entry (add a read-after-write comment on that path — O2).
   - **(d) `completeGetMetadataFn` read-error ⟹ REFUSE (fail-closed, O1#1).**
     The reconciliation read (and the (a) union read) are fail-closed: a
     `bead.GetMetadata` read/parse error is NOT a licence to proceed — it
     Refuses (nothing closed/merged), because an unreadable metadata store
     cannot prove the bead is obligation-free.
   - **(e) HATCH paths reconcile too — hatches except the GATE, not the
     obligation (round-5 item 3 / G3; F2 hatch-tally correction).** The env-only
     `MINDSPEC_SKIP_PANEL` and the config `enforcement.panel_gate: false` bypass
     the GATE DECISION (they allow the merge despite unresolved RCs) but NOT a
     pre-existing durable `refutation_pending` obligation from a prior non-hatch
     run. So the reconciliation (c) runs before close on the hatch paths TOO: an
     existing pending must be satisfied / validly-discharged / refused, never
     silently merged. **Correction (F2):** `panelGate` does NOT tally on the
     hatch paths — the skip-env branch builds a bare
     `panel.GateFacts{BeadID, SkipEnv: true}` (panel_advisory.go:169-175) and the
     config-disabled branch `continue`s before `ResolveGateFacts`
     (panel_advisory.go:176-179), and `writePanelAuditMetadata` never tallies —
     so an earlier draft's claim that "the `Result` is available" on hatch paths
     was FALSE. Reconciliation instead uses the same uniform mechanism as every
     other path: it re-tallies `panelReg.Dir` via `panel.Tally` (c). On a hatch
     path the current run applies no NEW refutation (gate skipped), so Satisfy is
     unavailable; Discharge fires only if that re-tally affirmatively shows the
     slot resolved (config-disabled discharge is thus best-effort — the fail-
     closed Refuse fallback is always safe); otherwise Refuse — audit integrity
     overrides the hatch (a `GetMetadata` error, no panel dir, or an unverified
     re-tally all → Refuse). The hatch's own `panel_gate_skipped` audit is still
     written.
   **Net invariant (state it in the code comment):** a refutation that EVER
   durably clears an RC (its `refutation_pending` was persisted — the only way
   an RC is cleared, per (a)) can never let its bead close or merge — across ANY
   number of `mindspec complete` retries, ANY completion path (panel-present,
   no-panel, or hatch), and even after the panel.json is removed — without a
   durable `panel_refuted` (satisfied) or VERIFIED `refutation_discharged`
   (naturally-moot) audit for that entry on bead metadata. **Hatches except the
   GATE, NOT the obligation reconciliation (round-5):** a hatch lets a bead
   merge despite unresolved RCs, but a pre-existing durable refutation
   obligation is still reconciled/audited before close. The step-5.6
   `writePanelAuditMetadata` call (complete.go:778-785) is byte-unchanged and
   does NOT write `panel_refuted`. **The durable-vs-best-effort asymmetry is
   grounded SOLELY in AppliedRefutations (round-2 item 5):** a refutation
   produces a non-empty `AppliedRefutations` set and thus a durable obligation;
   abandon and env-skip both Warn→Allow→MERGE too (they are NOT "terminal
   non-merge exits") but produce NO `AppliedRefutations`, so they add no NEW
   obligation (a PRE-EXISTING one is still reconciled per (e)).
6. **Fail-closed `bead.MergeMetadata` + `GetMetadata` read helper
   (`internal/bead/bdcli.go:266-300`; carry-forward #1 / review item 2 —
   closes the post-write erasure hole).** Today `MergeMetadata` IGNORES a
   metadata-READ failure: on a `bd show` error it proceeds from an EMPTY
   `merged` map and replace-writes, ERASING every existing key — so a
   transient read failure during ANY later write (the step-5.5 doc-skew /
   adr-override / adr-supersede writes, or the step-6.5 epic-phase sync, all
   via the same helper) could wipe a just-written `panel_refuted`, yielding a
   successful completion with the audit LOST. FIX: make `MergeMetadata`
   **fail-closed** — when the existing-metadata read (`tracedOutput("show",
   ...)`) errors OR its JSON fails to unmarshal, RETURN that error instead of
   proceeding from empty (a genuinely absent metadata map — a clean read that
   yields no items — still merges from empty, unchanged; only a READ/PARSE
   FAILURE now aborts). ALSO add an exported
   `func GetMetadata(issueID string) (map[string]interface{}, error)` (the
   same `bd show <id> --json` read `MergeMetadata` already does, returning the
   parsed `metadata` map or a read error) so the step-5(c)/(d) reconciliation
   can consult the obligation; a read/parse error is surfaced to the caller
   (which Refuses per (d), never fail-opens). In `internal/complete` this read
   is reached through a package seam coherent with the existing
   `completeMergeMetadataFn` var (a `completeGetMetadataFn = bead.GetMetadata`
   seam, swappable in tests exactly like the write seam), so the
   durable-obligation e2e tests can drive both the read and the write without a
   real `bd`. **Blast-radius — the ACTUAL caller taxonomy (round-2 F3
   correction; do NOT claim "all best-effort"):** the change makes a read-error
   fatal where it was silently swallowed. By class, no caller regresses: (1)
   **already-fatal** callers — `cmd/mindspec/repair.go:86`
   (`repairMergeMetadataFn`), and `phase.EnsureMigrated` (the migration write;
   entered at `complete.go:283` via `EnsureMigratedWithCache`, and from both
   approve paths) — already propagate write errors
   fatally; making the rare read-error also fatal is strictly safer (a
   swallowed read-error there was a latent silent corruption). (2)
   **best-effort/warn** callers — complete's
   doc-skew/adr-override/adr-supersede/panel audits and `approve/impl.go`
   writes — are unchanged on the common path and merely warn (never erase) on
   the rare read-error. (3) **ignore** caller — `release.go:133` discards the
   return — unchanged. Keep a blast-radius grep
   (`grep -rn 'MergeMetadata(' --include='*.go'`) as a Bead-2 verification
   step (round-2 F1). This is the `internal/bead` (execution-domain) edit; the
   complete-time ADR-divergence gate accepts it because the spec's
   `## Impacted Domains` now declares execution / `internal/bead/**` (the
   round-3 O3 amendment — a brief-side OWNERSHIP note alone would NOT have
   satisfied the gate, which derives candidate domains from the spec).
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
   **(B) Durable-obligation protocol + fail-closed metadata
   (`internal/complete` + `internal/bead`; new e2e tests named under the
   `TestPanelRefuted…`/`…Refut…` prefix so `-run 'PanelRefuted|PanelGate'`
   selects them — round-3 F1).** (i) **AC2 audit-half**:
   `TestPanelRefuted_WriteMetadata` beside `TestWritePanelAuditMetadata_Abandoned`
   (panel_advisory_test.go:111-134): captures the merged map — asserts
   `panel_refuted == true`, an RFC3339 `panel_refuted_at`, and the
   slot/round/reason/evidence entries — and asserts the ERROR RETURN
   propagates (non-swallowing); plus e2e on the `setupPanelGateRepo` harness:
   a 5A+1RC panel with a matching `refutations` entry → `Run` succeeds,
   `ex.completeCalled == true`, and the captured metadata carries BOTH
   `refutation_pending` and `panel_refuted`. (ii) **AC11
   `panel_refuted`-write-fails → not-closed → merge never runs**:
   `TestPanelRefuted_SatisfyWriteFailure_FailsCompletion` — stub
   `completeMergeMetadataFn` to fail ONLY on maps containing `panel_refuted`
   (the pending write + others succeed) → `Run` returns non-zero,
   `ex.completeCalled == false`, `closeBeadFn` NOT called. (iii) **AC11
   applied≡persisted — marker-write OR union-read fails ⟹ RC BLOCKS, not
   abort-with-applied (round-4 + round-5 item 1)**:
   `TestPanelRefuted_MarkerWriteFailure_Blocks` — stub the marker write (and,
   in a sub-case, the union `completeGetMetadataFn` read) to fail → `Run`
   returns non-zero with a BLOCK naming the still-unresolved RC slot (guard
   failure), `closeBeadFn` NOT called, NO `panel_refuted` written; assert the
   message is the RC-unresolved Block, NOT an "aborted with refutation applied".
   (iv) **AC11 CROSS-RUN panel-removed retry (G2's strong assertion)**:
   `TestPanelRefuted_CrossRun_PanelRemoved_Refuses` — run 1 fails its
   `panel_refuted` write (bead `in_progress`, `refutation_pending` durably
   recorded, no `panel_refuted`); then REMOVE `panel.json` and run `Run` a
   SECOND time with writes succeeding → the second run REFUSES via the
   every-path reconciliation (no-panel + unsatisfied pending → Refuse):
   non-zero, `closeBeadFn` NOT called, `ex.completeCalled == false`. Positive
   control: RESTORE the panel (RC still present) → run 3 satisfies (writes
   `panel_refuted`) and completes. (v) **AC11 G3 verified-vote-state discharge**:
   `TestPanelRefuted_CrossRun_NaturalResolution_Discharges` — run 1 records
   `refutation_pending` (X@1) then fails `panel_refuted`; run 2 presents a live
   re-panel whose latest round is all-APPROVE (X flipped, or the round
   advanced) so the gate passes with NO applied refutation; assert `Run` does
   NOT silently merge but RECONCILES: it writes `refutation_discharged` for X
   ONLY because the **re-tally of `panelReg.Dir`** affirmatively shows X is no
   longer a latest-round RC at/after round 1, then completes; assert the
   discharge entry names X@1. A negative twin: the discharge write stubbed to
   fail leaves the bead `in_progress` (`closeBeadFn` NOT called). (va) **AC11
   round-5 item 2 — Warn paths must NOT falsely discharge**:
   `TestPanelRefuted_CrossRun_WarnPathDoesNotDischarge` (table over the three
   live-panel Warn variants — abandoned, missing-ref, transient-gitErr) — run 1
   leaves `refutation_pending` (X@1); run 2 presents a panel whose gate returns
   a WARN while its verdict files (the reconciliation **re-tally of
   `panelReg.Dir`**) STILL show X as a latest-round `REQUEST_CHANGES` at round 1;
   assert `Run` does NOT write `refutation_discharged` for X (no affirmative
   resolution evidence) and REFUSES (`closeBeadFn` NOT called) — proving
   discharge keys on the re-tally, not on the bare Allow/Warn gate action. (vb)
   **AC11 round-5 item 1 — UNION multi-entry reconciliation**:
   `TestPanelRefuted_CrossRun_UnionReconcilesAll` — run 1 refutes X@1, marker
   (X,1), `panel_refuted` fails (bead `in_progress`); run 2 is a round-2
   re-panel where X is now APPROVE and a NEW slot B is refuted; assert the run-2
   marker write UNIONS (does not clobber (X,1)) and reconciliation writes BOTH
   `refutation_discharged` for X@1 AND `panel_refuted` for B@2 before close; any
   read/write failure mid-reconcile leaves the bead `in_progress` (no partial
   close). (vi) **AC11 O1#1 —
   GetMetadata read-error ⟹ REFUSE (fail-closed)**:
   `TestPanelRefuted_GetMetadataError_Refuses` — a bead reaching the
   reconciliation with `completeGetMetadataFn` stubbed to return a read error
   → `Run` returns non-zero, `closeBeadFn` and `ex.completeCalled` both false
   (an unreadable store cannot prove the bead is obligation-free). (vii)
   **Pristine-panel-removed = §6 boundary (round-4 reasoning + test)**:
   `TestPanelRefuted_PristineNoPanel_FailsOpen` — a bead that NEVER had a
   refutation applied (no `refutation_pending` on metadata) with no panel →
   completes via §6 fail-open exactly as today (this is NOT a refutation hole;
   the reconciliation no-ops on an empty obligation). (viii) **Post-write
   erasure survival (round-2 item 2)**: `TestPanelRefuted_AuditSurvivesLaterReadError`
   — a successful refutation completion where a LATER best-effort write (the
   step-5.5 doc-skew write via `AllowDocSkew`) hits a `MergeMetadata` read
   failure → `panel_refuted` SURVIVES (fail-closed `MergeMetadata` no-ops the
   erasing write). (ix) **`internal/bead` units**:
   `TestMergeMetadata_FailClosedOnReadError` in `bdcli_test.go` — inject a
   `show` read failure → `MergeMetadata` returns an error and performs NO
   replace-write (existing keys preserved), while a clean empty read still
   merges; and `TestGetMetadata` — the read helper returns the parsed
   metadata map and surfaces a read error. (x) **Already-closed recovery-path
   audit (round-2 item 8 / round-3 O3 minor)**: the recovery branch
   (complete.go:547-554) does NOT force the dolt-commit + committed-state
   verify the normal close path (complete.go:555-650) runs, so the durability
   rationale must not lean on it. Because the marker + reconciliation run
   BEFORE the close, a recovery re-run (bead already closed, panel still
   present) still reconciles (satisfy or discharge) before reaching the
   recovery branch; `TestPanelRefuted_RecoveryPath_AuditPresent` drives a
   pre-closed bead with a live refutation panel and asserts `panel_refuted` is
   present after completion. (xi) **Asymmetry control**: an ABANDONED panel
   with the same `panel_refuted`-failing stub and NO prior pending still
   completes successfully with only a warning (abandon adds no
   `AppliedRefutations` → no NEW obligation, no marker). (xii)
   **HATCH-reconciliation (round-5 item 3 / G3)**:
   `TestPanelRefuted_HatchStillReconcilesPendingObligation` — a table over the
   hatch paths (`MINDSPEC_SKIP_PANEL`, `enforcement.panel_gate: false`, and an
   abandoned panel) each run against a bead that ALREADY carries a durable
   unsatisfied `refutation_pending` (X@1) from a prior run: assert the hatch
   still allows the gate merge BUT the completion does NOT close/merge unless
   the pending is reconciled — it must write a covering `panel_refuted`/
   `refutation_discharged` (when the tally affirmatively shows X resolved) or
   REFUSE (`closeBeadFn` NOT called); and a `completeGetMetadataFn` read error
   on a hatch path REFUSES (audit integrity overrides the hatch). A
   companion no-obligation control: the SAME hatch over a pristine bead (no
   pending) completes and writes only `panel_gate_skipped` (round-2 item 10 —
   the hatch excepts the GATE, not the obligation). (xiii) An
   applied-refutations-empty completion (plain all-APPROVE pass, no prior
   pending) writes neither `refutation_pending` nor `panel_refuted` — only
   genuine clears are recorded.

**Verification**

- [ ] `go test ./internal/panel -run 'Refut|Round|Dup|PanelGateDecision'` —
  AC2(gate)/AC3/AC4/AC7/AC8/AC9/AC12 decision rows + the (slot,round) dedup
  row green.
- [ ] `go test ./internal/complete -run 'PanelRefuted|PanelGate'` —
  AC2(audit) write content; the AC11 `panel_refuted`-write-fail fence; the
  applied≡persisted marker-write / union-read-fail ⟹ RC-BLOCKS test; the
  CROSS-RUN panel-removed retry (run 2 REFUSES) + restore control; the
  verified-vote-state DISCHARGE test; the Warn-paths-do-NOT-falsely-discharge
  table; the UNION multi-entry reconcile-all test; the GetMetadata-read-error
  ⟹ REFUSE test; the pristine-no-panel §6 fail-open boundary; the
  HATCH-still-reconciles table (env-skip / config-disabled / abandoned +
  read-error refuse); the post-write erasure-survival test; the already-closed
  recovery-path audit test; and the whole pre-existing `TestPanelGate_*` suite
  green.
- [ ] `go test ./internal/bead -run 'MergeMetadata|GetMetadata'` — the
  fail-closed read-error test (returns error, no replace-write) and the
  `GetMetadata` read-helper test green.
- [ ] `grep -rn 'MergeMetadata(' --include='*.go'` — blast-radius review
  confirms the caller taxonomy (already-fatal: repair.go / phase.EnsureMigrated;
  warn: complete + approve/impl; ignore: release.go) so making the read-error
  fatal regresses no class (round-2 F1/F3).
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
- **The durable-obligation invariant (round-5, TOTAL):** a refutation that
  EVER durably clears an RC (its `refutation_pending` was persisted — the only
  way an RC clears, since a non-persisted marker returns a Block) can never let
  its bead close or merge — across ANY number of retries, ANY completion path
  (panel-present, no-panel, OR hatch), and after `panel.json` removal — without
  a durable `panel_refuted` (satisfied) or VERIFIED `refutation_discharged`
  (tally-proven moot) audit for that entry. Enforced by: applied≡durably-
  persisted (marker write, UNIONED not replaced, is part of the allow-decision;
  read-or-write failure ⟹ Block), pre-close reconciliation of the FULL unioned
  pending set on EVERY path (satisfy / verified-discharge / refuse), and
  fail-closed `GetMetadata` + `MergeMetadata`. **Hatches except the GATE, NOT
  the obligation:** `MINDSPEC_SKIP_PANEL` / `enforcement.panel_gate: false`
  bypass the gate decision (merge despite unresolved RCs) but a pre-existing
  durable pending is STILL reconciled/audited before close. abandon/env-skip
  add no NEW obligation (empty `AppliedRefutations`) but do not exempt an
  existing one (AC11).
- A written `panel_refuted`/`refutation_discharged` audit cannot be erased by
  a later transient metadata read failure: `bead.MergeMetadata` is fail-closed
  (item 2).

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
   none. Concretely, the round-5 DURABLE-OBLIGATION protocol: (1) a refutation
   is "applied" ONLY once its fail-closed `refutation_pending` entry is durably
   persisted, UNIONED with any existing unsatisfied entries (never a bare
   replace) — the marker read-or-write is part of the allow-decision, so a
   failure returns a BLOCK (the RC stays unresolved), making "applied ⟹ durable
   obligation" definitional; (2) reconciliation of the FULL unioned pending set
   runs BEFORE the close on EVERY completion path (panel-present, no-panel, AND
   hatch), satisfying (`panel_refuted`), VERIFIED-discharging
   (`refutation_discharged`, ONLY on affirmative tally evidence the slot is no
   longer a latest-round REQUEST_CHANGES at/after its round — never a bare
   Warn/gate-pass), or refusing, never silently ignoring any entry; (3)
   `bead.GetMetadata` and `bead.MergeMetadata` are fail-closed so a read error
   refuses rather than fail-opens, and no later write can erase a recorded
   audit. Like the skip hatch, the Block message never advertises a paste-able
   refutation incantation (OQ4); the escape is documented here and in
   /ms-panel-tally only. State (review item 7) that of the hatches only env-skip
   and abandonment carry a bead-metadata audit (`panel_gate_skipped` /
   `panel_abandoned`); the `enforcement.panel_gate: false` config toggle writes
   no audit. **Crucially (round-5 item 3): hatches except the GATE, NOT the
   obligation** — a hatch lets a bead merge despite unresolved RCs but a
   pre-existing durable `refutation_pending` is STILL reconciled/audited before
   close on the hatch path. Re-affirm §6 (fail-open without a panel — UNCHANGED
   for pristine beads with no recorded obligation; the reconciliation only
   refuses/discharges a bead that carries a durable unsatisfied
   `refutation_pending`).
4. **ADR-0037 §8 amendment (operator-authorized — the durable-refutation
   AUDIT-DURABILITY carve-out; label softened from "anti-tamper" per O2 to
   match the delivered guarantee).** §8's standing posture is "anti-footgun,
   not anti-adversary — every gate input is an agent-writable artifact; do not
   fix perceived forgeability." Amend it with a NARROW, good-reason carve-out:
   a refutation is different in kind from the passive footguns §8 scopes out —
   it ACTIVELY clears a specific reviewer's evidence-bearing finding, so
   allowing that clearance to later vanish without a trace (panel removed after
   a refutation was applied) would defeat 114's whole purpose. Therefore the
   AUDIT of an applied refutation is made **durable across retries and paths**
   via the bead-metadata obligation (applied≡persisted+unioned marker +
   every-path — incl. hatch — verified-discharge reconciliation + fail-closed
   `GetMetadata`/`MergeMetadata`). Also record the companion **§6
   affirmative-evidence refusal branch**: a bead carrying a
   durable, unsatisfied refutation obligation does not fail open. State the
   scope precisely so the carve-out does not over-claim: this is an
   audit-durability guarantee for an applied refutation ONLY — NOT general
   tamper-resistance — the posture is otherwise UNCHANGED (an empty `reason`
   still clears; the panel is still freely removable for a bead with NO
   refutation applied; verdicts and panel.json remain agent-writable; no
   signing/hashing is added; a party who never records a refutation is not the
   target). Cite this active-clearance-vs-passive-footgun distinction as the
   amendment rationale, and the operator's explicit authorization. Re-affirm
   the §8
   empty-`reason`-still-clears footgun (parity with empty `abandon_reason`)
   and that reason + evidence carry the refutation's legitimacy (the O1
   minor) within the otherwise-unchanged posture.
5. **/ms-panel-tally SKILL.md**: add item 5, "Refutation procedure
   (per-slot, always audited)", directly beside the abandon procedure (item
   4, SKILL.md:84): hand-edit `panel.json`, appending to `refutations` one
   entry naming the filename-derived slot, the latest round, a who/why
   `reason`, and `evidence` for where the disproof lives; the GATE
   validates (it clears only that slot's latest-round REQUEST_CHANGES —
   never a REJECT, hard_block, or unrecognized verdict, never
   staleness/incompleteness/the approve floor; a re-RC at a newer round
   blocks again); completion durably records the obligation and writes the
   `panel_refuted` audit (or, if the RC later resolves naturally, a
   `refutation_discharged` audit) — a recorded refutation is never dropped
   silently, and a bead whose refutation obligation is unaudited will not
   complete even if the panel is later removed. Carry the abandon-parallel
   framing: do NOT refute to dodge a finding you
   have not actually disproven — legitimate precisely because always
   audited, never silent; reason + evidence carry its legitimacy (an empty
   reason still clears — footgun, not endorsement). Update the two
   now-stale anti-pattern bullets: "Don't drop a REQUEST_CHANGES because
   'only one reviewer flagged it'" gains "— the gate now BLOCKS on any
   unresolved REQUEST_CHANGES; the only per-slot escape is the audited
   refutation above", and the N−1 bullet notes the threshold is now a
   floor, not a sufficient condition. Copy the updated file byte-identically
   to `.claude/skills/ms-panel-tally/SKILL.md`.
6. **Proof pass**: run the AC6 grep and the mirror diff; run the build and
   the two core test packages once to confirm the docs-only diff is clean.

**Verification**

- [ ] `grep -n 'refut' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md plugins/mindspec/skills/ms-panel-tally/SKILL.md` returns hits in BOTH files (AC6).
- [ ] `diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` reports identical (mirror in sync).
- [ ] Manual review checklist for the bead panel: the amendment text names
  the §1, §3, §7, §8, and §6 changes (§8 AMENDED with the narrow
  audit-durability carve-out — label softened from "anti-tamper" per O2; §6
  gains the affirmative-evidence refusal branch), re-affirms the §8
  empty-reason footgun, and states the round-5 durable-obligation protocol
  (applied≡persisted+UNIONED marker + FULL-set every-path — incl. hatch —
  satisfy/verified-discharge/refuse reconciliation + fail-closed
  GetMetadata/MergeMetadata) in §7/§8.
- [ ] `go build ./... && go test ./internal/panel ./internal/complete` still
  green (docs-only diff; no code drift).

**Acceptance Criteria**

- ADR-0037 carries a dated 2026-07-10 amendment enumerating the §1
  `refutations` schema, the §3 layered-threshold rule (with the explicit
  ADR-0040 composition statement), the §7 durable, non-advertised refutation
  escape, the **§8 narrow operator-authorized audit-durability carve-out**
  (applied refutations only; general anti-adversary posture unchanged), and
  the **§6 affirmative-evidence refusal branch** (AC6).
- Both /ms-panel-tally copies document the refutation procedure beside the
  abandon procedure with the reason+evidence-carry-legitimacy /
  empty-reason-footgun framing (AC6, O1 minor).

**Depends on**
Bead 2 (documents the shipped schema, audit keys, and durable semantics so
the AC6 grep hits real names).

## ADR Fitness

- **ADR-0037 (panel gate as enforced contract) — AMENDED (§1/§3/§7/§8);
  remains the right home.** This spec is a §3/§7 semantics change, a §1
  schema addition, and (round-3, operator-authorized) a §8 carve-out — all to
  the exact contract ADR-0037 records; Bead 3 amends it in place following its
  own amendment convention (five prior dated amendments already live in the
  file). Creating a new ADR was considered and rejected: the single-home
  property ("this is the SINGLE home of the panel-gate enforced contract",
  internal/panel/gate.go:10-17) is itself part of the contract, and splitting
  the RC rule from the threshold rule it layers on would recreate the
  two-sources drift the ADR exists to prevent. **§6 gains an affirmative-
  evidence refusal branch:** a bead carrying a durable, unsatisfied refutation
  obligation does not fail open (a pristine bead still does). **§8 is amended,
  not merely re-affirmed:** the operator authorized a NARROW **audit-durability**
  carve-out (label softened from "anti-tamper" per O2 to match the delivered
  guarantee) for an applied refutation's audit — a refutation actively clears a
  finding (unlike the passive footguns §8 scopes out), so its audit is durable
  across retries and paths via the applied≡persisted+unioned marker + full-set
  every-path (incl. hatch) verified-discharge reconciliation + fail-closed
  `GetMetadata`/`MergeMetadata`. The general §8 posture is otherwise UNCHANGED
  (empty `reason` still clears; the panel is freely removable for a bead with
  NO refutation; verdicts/panel.json stay agent-writable; no signing/hashing —
  the "do not fix general forgeability" fence still holds; a party that never
  records a refutation is not the target). The §7 amendment records the round-5
  durable-obligation protocol (applied≡persisted+unioned marker, full-set
  every-path incl. hatch satisfy/verified-discharge/refuse reconciliation,
  fail-closed metadata).
- **ADR-0035 (agent error contract) — best choice, unchanged.** The new
  leg-9.5 Block routes through the existing `guard.NewFailure` wiring
  (panel_advisory.go:201-203) so the final line is a genuine `recovery:`
  command, and every new pre-close failure — the marker read-or-write-fails
  BLOCK (RC unresolved), the satisfy/discharge write failures, and the
  reconciliation Refuse (no-panel / unverified-Warn / GetMetadata read error /
  hatch with an unsatisfiable obligation) — constructs its own
  `guard.NewFailure` with an idempotent-rerun recovery line (restore the panel
  / the audit, then re-run), all conforming to the convention
  `internal/guard/recovery_convention_test.go` enforces. No amendment needed:
  the contract already covers new failure modes by construction.
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
  bd: the durable obligation + audits land on bead METADATA
  (`refutation_pending`, `panel_refuted`, `refutation_discharged`) via the
  `bead.MergeMetadata` seam, exactly like `panel_abandoned`. Three refinements
  from the plan-approve panel: (1) the obligation marker + audit are written
  BEFORE the bead CLOSE (not merely before the merge) — and applied≡persisted
  makes the marker part of the allow-decision, UNIONED so no re-panel round
  clobbers an older obligation — so the state authority never records a closed
  bead whose refutation obligation is unwritten; (2) EVERY completion path
  (panel-present, no-panel, AND hatch) reconciles the FULL obligation set
  against bead metadata (satisfy / verified-discharge / refuse) so it is
  enforced even when the panel artifact is gone or a hatch is used — bead
  metadata (the state authority) is the durable record, not the removable
  review artifact, which is exactly ADR-0023's division of labor; (3)
  `bead.GetMetadata`/`bead.MergeMetadata` are made fail-closed (the
  `internal/bead` execution-domain edit) so the single state authority's
  metadata can never be silently mis-read or corrupted by a transient read —
  a strengthening of ADR-0023's own invariant, fully within its spirit.
  ADR-0037's Domain(s) already include execution, so this edit needs no new
  ADR; the spec's `## Impacted Domains` is amended to declare execution /
  `internal/bead/**` (round-3 O3), recorded in the Decomposition amendment
  note.

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
- **`internal/complete` — gate wiring + durable-obligation protocol
  (`TestPanelRefuted…` e2e family + `completeGetMetadataFn` seam; discharge
  evidence comes from `reconcilePendingRefutations` re-tallying `panelReg.Dir`
  via `panel.Tally`, so the e2e fixtures drive the discharge/refuse split by
  writing the run-2 panel's verdict files — a flipped-to-APPROVE round for
  discharge, a still-RC round for refuse — not by stubbing a tally).** Reuse the
  real-temp-repo `setupPanelGateRepo` e2e suite (panel_gate_e2e_test.go:43-70)
  for AC1's end-to-end Block, AC2's passing-by-refutation completion (asserting
  BOTH `refutation_pending` and `panel_refuted`), the AC11 `panel_refuted`-
  write-fail fence, the **applied≡persisted** marker-write / union-read-fail ⟹
  RC-BLOCKS test, the **cross-run panel-removed** retry (run 2 REFUSES) +
  restore control, the **verified-vote-state DISCHARGE** test, the
  **Warn-paths-do-NOT-falsely-discharge** table (abandoned/missing-ref/
  transient with the RC still present), the **UNION multi-entry** reconcile-all
  test (discharge X + satisfy B, no clobber), the **HATCH-still-reconciles**
  table (env-skip / config-disabled / abandoned + read-error refuse), the
  **GetMetadata-read-error ⟹ REFUSE** test, the **pristine-no-panel §6
  fail-open** boundary, the post-write erasure-survival test, and the
  already-closed recovery-path audit test; and the seam-stub pattern of
  `TestWritePanelAuditMetadata_Abandoned`
  (panel_advisory_test.go:111-134, swapping `completeMergeMetadataFn` and the
  new `completeGetMetadataFn`) for the audit-content, the selective
  `panel_refuted`-only / `refutation_pending`-only / `refutation_discharged`-only
  failing stubs, and the abandon best-effort asymmetry control. The
  not-closed / refuse guarantee is asserted via a `closeBeadFn` spy plus the
  harness's `readStubMergeExecutor.completeCalled` flag (neither close nor
  merge ran).
- **`internal/bead` — fail-closed `MergeMetadata` + `GetMetadata` helper.**
  `TestMergeMetadata_FailClosedOnReadError` in `bdcli_test.go` (stubbing the
  `bd show` read via the package's existing test seam) asserts a read/parse
  failure returns an error and performs NO replace-write (existing keys
  preserved), while a clean empty read still merges; `TestGetMetadata` pins
  the read helper the reconciliation consults (parsed map on success, error
  surfaced on a read failure). These are the unit floor under the
  complete-side erasure-survival, discharge, and refuse tests. Plus the
  blast-radius grep confirming the caller taxonomy (already-fatal:
  `cmd/mindspec/repair.go` / `phase.EnsureMigrated`; warn; ignore) so making
  the read-error fatal regresses no class (round-2 F1/F3).
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
| AC11 (durable-obligation: applied≡persisted+unioned; TOTAL reconciliation; verified discharge; every path incl. hatches; fail-closed) | Bead 2 | `TestPanelRefuted_SatisfyWriteFailure_FailsCompletion` + `TestPanelRefuted_MarkerWriteFailure_Blocks` (marker-write/union-read-fail ⟹ RC BLOCKS) + `TestPanelRefuted_CrossRun_PanelRemoved_Refuses` (+ restore) + `TestPanelRefuted_CrossRun_NaturalResolution_Discharges` (verified) + `TestPanelRefuted_CrossRun_WarnPathDoesNotDischarge` (item 2) + `TestPanelRefuted_CrossRun_UnionReconcilesAll` (item 1) + `TestPanelRefuted_HatchStillReconcilesPendingObligation` (item 3) + `TestPanelRefuted_GetMetadataError_Refuses` + `TestPanelRefuted_PristineNoPanel_FailsOpen` + `TestPanelRefuted_AuditSurvivesLaterReadError` + `TestMergeMetadata_FailClosedOnReadError` + `TestGetMetadata`; `go test ./internal/complete -run 'PanelRefuted\|PanelGate' && go test ./internal/bead -run 'MergeMetadata\|GetMetadata'` |
| AC11 supporting — already-closed recovery-path audit (round-3 O3 minor) | Bead 2 | `TestPanelRefuted_RecoveryPath_AuditPresent` (pre-closed bead + live refutation panel → audit present after completion, step 7(B)(x)); `go test ./internal/complete -run 'PanelRefuted\|PanelGate'` |
| AC12 (duplicate slot files at latest round — chosen behavior stated + pinned; dedup extended into the audit path) | Bead 2 (step 2: byte-exact slot matching against the slot's tallied latest-round verdict; a REJECT under a near-duplicate slot is never cleared; same-slot+round entries collapse to one applied record) | `Dup`-named rows in `TestPanelGateDecision_Refutations`; `go test ./internal/panel -run 'Refut\|Dup'` |
