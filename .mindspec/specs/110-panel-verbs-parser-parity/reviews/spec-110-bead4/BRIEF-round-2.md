# spec-110-bead4 — Round 2 (targeted re-verification: R5, R6)

**Under review**: `bead/mindspec-fbel.4` @ **989e26845746038ea6b748d4a125fa5f7c0b06be** (fix commit on top of round-1 `75ae9c6a`; 3 files, +31/−9).
**Pass = ≥7 APPROVE, no REJECT.** Round-1 approvers R1, R2, R3, R4, R7, R8 (6) carry forward — their lenses are untouched by a stricter control-byte predicate + an added stdout line. Round-2 re-runs: R5 + R6 (the round-1 RC voters).

**READ-ONLY**: verdict JSON only; pin reads to `989e2684`; scratch under ABSOLUTE /tmp only (never a relative `.mindspec/` write). Delta: `git diff 75ae9c6a..989e2684`.

## The fixes
- **R5 (C1 control bytes)**: `rejectControlBytes` (`cmd/mindspec/panel.go`) predicate changed from `r < 0x20 || r == 0x7f` (C0+DEL only) to `unicode.IsControl(r)` (full C0+DEL+C1, mirroring `report.go`'s `stripControl`). 3 C1-CSI (U+009B) cases added to `TestPanelCreate_RejectsUnsafeSlugAndControlBytes` for slug/`--bead`/`--target`. interfaces.md "Shared slug validation" updated.
- **R6 (panel directory line)**: `panel create` now prints a second stdout line `panel directory: <dir>` (the exact dir `panel.Create` wrote to), documented in interfaces.md's `panel create` section, so spec 111 can capture it instead of re-deriving layout.

## Per-slot jobs
- **R5**: confirm the predicate now rejects the C1 range (U+0080–U+009F incl. U+009B CSI) for slug AND `--bead`/`--target`, that `unicode.IsControl` is the right/complete predicate (matches `stripControl`), and the new tests genuinely pin it (would fail on the old predicate). Disposition ADDRESSED/PARTIAL/MISSED/NEW_ISSUE.
- **R6**: confirm `panel create`'s stdout now emits a stable, greppable `panel directory: <dir>` line pointing at the real resolved dir (build to /tmp, run it), and that interfaces.md documents it as the capture surface for 111. Disposition ADDRESSED/PARTIAL/MISSED/NEW_ISSUE.

## Output
Write `<slot>-round-2.json` here. Keys: `reviewer_id` ("R5 sonnet" / "R6 sonnet"), `verdict`, `confidence`, `rationale` (≤160 words), `concrete_changes_required` (empty if APPROVE), `findings`.
