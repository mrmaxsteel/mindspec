# spec-115-bead2 — Round 1 Bead Panel (8 reviewers, four families)

**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-115-impl-approve-panel-gate/.worktrees/worktree-mindspec-fgmg.2`
**Branch**: `bead/mindspec-fgmg.2` @ **8b664359** (single commit). Base = the Bead-1 merge `075fd630` (Bead-1 exports present). Diff under review = `git diff 075fd630..8b664359`.
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex. **Pass = 8/8 UNANIMOUS.** Every RC is fixed or evidence-refuted (never out-voted).

**READ-ONLY**: verdict JSON only; ALL scratch ABSOLUTE `/tmp` (or `t.TempDir()`); NEVER edit; leave `git status --porcelain` clean.

## What Bead 2 is — the impl-approve refusal gate (the load-bearing bead)
Spec 115 makes `mindspec impl approve` REFUSE to finalize a spec while any closed bead under its epic lacks proof of panel settlement. Bead 2 adds the pre-terminal refusal gate `runOrphanObligationGate` in `internal/approve/impl.go`, consuming Bead-1's exports. Three files changed (1070+/19−): `impl.go` (+273: the gate + 7 seams + helpers), `impl_test.go` (+108/−19: seam save/restore + AC4 anchor + fixture/one-assertion changes — SEE DEVIATIONS), `orphan_gate_test.go` (new +708: the 11 named tests).

**Gate placement (AC4):** `runOrphanObligationGate` is called at `impl.go:369`, AFTER the last read-only gate (ADR-divergence refusal `:355`) and BEFORE the Spec-092 phase-reconcile write, MUTATION(1/3) epic close, `mindspec_phase=done` write, CommitCount preflight, and `exec.FinalizeEpic` — so a refusal performs NO epic close, NO phase write, NO merge, NO push. Pinned in `TestApproveImplCallOrder` via a new AST anchor (`runOrphanObligationGate`, label references `lifecycle.ScanOrphanedClosedBeads` for the AC4 grep discriminator).

**The four legs + their fail-directions (verify each is EXACTLY right — the codex slot will hammer this):**
- **Leg 1 — R1 orphan scan (FAIL-CLOSED).** `implScanOrphansFn(specID, root, "")`; any infra error → refuse; any orphan → `implOrphanRefusal` naming bead/branch/spec-branch + `mindspec complete <bead>` final line (ADR-0035).
- **Leg 2 — Option-B worktree-enum (FAIL-CLOSED on own infra).** `runWorktreeEnumerationLeg`: enumerates `implWorktreeListFn()` (same source FinalizeEpic merges from), for every `bead/`-prefixed entry whose bead ID ∈ `implClosedEpicBeadIDsFn(specID)`, checks `implIsAncestorFn` and refuses on a non-ancestor — regardless of the branch-existence probe (never consults `branchExistsFn`). WorktreeList/ClosedEpicBeadIDs/ancestry error → refuse.
- **Leg 3 — R3 obligation backstop (FAIL-CLOSED on data AND enumeration).** `planErr != nil` (missing/corrupt plan, empty bead_ids) → refuse naming the unreadable plan (NOT the gate-1/3 silent skip at `:226-227`, which stays unchanged). Per bead: `implCheckObligationsFn(bid, implGetMetadataFn)` → refuse on uncovered/read-error/corrupt/shape-invalid. Recovery is branch-state-truthful via `implBranchExistsFn`: branch exists → `mindspec complete <bead>`; absent → restoration-prerequisite recourse.
- **R2 — advisory slot naming (DECORATION, never load-bearing).** best-effort `complete.PanelGateRoots`+`panel.ForBead`/`Tally`; omits the slot line on any read failure — never a pass/crash; never prints `MINDSPEC_SKIP_PANEL` (HC-7) nor a `refut` substring.
- **Hatches bypass NOTHING** (`MINDSPEC_SKIP_PANEL`, `enforcement.panel_gate:false` never consulted by this gate).

## FIX-AUTHOR DEVIATIONS — assess each explicitly (ADDRESSED / legitimate, or NEW_ISSUE / masks-regression)
The plan/brief required "ZERO semantic change to existing `TestApproveImpl*` (benign seam defaults)." Adding a gate that READS plan.md had a broader-than-expected test blast radius. The fixer made these changes — **judge whether each is a legitimate spec-mandated consequence or a masked regression:**
- **A. ~13 existing tests gained a `writePlanWithBeads(...)` fixture.** Rationale: Leg 3 reads plan.md fail-closed, so a test driving `ApproveImpl` now needs a plan.md (every real spec has one at impl-approve). Assertions unchanged. Assess: is adding a plan.md fixture a pure mechanical accommodation, or does it hide a behavior change?
- **B. The default bead-status stub changed `"open"`→`"closed"`** (in `saveAndRestore` + several happy-path tests). Rationale: with a plan.md now present, the bead-status loop actually runs; happy-path tests need closed beads to model a ready-for-impl-approve spec. Assess: legitimate fixture correction, or masking?
- **C. `TestApproveImpl_NoCommitsNoBeads` ASSERTION changed** (the one real semantic change): old expected `"no commits beyond main"`; new expects `"plan bead list could not be read"` + a recovery line + FinalizeEpic uncalled. Rationale: with no plan.md, R3's fail-closed Leg 3 now intercepts BEFORE the CommitCount preflight. Assess: is this a correct, spec-mandated tightening (R3's whole point is that a missing plan.md is never silently tolerated), or a regression hidden by editing the test? THIS IS THE KEY DEVIATION.
- **D. `TestApproveImplPrintsDocSyncWarningAndProceeds` gained `ImplOpts{OverrideADR:...}`** because its now-present plan.md activates the previously-dormant ADR-divergence gate for its uncovered `cmd/` fixture. Assess: does the override change what the test pins (it should still pin the doc-sync WARN + FinalizeEpic-call), or mask something?
- **E. A 7th seam `implBranchExistsFn = gitutil.BranchExists`** (plan listed 6). Used only by Leg 3's branch-state-truthful recovery. Assess: consistent with the plan's Leg-3 wording + the `implXxxFn` convention?

## What to verify at `8b664359`
1. **The gate faithfully implements plan Bead-2 steps 1–8** (all four legs + placement + seams + AC4 anchor), and the 11 named tests match the spec's AC1a/1b, AC2/2d, AC3, AC5, AC6/6e, AC7b, AC11, AC13 exactly (each RED-on-revert, absent at the discriminator SHAs).
2. **Fail-directions EXACT** — detection legs (R1 infra, worktree-enum infra, R3 data+enum) FAIL-CLOSED; `branchExistsFn` bool stays fail-open (false = no trigger; a genuinely-deleted branch never false-refuses — AC2d); advisory degrades to omitting the slot line.
3. **The five deviations (A–E) are each legitimate, not a masked regression** — especially C (the assertion change). Run the affected tests; reason about whether the new behavior is spec-mandated.
4. **Import edges acyclic** — `approve → lifecycle/complete/gitutil` all acyclic (neither imports approve); `approve` already imported `bead`. ADR-0030 clean (no executor seam; no executor git-I/O in approve).
5. **Scope + fences + green** — only the 3 approve files changed; NO `internal/gitutil` change (`git diff main -- internal/gitutil/` empty); NO re-touch of Bead-1's `orphans.go`/`panel_advisory.go` logic; `BranchExistsE`/`show-ref` 0-hit; `go build ./...` + `go test -count=1 ./internal/approve ./internal/executor ./internal/complete ./internal/lifecycle` green; gofmt/vet/golangci-lint clean.

## Per-slot lens
- **R1 Opus** author-of-record: diff ↔ plan Bead-2 steps 1–8; all four legs present; the 11 tests map to the named ACs; deviations A–E faithful.
- **R2 Opus** fail-open/fail-closed correctness of every leg + the advisory degradation + hatch non-bypass (the security core).
- **R3 Opus** RED-on-revert: each of the 11 named tests + the AC4 anchor genuinely pins (fails on revert); AC11 (MergeBase pre-scan abort) + AC13 (worktree-enum race) truly discriminate.
- **R4 Sonnet** empirical: build / `go test -count=1` (4 pkgs) / each named `-run` / AC4 anchor / fences / gofmt / vet / golangci-lint — actual output.
- **R5 Sonnet** seam/type + import-edge acyclicity: the 7 seams' signatures + benign defaults; `implCheckObligationsFn` DI wiring; the 3 new edges acyclic.
- **R6 Sonnet** NO-REGRESSION — the DEVIATIONS lens: assess A–E rigorously; confirm the ~13 fixture additions + the `open→closed` change + the one assertion change do NOT mask a real behavior regression; the happy-path tests still pin the same real behavior.
- **R7 Fable** scope/grounding/contradiction: fix confined to the 3 approve files; no gitutil/Bead-1-logic/docs change; citations resolve; no design contradiction (C+B unchanged bool probe; Option B added-not-replacing; single settlement home; ADR-0030).
- **R8 codex** adversarial + empirical: attack each leg's fail-direction (find any fail-open); probe deviation C (is the assertion change hiding a regression? construct the missing-plan input and confirm R3 fail-closed is correct); verify AC11/AC13 discriminate; run the full empirical sweep + golangci-lint; hunt any un-gated-merge path the legs miss.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<slot>-round-1.json`: `reviewer_id`, `verdict`, `confidence`, `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings` (incl. an A–E deviation verdict each).
