# spec-113-bead4 — Round 1 Bead Panel (8 reviewers)

**Bead**: `mindspec-r6hk.4` (spec 113, Bead 4 = R4). **Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-113-panel-verb-workflow-followups/.worktrees/worktree-mindspec-r6hk.4`
**Branch**: `bead/mindspec-r6hk.4` @ **954d6ca8eb2344f382a752926733b8b3cf7f27ee** — `feat(config): document {model:"",family} resolves-to-family (R4); pin with TestLoad_EmptyStringModel`
**Panel**: 8 slots — O1–O3 Opus, S1–S3 Sonnet, F1 Fable, **R8 sonnet-sub** (Claude standing in for the codex empirical slot — this session runs NO codex on bead panels). **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: edit nothing but your verdict JSON; pin reads to `954d6ca8`; scratch under ABSOLUTE /tmp only (or `t.TempDir()`); leave `git status` clean. (Relative `.mindspec/` writes + harness cwd-reset corrupt SIBLING worktrees — never do it.)

## What the bead does
R4 reconciles a spec-112 text contradiction: 112's R4 said an "empty-string `model`" reviewer entry is refused, but 112's R1 said "a `family` and no `model` is valid". The CODE already sides with R1 (`Reviewer.model()` falls back to `Family` when `Model==""`). So this bead is **documentation + a pinning test, NO behavioral change**:
1. A doc comment in `internal/config/config.go` (on `Reviewer.model()` ~line 139 and `validateReviewerEntries` ~line 535) recording that `{model:"", family:<f>}` is valid and resolves to the family, using the word **`supersedes`** to flag it overrides 112 R4's "or an empty-string `model`" phrasing.
2. `TestLoad_EmptyStringModel` in `internal/config/config_test.go` with two subtests: (a) `[{model:"",family:codex},{family:claude}]` loads + `PanelGateReviewerSlots("bead")` expands entry 0 to `Model=="codex"`; (b) `[{model:""},{family:claude}]` (empty model, NO family) FAILS `Load` via the neither-set branch, error naming `panel.reviewers[0]` + carrying the ADR-0035 `recovery:` line.
3. Fence: commit touches ONLY `internal/config/config.go` (comments) + `internal/config/config_test.go`. `Reviewer.Model` stays `string` (NOT `*string`); no `UnmarshalYAML`; every validation branch behaviorally unchanged.

## Files in scope (final state at 954d6ca8)
- `internal/config/config.go` (comment-only change)
- `internal/config/config_test.go` (added `TestLoad_EmptyStringModel`)

## What to verify (evaluate cold; each concern → a disposition)
1. **Diff matches plan Bead 4** — read `.mindspec/specs/113-panel-verb-workflow-followups/plan.md` Bead 4. The diff is exactly comment + test, no struct/logic change.
2. **Test is REAL, not hollow** — `TestLoad_EmptyStringModel` must exercise the actual `Load` + `PanelGateReviewerSlots`/`expandSlots` paths and the real neither-set refusal branch — NOT assert a hardcoded constant. Subtest (a) proves resolve-to-family end-to-end (slot expands to `codex`); subtest (b) proves the neither-set refusal still fires with the recovery line. If either subtest could pass without the real code path, that's a finding.
3. **Comment accuracy** — the comment must correctly describe the ACTUAL behavior (`Model==""` → falls back to `Family`; only neither-set refused) and correctly characterize what it supersedes (112 R4's phrase). A comment that misdescribes the code is a finding.
4. **No-behavioral-change fence** — `Reviewer` struct + every validation branch behaviorally unchanged; existing 112 tests (`TestLoad_RefusesPerGateKnobs` etc.) pass UNMODIFIED. `git show --name-only` lists only the two files.
5. **Empirical (R8 sonnet-sub)** — actually RUN: `cd` the worktree; `go test ./internal/config -run 'TestLoad_EmptyStringModel' -v` (both subtests PASS); `go test ./internal/config` (all green, existing tests unmodified); `go build ./...`; `git show --name-only HEAD` (two files); `grep -q 'supersedes' internal/config/config.go` (exit 0); `grep -q 'Model  string' internal/config/config.go` (exit 0 — field type untouched). Then try to BREAK it: does subtest (b) actually fail Load for the right reason (neither-set), or could a different error make it pass spuriously? Is the recovery line assertion real?

## Per-slot lens defaults
- **O1 Opus** — author-of-record (diff ↔ plan Bead 4). **O2 Opus** — test correctness (both subtests exercise real paths, not faked). **O3 Opus** — contract/fence (no behavioral change, struct untouched, existing tests unmodified).
- **S1 Sonnet** — codebase-pin (named symbols/tests exist + green: `Reviewer.model()`, `validateReviewerEntries`, `PanelGateReviewerSlots`/`expandSlots`). **S2 Sonnet** — comment accuracy vs real behavior + correct supersession of 112 R4. **S3 Sonnet** — scope fence (only 2 files, one commit, in-scope).
- **F1 Fable** — adversarial: is the test hollow/tautological? any way subtest (a) or (b) credits a pass without the real branch? empty-string edge cases.
- **R8 sonnet-sub** — empirical prober (run everything in §5; try to break it).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", e.g. "R8 sonnet-sub"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
