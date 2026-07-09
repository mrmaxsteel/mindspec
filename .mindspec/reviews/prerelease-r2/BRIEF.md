# v0.10.0 Pre-Release — Round 2 Verification Panel

**Repo**: `/Users/Max/replit/mindspec` · **Base**: `main` @ `fa3e7f40`.
**Consolidated deliverable under review** = TWO open PRs that together must make the repo ready to tag **v0.10.0**:
- **PR #159** `chore/v0.10.0-prerelease-hardening` @ `a5ea5b6f` — docs/governance/state. `gh pr diff 159`.
- **PR #160** `fix/migrate-layout-hardening` @ `21181776` — migrate-layout code fixes + tests. `gh pr diff 160`.
Both are MERGEABLE/CLEAN with all CI checks green. You may `gh pr diff <n>`, or `gh pr checkout <n>` to run code/tests locally.

## Why round 2
A round-1 panel reviewed the pre-release plan and returned **4 REQUEST_CHANGES** (2 APPROVE). The round-1 verdicts are in `.mindspec/specs/106-layout-flatten/reviews/prerelease-plan/`. The user chose to **fix** (not document-around) the issues. These two PRs implement the fixes. **Your job: verify every round-1 ask is genuinely ADDRESSED, the new code is correct and regression-free, and the repo is safe to tag.**

## Round-1 asks → where addressed (verify each: ADDRESSED / PARTIAL / MISSED)
1. **ADR-0039 Proposed → Accepted** (mandatory) — PR #159.
2. **Stale user-facing path docs** still showing the pre-flatten layout as live, fixed — PR #159 (`.mindspec/core/*`, `.mindspec/domains/*`, `project-docs/user/guides/*`, `spec-orchestrator.md`). Verify it fixed *live* references and **preserved** legitimate resolver-tier/historical mentions (over-rewriting those would be a NEW bug).
3. **Curated release notes + changelog mechanism** — PR #159: `RELEASE-NOTES-v0.10.0.md` + `^Merge` added to `.goreleaser.yml` changelog excludes. Verify the notes are accurate (esp. that migrate is described as `--abort` rollback + forward-only, NOT overclaiming crash-resume) and non-breaking/opt-in is stated.
4. **migrate layout bugs FIXED (not just documented)** — PR #160:
   - **sc0w**: precondition no longer false-positives on unrelated stale branches (`internal/layout/precondition.go`; `classifyRefs` excludes remote-default; new `--allow-branch`/`--force` escapes). Scrutinize the **scoping judgment** (git refs can't prove a local branch's merge intent — is `--force`/allowlist + remote-default exclusion a sound, safe resolution? Could the new escape let a genuinely-dangerous migration through silently?).
   - **3jq7**: 404 link-check widened from `{README,AGENTS}` to every repo-root `*.md` + flat `context-map.md` (`internal/doctor/links.go`, `internal/validate/docsync.go`). Verify it now catches a broken link in a non-README root doc and didn't over-widen into frozen snapshots.
   - **crash-resume ordering (R5)**: CLI now resumes a detected non-terminal run BEFORE the fresh-run clean-tree precondition (`internal/layout/runstate.go` `IsResumable`/`FindResumableRun`, `cmd/mindspec/migrate.go`). Verify the precondition still guards a *fresh* start and resume actually works post-dirty-crash.
5. **Binary/resolver downstream-compat test** (canonical + legacy, read AND write byte-identical) — PR #160 `internal/workspace/workspace_test.go` `TestDownstreamCompat_NoFlatFlip_ReadWrite`. Scrutinize B1's note that a brand-NEW spec id in a legacy tree resolves to the canonical default (claimed documented behavior) — is that acceptable, and does the test still prove "do-nothing upgrader isn't broken"?
6. **Self-revert fixed + lifecycle bug filed** — PR #159 `.beads` sync (committed JSONL == Dolt: spec-106 epic+`.6` closed) + bead `mindspec-wu7t` (P1) filed. Confirm the committed JSONL now matches Dolt so the post-merge re-import is idempotent.

## Also confirm (round-1 already validated; check nothing regressed)
- Non-breaking/opt-in still holds; v0.10.0 (not v1.0.0) still the right bump.
- The new PR-B tests actually exercise the fixes (run them: `go test ./internal/layout/... ./internal/workspace/... ./internal/validate/... ./internal/doctor/... ./cmd/...`). **NEVER run `go test ./internal/harness/...` (LLM suite).**
- No NEW regression introduced by either PR.

## Verdict
**APPROVE / REQUEST_CHANGES / REJECT.** APPROVE = the repo is ready to tag v0.10.0. REQUEST_CHANGES only for a genuine remaining blocker (an ask not actually addressed, a real bug in the new code, a regression). Output JSON to `/Users/Max/replit/mindspec/review/prerelease-r2/<your-slot>-round-2.json`:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `asks_status` (object mapping ask number → ADDRESSED/PARTIAL/MISSED), `concrete_changes_required` (array, empty if APPROVE), `findings` (array of {severity, area, issue}).
