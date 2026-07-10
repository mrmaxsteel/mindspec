---
adr_citations:
    - ADR-0037
    - ADR-0035
    - ADR-0040
approved_at: "2026-07-09T20:43:14Z"
approved_by: user
bead_ids:
    - mindspec-r6hk.1
    - mindspec-r6hk.2
    - mindspec-r6hk.3
    - mindspec-r6hk.4
spec_id: 113-panel-verb-workflow-followups
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - internal/panel/gate.go
        - internal/instruct/panelstate.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 2
      key_file_paths:
        - plugins/mindspec/workflows/ms-panel.js
        - .claude/workflows/ms-panel.js
        - plugins/mindspec/workflow_test.go
    - depends_on: []
      id: 3
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - internal/panel/create.go
        - internal/panel/create_test.go
        - internal/config/config.go
        - cmd/mindspec/config.go
        - cmd/mindspec/config_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 4
      key_file_paths:
        - internal/config/config.go
        - internal/config/config_test.go
---
# Plan: 113-panel-verb-workflow-followups

Four small, bounded, independently-landable fixes to the panel-verb and
workflow surface 110/111/112 shipped: truthful non-bead staleness in the CLI
verbs (R1), a bare-`$` metacharacter rejection in `ms-panel.js` (R2), the
deferred writer-side `panel create --gate` stamping (R3), and the pinned
resolve-to-family reconciliation for `{model: "", family: <f>}` (R4). All
claims below are pinned against HEAD `3ddf2a1f`.

## Decomposition and land order

Four beads, 1:1 with the spec's four requirements (target count 3–5; the
spec-approval panel confirmed the clean decomposition). **No bead logically
depends on another** — `work_chunks` declares no `depends_on` edges and the
longest serial chain in the dependency graph is 1 (a single bead). One
**merge-scheduling note** that is deliberately NOT a dependency edge: Bead 1
(`mindspec-4d9m`, P2, load-bearing) and Bead 3 (`mindspec-zw81`) both edit
`cmd/mindspec/panel.go` and `cmd/mindspec/panel_test.go`. Land **Bead 1
first**, then Bead 3, purely to avoid a textual merge conflict in those two
files; Beads 2 and 4 touch disjoint files and can land at any point, in
parallel. Each bead is one PR-sized commit, verifiable by a single bead-panel
round.

**Reviewer/fixer scratch discipline (inherit into every bead brief and
reviewer prompt)**: reviewers and fixers MUST use ABSOLUTE `/tmp` scratch
paths (or `t.TempDir()` inside Go tests) for any file they create, and must
NEVER write relative `.mindspec/` (or any relative repo) paths — the agent
harness resets cwd between bash calls, and a relative write from a reviewer
has previously corrupted SIBLING worktrees, which `mindspec complete` then
auto-committed past review. Verify the bead worktree is CLEAN (`git status
--porcelain` empty) before every `mindspec complete`.

## Bead 1: cmd/mindspec — truthful non-bead staleness in `panel verify`/`panel tally` (mindspec-4d9m, R1, P2)

**The bug** (all line refs at `3ddf2a1f`): `resolvePanelGateFacts`
(`cmd/mindspec/panel.go:302-315`) leaves `beadID == ""` for a non-bead panel
(`reg.Panel.IsBead()` false), so `panel.ResolveGateFacts`
(`internal/panel/gate.go:372`) rev-parses the literal ref `bead/` (`"bead/"+
beadID`). That exits 1 → `ErrRefNotFound` → `facts.MissingRef` always true →
`PanelGateDecision` short-circuits at leg (5) (`gate.go:186-189`) with the
malformed Warn `"panel for  references branch bead/, which no longer exists —
assuming the merge already landed …"` (empty interpolations). Consequently
`panel verify` prints `PASS (advisory: …)` and `panel tally` exits 0 even
when the target advanced past `reviewed_head_sha`, and even with a REJECT or
zero verdicts on file.

**Precision (panel carry-forward F2)**: legs (2) unreadable `panel.json` and
(4) round-mismatch PRECEDE leg (5) in `PanelGateDecision`'s pinned order and
CAN still Block a non-bead panel today. The false `MissingRef` short-circuit
shadows legs (6) staleness and (8)–(10) incomplete/REJECT–hard_block/
threshold; the fix **un-shadows those legs** — it is not the case that the
verbs "never Block" a non-bead panel today.

**The fix — two halves, both confined to `cmd/mindspec/panel.go` (panel
carry-forward O1)**. `internal/panel`, `internal/instruct`, and
`internal/complete` receive a ZERO-byte diff in this bead. In particular,
R1's message hygiene lives in the CLI rendering layer, NOT in
`PanelGateDecision`'s shared `Decision.Message`: `internal/instruct`'s
`verdict()` (`internal/instruct/panelstate.go:99-136`) consumes `d.Message`
for non-bead panels (its non-bead facts leave staleness zero/inert but legs
(8)–(10) messages — including `RawMergeFence("")` — flow into the
`--panel-state` snapshot today), so mutating the shared message would break
the AC-global "instruct/complete tests pass unmodified" fence AND R1's own
falsification ("if any bead-panel decision or message changes; or if
`internal/complete`'s or `internal/instruct`'s non-bead handling changes").

Half 1 — **fact gathering (resolve-from-target, OQ1)**: in
`resolvePanelGateFacts`, when the panel is non-bead, override the
`panel.GateIO.RevParse` closure so it ignores the `"bead/"+beadID` ref
argument `ResolveGateFacts` passes and instead rev-parses the panel's
RECORDED `reg.Panel.Target` in `scanRoot`, routed through the existing
`revParseForPanelFn` seam (`panel.go:37-39`; extend its doc comment — it
becomes the staleness seam for non-bead verify/tally as well as `create`'s
write-time capture). The result feeds the SAME `PanelGateDecision` legs a
bead panel uses: stale target ⇒ Block (leg 6), genuinely-deleted target
(`exec.IsRefNotFound`, i.e. `errors.Is(err, gitutil.ErrRefNotFound)`) ⇒ the
missing-ref Warn (leg 5), transient git error ⇒ the honest transient Warn
(leg 5b), and legs (8)–(10) become reachable at an un-advanced target. The
bead path stays byte-identical (`exec.RevParseRef`, `"bead/"+beadID`).
`internal/panel/gate.go` is untouched — the GateIO seam was designed for
exactly this caller-supplied wiring (leaf invariant holds trivially).

Half 2 — **CLI-layer message hygiene**: a new pure function in
`cmd/mindspec/panel.go` (e.g. `sanitizeNonBeadDecision(d panel.Decision,
slug, target string) panel.Decision`), applied in `renderPanelVerify` and
`renderPanelTally` ONLY when the panel is non-bead. It NEVER changes
`d.Action` (the existing `TestPanelVerbs_DecisionIsPanelGateDecision` and
`TestPanelTally_ExitCodeTracksDecision` — which asserts bead-row `d.Message`
equality with `panel.PanelGateDecision` — must pass unmodified, which is
itself the bead-panel byte-identity falsifier). Deterministic rewrite rules
over the known message templates, each pinned by a table test:

- strip the exact empty-bead fence suffix `panel.RawMergeFence("")` (an
  exported function — the expected constant is computed, not copied); a
  non-bead panel is not complete-gated and must not carry `git merge bead/ `
  advice with an empty interpolation;
- a message containing `"references branch bead/,"` (the leg-5 constant when
  `beadID == ""`) is replaced wholesale with a target-naming missing-ref
  advisory, e.g. `panel <slug> target <ref> no longer exists — the reviewed
  ref was deleted; re-create the panel against a live ref (mindspec panel
  create <slug> --spec <id> --target <ref>)` (Warn action preserved);
- the leg-5b fragments `"panel for : "` and `"could not verify branch
  bead/"` are rewritten to `panel <slug>: ` / `could not verify target
  <ref>` (honest transient Warn, names the recorded target);
- no rule ever introduces a `mindspec complete` instruction.

`sanitizeNonBeadDecision` and the non-bead reviewed-head-sha line are applied
inside the render layer (`renderPanelVerify`/`renderPanelTally`), gated on
non-bead — NEITHER function's signature changes, and because every pinned
render test (`TestPanelVerbs_DecisionIsPanelGateDecision`,
`TestPanelTally_ExitCodeTracksDecision`,
`TestPanelVerify_MatchesGateAndWritesNothing`) uses BEAD fixtures, the
non-bead-gated rewrite never fires for them and they stay unmodified. The one
piece that cannot live in a shared helper is the non-bead Block's exit
recovery line: `tallyExitAction(d, slug)` is a pinned 2-arg helper (see step
4), so the non-bead recovery is rendered in the `panelTallyCmd` RunE handler
instead, leaving that helper and its test byte-identical.

**Steps**

1. `resolvePanelGateFacts` (`cmd/mindspec/panel.go:302-315`): compute the
   non-bead branch — when `beadID == ""`, supply a `GateIO.RevParse` closure
   that rev-parses `strings.TrimSpace(reg.Panel.Target)` in the passed
   scanRoot via `revParseForPanelFn`, ignoring the `"bead/"`-derived ref
   argument; when `reg.Panel` is unparsed (`reg.Err != nil` / zero struct)
   or `Target` is empty, return a plain non-`ErrRefNotFound` error (e.g.
   `panel.json records no target ref`) so the facts surface as the honest
   transient `GitErr` Warn — for an unreadable registration leg (2) Blocks
   first regardless. Bead path byte-identical. `IsRefNotFound` stays
   `exec.IsRefNotFound`. Subprocess budget unchanged: one rev-parse; the
   worktree resolver still short-circuits on `beadID == ""` inside
   `panelBeadWorktreePath` BEFORE `panelWorktreeListFn` is invoked
   (ADR-0030). Update `revParseForPanelFn`'s doc comment.
2. Add `sanitizeNonBeadDecision` with the four rewrite rules above; wire it
   into `renderPanelVerify` and `renderPanelTally` gated strictly on
   non-bead (`facts.Res.Panel == nil || !facts.Res.Panel.IsBead()`). It
   never mutates `Action`, and neither render function's signature changes;
   the non-bead gate means the bead-fixture pinned tests
   (`TestPanelVerbs_DecisionIsPanelGateDecision`,
   `TestPanelTally_ExitCodeTracksDecision`,
   `TestPanelVerify_MatchesGateAndWritesNothing`) never exercise the rewrite
   and stay unmodified.
3. `renderPanelVerify`'s reviewed_head_sha line (`panel.go:361-369`): the
   `facts.MissingRef` case renders `(target <ref> no longer exists)` for a
   non-bead panel instead of `(target branch no longer exists — assumed
   merged)`; bead rendering byte-identical.
4. Non-bead recovery-line hygiene WITHOUT changing any pinned helper's
   signature. `tallyExitAction(d, slug)` (`panel.go:526-537`) stays **2-arg
   and byte-identical** — its only test caller,
   `TestPanelTally_ExitCodeTracksDecision` (`panel_test.go:429`, bead
   fixtures), must remain literally unmodified, and `panel.Decision` is
   `{Action, Message}` only (no field to carry non-bead-ness) while
   `internal/panel` takes a zero-byte diff, so threading a flag through
   `tallyExitAction` is impossible without editing that pinned test. Instead,
   branch in the `panelTallyCmd` RunE handler (`panel.go:~189-207`), which
   already holds `reg` and therefore `reg.Panel.IsBead()`:
   - **bead panel** (`reg.Panel.IsBead()` true): exactly today —
     `return tallyExitAction(d, slug)`; the bead recovery string
     (`… then mindspec complete <bead>`) is unchanged.
   - **non-bead panel**: the handler applies the same `d.Action`→exit
     contract but supplies the non-bead recovery line ITSELF — a
     target-naming re-panel instruction
     (`re-run the panel: mindspec panel create <slug> --round <N+1> --spec
     <id> --target <ref>`) with NO `then mindspec complete <bead>` clause —
     via a handler-local Block path (its own `guard.NewFailure` over the
     already-sanitized `d.Message`), and NEVER routes the non-bead Block
     through `tallyExitAction`'s bead-templated recovery (whose `<bead>`
     literal would otherwise re-introduce the forbidden `mindspec complete`
     string on a non-bead panel). Warn/Allow map to exit 0 as today.
   This keeps `mindspec complete <bead>` out of every non-bead rendering
   while leaving the pinned `tallyExitAction` helper and its test byte-for-byte
   untouched — the relocation is purely WHERE the non-bead recovery is
   rendered (the RunE handler, not inside a pinned helper), not a change to
   the approved design.
5. Tests in `cmd/mindspec/panel_test.go`, all via the existing shared infra
   (`mkPanelTestRoot`, `writePanelFixture`, `snapshotTree`,
   `stubWorktreeListEmpty`, `withTestChdir`, stubbed `revParseForPanelFn`):
   - `TestPanelVerify_NonBeadStaleness`: non-bead fixture (BeadID nil,
     `Target: "spec/113-x"`, recorded SHA A, 1 expected reviewer + APPROVE
     verdict); stub returns SHA B and RECORDS the requested ref — assert the
     ref rev-parsed IS the recorded target (never `bead/`), output contains
     `BLOCK`, contains no `PASS`, no `references branch bead/`, no
     `git merge bead/`, no `mindspec complete`, and names the target.
   - `TestPanelTally_NonBeadRejectBlocks`: target NOT advanced (stub returns
     the recorded SHA) + a REJECT verdict file → `rootCmd.Execute` returns a
     non-nil error (non-zero exit) whose text has a final recovery line
     (`guard.HasFinalRecoveryLine`) WITHOUT `mindspec complete`, and no
     empty-interpolation fence.
   - `TestPanelTally_NonBeadStaleBlocks`: advanced target → non-zero exit.
   - `TestPanelVerify_NonBeadMissingTargetRef`: stub returns an error
     wrapping `gitutil.ErrRefNotFound` → Warn advisory naming the target,
     no `references branch bead/,`, exit 0 (verify is read-only).
   - `TestSanitizeNonBeadDecision`: table test that builds messages by
     calling the REAL `panel.PanelGateDecision` over `beadID == ""` fact
     rows for legs (2), (5), (5b), (6), (8), (9), (10), then asserts the
     sanitized output bans all three malformed patterns, preserves `Action`
     byte-for-byte, and names the target on the (5)/(5b) paths — so the pin
     cannot drift from the real gate templates.
6. Doc-sync (workflow): `.mindspec/domains/workflow/interfaces.md` § panel
   verbs — record that non-bead staleness now resolves from the recorded
   `panel.json.target` (spec 113 R1), and keep the verb characterization
   precise: `panel verify` is READ-ONLY (writes nothing, always exits 0 — a
   report, not a gate); `panel tally` is ADVISORY-BUT-BLOCK-CAPABLE (exit
   code tracks the decision; non-zero on Block). Do not describe both as
   "read-only/advisory".

**Verification**
- [ ] `go test ./cmd/mindspec -run 'TestPanelVerify_NonBead|TestPanelTally_NonBead|TestSanitizeNonBeadDecision' -v` passes
- [ ] `go test ./cmd/mindspec` passes with `TestPanelVerbs_DecisionIsPanelGateDecision` and `TestPanelTally_ExitCodeTracksDecision` UNMODIFIED (bead-panel decisions and messages byte-identical)
- [ ] `tallyExitAction`'s signature is still `tallyExitAction(d panel.Decision, slug string) error` (2-arg, byte-identical) and its sole caller in `panel_test.go` (`tallyExitAction(d, "demo")`) is unchanged — the non-bead recovery line is rendered in the `panelTallyCmd` RunE handler, not by threading args through this pinned helper
- [ ] `git show --name-only HEAD` lists NO file under `internal/panel/`, `internal/instruct/`, or `internal/complete/` (the R1 consistency fence as a zero-diff fact)
- [ ] `go test ./internal/panel ./internal/instruct ./internal/complete` passes with zero test files modified (AC-global fence)
- [ ] AC1 shell sequence in a scratch git repo with the built binary: `mindspec panel create p113 --spec <id> --target <branch>` (no `--bead`) → one more commit on `<branch>` → `mindspec panel verify p113` output has no `PASS` and no `references branch bead/`; `mindspec panel tally p113` exits non-zero; with the target un-advanced but a REJECT verdict file present, tally still exits non-zero
- [ ] `go build ./...` exits 0

**Acceptance Criteria**
- [ ] AC1 (spec): stale non-bead target ⇒ verify reports non-PASS and tally exits non-zero; REJECT at an un-advanced target ⇒ tally exits non-zero; no non-bead rendering emits `references branch bead/,`, an empty-interpolation `git merge bead/ `, or a `mindspec complete <bead>` instruction; all pinned by the new `cmd/mindspec` tests through the `revParseForPanelFn`/executor seams
- [ ] Every bead-panel decision and message is byte-identical (existing pins unmodified); `internal/complete`/`internal/instruct` non-bead handling untouched (zero diff)
- [ ] Non-bead subprocess budget unchanged: one rev-parse, no worktree list (ADR-0030)

**Depends on**
None (land FIRST among the four — P2 load-bearing; Bead 3 lands after it only to avoid a textual conflict in `cmd/mindspec/panel.go`, not because of any logical dependency)

## Bead 2: ms-panel.js — SHELL_METACHAR_RE rejects a bare `$`, mirror byte-identical (mindspec-lczt, R2)

`plugins/mindspec/workflows/ms-panel.js:138` has `const SHELL_METACHAR_RE =
/[\x60;|&\n]|\$\(/;` — it rejects the `$(` digraph but not a bare `$`, so
`$HOME`/`${x}`/`$x` survive `validateShellSafe` (line 140-144) into the
shell-executed template `buildCommand` assembles (line 78). Fix: fold `$`
into the character class — `/[\x60;|&\n$]/` — which is strictly monotone
(anything the old regex matched still matches: the class chars are retained
and every `$(` contains `$`). The `\x60`-not-literal-backtick comment above
the constant (line 133-137) is preserved. The `.claude/workflows/ms-panel.js`
mirror (byte-identical today, `cmp`-verified) is regenerated in the same
commit.

**Steps**

1. Edit `plugins/mindspec/workflows/ms-panel.js:138`: change the regex to
   `/[\x60;|&\n$]/` and extend the adjacent comment with one line recording
   the spec-113-R2 tightening (bare `$` closes variable-expansion survival;
   `$(` is subsumed by the class).
2. Regenerate the mirror byte-identically:
   `cp plugins/mindspec/workflows/ms-panel.js .claude/workflows/ms-panel.js`.
3. Add `TestMsPanelWorkflow_ShellMetacharRejectsBareDollar` to
   `plugins/mindspec/workflow_test.go`: assert the embedded
   `WorkflowFiles()["ms-panel.js"]` contains the exact declaration line
   `const SHELL_METACHAR_RE = /[\x60;|&\n$]/;` exactly once (a string-level
   floor pin in the same style as `TestMsPanelWorkflow_AllowedCLIExactSet`;
   Go cannot execute the JS regex — the behavioral matrix runs under node in
   verification).
4. Run the existing floor tests unmodified: `TestWorkflowFiles_EmbedsMsPanel`
   (embed == plugin source) and `TestMsPanelWorkflow_AllowedCLIExactSet`
   (spec 111's exact-set/chokepoint pins).

**Verification**
- [ ] `node /tmp/check-metachar.js` exits 0 — the scratch script (ABSOLUTE
      `/tmp` path, per the scratch discipline above) reads
      `plugins/mindspec/workflows/ms-panel.js`, extracts
      `SHELL_METACHAR_RE`, asserts it MATCHES each of `$HOME`, `${x}`, `$x`,
      `` a`b ``, `a;b`, `a|b`, `a&b`, `"a\nb"`, `$(x)`, and does NOT match
      the clean negative controls `p113`, `113-panel-verb-workflow-followups`,
      `bead/mindspec-x.1` (monotone: nothing legitimate newly rejected)
- [ ] `cmp plugins/mindspec/workflows/ms-panel.js .claude/workflows/ms-panel.js` exits 0
- [ ] `go test ./plugins/mindspec` passes (both named floor tests plus the new bare-`$` pin)
- [ ] `go build ./...` exits 0

**Acceptance Criteria**
- [ ] AC2 (spec): the metachar matrix is green under node, the two copies are byte-identical, and `go test ./plugins/mindspec` passes with both existing floor tests unmodified
- [ ] The tightening is strictly monotone — every input the old regex rejected stays rejected (the positive rows of the node matrix cover all old classes)

**Depends on**
None (fully independent; disjoint files)

## Bead 3: `panel create --gate <name>` — deferred writer-side stamping (mindspec-zw81, R3)

Spec 112 shipped the decision-inert `Panel.Gate` field
(`internal/panel/panel.go:91`, R9 stable contract: name `gate`, type string,
`omitempty`, parse-lenient) and the gate-scoped resolvers
(`PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`,
`internal/config/config.go:750/766`), but deferred the WRITER. Today
`panel.CreateInput` (`internal/panel/create.go:51-74`) has no Gate field,
`panel create` has no `--gate` flag (`cmd/mindspec/panel.go:210-218`), and
creation stamps only the global defaults (`panel.go:130-131`) — so a
CLI-created non-bead panel has empty `gate` + `isBead == false` and
`PanelGateAdvisoryDefault` (`config.go:832`) returns `(0, false)`,
advisory-skipped. This bead completes the loop. Land AFTER Bead 1 merges
(same-file textual adjacency only).

**Steps**

1. `internal/panel/create.go`: add `Gate string` to `CreateInput` (doc
   comment: decision-inert recorded metadata per the spec-112-R9 stable
   contract; `""` means the caller omitted `--gate` and, via the existing
   `omitempty` on `Panel.Gate`, no `gate` key is written) and set
   `p.Gate = in.Gate` in `Create` (`create.go:119-127`). Plain value only —
   the leaf stays free of `internal/config` and git imports. Add
   `TestCreate_StampsGate` to `internal/panel/create_test.go`: `Create` with
   `Gate: "final_review"` yields a `panel.json` containing
   `"gate": "final_review"`; `Gate: ""` yields bytes with NO `gate` key,
   byte-identical to a pre-change `Create` over the same input.
2. `cmd/mindspec/panel.go`: register the flag —
   `panelCreateCmd.Flags().String("gate", "", ...)` — and validate BEFORE
   any filesystem write or root/config resolution side effect, in the same
   block as the existing `--bead`/`--target` checks: (a)
   `rejectControlBytes("--gate", gate)` (same control-byte discipline); (b)
   when non-empty, membership by ITERATING `config.PanelGateKeys`
   (`config.go:101` — the single enum declaration; never a second literal
   copy); a value outside the enum returns `guard.NewFailure` (non-zero
   exit) whose message and recovery line each name all five keys via
   `strings.Join(config.PanelGateKeys, ", ")` (ADR-0035), e.g. recovery:
   `pass one of spec_approve, plan_approve, bead, final_review, adhoc to
   --gate`. Provenance note for the message wording: the review-event vs
   approval-act (loop.gate_authority) vocabulary disambiguation carries over
   from SPEC 112 R1's PanelGateKeys contract — not from ADR-0034 (see ADR
   Fitness).
3. Defaults resolution in the create handler: when `--gate` is set,
   `ExpectedReviewers` ← `cfg.PanelGateExpectedReviewers(gate)` and
   `ApproveThresholdExpr` ← `cfg.PanelGateApproveThresholdExpr(gate)`
   (resolver errors are unreachable post-validation but still returned
   defensively), plus `CreateInput.Gate = gate`; when absent, EXACTLY
   today's `cfg.PanelExpectedReviewers()`/`cfg.PanelApproveThresholdExpr()`
   calls with `Gate: ""` — preserving the 112-R9 byte-identical-when-absent
   contract.
4. Tests in `cmd/mindspec/panel_test.go` (shared infra as Bead 1; add
   `"gate"` to `resetPanelCreateFlags`' flag list):
   - `TestPanelCreate_GateStampsPerGateDefaults`: config whose
     `panel.gates.final_review` declares a distinct reviewer mix +
     threshold; `panel create … --gate final_review` writes
     `"gate": "final_review"` and the GATE-resolved
     `expected_reviewers`/raw `approve_threshold` (asserted equal to
     `cfg.PanelGateExpectedReviewers("final_review")` /
     `cfg.PanelGateApproveThresholdExpr("final_review")`).
   - `TestPanelCreate_GateInvalidRejectedBeforeWrite`: `--gate nonsense` →
     `Execute` returns an error naming ALL FIVE keys with a final recovery
     line, and `snapshotTree` before/after is identical (nothing written);
     include a control-byte `--gate` row.
   - `TestPanelCreate_GateOmittedByteIdentical`: `panel create` without
     `--gate` produces `panel.json` bytes with no `gate` key, equal to the
     expected marshal of today's field set (the 112-R9 contract pin).
5. Advisory read-through (AC3): a `cmd/mindspec` test exercising
   `PanelGateAdvisoryDefault("final_review", false)` through the REAL call
   site `reviewerCountNotesFor` (`cmd/mindspec/config.go:535-541`): root
   with a `panel.gates.final_review` config and a CLI-created
   `--gate final_review` panel whose recorded count is then made to differ
   from the gate's current default → the note is EMITTED comparing against
   the final_review default (previously: empty gate + non-bead →
   `(0, false)` → skipped). Decision-inertness needs no new pin — spec
   112's `TestPanel_GateFieldDecisionInert` already covers it, and this bead
   never touches `PanelGateDecision`/`ApproveThreshold()`.
6. Doc-sync (workflow): `.mindspec/domains/workflow/interfaces.md` § panel
   verbs — document `--gate` (optional; validated against the five-key
   enum; stamps the decision-inert `gate` field and that gate's
   creation-time defaults via the 112 R3 resolvers; omitted ⇒ byte-identical
   legacy behavior).

**Verification**
- [ ] `go test ./cmd/mindspec -run 'TestPanelCreate_Gate' -v` passes
- [ ] `go test ./internal/panel -run 'TestCreate_StampsGate' -v` passes; `go test ./internal/panel` green (incl. `TestPanel_GateFieldDecisionInert` unmodified)
- [ ] `go list -deps ./internal/panel | /usr/bin/grep -q 'internal/config'` returns non-zero (leaf invariant intact)
- [ ] AC3 shell sequence with the built binary: `mindspec panel create p113g --spec <id> --target <ref> --gate final_review` → `panel.json` contains `"gate": "final_review"` and its `expected_reviewers`/`approve_threshold` match `mindspec config show --gate final_review --json`; `mindspec panel create x --spec s --target t --gate nonsense` exits non-zero naming all five keys; flag-less create yields a `panel.json` with no `gate` key
- [ ] `/usr/bin/grep -rn '"spec_approve"' cmd/mindspec internal/panel` shows no second enum-slice literal (the CLI iterates `config.PanelGateKeys`)
- [ ] `go build ./...` and `go test ./cmd/mindspec` exit 0

**Acceptance Criteria**
- [ ] AC3 (spec): gate stamped; per-gate defaults resolved through the 112 R3 resolvers; invalid gate rejected before any write with the five-key recovery line; omitted `--gate` byte-identical to today; advisory read-through pinned via `reviewerCountNotesFor`
- [ ] The gate field stays decision-inert end to end and the R9 stable contract (name/type/`omitempty`/parse-lenience) is unaltered

**Depends on**
None logically (schedule after Bead 1 merges — textual adjacency in `cmd/mindspec/panel.go` only; NOT a dependency edge)

## Bead 4: internal/config — pin resolve-to-family for `{model: "", family: <f>}` (mindspec-cthj, R4)

Spec 112 R4's text ("or an empty-string `model`") disagrees with 112 R1
("family and no model is valid") for exactly `{model: "", family: <f>}`. The
CODE already resolves to family: `Reviewer.model()`
(`internal/config/config.go:139-144`) returns `Family` when `Model == ""`,
and `validateReviewerEntries` (`config.go:540-550`) refuses only the
NEITHER-set case (`r.model() == ""`). Per OQ2 (resolved: resolve-to-family)
this bead is a documenting comment + a pinning test — NO struct or unmarshal
change (`Model` stays a plain `string`; no `*string`, no custom
`UnmarshalYAML`).

**Steps**

1. Extend the doc comments at both places the ambiguity lives —
   `Reviewer.model()` (`config.go:139-144`) and `validateReviewerEntries`
   (`config.go:535-550`) — recording: `{model: "", family: <f>}` is VALID
   and resolves to the family string (the typed `string` cannot distinguish
   absent from empty, and 112 R1 blesses family-fallback); this resolution
   **supersedes spec 112 R4's "or an empty-string `model`" phrase** (say so
   verbatim, citing spec 113 R4/OQ2); `{model: ""}` with no family remains
   refused via the neither-set branch.
2. Add `TestLoad_EmptyStringModel` to `internal/config/config_test.go`:
   (a) a config whose `panel.reviewers` is `[{model: "", family: "codex"},
   {family: "claude"}]` (two entries — the sum≥2 floor) passes `Load`, and
   `PanelGateReviewerSlots("bead")` expands the first entry to
   `Model == "codex"`; (b) `panel.reviewers: [{model: ""}, {family:
   "claude"}]` (empty model, NO family) fails `Load` via the neither-set
   branch with an error naming `panel.reviewers[0]` and carrying the
   ADR-0035 `recovery:` line.
3. Confirm the no-code-change fence: the bead commit touches ONLY
   `internal/config/config.go` (comments) and
   `internal/config/config_test.go`; the `Reviewer` struct and every
   validation branch are behaviorally unchanged.

**Verification**
- [ ] `go test ./internal/config -run 'TestLoad_EmptyStringModel' -v` passes
- [ ] `go test ./internal/config` passes (existing 112 validation tests unmodified)
- [ ] `git show --name-only HEAD` lists only `internal/config/config.go` and `internal/config/config_test.go`
- [ ] `/usr/bin/grep -q 'supersedes' internal/config/config.go` exits 0 and `/usr/bin/grep -q 'Model  string' internal/config/config.go` exits 0 (comment present; field type untouched — no `*string`)
- [ ] `go build ./...` exits 0

**Acceptance Criteria**
- [ ] AC4 (spec): `{model: "", family: "codex"}` loads and slot-expands to model `codex`, pinned by `TestLoad_EmptyStringModel`; `{model: ""}` with no family is refused with a recovery line; the superseding comment is present; no struct/unmarshal change

**Depends on**
None (fully independent; disjoint files)

## ADR Fitness

Impacted domains: **workflow** (`cmd/mindspec/panel.go`, `internal/panel`,
`plugins/mindspec/workflows/ms-panel.js` + `.claude/workflows` mirror) and
**core** (`internal/config`). Every ADR the spec's Touchpoints section names
was evaluated against the real tree; three genuinely constrain the
implementation and are cited (`adr_citations`), two are evaluated in prose
below and deliberately NOT cited (one because its declared domains do not
intersect this spec's, one because its provenance attribution needed
correcting). **All remain best-choice; no divergence is planned; no new ADR
is needed.**

- **ADR-0037 — Panel Gate as Enforced Contract** (Accepted; Domain(s):
  workflow, execution — covers workflow). The binding constraint on Bead 1:
  `PanelGateDecision` stays the SINGLE decision home ("identical decision
  over identical facts"). This plan adheres by construction: R1 changes only
  the FACTS the CLI verbs gather for a non-bead panel (a caller-side
  `GateIO.RevParse` closure — the seam ADR-0037's leaf design exists for)
  plus CLI-layer message rendering that never alters `Decision.Action`;
  `internal/panel/gate.go` receives a zero-byte diff, so the Allow/Block
  matrix for bead panels and for `mindspec complete` cannot have changed.
  Bead 3 stamps the `gate` field ADR-0037's second amendment note (112)
  already records as decision-inert metadata; §§3/6/8 are untouched and the
  inertness pin (`TestPanel_GateFieldDecisionInert`) survives unmodified.
  ADR-0037 remains the correct frame — **adhere**.
- **ADR-0035 — Agent Error Contract** (Accepted; Domain(s): workflow,
  execution, core — covers both impacted domains). Drives three surfaces:
  Bead 1's corrected non-bead messages (no malformed `bead/<empty>`
  fragments; the tally Block recovery drops the false `then mindspec
  complete <bead>` advice for a panel that is not complete-gated and instead
  names a genuine re-panel command); Bead 3's invalid-`--gate` rejection (a
  `guard.NewFailure` whose recovery line enumerates all five gate keys, with
  `rejectControlBytes` running first so an attacker-controlled value never
  reaches a recovery line unquoted); Bead 4's surviving neither-set refusal
  keeps its existing recovery line. **Adhere** — the contract is
  strengthened, not bent.
- **ADR-0040 — Orchestration Layering Ratchet** (Accepted; Domain(s): core,
  workflow — covers both impacted domains). The panel verbs are the ADR-0040
  portability contract's CLI half (`cmd/mindspec/panel.go` header); R1 keeps
  that surface TRUTHFUL for the non-bead panels 112's per-gate config makes
  first-class — a lying L2 surface is exactly the drift the ratchet exists
  to prevent. R2 hardens the L3 workflow runner without changing its L2
  contract (the `ALLOWED_CLI` exact-set floor and `buildCommand` chokepoint
  are untouched; the workflow's arg schema gains nothing — `--gate`
  pass-through stays explicitly out of scope per the spec). R3 is the
  ratchet's writer-side completion: per-gate creation defaults move from
  operator memory into the recorded `panel.json`. **Adhere**.
- **ADR-0030 — Executor Boundary** (Accepted; Domain(s): execution,
  validation, lifecycle, lint — evaluated but NOT cited: its declared
  domains do not intersect this spec's workflow/core, so a citation would be
  flagged architecturally irrelevant by the plan gate; the evaluation lives
  here). The relevant constraint is the per-verb subprocess budget for R1's
  non-bead staleness rev-parse: the fix REPLACES the doomed `bead/`
  rev-parse with one target rev-parse through the same executor seam
  (`revParseForPanelFn` → `newExecutor(root).RevParseRef`), and the
  worktree/dirty legs stay bead-only (`panelBeadWorktreePath` returns `""`
  for `beadID == ""` before `panelWorktreeListFn` can spawn `bd`). Net
  subprocess count for a non-bead verify/tally: one, exactly as today.
  **Adhere; budget unchanged**.
- **ADR-0034 — Ceremony Collapse** (Accepted; Domain(s): workflow —
  evaluated but not cited, with a provenance correction). The spec's ADR
  Touchpoints section attributes the gate-vocabulary disambiguation
  (review-event keys `spec_approve`/…/`adhoc` vs `loop.gate_authority`'s
  approval-act vocabulary) to ADR-0034; that attribution is imprecise —
  ADR-0034 is the ceremony-collapse decision and does not carry the enum.
  The disambiguation's actual provenance is **spec 112 R1's
  `PanelGateKeys` contract** (`internal/config/config.go:93-101`, whose doc
  comment records the deliberate vocabulary split). This plan re-attributes
  accordingly: Bead 3's rejection message preserves 112's disambiguation
  wording and cites spec 112, not ADR-0034 (the spec itself is not edited —
  re-opening 110/111/112 texts is a spec non-goal). No ADR-0034 obligation
  binds this work beyond that correction.

## Testing Strategy

**Unit — `internal/config` (core)**: `TestLoad_EmptyStringModel` (Bead 4) in
`internal/config/config_test.go`, exercising `Load` + the
`PanelGateReviewerSlots` chain over inline YAML fixtures written to
`t.TempDir()` roots — the package's existing pattern.

**Unit — `internal/panel` (workflow)**: `TestCreate_StampsGate` (Bead 3) in
`internal/panel/create_test.go` over `t.TempDir()` panel dirs. R1
deliberately adds NO `internal/panel` test because it adds no
`internal/panel` code; the package's existing decision/create/tally suites
must pass byte-unmodified (that is itself an R1 fence assertion).

**cmd/mindspec command-level (workflow)**: all new R1/R3 tests drive the
real cobra commands via `rootCmd.Execute()` over scratch roots built with
the EXISTING shared infra in `cmd/mindspec/panel_test.go` +
`testhelpers_test.go`: `mkPanelTestRoot` (`.mindspec`-marked `t.TempDir()`
with optional config.yaml), `writePanelFixture` (direct
`review/<slug>/panel.json`), `snapshotTree` (wrote-nothing proofs),
`resetPanelCreateFlags` (cobra flag bleed — extended with `"gate"`),
`stubWorktreeListEmpty` (`panelWorktreeListFn` stub, no real `bd`), and
`withTestChdir`. Git is stubbed at the `revParseForPanelFn` seam — including
returning errors wrapping `gitutil.ErrRefNotFound` so the real executor's
`IsRefNotFound` classification (`errors.Is`) is exercised end-to-end — so no
test spawns a real git subprocess for staleness. The pure renderers
(`renderPanelVerify`/`renderPanelTally`/`sanitizeNonBeadDecision`/
`tallyExitAction`) additionally get direct table tests over facts built with
`buildResult`, with messages sourced from the REAL `panel.PanelGateDecision`
so pins cannot drift from the gate templates.

**plugins/mindspec (workflow)**: `go test ./plugins/mindspec` — the two
existing floor tests (`TestWorkflowFiles_EmbedsMsPanel`,
`TestMsPanelWorkflow_AllowedCLIExactSet`) unmodified plus the new
`TestMsPanelWorkflow_ShellMetacharRejectsBareDollar` string pin; behavioral
regex matrix via a node scratch script at an ABSOLUTE `/tmp` path (AC2);
mirror identity via `cmp`.

**Whole-spec gate (AC-global, every bead's final check)**:
`go build ./... && go test ./cmd/mindspec ./internal/panel ./internal/config
./internal/complete ./internal/instruct ./plugins/mindspec` — with
`internal/complete`'s and `internal/instruct`'s existing panel tests passing
UNMODIFIED (the R1 consistency fence made observable).

**Isolation discipline**: every test uses `t.TempDir()` roots +
`withTestChdir`; every scratch script uses an absolute `/tmp` path; no test
or reviewer writes a relative `.mindspec/` path (the sibling-worktree
corruption class). Bead worktree must be clean before `mindspec complete`.

## Provenance

- **AC1 (R1) → Bead 1**: `TestPanelVerify_NonBeadStaleness`,
  `TestPanelTally_NonBeadRejectBlocks`, `TestPanelTally_NonBeadStaleBlocks`,
  `TestPanelVerify_NonBeadMissingTargetRef`, `TestSanitizeNonBeadDecision`
  (steps 1–5) plus the AC1 scratch-repo shell sequence and the zero-diff
  fence checks in Bead 1's verification.
- **AC2 (R2) → Bead 2**: the `/tmp/check-metachar.js` node matrix, the `cmp`
  mirror check, and `go test ./plugins/mindspec` (steps 1–4 / verification).
- **AC3 (R3) → Bead 3**: `TestPanelCreate_GateStampsPerGateDefaults`,
  `TestPanelCreate_GateInvalidRejectedBeforeWrite`,
  `TestPanelCreate_GateOmittedByteIdentical`, `TestCreate_StampsGate`, the
  `reviewerCountNotesFor` advisory read-through test, and the AC3 shell
  sequence (steps 1–5 / verification).
- **AC4 (R4) → Bead 4**: `TestLoad_EmptyStringModel` plus the
  comment-presence and no-struct-change greps (steps 1–3 / verification).
- **AC-global → all beads**: the whole-spec gate command in every bead's
  verification; Bead 1 additionally proves `internal/complete`/
  `internal/instruct`/`internal/panel` carry zero diff and unmodified tests.
