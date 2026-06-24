# 106-bead2 — Round 1 Review Panel

**Bead**: `mindspec-3d3i.2` — Bead 2 (Phase 1: multi-prefix gate classifier, WORKFLOW+context-system). **Branch**: `bead/mindspec-3d3i.2`.
**Commit**: `288eda9787ce51591fd0d8d84066fc0ba0115a7d` (single, 15 files, +800/-70, ZERO renames/copies/deletes).
**Bead worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.worktrees/worktree-mindspec-3d3i.2`
**Diff**: `git -C <bead-worktree> show 288eda97`. **Plan/spec**: `<bead-worktree>/.mindspec/docs/specs/106-layout-flatten/` (plan Bead 2 = Reqs 3, 6, 14).

## What the bead does (behavior-preserving, NO file moves)
- **Req 6 — permanently multi-prefix gate matchers**: a relative-path classifier (`artifactPrefixes`/`hasArtifactPrefix`) recognizing flat `.mindspec/<name>/` + canonical `.mindspec/docs/<name>/` + legacy `docs/<name>/`, wired into `isDocFile`/`specMDID`/`isADRMarkdown`/`checkInternalPackages`/cmd-docs accept-set/`validateSpecArtifactSync` (docsync.go), and `isProcessArtifact` (divergence.go — KEEPS root `review/**` + ADDS `/reviews/` segment).
- ‼️ **THE LINCHPIN — ref-anchored ownership pair multi-prefix**: `domainManifestRelPath`→`domainManifestRelPaths` (3 candidates); `LoadOwnershipAtRef` tries each first-present-wins; `listDomainDirsAtRef` unions the 3 tree roots. This is the pair the `complete` ADR-divergence gate uses — if flat manifests don't resolve here, **Bead 5's own flat merge hard-blocks `adr-divergence-unowned`**.
- **Req 3 — tier-aware enumerators** via Bead-1 `workspace.SpecsDir`/`DomainsDir`: `budgeter.go` (~170/218), `spec/list.go`, `domain/list.go`+`show.go`, `doctor/docs.go`.
- **Req 14**: `project-docs/**` → non-source docs; cmd-docs accepts `core/USAGE.md` + `project-docs/user/**`.

## Fix-author deviations — ASSESS
A. Corrected a pre-existing wrong TEST expectation: `docs/specs/spec.md` → `specMDID` returns id `"spec.md"` (degenerate `TrimSuffix` quirk of the ORIGINAL code, preserved as-is; only the test expectation was fixed). OK?
B. **doctor changes confined to `docs.go`'s 4 named funcs (per plan).** `doctor/ownership.go` + `doctor/orphaned_beads.go` ALSO hardcode canonical `.mindspec/docs/...` paths but are OUTSIDE Bead 2's listed files — left canonical-correct (flat-blind post-flatten). Is this an acceptable deferral (Bead 6 / follow-up), or a gap that affects Bead 5/6 or the bd_close floor on a flat tree?
C. `isSourceFile` unchanged (source paths `internal/`/`cmd/` `.go` are layout-invariant). OK?

## ‼️ CRITICAL VERIFICATION
The ref-anchored `LoadOwnershipAtRef`/`domainManifestRelPaths` MUST resolve a domain's OWNERSHIP.yaml on a FLAT ref — otherwise after Bead 5 flattens, every domain reads "missing" and EVERY flat bead's `complete` (starting with Bead 5's own) hard-blocks `adr-divergence-unowned`. Verify this is genuinely covered + tested. Also confirm behavior-preservation: canonical/legacy classification is BYTE-IDENTICAL (Bead 2 lands on the still-canonical repo and must not change current gate behavior).

## Your job
Review per your lens. Scope: no Bead 1 (resolvers), Bead 4 (panel gate/merge guard/doctor layout-detect), Bead 5 (moves), Bead 6 (skills/governance) work. `internal/instruct/TestRun_IdleNoBeads` (z4ps) flake is unrelated. Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/106-bead2/<your-slot>-round-1.json`: `reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array), `findings` (array of {severity, area, issue, hard_block?}).
