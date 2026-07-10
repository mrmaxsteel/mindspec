# spec-113-final — Round 1: 9 APPROVE / 3 REQUEST_CHANGES (G1, G2, G3)

APPROVE (9): F1 0.93, F2 0.93, F3 0.90, O1 0.95, O2 0.95, O3 0.90, S1 0.85, S2 0.93, S3 0.94.
REQUEST_CHANGES (3): G2 0.94, G3 0.88, G1 0.86.

## Adjudication (findings never out-voted)
### FIXED
- **G2 (security, empirically reproduced)** — R1's new non-bead target-rendering was RAW: (a) copyable command-injection footgun in the tally recovery command (`--target spec/poc;touch_PWNED` executes if copied; `panel create --target` accepts `;`), (b) control-byte output-forgery from a hand-edited panel.json reaching verify/tally stdout (spec-109 terminal-injection class). FIX (commit 82b28b2f): reuse 109's `escapeConfigValue` for control bytes on all target DISPLAY renders (renderPanelVerify/sanitizeNonBeadDecision) + new `shellQuoteTarget` (single-quote wrap, `'`→`'\''`) on the target/gate in the copyable recovery command (tallyExitActionNonBead). New test `TestPanelTally_NonBeadHostileTargetEscapedAndQuoted` (metachar + control-byte subtests, mutation-verified: pre-fix the `;`-target prints unquoted + the control-byte target trips guard's single-line invariant). Zero internal diff preserved; tallyExitAction 2-arg.
- **F3-2 (advisory)** — empty `--target ` on unreadable-registration edge → now omitted (folded into G2 fix).
- **F3-1 (advisory)** — recovery omitted `--gate` → now includes `--gate '<gate>'` when stamped (folded in; tallyExitActionNonBead gained a gate param, fed from reg.Panel.Gate).
- **G3 finding-1 (data)** — committed .beads/issues.jsonl showed mindspec-r6hk.3 in_progress (the last-completed bead's close lagged the JSONL export) while live Dolt has it closed. FIX: regenerated .beads/issues.jsonl from live Dolt via `bd export` (minimal 4-line diff: r6hk.3 in_progress→closed + epic). Now consistent.

### EVIDENCED-REFUTED (environmental, not spec-113 defects)
- **G1 RC + G3 finding-2** — `go test ./...` red beyond the accepted internal/instruct z4ps flake. ROOT CAUSE (S1 empirical proof + G1's own log): `internal/harness` TestLLM_* fail with "Not logged in · Please run /login" (the sandbox has no Claude Code auth) and time out under ~40-package parallel load; `internal/validate` TestValidateSpec_ADRTouchpointExtractionBoundary times out under the same contention. S1 RE-RAN both in isolation with `-timeout 5m` → BOTH PASS (217s/93s); neither package is touched by the 113 diff. This was a BRIEF-scope error (instructing `go test ./...` pulls in the auth-dependent harness). Round-2 scopes the test command to the TOUCHED packages. Not a code defect; refutation recorded per the audited-refutation policy.

### DEFERRED (evidenced, out of this wave's fenced scope)
- **F1 info** — gate.go skip-env leg's trailing empty bead-ID interpolation is cosmetic and lives in internal/panel/gate.go, which R1 deliberately keeps at ZERO diff this wave; fixing it would break the consistency fence. Follow-up material (not a 113 falsifier).

All 9 APPROVEs confirmed: R1 zero-internal-diff fence holds NET, R2 no bypass + monotone, R3 decision-inert across all 5 enum values + single-enum + config-free leaf, R4 comment+test only, ADRs adhere, no private content, clean merge to main.
