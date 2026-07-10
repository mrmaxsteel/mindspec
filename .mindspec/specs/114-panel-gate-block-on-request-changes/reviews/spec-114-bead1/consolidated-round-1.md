# spec-114-bead1 round-1 consolidated changes (fix-up)

Round-1 tally: 7 APPROVE, 1 REQUEST_CHANGES (S2). Per findings-never-outvoted, S2's finding is fixed, not out-voted. Test-only fixes (+ one comment). One commit on `bead/mindspec-mvp8.1`.

## 1. [MUST — S2 medium, mutation-proven hollow] `internal/panel/panel_decision_test.go` — "two unresolved RC slots → Block naming both" row
The row (currently ~line 330-340) uses slots `"x"` and `"y"`. Those single letters are substrings of fixed text in EVERY Block message (`/ms-bead-fix` contains `x`; `bypass`/`only` contain `y`), so the `mustHave: []string{"x", "y", ...}` assertion is trivially satisfied even when leg 9.5's multi-slot naming is completely disabled — S2 proved this by disabling leg 9.5 and watching the row stay green while rows 1-2 went red.

**Fix**: rename both RC slots to names that are NOT substrings of the leg-9.5 / leg-10 message template — e.g. `revA` / `revB` (verify: the fixed message format string in gate.go must not contain your chosen names as fixed substrings; they may appear ONLY via the `%s` slot-list substitution). Update BOTH the `Verdict{Slot: ...}` entries (and their `File:` fields to match, e.g. `revA-round-1.json`) AND the `mustHave` slice to the new names.

**Do NOT** try to raise the approve count to clear the threshold — it is mathematically impossible for a 2-RC row: two non-APPROVE slots means APPROVE ≤ N−2 < N−1 threshold, so any 2-RC case is inherently sub-threshold and leg 10 will also block. The rename is the correct and sufficient fix: with non-colliding names, disabling leg 9.5 makes leg 10 fire but its generic message contains neither name → `mustHave` fails → RED-on-revert restored, so the row genuinely discriminates leg 9.5's multi-slot naming.

**Acceptance (you MUST perform this in an ABSOLUTE /tmp copy, never the live worktree)**: in a `git archive`/clone copy pinned to the current tip, disable leg 9.5 (`if unresolved := f.Res.UnresolvedVerdicts(); false && len(unresolved) > 0 {`), run `go test ./internal/panel -run UnresolvedRequestChanges -v`, and confirm the "two unresolved RC slots" row now FAILS (it must go RED, like rows 1-2). Restore/discard the /tmp copy. This proves the fix.

## 2. [SHOULD — S2 low, hardens the e2e falsifier] `internal/complete/panel_gate_e2e_test.go` — `TestPanelGate_RequestChangesBlocksComplete`
Under the leg-9.5-disabled mutation this e2e test still fails, but for the WRONG reason (an unrelated doc-sync gate trips on the fixture's undocumented change; the panel gate itself would Allow at 5/6). Add an assertion that the block reason is specifically the PANEL gate — assert the returned error / block message contains a leg-9.5-distinctive substring (e.g. `unresolved REQUEST_CHANGES` or `every latest-round verdict must be APPROVE`), not merely that `err != nil`. This makes RED-on-revert catch a genuine leg-9.5 regression directly rather than by incidental message overlap.

## 3. [NIT — O2, cosmetic] `internal/panel/gate.go` — leg-9.5 message comment
If a comment describes the leg-9.5 message as a "byte-superset" of leg 10's, reword to "substring-set superset" (it is NOT a contiguous byte-superset — leg 9.5 renders `APPROVE (threshold is` where leg 10 renders `APPROVE — threshold is`; the real invariant is that every substring the sub-threshold fixtures assert survives). Plain ASCII only — do NOT introduce a backtick code-span containing shell-escape characters (gofmt doc-comment corruption gotcha). Skip if no such comment exists.

## Constraints
- Test-only changes plus at most the one comment tweak. Do NOT touch production logic in gate.go/tally.go (leg 9.5, UnresolvedVerdicts, VoteDecision are all APPROVED as correct).
- CI runs gofmt with go 1.23.0 — run `gofmt -l ./cmd ./internal` (must be empty) before committing; avoid backtick doc-comment shell-escape spans.
- Full quality gate before commit: `go build ./...`, `gofmt -l`, `go vet ./internal/panel ./internal/complete`, `go test -count=1 ./internal/panel ./internal/complete ./cmd/mindspec` all green (z4ps `TestRun_IdleNoBeads` flake in internal/instruct is pre-existing — ignore).
- Exactly ONE commit. ABSOLUTE /tmp scratch only. Leave the worktree clean.
