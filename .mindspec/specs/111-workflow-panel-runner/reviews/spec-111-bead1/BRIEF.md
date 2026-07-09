# spec-111-bead1 — Round 1 Review Panel (8 reviewers, Claude-only)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner/.worktrees/worktree-mindspec-9cyu.1`
**Branch**: `bead/mindspec-9cyu.1`
**Commit under review**: `68c87106d9901a78daab0018686b65dd48a1c811` — `feat(111): claim .claude/workflows/** for the workflow domain + attribution test [mindspec-9cyu.1]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, **R8 Sonnet-sub** (codex is walled; the empirical/injection slot runs as a Sonnet substitute — write reviewer_id "R8 sonnet-sub"). **Pass = ≥7 APPROVE, no REJECT.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `68c87106`; leave `git status` clean. Any scratch under ABSOLUTE /tmp only (never a relative `.mindspec/` write); remove scratch when done (disk is tight).

## What the work does (bead 9cyu.1 — spec 111 R9, ownership root)
A small, independent ROOT bead: it claims `.claude/workflows/**` for the **workflow** domain BEFORE Bead 2 lands the `.claude/workflows/ms-panel.js` runner artifact there (governable source must be owned before it's edited, or it trips `adr-divergence-unowned`). Plus an attribution test.

3 files changed (+43): `.mindspec/domains/workflow/OWNERSHIP.yaml` (the `- .claude/workflows/**` claim, adjacent to the `.claude/skills/**` claim), `internal/validate/ownership_wave2_test.go` (`TestWorkflowOwnsClaudeWorkflows`), `.mindspec/domains/workflow/architecture.md` (ownership-claim lineage doc-sync).

## Note on the branch base
spec/111 branched before specs 110/112 merged to main, so this branch's `architecture.md` is an OLDER version (no 110/112 sections) — that is expected; the spec→main integration (a doc-union resolve) happens at 111's impl-approve. Do NOT flag the missing 110/112 sections as a defect.

## What to verify
1. **The OWNERSHIP claim is correct**: `- .claude/workflows/**` added to the workflow domain's `paths:` (adjacent to `.claude/skills/**`). It should NOT over-claim (e.g. not claim `plugins/mindspec/workflows/**`, which is already covered by the existing `plugins/mindspec/**` glob — a redundant/overlapping claim would be a finding). The glob matches `.claude/workflows/ms-panel.js` (Bead 2's target) but nothing it shouldn't.
2. **Classification correctness (R9 AC)**: `.claude/workflows/ms-panel.js` classifies as GOVERNABLE SOURCE — `isDocFile(...)` is false (no docs/, .mindspec/, project-docs/, or root-operator-doc prefix) AND `isProcessArtifact(...)` is false (no /reviews/ segment, not under .beads/ or review/). So without the claim it would trip `adr-divergence-unowned`; with it, it resolves to the workflow domain. Verify empirically (grep/read `internal/validate`'s `isDocFile`/`isProcessArtifact` + the ownership resolver; or run the resolution).
3. **The attribution test genuinely pins the claim**: `TestWorkflowOwnsClaudeWorkflows` asserts the workflow domain now owns `.claude/workflows/**` / a representative path resolves to workflow. Confirm it would FAIL without the OWNERSHIP claim (not vacuous). Confirm it reuses the existing `repoRootForWorkflowManifest(t)` helper correctly (walks to the LIVE repo root — cross-check it reads the right OWNERSHIP.yaml).
4. **No over/under-reach**: the glob doesn't accidentally claim a path another domain owns; no ambiguity/double-ownership introduced (run any ownership-lint / doctor-conventions test).
5. **Doc-sync**: architecture.md's ownership-claim lineage note is accurate.

## Verify green
`go build ./...`; `go test ./internal/validate/...` (incl. the new test + existing ownership/wave2 + any OWNERSHIP-lint). The 2 KNOWN pre-existing failures (`internal/harness` timeout, `internal/instruct` z4ps) are unrelated — this bead touches neither.

## Per-slot lens defaults
- **R1 Opus** — author-of-record: diff matches plan Bead 1 / R9, no more no less.
- **R2 Opus** — codebase-pin: the claim + test + classification all real and green; the new test passes.
- **R3 Opus** — scope/correctness: only the 3 files; claim placement + non-redundancy vs the existing `plugins/mindspec/**` glob.
- **R4 Sonnet** — empirical: run the ownership resolution for `.claude/workflows/ms-panel.js` → workflow; confirm it would trip `adr-divergence-unowned` without the claim (temporarily removing the claim in a /tmp copy).
- **R5 Sonnet** — schema/type: the test assertion shape; glob-matching semantics (does `.claude/workflows/**` match nested files? does it NOT match `.claude/workflowsX`?).
- **R6 Sonnet** — next-bead integration: does this claim correctly enable Bead 2 to land `.claude/workflows/ms-panel.js` as governable source without an ownership gate failure? Is the claim's timing (root, before Bead 2) right?
- **R7 Fable** — adversarial: is the glob too broad (claims something it shouldn't) or too narrow (misses ms-panel.js)? Any ambiguity/double-ownership? Is the test falsifiable (fails without the claim)?
- **R8 Sonnet-sub** — empirical/lint: run the doctor-conventions / ownership-lint checks; confirm the resolution + no drift; verify the glob against the real classification functions.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", R8 = "R8 sonnet-sub"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`.
