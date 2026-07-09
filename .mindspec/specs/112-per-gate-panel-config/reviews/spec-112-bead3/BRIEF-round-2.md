# spec-112-bead3 — Round 2 (targeted re-verification: R5, R7)

**Under review**: `bead/mindspec-lma4.3` @ **75c17fd99575809c65a3ba2356445d8a363adee2** (fix commit on top of round-1 `b27e793b`; 2 files, +25/−1) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.worktrees/worktree-mindspec-lma4.3`.
**Panel**: 8 slots. **Pass = ≥7 APPROVE, no REJECT.**
**Round-1 standing (carried)**: R1, R2, R3, R4, R6, R8 = APPROVE (6). Their lenses (author-of-record, codebase-pin, scope-fence, empirical-headline, integration, codex escaping/empirical) are untouched by a nil→`{}` JSON normalization and are **carried forward** — not re-running. R4/R8 tested the JSON path and this fix only makes the empty-substitutes case emit `{}` instead of `null` (a strict improvement neither objected to).
**Round-2 re-runs (you)**: R5 sonnet + R7 fable — the two round-1 REQUEST_CHANGES voters, who converged on the single defect.

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin all reads to SHA `75c17fd9`; leave `git status` clean. **Any scratch config/binary MUST use ABSOLUTE `/tmp` paths — NEVER a relative `.mindspec/config.yaml` write** (a round-1 reviewer's relative-path scratch write, combined with this harness's cwd-reset-between-Bash-calls, contaminated a sibling worktree). Use `t.TempDir()`-style absolute paths only.

## The fix under review
Your round-1 RC (both slots): `config show --gate <g> --json` emitted `"substitutes":null` for the default (unconfigured-substitutes) config, breaking jq consumers (`.substitution.substitutes|keys`) and diverging from the text path's `{}`.

The fix (`75c17fd9`, in `buildGateResolvedDoc`, `cmd/mindspec/config.go`):
```go
substitutes := cfg.Panel.Substitution.Substitutes
if substitutes == nil {
    substitutes = map[string]string{}
}
// ...assigned into gateResolvedDoc.Substitution.Substitutes
```
Plus a raw-string test assertion in `TestConfigShowGate_ResolvedJSON` (`config_test.go`) that the JSON contains `"substitutes":{}` and NOT `"substitutes":null` (asserted on raw `[]byte`, not a typed unmarshal — since nil and {} decode identically).

Your delta: `git -C <worktree> diff b27e793b..75c17fd9`.

## Per-slot jobs
- **R5 (schema/type)**: confirm the normalization is correct and complete — the JSON now emits `"substitutes":{}` for a nil map; `in_force` still correctly flips (empty map ⇒ `claude_sub_on_quota`, non-empty ⇒ `substitutes`); no other JSON member regressed; the raw-string test genuinely fails on `null` (i.e., it would have caught the round-1 bug). Confirm nothing else in the doc uses a nil-map-marshals-to-null pattern. Disposition your round-1 item ADDRESSED / PARTIAL / MISSED / NEW_ISSUE.
- **R7 (fable adversarial)**: confirm the fix closes the defect without introducing a new one; the escaping class you cleared in round 1 is unaffected (the fix touches only the substitutes-map nil-guard, not any escaping path); the `{}` value is what jq consumers and the R9 additive-only contract need; no other default-config member still emits `null`. Disposition your round-1 item ADDRESSED / PARTIAL / MISSED / NEW_ISSUE. (Your two F4 non-blocking items — global reviewers model-only empty `- family:` line, and `reg.Slug()` unescaped at config.go:539 — are already logged for the `mindspec-naq0` follow-up; do NOT block on them.)

## Output
Write `<slot>-round-2.json` in this directory. Keys: `reviewer_id` ("R5 sonnet" / "R7 fable"), `verdict` (APPROVE/REQUEST_CHANGES/REJECT), `confidence`, `rationale` (≤160 words), `concrete_changes_required` (empty if APPROVE), `findings` (per-item disposition).
