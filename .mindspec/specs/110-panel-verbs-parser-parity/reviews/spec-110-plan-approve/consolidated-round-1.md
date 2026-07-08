# spec-110-plan-approve — consolidated round-1 changes

Tally: 3 APPROVE (F2 0.87, O2 0.90, O3 0.88) / 6 REQUEST_CHANGES (F1 0.78, F3 0.83, O1 0.80, G1 0.82, G2 0.86, G3 0.86) / 0 REJECT / no hard_block. Threshold 8/9 not met → fix round. All asks are plan-text amendments (added constraints/tests/verification wording) — **no design rework**; the DAG, bead boundaries, ADR set, and coverage were approved by the structural lenses.

22 raw asks deduped to 17 items, grouped by bead. "(slots)" = who asked; convergent asks merged.

## Bead 2 — validate parity

1. **Severity pin (F3.1) — must.** In both new ValidateSpec tests: failing cases assert the issue is `SevError` (or `vr.HasFailures()==true`); tolerated cases assert `!vr.HasFailures()`. Code-only scanning passes on an `AddWarning` implementation while `ApproveSpec` (blocks on SevError only, validate.go:43-45) enforces nothing — R5's "identical severity" contract currently unverified.
2. **Widen the R6 regex to filename-form anchors (F1.1) — must.** `\[(ADR-\d{4})[^\]]*\]\([^)]+\)` — the repo convention in merged specs 085–094 writes `[ADR-0031-doc-sync-gate.md](…)`; the planned regex requires `]` after the 4 digits and is blind to it (an anchored link to a nonexistent ADR in that form would pass). Add a filename-form pair to `TestValidateSpec_ADRTouchpointExtractionBoundary` (existing passes / nonexistent fails). F1 verified widening is parity-safe today (all anchored IDs across specs resolve).
3. **Self-check must run the bead's build (F3.2) — must.** Replace `~/.local/bin/mindspec validate spec 110-…` with `go build -o /tmp/ms110b2 ./cmd/mindspec && /tmp/ms110b2 validate spec 110-…`. The installed binary can't fail on B2's code.

## Bead 3 — instruct delegation

4. **Empty Allow Message (O1.1 + O1.2) — must.** `PanelGateDecision`'s Allow return has NO Message (gate.go:142, :258), so "use Decision.Message as the reason" yields an empty PASS reason. Pin the PASS-reason synthesis (e.g. from `res.Approves`/threshold), AND reconcile `TestPanelStateEntry_Verdict` (name matches B3's `-run 'PanelState…'` filter; its `at_threshold_fresh`/`above_threshold_fresh` rows assert reason substrings) — either the synthesized reason satisfies them or the plan explicitly updates those two `wantReason` values instead of claiming the tests "stay green" unchanged.
5. **Delegation-table completeness (F3.4) — must.** Add round-mismatch, malformed-panel.json (`Res.PanelErr`), and missing-ref/transient-GitErr Warn rows to `TestPanelStateVerdict_DelegatesToPanelGateDecision` — the deleted matrix has a RoundMismatch branch (panelstate.go:113) no row exercises. Also make the "structurally assert verdict() no longer carries its own logic" clause falsifiable (e.g. assert the old matrix's distinctive literals are gone from panelstate.go and PanelGateDecision is referenced) or drop it from AC6.

## Bead 4 — verb tree

6. **Close the RunE exit-code hole (F1.2 + F3.3) — must.** The contract test pins the pure renderers, but a RunE that re-derives Allow/Block from `res` passes every planned gate yet exits 0 on stale-SHA (lola-f4a8 class), round-mismatch, and hard_block Blocks. Route the RunE through a single decision-to-exit helper asserted over the SAME GateFacts table as the contract test (or add Block-for-staleness + hard_block rows to `TestPanelTally_ExitCodeTracksDecision` via the rev-parse seam), and assert the rendered PASS/BLOCK token derives from `d.Action`, not recomputed.
7. **Branch-complete facts table (F3.3) — must.** The shared contract-test table must span gate.go branches (2)–(10): malformed-registration, round-mismatch, stale-SHA, dirty-tree, incomplete, REJECT/hard_block, sub-/at-threshold, and the Warn variants (abandoned, missing-ref, transient GitErr). A 3-row Allow/Block/Warn table is tautological. Ideally one fixture table shared with Bead 3's test.
8. **Specify Warn exit semantics (F1.3) — must.** `panel tally` on `d.Action == panel.Warn` (abandoned / missing ref / transient git): exit 0 with the advisory printed — parity with `internal/complete`'s non-blocking Warn (panel_advisory.go:204-212). Warn row in the exit-code test. As drafted the prose is if-Allow/else-Block and would halt orchestration on abandoned panels with a bogus recovery line.
9. **Slug validation at the CLI boundary (G2.1) — must.** Reject empty, `.`, `..`, slash/backslash, absolute, and control-character slugs before any `filepath.Join`; tests for traversal + control/newline rejection.
10. **Output/recovery-line escaping (G2.3) — must.** User-controlled strings (slug, config values) rendered into command output or `guard.NewFailure` recovery lines must be rejected-or-escaped for newlines/ANSI/control bytes (the 109-final-G2 terminal-injection class); tests.
11. **Tally CCR decode: policy + coverage (G2.4 + F3.5) — must.** Define the second-pass decode policy for `concrete_changes_required` (wrong type / malformed JSON / omitted field / newline-laden entries: never affects `PanelGateDecision` or the exit code; reported or escaped deterministically) AND cover the re-decode wiring with a failing check — the pure renderer takes pre-read `[]slotChanges`, so the file-read path (latest round, RC/REJECT verdicts, `filepath.Join(reg.Dir, v.File)`) is currently exercised by nothing; seed a change string and assert it renders attributed to its slot (test or e2e grep).
12. **Rewrite the manual e2e to be runnable (G1.1) — must.** `panel create` dirties the tree, so the "git status clean" claim after `verify` needs a before/after comparison (or commit/snapshot first), and the sub-threshold tally leg needs its exact verdict-file setup spelled out.
13. **key_file_paths completeness (F2 finding) — should.** Add `cmd/mindspec/root.go` (command registration) and `cmd/mindspec/help_golden_test.go` (new verb changes help output; golden must be regenerated in-bead) to B4's key_file_paths; optionally re-run `TestPanelSchemaDoc_MatchesConstants` in B4's verification since B4 edits the interfaces.md file that test pins.

## Bead 1 — Create writer + schema doc

14. **BRIEF marker edge cases: specify + test (G2.2 + F3.6) — must.** Behavior for legacy no-marker (fresh region prepended, body byte-identical — specified but untested: add the test case), marker-only-open, duplicated markers, CRLF. Corrupt/ambiguous marker states fail WITHOUT touching `panel.json` or `BRIEF.md` (or are deterministically repaired with body preservation tested).
15. **Make the schema-doc drift test bind to the doc (F3.7) — must.** `TestPanelSchemaDoc_MatchesConstants` must extract the documented examples FROM the doc (e.g. every backtick-quoted `*-round-*.json` token in the schema region) and assert each conforming example matches `verdictFileRE` while the doc's explicitly-marked nonconforming `-round-0` example does not. "Doc contains panel.json" + a test-held `-round-0` literal tests the regex, not the doc.
16. **Document the verdict PAYLOAD contract (G3.1) — must.** The R4 schema doc must cover the verdict JSON payload, not just filenames: required top-level `verdict` enum, top-level `hard_block` semantics, reviewer_id/confidence/rationale/concrete_changes_required/findings expectations, and which fields feed the gate decision vs tally presentation. Pin the load-bearing parts in the drift test.

## Cross-bead

17. **Leaf assertion false-green (G1.2) — must; B1 + B4 verification.** `! go list -deps ./internal/panel | grep -q internal/config` exits 0 when `go list` itself fails. Use the two-step form: `deps=$(go list -deps ./internal/panel) && ! printf '%s\n' "$deps" | grep -q internal/config`.
18. **BRIEF-stub verdict instruction consistency (G3.2) — must; B1 stub + B5 skill text.** The stub `create` writes and the trimmed `ms-panel-run` must carry a single, non-conflicting verdict-JSON instruction matching item 16's documented schema — top-level `hard_block` placement explicit; remove/clarify any wording implying finding-level `hard_block` is gate-consumed.

## Constraints for the fix author

- Plan-text only; ONE commit `docs(spec-110): apply round-1 plan-panel changes`. Do not renumber beads or change the DAG (approved by F2/O2/O3).
- Keep every bead ≤7 steps — fold asks into existing steps/verification items rather than adding steps where possible.
- `mindspec validate plan 110-panel-verbs-parser-parity` must still pass (WARN decomposition-scope-redundancy acceptable).
- Update the Provenance table where the verification anchors change.
