# spec-128-final — consolidated concrete_changes_required, round 1

Tally: 4 APPROVE (R1 fidelity, R3 AC-map, R8 test-arch, R10 regression) / 8 REQUEST_CHANGES — 28 asks in 8 groups. No hard_block flags, but G1/G2 are R6-EXECUTED majors and MUST be treated as blocking. Read the full asks in R{2,4,5,6,7,9,11,12}-round-1.json.

## G1 (R6 F1, MAJOR, EXECUTED) — Write-ahead marker provenance — MUST FIX
A hand-authored commit whose message ends with a clean `Mindspec-Stage: artifact-sync` trailer is honored as a resume: the S-resume route then skips the panel/content gates (the discriminator trusts trailer content without verifying the commit's provenance). Authenticate provenance before honoring a resume — the marker ref (refs/mindspec/materialized/<bead>) must point AT the tip commit AND the trailer commit must be marker-published (the ref is the authority, never the trailer alone); a trailer-bearing tip WITHOUT the matching marker ref = foreign (clear + normal run). Add R6's repro as a fixture leg.

## G2 (R6 F2, MAJOR, executed+traced) — derive.go:249/252 swallowed child-fetch errors — MUST FIX
The no-stored-phase fallback: a failed `bd list --parent` yields nil children → DerivePhaseFromChildren(nil) → fabricated `plan` mode. The spec's own core sin (error-discarding derivation) in one remaining arm. Propagate as DerivationFailed → indeterminate; regression test.

## G3 (R2 ×4 + R5 ×2, convergent) — The bd-stderr render split has unconverted sinks — MUST FIX
Raw %v renders of wrapBDError'd chains: next.go:594 (advancePhaseOnClaim warning), release.go:182 (describeReleaseSyncFailure), complete.go:522 (lineageErr — sibling :503 already escapes), approve/spec.go:105 (epicErr → result.Warnings printed verbatim), state.go's stateDiscoveryFailure + returned resolve errors, beads.go's scopeResolutionRefusal. Convert ALL to the established per-line-escape discipline; add hostile-stderr tests per RENDERER FAMILY (R5: not just internal/next.RenderBDError).

## G4 (R5) — Shell-safe operands in command-shaped examples — MUST FIX
select.go prints executable-looking `mindspec next ...` example lines interpolating selectors via termsafe.Escape — printable-hostile input (`x;evil`, `$(evil)`) survives into a copy-pasteable command. Shell-safe operand emitter or stop printing an executable shape; printable-hostile cases pinned.

## G5 (R4 ×2) — spec-approve success tail lies — MUST FIX
The tail instructs "Commit approval artifacts before continuing" AFTER the command auto-committed them. State the actual result + truthful next action; e2e regression (fresh approve → clean worktree + approval commit present + no false instruction).

## G6 (R5 + R11 ×2 + R7 F6, convergent) — WARN naming + the sExitContract fallback — MUST FIX
Rename `WARN complete.residual_artifact_dirt` → kebab-case `residual-artifact-dirt` (+ the pinned assertion at complete_test.go:2850). R7 F6: the `else if` fallback branch at materialize.go:430-434 is reached only under a condition making its message misleading — fix the branch logic/message to match reality.

## G7 (R7 F1-F5 + R9 F1-F6 + R2 minor, 12 asks) — Comment/doc precision sweep — MUST FIX (mechanical)
(a) reattach claimFailureError's orphaned doc comment (next.go:671-699); (b) phaselock.go header's writer enumeration incomplete (add bead-2's two writers); (c) lifecycle-modes.md's completeSyncPhase citation → function-name anchor (three slots hit this: R7 F3, R9 F3, R2 minor); (d) semantics_v2.go:7's plan.go citation → internal/approve/plan.go:517; (e) completeSyncPhase's doc comment still reads as an OPEN action item — reword to the discharged disposition; (f) R9 F1: runbook's "override is the whole recovery" → the plain-retry-first wording; (g) R9 F2: DerivePhaseWithStatus citations :166/:174 → :182/:190; (h) R9 F4: core architecture's "Every writer" sentence → scoped honestly; (i) R9 F5: the eight-vs-seven row count nit; (j) R9 F6: ResetIndexToHead flag description unified to the actual `git reset --quiet --mixed HEAD` everywhere (executor.go:287 comment + execution architecture + ADR-0041 cross-check).

## G8 (R12) — argv-table entry justification — MUST FIX (small)
The ClearMaterializationMarker entry claims the same shape as UpdateRefCAS but the actual call differs — correct the entry's justification text to the real shape (honest registration, no gate weakening).
