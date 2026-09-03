# spec-128-bead7 — consolidated concrete_changes_required, round 1

Tally: 2 APPROVE (R2, R4) / 4 REQUEST_CHANGES, hard_block from R6. 10 asks in 5 groups. Artifact-gate screen: clear. The headline: the plan's "derived-empty enumeration" is FACTUALLY WRONG — two real claim-path writes into the invoking checkout exist (R3's recording markers; R6's .gitignore write). The directive and the architecture survive; the enumeration must become honest.

## G1 — The enumeration is unsound: two missed claim-path write classes (R3 F1 + R6, convergent, drives the hard block) — MUST FIX
1. **Recording markers** (R3, with file:line evidence): claim step 7.5 (next.go:373-377) calls recording.EmitBeadMarker + AddBeadToPhase → appends events.ndjson + read-modify-writes manifest.json in the INVOKING checkout when recording.enabled and the spec has a recording dir (spec create builds one automatically when enabled). These are TRACKED paths — dirty manifest.json is genuinely entangleable (the RMW absorbs uncommitted user state). Disposition: make the enumeration per-spec-aware and include the target spec's two recording paths as entangleable (R3's option (i) — the honest fix; options (ii)/(iii) only if (i) proves structurally impossible, with spec/ADR amendment).
2. **DispatchBead's .gitignore write** (R6): EnsureGitignoreEntry can modify the checkout's .gitignore during claim. Include .gitignore as entangleable when its current state could be touched (i.e. when the entry is absent), or restructure so the write can't collide with user dirt; add modified/deleted/renamed-away/symlinked .gitignore claim-path tests proving a refused claim leaves everything byte-identical.

## G2 — The AC-8(ii) anti-drift pin is fixture-conditional (R3 F1b + R6, convergent) — MUST FIX
The pin derives writes under the FIXTURE's config: recording-off + .worktrees/ already present structurally can't observe either missed class. Add (a) a recording-ENABLED leg (enable in fixture config, create spec so the recording dir exists, claim, diff) and (b) a leg where .gitignore lacks the entry so EnsureGitignoreEntry's write branch fires. The pin must detect potential writes, not just residual status paths on the happy fixture.

## G3 — Porcelain-shape conservatism (R5 + R6, convergent) — MUST FIX
Renames/copies: block when EITHER side intersects an entangleable path; collapsed untracked directories: block when the dir contains or is an ancestor of an entangleable path. Prefer `git status -z` (unambiguous parsing); preserve both sides of rename records; render notice entries with unambiguous boundaries so hostile printable filenames can't masquerade. Seam-driven tests for both shapes; deterministic ordering preserved.

## G4 (R5) — backfillExistingEpicPhase absent-vs-invalid honesty — MUST FIX
Consume the lock's presence bit; write plan ONLY when mindspec_phase is genuinely absent. Present-but-invalid stays untouched or is explicitly refused, with output distinct from a real backfill; regression test for the invalid case.

## G5 (R1) — Pin the notice's identifying wording — MUST FIX (small)
Unit test asserts the stable substrings ('claiming despite unrelated dirty path(s)', 'not blocked — ADR-0025 R5.1'); the real-binary stderr assertion requires the prefix, not just the path. Silent rewording goes red at both levels.
