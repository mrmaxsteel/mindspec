# spec-110-bead1 — consolidated round-1 changes

Tally: 5 APPROVE (O1, O2, O3, S1, S3) / 3 REQUEST_CHANGES (G1 0.86, S2, F1 0.80) / 0 REJECT. Threshold 7/8 not met → fix round. Core implementation survived adversarial + empirical attack (byte-preservation, fail-closed corrupt paths, verdict-file safety, leaf invariant all verified independently by three reviewers); every ask targets the marker-parsing edge or the schema-doc contract accuracy. Fix = ONE follow-up commit on `bead/mindspec-fbel.1` touching only `internal/panel/create.go`, `internal/panel/create_test.go`, `.mindspec/domains/workflow/interfaces.md`.

## Code

1. **Scope marker detection to real managed headers (G1.1; S1 reproduced independently) — must.** The scanner counts marker strings globally across the BRIEF, so marker-like text in the preserved body (e.g. inside a fenced code block documenting the header syntax) jams every subsequent re-panel with a duplicated-markers rejection. Detect only genuine markers (e.g. marker comment alone on its line at column 0, outside the managed region's body remainder — pick a precise rule and test it) so a body that QUOTES the marker text still re-panels. Add the fenced-quote case to `TestCreate_BRIEFMarkerEdgeCases`.
2. **Whitespace-mangled markers must not silently become "legacy" (G1.2) — must.** A header whose marker comments carry stray whitespace is currently treated as marker-absent → a second header gets prepended (silent duplication). Decide ONE behavior and test it: reject-as-corrupt (fail without touching either file — recommended, consistent with the other corrupt states). Test case: `<!--  mindspec:panel-header  -->` variants.
3. **Section-scope the drift test (F1) — must.** `TestPanelSchemaDoc_MatchesConstants` extracts backtick tokens from the WHOLE doc; F1 proved renaming `panel.json` → `registration.json` inside the normative schema section still passes because the Maintenance Notes bullet's backticks satisfy the pin. Extract ONLY from the "Panel Artifact Schema" section (delimit by heading boundaries) and add F1's exact attack as a negative fixture (a doc copy with the normative name wrong must FAIL).

## Doc / contract accuracy

4. **Align the verdict contract with tally.go's real parsing (G1.3) — must.** The doc declares `verdict` an enum; `internal/panel/tally.go` accepts any non-empty string and counts it toward completeness. Do NOT change tally.go (out of bead scope — flag as a possible Bead 4 note); fix the doc to state the actual semantics: APPROVE/REQUEST_CHANGES/REJECT are the meaningful values; any other non-empty string parses as present-but-neither-approving-nor-rejecting (counts toward completeness, cannot help reach threshold, does not REJECT-halt). Keep the drift test consistent.
5. **Remove the false "never hand-edited" claim (S2.1) — must.** panel.json is hand-authored today (the CLI arrives in Bead 4) and the SANCTIONED abandon procedure (`ms-panel-tally` § Abandon) hand-edits `abandoned`/`abandon_reason` into panel.json. Reword to describe the steady-state intent (written by `mindspec panel create` once Bead 4 lands) while acknowledging the abandon-fields hand-edit path.
6. **Document the overwrite asymmetry (S2.2) — must.** `Create` full-struct-overwrites panel.json without reading it first (unlike its read-before-splice BRIEF handling), so a re-panel on an abandoned dir silently clears `abandoned`/`abandon_reason`. Document this in the doc + `Create`'s docstring (re-panel of an abandoned panel deliberately revives it), and pin it with a small test (pre-seed an abandoned panel.json, Create round 2, assert the fields are gone — the KNOWN behavior, so future changes are conscious).

## Non-blocking (carry, do not fix in this bead)

- Sequential two-file write is not crash-atomic (S2 info; F1 demonstrated panel.json-bumped/BRIEF-stale under EACCES). Note it in the `Create` docstring as a known bound ("two writes, panel.json first; a crash between them is repaired by the next Create"). No temp+rename rework in this round.
- S3's integration notes → for LATER beads, not this diff: B4's plan section shows stale `panel.Create(dir, Panel{...})` sample (B4 implementer must follow the real CreateInput signature); BRIEF template's Worktree/H1/prior-round fields need a conscious home when B5 fills stubs; stub headings unpinned by test (optional cheap pin if touching create_test.go anyway).

## Constraints for the fix author

- ONE commit on `bead/mindspec-fbel.1`: `fix(panel): scope marker detection + schema-doc contract accuracy (bead panel r1) [mindspec-fbel.1]`.
- Only the three files above. All existing tests must stay green; the full Bead-1 Verification checklist (incl. two-step leaf assertion, doc-sync grep — interfaces.md is already in the commit set) must pass again.
- No push, no bd, no `mindspec complete`, no files outside the bead worktree.
