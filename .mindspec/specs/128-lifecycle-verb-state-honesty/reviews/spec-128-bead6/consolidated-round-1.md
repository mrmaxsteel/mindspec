# spec-128-bead6 — consolidated concrete_changes_required, round 1

Tally: 0 APPROVE / 6 REQUEST_CHANGES, hard blocks from R5+R6. R4 slot ran claude-sub (codex security-refusal false positive on fault-injection framing). 24 asks dedupe to 7 groups — the convergence is extraordinary: G1 is FIVE-slot convergent with an executed repro. Artifact-gate screen: clear.

## G1 — Deviation C's classification→snapshot race is REAL and exploited (R3+R4+R5+R6 + implicitly R1; R4-sub EXECUTED the repro: a file written before the second SnapshotWorktreeTree call ends up IN the final tip on a successful complete) — MUST FIX, drives both hard blocks
Close it mechanically at the exact tree being committed: after computing changedFromCurrent (materialize.go:~296), REFUSE (named paths, sExitGenuineDirtRefusal-style, BEFORE MaterializeStageCommit/publication) if ANY path in D's delta from currentTip is outside materializedArtifactPaths — on BOTH the normal route and the resumeStage==stageMaterialization route. (R5's alternative — build D from currentTip + pathspec-scoped artifact refresh — is also acceptable; the delta-refusal is the minimal form.) Keep the post-D defensive re-check. Fixture: R4's exact recipe (decorator executor counting SnapshotWorktreeTree calls, writing race-file.txt on call 2) → refusal naming the path, edit preserved on disk, tip not advanced past C, never a commit containing it.

## G2 — The Mindspec-Stage discriminator is OPEN over unknown values (R1+R5+R6, 3-slot) — MUST FIX (gate-bypass class)
runMaterialization's else-catchall routes ANY non-empty unknown stage down the S-resume path — skipping the panel AND content gates. Validate stage ∈ {materialization, artifact-sync} in resolveMaterializationDiscriminator; unknown/empty/ambiguous/duplicate trailer → foreign/corrupt handling (clear + normal run, or named refusal). Malformed + duplicate trailer tests.

## G3 — Error-discard sweep in the new machinery (R1+R3+R6, convergent — the five-bead hazard class again) — MUST FIX
1. materialize.go:302: ReadStageMarker error silently degrades to CREATE semantics — propagate; red-on-revert error-injection test (no D build/publication/ref advance after a marker-read failure).
2. resolveMaterializationDiscriminator:100: oid^ resolution error falls through to FOREIGN and DELETES the write-ahead marker (destroying recovery evidence on a transient read failure) — propagate.
3. Same function: RevParseRef failure for the bead tip → propagate, never clear-and-proceed.

## G4 (R6) — Marker clearing must be CAS — MUST FIX
Clear compare-and-swaps against the OID this invocation observed/owns (foreign-marker and terminal-clear routes included). Interleaving test: clear reads X, another invocation publishes Y, the stale clear cannot delete Y.

## G5 (R1+R3, convergent) — The AC-6 composite flagship fixture is MISSING — MUST FIX
The spec's headline two-run retry-convergence assertion has no regression pin: real-git fixture with panel at reviewed SHA A + dirty tracker + doc-sync-failing change; run 1 refuses at doc-sync with tip asserted STILL A (rev-parse); run 2 with --allow-doc-skew decided by doc-sync, asserted NOT stale-SHA. Plus R2's positive resume leg: panel at A, kill after first AdvanceBranchExpectedOld (marker==tip==C), re-run WITHOUT foreign commit → converges (BeadClosed, ancestor, marker cleared) — proving write-ahead resume doesn't trip stale-SHA.

## G6 (R1+R6, convergent) — Deviation A rejected as delivered: preserve --commit-msg — MUST FIX
The CLI/help/recovery contract promises the supplied description drives the auto-commit; a fixed subject is an audit-history regression. Preferred: widen the MaterializeStageCommit seam by one subject parameter (disclose the seam-list deviation). Minimum: WARN on unused --commit-msg + help-text documentation — but prefer the real fix.

## G7 (R1+R3+R4+R5, 4-slot) — ADR-0041 must describe the DELIVERED ordering + small pins — MUST FIX
§1's S bullet and §3(i) still narrate the plan-literal post-D check ordering deviation C rejected. Amend both: primary classification BEFORE the S snapshot (refusing pre-D), post-D re-check defensive, D = full re-snapshot guarded by G1's delta-refusal (the "narrowed-not-eliminated window" language goes away once G1 lands — describe the mechanical closure). Extend the pinning tests to anchor the new sentences. Plus: add the stage-constants pinning test materialize.go:34-36 cites as existing (materialize_stage_constants_test.go — no such file).
