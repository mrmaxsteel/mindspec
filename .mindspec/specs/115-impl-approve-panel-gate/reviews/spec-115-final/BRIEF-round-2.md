# spec-115-final — Round 2 Final Review (12 reviewers, four families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate`
**Branch**: `spec/115-impl-approve-panel-gate` @ **389fcc41** (round-1 tip `21374bb8` + ONE boundary-fix commit). **Base**: `origin/main` `f02a3a49`.
**Panel**: 12 slots — F1–F3 Fable-sub (Opus/Sonnet), O1–O3 Opus, S1–S3 Sonnet, G1–G3 codex. **Pass = ≥11 APPROVE, no REJECT.** Findings never out-voted.

**READ-ONLY**: verdict JSON only; ABSOLUTE `/tmp` scratch; NEVER edit; leave `git status --porcelain` clean.

## Round 1 → the round-2 fix (the ONLY delta)
Round 1 was 9 APPROVE / 3 RC. The gate's BEHAVIOR was cleared by every lens (G1/G2 codex security + fail-closed; O1 o4fd guarantee; F1 all-AC RED-on-revert; grounding/ADR/provenance/no-regression all APPROVE). The 3 RC:
- **S1 + S3 (REAL, now FIXED):** `internal/lint.TestEnforcementHasNoGitLeaks` failed — Bead 2 had added a direct `internal/gitutil` import to the ADR-0030 enforcement package `internal/approve`. Fixed by commit `389fcc41`: rerouted the two read-only calls through thin `internal/lifecycle` wrappers.
- **G3 (pre-existing, out of scope):** 2 gosec G115 in `internal/journal/lock_unix.go` — on `origin/main` already, untouched by this spec (`mindspec-8ud6`, CI-tolerated).

**The fix (`git diff 21374bb8..389fcc41` — 2 files, seam-transparent):**
1. New `internal/lifecycle/gitquery.go`: thin exported wrappers `IsAncestor(workdir, ancestor, descendant string) (bool, error)` and `BranchExists(name string) bool` delegating to gitutil (lifecycle already imports gitutil; not an enforcement package).
2. `internal/approve/impl.go`: `implIsAncestorFn = gitutil.IsAncestor` → `lifecycle.IsAncestor`; `implBranchExistsFn = gitutil.BranchExists` → `lifecycle.BranchExists`; REMOVED the `internal/gitutil` import; updated the affected comments. Seam names/types/injection points UNCHANGED → all AC tests unaffected, gate behavior identical.

## CORRECTED CI-readiness scope (fixing a round-1 BRIEF error)
The round-1 BRIEF over-strictly said "any golangci-lint issue = hard block." Corrected: **only issues INTRODUCED by spec 115 block.** Two conditions are PRE-EXISTING (on `origin/main`, untouched by this spec, CI-tolerated — main's last 3 CI runs are all green) and are OUT OF SCOPE:
- **`internal/journal/lock_unix.go` gosec G115 ×2** — bead `mindspec-8ud6` (P3). Spec 115 does not touch `internal/journal`.
- **`internal/instruct.TestRun_IdleNoBeads`** — bead `z4ps`. Fails ONLY from an active mindspec worktree (this one); passes in a clean CI clone (CI clones fresh). Also `internal/harness` LLM-timeout only without `-short` (CI uses `-short`).
Do NOT block spec 115 on these three pre-existing conditions. Flag any NEW issue introduced by the branch.

## What to verify at `389fcc41`
1. **The boundary blocker is FIXED (S1/S3/O2/G3 lenses):** `go test ./internal/lint/ -run TestEnforcementHasNoGitLeaks` PASSES; `internal/approve` no longer imports `internal/gitutil` (`go list -f '{{.Imports}}' ./internal/approve | grep gitutil` empty); the reroute is ADR-0030-clean (git-I/O routes through the non-enforcement `internal/lifecycle`, which owns gitutil access). The new `lifecycle.IsAncestor`/`BranchExists` are correct thin wrappers (identical signatures).
2. **The reroute is SEAM-TRANSPARENT — gate behavior + all ACs unchanged (security/falsifiability lenses):** the `implIsAncestorFn`/`implBranchExistsFn` seams keep identical names/types, so every AC test (AC1-13) still passes with no assertion change; the four legs' fail-directions, the Option-B TOCTOU closure, and the o4fd guarantee are byte-behavior-identical. Confirm the round-1 security/falsifiability/grounding/provenance conclusions still hold (they reviewed unchanged behavior).
3. **Whole-tree CI-green except the 3 disclosed pre-existing conditions:** `go build ./...`; `go test ./...` (only `internal/instruct` z4ps fails, worktree-only — verify by reasoning or a clean check); `go vet ./...`; `gofmt -l ./cmd ./internal` empty; `golangci-lint run ./...` (only the 2 pre-existing G115); `mindspec validate spec`. Fences: `BranchExistsE`/`show-ref` 0-hit; `git diff origin/main -- internal/gitutil/` empty.
4. **No NEW issue from the reroute:** the 2-file change introduces no new lint/vet/test failure; no import cycle (`lifecycle` still leaf-relative to `approve`; approve→lifecycle pre-existed).

## Per-slot lens (same as round 1; center on the reroute delta + corrected CI scope)
- **F1** falsifiability (all AC RED-on-revert — unchanged by the seam-transparent reroute) · **F2** grounding · **F3** contradiction/completeness · **O1** security completeness (o4fd guarantee — unchanged behavior) · **O2** ADR/import-edge (the reroute makes approve→gitutil GONE; verify ADR-0030 now honored WITHOUT an allowlist exception; edges acyclic) · **O3** provenance · **S1** empirical full-tree (whole `go test ./...` + golangci-lint — confirm only the pre-existing conditions remain) · **S2** no-regression (the reroute changes no behavior) · **S3** CI-readiness (the boundary test passes; the branch would pass CI modulo the pre-existing/CI-tolerated conditions) · **G1** adversarial security · **G2** fail-closed completeness · **G3** whole-tree CI + integration (boundary fixed; apply the CORRECTED pre-existing scope — do NOT block on 8ud6/z4ps).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<slot>-round-2.json`: `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
